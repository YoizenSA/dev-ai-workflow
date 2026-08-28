package configapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	userconfig "github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// toolCacheTTL is how long an assembled tools payload is considered fresh.
// The tool catalog only changes when plugins or MCP servers are added/removed,
// and assembling it is expensive (stdio MCP probes, the opencode HTTP call,
// file scans), so a long TTL is appropriate. A manual resync (?refresh=1)
// bypasses it when the user actually changes their setup.
const toolCacheTTL = 10 * time.Minute

// toolCache caches the /api/config/tools payload with a stale-while-revalidate
// policy: the first request pays the slow discovery cost, later requests return
// instantly, and a stale entry is served immediately while a single background
// refresh picks up newly added MCPs/plugins. The zero value is ready to use.
type toolCache struct {
	mu         sync.Mutex
	resp       map[string]interface{}
	fetchedAt  time.Time
	refreshing bool
}

func (c *toolCache) get(
	fetch func() (map[string]interface{}, error),
) (map[string]interface{}, error) {
	c.mu.Lock()
	cached := c.resp
	hasCache := !c.fetchedAt.IsZero()
	fresh := hasCache && time.Since(c.fetchedAt) < toolCacheTTL

	if fresh {
		c.mu.Unlock()
		return cached, nil
	}

	if hasCache {
		// Stale: serve what we have and refresh out of band (at most one in flight).
		if !c.refreshing {
			c.refreshing = true
			go c.refresh(fetch)
		}
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	// Cold cache: this request must block on the assembly.
	resp, err := fetch()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.resp = resp
	c.fetchedAt = time.Now()
	c.mu.Unlock()
	return resp, nil
}

// forceGet bypasses the freshness check, runs the assembly synchronously, and
// replaces the cache. Used by the manual resync button so a user who just
// added a plugin/MCP sees the change immediately instead of waiting out the TTL.
func (c *toolCache) forceGet(
	fetch func() (map[string]interface{}, error),
) (map[string]interface{}, error) {
	resp, err := fetch()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.resp = resp
	c.fetchedAt = time.Now()
	c.mu.Unlock()
	return resp, nil
}

func (c *toolCache) refresh(fetch func() (map[string]interface{}, error)) {
	resp, err := fetch()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshing = false
	// Keep the previous cache on failure rather than wiping a good payload.
	if err != nil || resp == nil {
		return
	}
	c.resp = resp
	c.fetchedAt = time.Now()
}

// --- Config Handlers ---

func opencodeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json"), nil
}

func agentsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode", "agents"), nil
}

// GET /api/config/opencode
func (h *Handlers) GetOpenCodeConfig(w http.ResponseWriter, r *http.Request) {
	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// PUT /api/config/opencode
//
// Body is treated as a sparse JSON patch: every top-level key present in the
// body replaces the matching key in opencode.json, while any key not in the
// body is preserved. This protects the file from clients that render only a
// subset of fields (e.g. the Settings UI which exposes 5 keys but the file
// also holds provider configs, mcp, etc.).
func (h *Handlers) PutOpenCodeConfig(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit

	var patch map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected JSON object: " + err.Error()})
		return
	}

	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Load existing config (if any) into a top-level map and merge patch over it.
	existing, _ := os.ReadFile(path)
	merged := map[string]json.RawMessage{}
	if len(existing) > 0 {
		// Preserve the existing file on disk as a .bak before mutating it.
		_ = os.WriteFile(path+".bak", existing, 0644)
		_ = json.Unmarshal(existing, &merged)
	}
	for k, v := range patch {
		merged[k] = v
	}

	// Strip provider if it's a flat string — the user wants model/small_model only.
	if raw, ok := merged["provider"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			delete(merged, "provider")
		}
	}

	pretty, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := os.WriteFile(path, pretty, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

type MCPToolGroup struct {
	Tools   []string `json:"tools"`
	Enabled bool     `json:"enabled"`
}

// GET /api/config/tools
func (h *Handlers) ListTools(w http.ResponseWriter, r *http.Request) {
	var resp map[string]interface{}
	var err error
	if r.URL.Query().Get("refresh") == "1" {
		resp, err = h.toolCache.forceGet(buildToolsResponse)
	} else {
		resp, err = h.toolCache.get(buildToolsResponse)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// buildToolsResponse assembles the /api/config/tools payload from the opencode
// config plus MCP and plugin discovery. It is the slow source behind toolCache.
func buildToolsResponse() (map[string]interface{}, error) {
	path, err := opencodeConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Built-in opencode v2 permission actions (v1 names like bash/task/
	// write/lsp no longer exist as enforcement points).
	builtIn := []string{
		"read", "edit", "glob", "grep", "websearch", "webfetch",
		"question", "skill", "subagent", "shell", "external_directory",
		"delegate", "memory", "intercom", "mcp",
	}

	// Collect all known tool names in a set
	toolSet := map[string]bool{}
	for _, t := range builtIn {
		toolSet[t] = true
	}

	// Also collect valid tools referenced in agent permissions
	// (skip deprecated keys like todoread/todowrite that aren't in ValidPermissionKeys)
	var agents map[string]json.RawMessage
	if agentRaw, ok := config["agent"]; ok {
		_ = json.Unmarshal(agentRaw, &agents)
		for _, agentRaw := range agents {
			var agent map[string]json.RawMessage
			if err := json.Unmarshal(agentRaw, &agent); err != nil {
				continue
			}
			var perm map[string]string
			if permRaw, ok := agent["permission"]; ok {
				if err := json.Unmarshal(permRaw, &perm); err == nil {
					for k := range perm {
						if ValidPermissionKeys[k] {
							toolSet[k] = true
						}
					}
				}
			}
			// v2 rule array: collect the actions used.
			var rules []map[string]string
			if rulesRaw, ok := agent["permissions"]; ok {
				if err := json.Unmarshal(rulesRaw, &rules); err == nil {
					for _, r := range rules {
						if a := agentprofiles.NormalizePermissionKey(r["action"]); ValidPermissionKeys[a] {
							toolSet[a] = true
						}
					}
				}
			}
		}
	}

	// MCP discovery — best effort for HTTP/SSE MCPs
	// Include disabled MCPs so the UI can show them as inactive.
	mcpTools := map[string]MCPToolGroup{}
	var mcpServers map[string]json.RawMessage
	if mcpRaw, ok := config["mcp"]; ok {
		mcpServers = openCodeServersFromRaw(mcpRaw)
		for name, serverRaw := range mcpServers {
			var server map[string]interface{}
			if err := json.Unmarshal(serverRaw, &server); err != nil {
				continue
			}
			disabled := false
			if d, ok := server["disabled"].(bool); ok {
				disabled = d
			}
			// "enabled: false" is equivalent to "disabled: true"
			if e, ok := server["enabled"].(bool); ok && !e {
				disabled = true
			}

			// Discover tools based on server type
			tools := []string{}

			// Try HTTP/SSE discovery first (remote servers)
			if urlStr, ok := server["url"].(string); ok && urlStr != "" {
				tools, _ = discoverMCPTools(urlStr)
			}

			// Try stdio discovery (local servers with command)
			if len(tools) == 0 {
				var command []string
				if cmdRaw, ok := server["command"]; ok {
					switch v := cmdRaw.(type) {
					case []interface{}:
						for _, arg := range v {
							if s, ok := arg.(string); ok {
								command = append(command, s)
							}
						}
					case string:
						command = strings.Fields(v)
						if argsRaw, ok := server["args"].([]interface{}); ok {
							for _, arg := range argsRaw {
								if s, ok := arg.(string); ok {
									command = append(command, s)
								}
							}
						}
					}
				}

				if len(command) > 0 {
					env := map[string]string{}
					envRaw, _ := server["env"].(map[string]interface{})
					if envRaw == nil {
						// v2 renamed env → environment.
						envRaw, _ = server["environment"].(map[string]interface{})
					}
					for k, v := range envRaw {
						if s, ok := v.(string); ok {
							env[k] = s
						}
					}
					tools, _ = discoverStdioMCPTools(command, env)
				}
			}

			mcpTools[name] = MCPToolGroup{Tools: tools, Enabled: !disabled}
			for _, t := range tools {
				toolSet[t] = true
			}
		}
	}

	// Plugin discovery — scan .cache/opencode/packages for npm plugin tools.
	pluginTools := discoverAllPluginTools()

	// Also discover plugins referenced from the opencode "plugin" array: ywai
	// seeds local bundles (e.g. background-agents-v2.js) there, which the npm
	// packages scan above never sees.
	if pluginRaw, ok := config["plugin"]; ok {
		var entries []string
		if err := json.Unmarshal(pluginRaw, &entries); err == nil {
			for name, tools := range discoverConfigPluginTools(entries) {
				if _, exists := pluginTools[name]; !exists {
					pluginTools[name] = tools
				}
			}
		}
	}

	for _, tools := range pluginTools {
		for _, t := range tools {
			toolSet[t] = true
		}
	}

	// Authoritative completeness pass: opencode itself enumerates every tool
	// ID (built-ins + dynamically registered plugin tools). When the server
	// is up this guarantees nothing is missing from "all"; when it is down we
	// keep the static config/bundle discovery above. Best-effort.
	for _, id := range discoverOpencodeToolIDs() {
		toolSet[id] = true
	}

	// Convert set to sorted slice
	var allTools []string
	for t := range toolSet {
		allTools = append(allTools, t)
	}
	sortStrings(allTools)

	return map[string]interface{}{
		"built_in":     builtIn,
		"all":          allTools,
		"mcp_tools":    mcpTools,
		"plugin_tools": pluginTools,
	}, nil
}

// GET /api/config/mcp
func (h *Handlers) ListMCP(w http.ResponseWriter, r *http.Request) {
	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config struct {
		MCP json.RawMessage `json:"mcp"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type mcpInfo struct {
		Name    string          `json:"name"`
		Config  json.RawMessage `json:"config"`
		Enabled bool            `json:"enabled"`
	}
	var mcps []mcpInfo
	for name, cfg := range openCodeServersFromRaw(config.MCP) {
		// Check if disabled or enabled flag is false
		var serverCfg map[string]interface{}
		enabled := true
		if err := json.Unmarshal(cfg, &serverCfg); err == nil {
			if disabled, ok := serverCfg["disabled"].(bool); ok && disabled {
				enabled = false
			} else if val, ok := serverCfg["enabled"].(bool); ok && !val {
				enabled = false
			}
		}
		mcps = append(mcps, mcpInfo{Name: name, Config: cfg, Enabled: enabled})
	}
	writeJSON(w, http.StatusOK, mcps)
}

// PUT /api/config/mcp/{name} - toggle enabled/disabled
func (h *Handlers) PutMCP(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid mcp name"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Read current config
	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	mcpRaw, hasMCP := config["mcp"]
	servers, mcpLevel := splitOpenCodeMCP(mcpRaw)
	if !hasMCP {
		servers = map[string]json.RawMessage{}
		mcpLevel = map[string]json.RawMessage{}
	}

	// v2 toggle: enabled servers carry no flag (absent = enabled),
	// disabled ones carry "disabled": true. The legacy "enabled" bool is
	// removed either way.
	if serverRaw, ok := servers[name]; ok {
		var serverCfg map[string]interface{}
		if err := json.Unmarshal(serverRaw, &serverCfg); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		delete(serverCfg, "enabled")
		if body.Enabled {
			delete(serverCfg, "disabled")
		} else {
			serverCfg["disabled"] = true
		}

		updated, _ := json.Marshal(serverCfg)
		servers[name] = updated
	} else {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mcp server not found"})
		return
	}

	// Write back as mcp.servers
	mcpJSON, _ := json.Marshal(joinOpenCodeMCP(mcpLevel, servers))
	config["mcp"] = mcpJSON
	pretty, _ := json.MarshalIndent(config, "", "  ")

	// Backup
	_ = os.WriteFile(path+".bak", data, 0644)

	if err := os.WriteFile(path, pretty, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DELETE /api/config/mcp/{name} - delete an MCP server
func (h *Handlers) DeleteMCP(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid mcp name"})
		return
	}

	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	mcpRaw, ok := config["mcp"]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no mcp section"})
		return
	}

	servers, mcpLevel := splitOpenCodeMCP(mcpRaw)
	if _, ok := servers[name]; !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mcp server not found"})
		return
	}

	delete(servers, name)

	mcpJSON, _ := json.Marshal(joinOpenCodeMCP(mcpLevel, servers))
	config["mcp"] = mcpJSON
	pretty, _ := json.MarshalIndent(config, "", "  ")

	if err := os.WriteFile(path, pretty, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func openCodeServersFromRaw(mcpRaw json.RawMessage) map[string]json.RawMessage {
	servers, _ := splitOpenCodeMCP(mcpRaw)
	return servers
}

func splitOpenCodeMCP(mcpRaw json.RawMessage) (servers map[string]json.RawMessage, level map[string]json.RawMessage) {
	servers = map[string]json.RawMessage{}
	level = map[string]json.RawMessage{}
	if len(mcpRaw) == 0 {
		return servers, level
	}
	var section map[string]json.RawMessage
	if err := json.Unmarshal(mcpRaw, &section); err != nil {
		return servers, level
	}
	if inner, ok := section["servers"]; ok {
		_ = json.Unmarshal(inner, &servers)
		if servers == nil {
			servers = map[string]json.RawMessage{}
		}
		for k, v := range section {
			if k == "servers" {
				continue
			}
			level[k] = v
		}
		return servers, level
	}
	for k, v := range section {
		if k == "timeout" {
			level[k] = v
			continue
		}
		var obj map[string]json.RawMessage
		if json.Unmarshal(v, &obj) == nil {
			servers[k] = v
			continue
		}
		level[k] = v
	}
	return servers, level
}

func joinOpenCodeMCP(level map[string]json.RawMessage, servers map[string]json.RawMessage) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for k, v := range level {
		if k == "servers" {
			continue
		}
		out[k] = v
	}
	raw, _ := json.Marshal(servers)
	out["servers"] = raw
	return out
}

// GET /api/config/providers - list all providers
func (h *Handlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Get provider section
	if providerRaw, ok := config["provider"]; ok {
		var providers map[string]json.RawMessage
		if err := json.Unmarshal(providerRaw, &providers); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, providers)
	} else {
		writeJSON(w, http.StatusOK, map[string]json.RawMessage{})
	}
}

// PUT /api/config/providers/{name} - create or update a provider
func (h *Handlers) PutProvider(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider name"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit

	var provider json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&provider); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Get provider section
	var providerSection map[string]json.RawMessage
	if providerRaw, ok := config["provider"]; ok {
		if err := json.Unmarshal(providerRaw, &providerSection); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	} else {
		providerSection = make(map[string]json.RawMessage)
	}

	providerSection[name] = provider

	// Write back
	providerJSON, _ := json.Marshal(providerSection)
	config["provider"] = providerJSON
	pretty, _ := json.MarshalIndent(config, "", "  ")

	// Backup
	_ = os.WriteFile(path+".bak", data, 0644)
	if err := os.WriteFile(path, pretty, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DELETE /api/config/providers/{name} - delete a provider
func (h *Handlers) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider name required"})
		return
	}

	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Get provider section
	var providerSection map[string]json.RawMessage
	if providerRaw, ok := config["provider"]; ok {
		if err := json.Unmarshal(providerRaw, &providerSection); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	if _, ok := providerSection[name]; ok {
		delete(providerSection, name)
	} else {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
		return
	}

	// Write back
	providerJSON, _ := json.Marshal(providerSection)
	config["provider"] = providerJSON
	pretty, _ := json.MarshalIndent(config, "", "  ")

	// Backup
	_ = os.WriteFile(path+".bak", data, 0644)
	if err := os.WriteFile(path, pretty, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ─── User Config (Role Defaults) ──────────────────────────────────────────

// GetUserConfig returns the full UserConfig as JSON.
// GET /api/config/user
func (h *Handlers) GetUserConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := userconfig.LoadConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Ensure RoleDefaults is materialized so the frontend sees seed values for
	// roles the user hasn't customised.
	if cfg.RoleDefaults == nil {
		cfg.RoleDefaults = userconfig.DefaultRoleDefaults()
	} else {
		seeds := userconfig.DefaultRoleDefaults()
		for _, role := range userconfig.CanonicalRoles {
			if _, ok := cfg.RoleDefaults[role]; !ok {
				cfg.RoleDefaults[role] = seeds[role]
			}
		}
	}
	writeJSON(w, http.StatusOK, cfg)
}

// PutUserConfig accepts a partial JSON body and merges it over the existing
// user config, then saves to disk.
// PUT /api/config/user
func (h *Handlers) PutUserConfig(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
	cfg, err := userconfig.LoadConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Decode into a sparse map so absent fields don't overwrite existing config.
	var patch map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	// Generic field merge: re-marshal cfg, overlay patch, unmarshal back.
	// Keeps unknown-to-server fields intact and avoids hand-listing every key.
	base, _ := json.Marshal(cfg)
	var merged map[string]json.RawMessage
	_ = json.Unmarshal(base, &merged)
	for k, v := range patch {
		merged[k] = v
	}
	blob, _ := json.Marshal(merged)
	if err := json.Unmarshal(blob, cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "merge failed: " + err.Error()})
		return
	}

	if err := userconfig.SaveConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// GetRoleDefaults returns just the role_defaults block (a flattened view of
// what the New Mission modal needs to pre-populate its selectors).
// GET /api/config/user/role-defaults
func (h *Handlers) GetRoleDefaults(w http.ResponseWriter, r *http.Request) {
	cfg, err := userconfig.LoadConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := userconfig.RoleDefaults{}
	for _, role := range userconfig.CanonicalRoles {
		out[role] = cfg.GetRoleDefault(role)
	}
	writeJSON(w, http.StatusOK, out)
}

// GetOrchestratorProfiles returns all orchestrator profiles plus the active profile name.
// GET /api/config/user/orchestrator-profiles
func (h *Handlers) GetOrchestratorProfiles(w http.ResponseWriter, r *http.Request) {
	cfg, err := userconfig.LoadConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	mergedProfiles, agentGroups := withAllInstalledAgents(cfg.OrchestratorProfiles)
	activeProfile := cfg.GetActiveOrchestratorProfile()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"profiles":           mergedProfiles,
		"active":             cfg.ActiveOrchestratorProfile,
		"shipped":            shippedProfileNames(cfg.OrchestratorProfiles),
		"omp_model_roles":    OmpModelRolesFor(activeProfile),
		"omp_thinking_level": activeProfile.OmpThinkingLevel,
		"agent_groups":       agentGroups,
	})
}

// shippedProfileNames lists the profiles ywai owns and rewrites on every
// install, so the UI can say so before an edit that will not survive.
func shippedProfileNames(profiles map[string]userconfig.OrchestratorModelProfile) []string {
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		if userconfig.IsShippedProfile(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// withAllInstalledAgents fills every profile out to the full set of agents that
// actually exist, leaving newcomers on the inherited model (empty string).
//
// Profiles are stored with only the agents someone assigned a model to, so a
// new agent stays invisible in the UI until the seed file is edited by hand —
// which is how `planning`, `designer` and `advisor` came to be unlistable.
// Deriving the list from the installed profiles instead means adding an agent
// is enough to make it configurable.
//
// Returns the merged profiles plus agentGroups: bare agent name → the folder it
// lives under (e.g. "qa-automation", "social-refactor"), derived from the
// loader's slash-path key. The UI uses agentGroups to group agents by their real
// agents/ folder instead of guessing from name prefixes.
func withAllInstalledAgents(
	profiles map[string]userconfig.OrchestratorModelProfile,
) (map[string]userconfig.OrchestratorModelProfile, map[string]string) {
	installed, err := agentprofiles.LoadProfiles(userconfig.AgentsSourceDir())
	if err != nil || len(installed) == 0 {
		return profiles, map[string]string{}
	}

	// Bare name → folder (first segment of the loader's slash-path key).
	// Prefer AgentProfile.Group when the manifest (groups.json) set it; fall
	// back to the folder prefix so unmanifested agents still land somewhere.
	agentGroups := make(map[string]string, len(installed))
	for key, prof := range installed {
		bare := key[strings.LastIndex(key, "/")+1:]
		if prof.Group != "" {
			agentGroups[bare] = prof.Group
			continue
		}
		if idx := strings.Index(key, "/"); idx > 0 {
			agentGroups[bare] = key[:idx]
		} else {
			agentGroups[bare] = "core"
		}
	}

	names := make([]string, 0, len(installed))
	for key := range installed {
		// Profiles key agents by bare name; the loader keys them by group path.
		names = append(names, key[strings.LastIndex(key, "/")+1:])
	}

	out := make(map[string]userconfig.OrchestratorModelProfile, len(profiles))
	for profileName, profile := range profiles {
		merged := profile.Clone()
		if merged.Agents == nil {
			merged.Agents = userconfig.RoleDefaults{}
		}
		for _, name := range names {
			if _, ok := merged.Agents[name]; !ok {
				// Empty model means "inherit from the lead agent".
				merged.Agents[name] = userconfig.RoleDefault{}
			}
		}
		out[profileName] = merged
	}
	return out, agentGroups
}

// SetActiveOrchestratorProfile sets the active orchestrator profile by name.
// PUT /api/config/user/orchestrator-profiles/active
func (h *Handlers) SetActiveOrchestratorProfile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected JSON body: " + err.Error()})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	applied, err := ActivateOrchestratorProfile(req.Name)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrOrchestratorProfileNotFound) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "saved", "agents_applied": applied})
}

// ErrOrchestratorProfileNotFound is returned when ActivateOrchestratorProfile
// is given a name that is not in the user's config.
var ErrOrchestratorProfileNotFound = errors.New("profile not found")

// ActivateOrchestratorProfile persists the active name and applies models.
func ActivateOrchestratorProfile(name string) (int, error) {
	if name == "" {
		return 0, fmt.Errorf("name is required")
	}
	cfg, err := userconfig.LoadConfig()
	if err != nil {
		return 0, err
	}
	if _, ok := cfg.OrchestratorProfiles[name]; !ok {
		return 0, fmt.Errorf("%w: %s", ErrOrchestratorProfileNotFound, name)
	}
	cfg.ActiveOrchestratorProfile = name
	if err := userconfig.SaveConfig(cfg); err != nil {
		return 0, err
	}
	applied := 0
	if n, err := ApplyActiveOrchestratorProfile(); err == nil {
		applied = n
	}
	return applied, nil
}

// UpdateOrchestratorProfile replaces a profile's editable fields (display name,
// description, per-agent models). If the edited profile is the active one, the
// new models are applied to each agent immediately.
// PUT /api/config/user/orchestrator-profiles/{name}
func (h *Handlers) UpdateOrchestratorProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		DisplayName      string                            `json:"display_name"`
		Description      string                            `json:"description"`
		Agents           map[string]userconfig.RoleDefault `json:"agents"`
		OmpModelRoles    map[string]string                 `json:"omp_model_roles"`
		OmpThinkingLevel string                            `json:"omp_thinking_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body: " + err.Error()})
		return
	}

	cfg, err := userconfig.LoadConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if cfg.OrchestratorProfiles == nil {
		cfg.OrchestratorProfiles = map[string]userconfig.OrchestratorModelProfile{}
	}
	existing := cfg.OrchestratorProfiles[name]
	if body.DisplayName != "" {
		existing.DisplayName = body.DisplayName
	}
	existing.Description = body.Description
	existing.Agents = userconfig.RoleDefaults(body.Agents)
	if body.OmpModelRoles != nil {
		existing.OmpModelRoles = body.OmpModelRoles
	}
	if strings.TrimSpace(body.OmpThinkingLevel) != "" {
		existing.OmpThinkingLevel = strings.TrimSpace(body.OmpThinkingLevel)
	}
	cfg.OrchestratorProfiles[name] = existing

	if err := userconfig.SaveConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// If editing the active profile, apply the new models + orchestration
	// policy + omp modelRoles right away (central apply path).
	applied := 0
	if cfg.ActiveOrchestratorProfile == name {
		if n, err := ApplyActiveOrchestratorProfile(); err == nil {
			applied = n
		}
	}

	mergedProfiles, agentGroups := withAllInstalledAgents(cfg.OrchestratorProfiles)
	activeProfile := cfg.GetActiveOrchestratorProfile()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":             "saved",
		"agents_applied":     applied,
		"profiles":           mergedProfiles,
		"active":             cfg.ActiveOrchestratorProfile,
		"shipped":            shippedProfileNames(cfg.OrchestratorProfiles),
		"agent_groups":       agentGroups,
		"omp_model_roles":    OmpModelRolesFor(activeProfile),
		"omp_thinking_level": activeProfile.OmpThinkingLevel,
	})
}

// ResyncOrchestratorProfiles restores profiles from the embedded seed.
// POST /api/config/user/orchestrator-profiles/resync
func (h *Handlers) ResyncOrchestratorProfiles(w http.ResponseWriter, r *http.Request) {
	cfg, err := userconfig.LoadConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cfg.ResyncOrchestratorModelProfiles()
	if err := userconfig.SaveConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	mergedProfiles, agentGroups := withAllInstalledAgents(cfg.OrchestratorProfiles)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"profiles":     mergedProfiles,
		"active":       cfg.ActiveOrchestratorProfile,
		"agent_groups": agentGroups,
	})
}

// BrowseDirectory opens a native OS directory picker dialog and returns the selected path.
func (h *Handlers) BrowseDirectory(w http.ResponseWriter, r *http.Request) {
	var selectedPath string

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("osascript", "-e", `tell application "System Events" to POSIX path of (choose folder)`)
		out, e := cmd.Output()
		if e != nil {
			http.Error(w, "Directory selection cancelled", http.StatusNoContent)
			return
		}
		selectedPath = strings.TrimSpace(string(out))
	case "linux":
		// Try zenity first, then kdialog
		cmd := exec.Command("zenity", "--file-selection", "--directory", "--title=Select Reference Directory")
		out, e := cmd.Output()
		if e != nil {
			cmd = exec.Command("kdialog", "--getexistingdirectory", "/")
			out, e = cmd.Output()
			if e != nil {
				http.Error(w, "Directory selection cancelled (install zenity or kdialog)", http.StatusNoContent)
				return
			}
		}
		selectedPath = strings.TrimSpace(string(out))
	case "windows":
		cmd := exec.Command("powershell", "-Command", `
			Add-Type -AssemblyName System.Windows.Forms
			$dialog = New-Object System.Windows.Forms.FolderBrowserDialog
			$dialog.Description = "Select Reference Directory"
			$dialog.ShowNewFolderButton = $true
			if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
				$dialog.SelectedPath
			}
		`)
		out, e := cmd.Output()
		if e != nil {
			http.Error(w, "Directory selection cancelled", http.StatusNoContent)
			return
		}
		selectedPath = strings.TrimSpace(string(out))
	default:
		http.Error(w, "Unsupported OS for directory picker", http.StatusBadRequest)
		return
	}

	if selectedPath == "" {
		http.Error(w, "No directory selected", http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"path": selectedPath})
}

// ListVisionModels returns the vision-capable models the vision-bridge plugin
// can actually prompt. The plugin resolves models through OpenCode, so the
// picker reads the same source OpenCode does — opencode.json — rather than a
// provider-specific catalog. Ids are provider-qualified ("provider/model").
// GET /api/config/vision-models
func (h *Handlers) ListVisionModels(w http.ResponseWriter, r *http.Request) {
	cfg, err := userconfig.LoadConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	models, err := visionModelsFromOpenCode()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"models":  []any{},
			"current": cfg.GetVisionModel(),
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"models":  models,
		"current": cfg.GetVisionModel(),
	})
}

// visionModelsFromOpenCode reads opencode.json and returns every model whose
// provider entry advertises image input. Mirrors the plugin's capability read
// so the picker and the plugin never disagree about what counts as vision.
func visionModelsFromOpenCode() ([]map[string]any, error) {
	path, err := opencodeConfigPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", path, err)
	}

	var oc struct {
		Provider map[string]struct {
			Models map[string]struct {
				Attachment bool `json:"attachment"`
				Modalities *struct {
					Input []string `json:"input"`
				} `json:"modalities"`
			} `json:"models"`
		} `json:"provider"`
	}
	if err := json.Unmarshal(data, &oc); err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", path, err)
	}

	out := make([]map[string]any, 0)
	for providerID, prov := range oc.Provider {
		for modelID, m := range prov.Models {
			vision := m.Attachment
			var inputs []string
			if m.Modalities != nil {
				inputs = m.Modalities.Input
				for _, mod := range inputs {
					if mod == "image" {
						vision = true
					}
				}
			}
			if !vision {
				continue
			}
			entry := map[string]any{
				"id":   providerID + "/" + modelID,
				"name": modelID,
			}
			if len(inputs) > 0 {
				entry["modalities"] = inputs
			}
			out = append(out, entry)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i]["id"].(string) < out[j]["id"].(string)
	})
	return out, nil
}
