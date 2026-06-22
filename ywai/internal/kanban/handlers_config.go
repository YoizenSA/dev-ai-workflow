package kanban

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	userconfig "github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// toolCacheTTL is how long an assembled tools payload is considered fresh.
const toolCacheTTL = 60 * time.Second

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

func skillsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode", "skills"), nil
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
	resp, err := h.toolCache.get(buildToolsResponse)
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

	// Built-in opencode tools
	builtIn := []string{
		"read", "edit", "write", "bash", "glob", "grep", "lsp",
		"ast_grep", "websearch", "code_search", "webfetch",
		"task", "delegate", "question", "skill",
		"memory", "intercom", "ado", "mcp",
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
		}
	}

	// MCP discovery — best effort for HTTP/SSE MCPs
	// Include disabled MCPs so the UI can show them as inactive.
	mcpTools := map[string]MCPToolGroup{}
	var mcpServers map[string]json.RawMessage
	if mcpRaw, ok := config["mcp"]; ok {
		_ = json.Unmarshal(mcpRaw, &mcpServers)
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
					if envRaw, ok := server["env"].(map[string]interface{}); ok {
						for k, v := range envRaw {
							if s, ok := v.(string); ok {
								env[k] = s
							}
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

	// Plugin discovery — scan .cache/opencode/packages for tool registrations
	pluginTools := discoverAllPluginTools()
	for _, tools := range pluginTools {
		for _, t := range tools {
			toolSet[t] = true
		}
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

// GET /api/config/skills
func (h *Handlers) ListSkills(w http.ResponseWriter, r *http.Request) {
	dir, err := skillsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	type skillInfo struct {
		Name        string `json:"name"`
		HasSkillMD  bool   `json:"hasSkillMD"`
		Description string `json:"description"`
	}
	var skills []skillInfo
	for _, e := range entries {
		if e.IsDir() {
			skillPath := filepath.Join(dir, e.Name(), "SKILL.md")
			hasSkill := false
			desc := ""
			if data, err := os.ReadFile(skillPath); err == nil {
				hasSkill = true
				// Extract description from frontmatter
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "description:") {
						desc = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
						break
					}
				}
			}
			skills = append(skills, skillInfo{
				Name:        e.Name(),
				HasSkillMD:  hasSkill,
				Description: desc,
			})
		}
	}
	writeJSON(w, http.StatusOK, skills)
}

// GET /api/config/skills/{name}
func (h *Handlers) GetSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill name"})
		return
	}

	skillsDirPath, err := skillsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	path := filepath.Join(skillsDirPath, name, "SKILL.md")

	// Prevent path traversal
	absPath, err := filepath.Abs(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	baseDir, err := filepath.Abs(skillsDirPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !strings.HasPrefix(absPath, baseDir) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "path outside allowed directory"})
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "skill not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"name": name, "content": string(data)})
}

// PUT /api/config/skills/{name} - update a skill file
func (h *Handlers) PutSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill name"})
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	skillsDirPath, err := skillsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	skillDir := filepath.Join(skillsDirPath, name)
	absPath, err := filepath.Abs(skillDir)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	baseDir, err := filepath.Abs(skillsDirPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !strings.HasPrefix(absPath, baseDir) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "path outside allowed directory"})
		return
	}

	if err := os.MkdirAll(skillDir, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(body.Content), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// DELETE /api/config/skills/{name}
func (h *Handlers) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill name"})
		return
	}

	dir, err := skillsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	path := filepath.Join(dir, name)
	cleanPath := filepath.Clean(path)
	if !strings.HasPrefix(cleanPath, filepath.Clean(dir)) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}

	if err := os.RemoveAll(cleanPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
		MCP map[string]json.RawMessage `json:"mcp"`
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
	for name, cfg := range config.MCP {
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

	// Get MCP section
	var mcpSection map[string]json.RawMessage
	if mcpRaw, ok := config["mcp"]; ok {
		if err := json.Unmarshal(mcpRaw, &mcpSection); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	} else {
		mcpSection = make(map[string]json.RawMessage)
	}

	// Toggle: add/remove "disabled" field, toggle "enabled" field
	if serverRaw, ok := mcpSection[name]; ok {
		var serverCfg map[string]interface{}
		if err := json.Unmarshal(serverRaw, &serverCfg); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if body.Enabled {
			delete(serverCfg, "disabled")
			serverCfg["enabled"] = true
		} else {
			serverCfg["disabled"] = true
			serverCfg["enabled"] = false
		}

		updated, _ := json.Marshal(serverCfg)
		mcpSection[name] = updated
	} else {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mcp server not found"})
		return
	}

	// Write back
	mcpJSON, _ := json.Marshal(mcpSection)
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

	var mcpSection map[string]json.RawMessage
	if err := json.Unmarshal(mcpRaw, &mcpSection); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if _, ok := mcpSection[name]; !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mcp server not found"})
		return
	}

	delete(mcpSection, name)

	mcpJSON, _ := json.Marshal(mcpSection)
	config["mcp"] = mcpJSON
	pretty, _ := json.MarshalIndent(config, "", "  ")

	if err := os.WriteFile(path, pretty, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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
