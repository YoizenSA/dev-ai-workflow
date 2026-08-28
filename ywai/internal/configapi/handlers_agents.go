package configapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	userconfig "github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// GET /api/config/agents
func (h *Handlers) ListAgents(w http.ResponseWriter, r *http.Request) {
	type agentInfo struct {
		Name  string `json:"name"`
		Size  int64  `json:"size"`
		Mode  string `json:"mode,omitempty"`
		Group string `json:"group,omitempty"`
	}

	seen := make(map[string]bool)
	var agents []agentInfo
	agentsDirPath, _ := agentsDir()

	// 1. Read agents from opencode.json config
	configPath, err := opencodeConfigPath()
	if err == nil {
		data, readErr := os.ReadFile(configPath)
		if readErr == nil {
			var cfg struct {
				Agent map[string]json.RawMessage `json:"agent"`
			}
			if json.Unmarshal(data, &cfg) == nil && cfg.Agent != nil {
				for name, raw := range cfg.Agent {
					var a struct {
						Mode string `json:"mode"`
					}
					_ = json.Unmarshal(raw, &a)
					info := agentInfo{Name: name, Mode: a.Mode}
					info.Group = resolveTeam(name, agentsDirPath)
					agents = append(agents, info)
					seen[name] = true
				}
			}
		}
	}

	// 2. Also scan agents directory for agents not in config
	if agentsDirPath != "" {
		entries, _ := os.ReadDir(agentsDirPath)
		for _, e := range entries {
			if e.IsDir() {
				// Scan subdirectory for .md files (e.g., core/architect.md, qa-automation/qa-analyst.md)
				subEntries, _ := os.ReadDir(filepath.Join(agentsDirPath, e.Name()))
				for _, se := range subEntries {
					if !se.IsDir() && strings.HasSuffix(se.Name(), ".md") {
						name := strings.TrimSuffix(se.Name(), ".md")
						if !seen[name] {
							info := agentInfo{Name: name, Group: e.Name()}
							agents = append(agents, info)
							seen[name] = true
						}
					}
				}
			} else if strings.HasSuffix(e.Name(), ".md") {
				name := strings.TrimSuffix(e.Name(), ".md")
				if !seen[name] {
					info := agentInfo{Name: name}
					info.Group = resolveTeam(name, agentsDirPath)
					agents = append(agents, info)
					seen[name] = true
				}
			}
		}
	}

	sort.Slice(agents, func(i, j int) bool { return agents[i].Name < agents[j].Name })
	writeJSON(w, http.StatusOK, agents)
}

// collectAgentNames returns the set of known agent names from opencode.json and
// the agents directory — the same roster surfaced by ListAgents. Used to keep
// delegation targets aligned with the configured agents instead of a hardcoded list.
func collectAgentNames() map[string]bool {
	names := map[string]bool{}

	if configPath, err := opencodeConfigPath(); err == nil {
		if data, err := os.ReadFile(configPath); err == nil {
			var cfg struct {
				Agent map[string]json.RawMessage `json:"agent"`
			}
			if json.Unmarshal(data, &cfg) == nil {
				for name := range cfg.Agent {
					names[name] = true
				}
			}
		}
	}

	if agentsDirPath, err := agentsDir(); err == nil && agentsDirPath != "" {
		entries, _ := os.ReadDir(agentsDirPath)
		for _, e := range entries {
			if e.IsDir() {
				subEntries, _ := os.ReadDir(filepath.Join(agentsDirPath, e.Name()))
				for _, se := range subEntries {
					if !se.IsDir() && strings.HasSuffix(se.Name(), ".md") {
						names[strings.TrimSuffix(se.Name(), ".md")] = true
					}
				}
			} else if strings.HasSuffix(e.Name(), ".md") {
				names[strings.TrimSuffix(e.Name(), ".md")] = true
			}
		}
	}

	return names
}

// resolveTeam detects the team for an agent.
func resolveTeam(agentName, agentsDirPath string) string {
	if agentsDirPath == "" {
		return ""
	}
	path := filepath.Join(agentsDirPath, agentName+".md")
	if mdData, err := os.ReadFile(path); err == nil {
		return detectAgentTeam(agentName, mdData)
	}
	return ""
}

// GET /api/config/agents/{name}
func (h *Handlers) GetAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	agentsDirPath, err := agentsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	path := resolveAgentFile(agentsDirPath, name)
	if path == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"name": name, "content": string(data)})
}

// PUT /api/config/agents/{name}
func (h *Handlers) PutAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	agentsDirPath, err := agentsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	path := resolveAgentFile(agentsDirPath, name)

	// Prevent path traversal (resolveAgentFile only returns paths under
	// agentsDirPath, but a flat or nested layout must still be verified so a
	// crafted name can never escape the base directory).
	absPath, err := filepath.Abs(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	baseDir, err := filepath.Abs(agentsDirPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if path == "" || !strings.HasPrefix(absPath, baseDir) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}

	// The permission: block is owned by PutAgentPermissions, not this handler.
	// Re-apply the on-disk permissions onto the incoming content so a stale
	// frontmatter coming from the client cannot overwrite toggles made via the
	// permissions API in between load and save.
	finalContent := body.Content
	if existing, err := os.ReadFile(path); err == nil {
		fm, _ := parseFrontmatter(string(existing))
		currentPerms := extractPermissionsFromFrontmatter(fm)
		if len(currentPerms) > 0 {
			finalContent = updatePermissionsInFrontmatter(body.Content, currentPerms)
		}
	}

	if err := os.WriteFile(path, []byte(finalContent), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// POST /api/config/agents
func (h *Handlers) CreateAgent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if body.Name == "" || !isValidName(body.Name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	agentsDirPath, err := agentsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	path := filepath.Join(agentsDirPath, body.Name+".md")

	// Prevent path traversal
	absPath, err := filepath.Abs(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	baseDir, err := filepath.Abs(agentsDirPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !strings.HasPrefix(absPath, baseDir) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "path outside allowed directory"})
		return
	}

	if _, err := os.Stat(path); err == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "agent already exists"})
		return
	}

	if err := os.WriteFile(path, []byte(body.Content), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

// DELETE /api/config/agents/{name}
func (h *Handlers) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	agentsDirPath, err := agentsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	path := resolveAgentFile(agentsDirPath, name)

	// Prevent path traversal (resolveAgentFile only returns paths under
	// agentsDirPath, flat or nested — still verify so a crafted name cannot
	// escape the base directory).
	absPath, err := filepath.Abs(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	baseDir, err := filepath.Abs(agentsDirPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if path == "" || !strings.HasPrefix(absPath, baseDir) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}

	if err := os.Remove(path); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GET /api/config/agents/{name}/permissions
// Reads permissions from opencode.json first; falls back to markdown frontmatter.
func (h *Handlers) GetAgentPermissions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	// Tool permissions (read/glob/engram_*/...) have a single source of truth:
	// the markdown frontmatter, which is what opencode actually enforces. The
	// opencode.json `permission` block carries only the `task` delegation
	// object, surfaced separately via the task-permissions endpoint. Reading
	// tool permissions from opencode.json would yield an empty map (the task
	// object is not a scalar) and the UI would render every tool as denied.
	mdPath := readAgentMarkdownPath(name)
	if mdPath == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}

	mdContent, err := os.ReadFile(mdPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	fm, _ := parseFrontmatter(string(mdContent))
	perms := extractPermissionsFromFrontmatter(fm)
	writeJSON(w, http.StatusOK, perms)
}

// ValidPermissionKeys is the canonical set of allowed permission keys.
// Includes all built-in Pi tools plus extended categories (memory, intercom, ado, mcp).
// v2 action names (shell, subagent) are accepted as synonyms and normalized
// onto the internal keys at render time.
var ValidPermissionKeys = map[string]bool{
	// File & code tools
	"read":        true,
	"edit":        true,
	"write":       true,
	"bash":        true,
	"shell":       true, // v2 name for bash
	"glob":        true,
	"grep":        true,
	"lsp":         true,
	"ast_grep":    true,
	"websearch":   true,
	"code_search": true,
	"webfetch":    true,
	// Task & orchestration (consolidated: task=full todo, delegate=full subagent)
	"task":     true,
	"subagent": true, // v2 name for task
	"delegate": true,
	"question": true,
	"skill":    true,
	// Extended categories (plugins, MCP, integrations)
	"memory":   true,
	"intercom": true,
	"mcp":      true,

	// Engram memory tools (from engram plugin)
	"mem_capture_passive":   true,
	"mem_compare":           true,
	"mem_context":           true,
	"mem_current_project":   true,
	"mem_delete":            true,
	"mem_doctor":            true,
	"mem_get_observation":   true,
	"mem_judge":             true,
	"mem_save":              true,
	"mem_save_prompt":       true,
	"mem_search":            true,
	"mem_session_end":       true,
	"mem_session_start":     true,
	"mem_session_summary":   true,
	"mem_stats":             true,
	"mem_suggest_topic_key": true,
	"mem_timeline":          true,
	"mem_update":            true,
}

// ValidPermissionValues are the only accepted permission values.
var ValidPermissionValues = map[string]bool{
	"allow": true,
	"ask":   true,
	"deny":  true,
}

// PUT /api/config/agents/{name}/permissions
// Writes permissions to opencode.json (primary) and markdown frontmatter (backward compat).
func (h *Handlers) PutAgentPermissions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit

	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Validate permission values only (keys are dynamic — anyone can add custom tools)
	var invalidValues []string
	for k, v := range body {
		if !ValidPermissionValues[v] {
			invalidValues = append(invalidValues, fmt.Sprintf("%s=%q", k, v))
		}
	}
	if len(invalidValues) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid permission value(s), must be allow, ask, or deny",
			"invalid": invalidValues,
		})
		return
	}

	found := false

	// Tool permissions are written only to the markdown frontmatter, the single
	// source of truth opencode enforces. The opencode.json `permission` block is
	// reserved for the `task` delegation object (managed by the task-permissions
	// endpoint) and is intentionally left untouched here.
	if mdPath := readAgentMarkdownPath(name); mdPath != "" {
		found = true
		mdContent, err := os.ReadFile(mdPath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		updated := updatePermissionsInFrontmatter(string(mdContent), body)
		_ = agents.WriteAgentBackup(mdPath, mdContent)
		if err := os.WriteFile(mdPath, []byte(updated), 0644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found in opencode.json or markdown files"})
		return
	}

	writeJSON(w, http.StatusOK, body)
}

// GET /api/config/agents/{name}/task-permissions
// Returns the per-subagent task delegation map (permission.task) from
// opencode.json. A scalar task value is normalized to {"*": value}; absence
// returns an empty map.
func (h *Handlers) GetAgentTaskPermissions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	result := map[string]string{}

	// The markdown frontmatter is the single source of truth opencode enforces
	// (opencode merges markdown agents on top of opencode.json, markdown wins),
	// so read the task map from the .md. Fall back to opencode.json only when
	// there is no markdown agent.
	if mdPath := readAgentMarkdownPath(name); mdPath != "" {
		if data, err := os.ReadFile(mdPath); err == nil {
			if m, ok := agents.ReadTaskPermission(string(data)); ok {
				writeJSON(w, http.StatusOK, m)
				return
			}
		}
	}

	path, err := opencodeConfigPath()
	if err == nil {
		if data, err := os.ReadFile(path); err == nil {
			if m, ok := lookupAgentTaskMap(data, name); ok {
				result = m
			}
		}
	}

	writeJSON(w, http.StatusOK, result)
}

// PUT /api/config/agents/{name}/task-permissions
// Writes the per-subagent task delegation map (permission.task) as an object
// into opencode.json. Keys are sub-agent name globs ("*" is the catch-all);
// values must be allow, ask, or deny.
func (h *Handlers) PutAgentTaskPermissions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit

	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var invalidValues []string
	for k, v := range body {
		if !ValidPermissionValues[v] {
			invalidValues = append(invalidValues, fmt.Sprintf("%s=%q", k, v))
		}
	}
	if len(invalidValues) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid permission value(s), must be allow, ask, or deny",
			"invalid": invalidValues,
		})
		return
	}

	// Write the delegation task map into the agent's markdown frontmatter —
	// the single source of truth opencode enforces. (Tool permissions use the
	// same path via PutAgentPermissions.)
	mdPath := readAgentMarkdownPath(name)
	if mdPath == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent markdown not found"})
		return
	}
	mdContent, err := os.ReadFile(mdPath)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent markdown not found"})
		return
	}

	task := body
	if len(task) == 0 {
		// Empty map clears the delegation restriction: allow delegating to all.
		task = map[string]string{"*": "allow"}
	}
	updated, ok := agents.InjectTaskPermission(string(mdContent), task)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "agent markdown has no permission block"})
		return
	}
	_ = agents.WriteAgentBackup(mdPath, mdContent)
	if err := os.WriteFile(mdPath, []byte(updated), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, body)
}

// GET /api/config/agents/{name}/model
// Returns the agent's default model from opencode.json (agent.<name>.model),
// falling back to the markdown frontmatter "model:" field. Empty when unset.
func (h *Handlers) GetAgentModel(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	model := ""

	// opencode.json is the primary source.
	if path, err := opencodeConfigPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			if raw := lookupAgentField(data, name, "model"); len(raw) > 0 {
				_ = json.Unmarshal(raw, &model)
			}
		}
	}

	// Fallback: markdown frontmatter.
	if model == "" {
		if mdPath := readAgentMarkdownPath(name); mdPath != "" {
			if mdContent, err := os.ReadFile(mdPath); err == nil {
				model = getScalarFrontmatterField(string(mdContent), "model")
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"model": model})
}

// PUT /api/config/agents/{name}/model
// Sets the agent's default model in opencode.json and markdown frontmatter.
// An empty model clears the override (the agent falls back to the runtime default).
func (h *Handlers) PutAgentModel(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit

	var body struct {
		Model string `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	model := strings.TrimSpace(body.Model)

	if !applyAgentModel(name, model) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found in opencode.json or markdown files"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"model": model})
}

// ApplyAgentModel is the exported multi-host form of applyAgentModel.
// Used by install, control UI profile activation, and config API.
func ApplyAgentModel(name, model string) bool {
	return applyAgentModel(name, model)
}

// agentMarkdownSearchDirs returns host dirs where ywai writes agent .md files.
func agentMarkdownSearchDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".config", "opencode", "agents"),
		filepath.Join(home, ".pi", "agent", "agents"),
		filepath.Join(home, ".omp", "agent", "agents"),
		filepath.Join(home, ".claude", "agents"),
		filepath.Join(home, ".cursor", "agents"),
	}
}

// applyAgentModel writes an agent's model into opencode.json (when present)
// and every installed host markdown copy (opencode, pi, omp, claude, cursor).
// An empty model clears the override. Returns true if any location was updated.
func applyAgentModel(name, model string) bool {
	model = strings.TrimSpace(model)
	found := false

	if path, err := opencodeConfigPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			var config map[string]json.RawMessage
			if json.Unmarshal(data, &config) == nil {
				var agents map[string]json.RawMessage
				if agentRaw, ok := config["agent"]; ok && json.Unmarshal(agentRaw, &agents) == nil {
					if existingRaw, exists := agents[name]; exists {
						var agentCfg map[string]json.RawMessage
						if json.Unmarshal(existingRaw, &agentCfg) == nil {
							if model == "" {
								delete(agentCfg, "model")
							} else {
								modelJSON, _ := json.Marshal(model)
								agentCfg["model"] = modelJSON
							}
							agentJSON, _ := json.Marshal(agentCfg)
							agents[name] = agentJSON
							agentsJSON, _ := json.Marshal(agents)
							config["agent"] = agentsJSON
							pretty, _ := json.MarshalIndent(config, "", "  ")
							_ = os.WriteFile(path+".bak", data, 0644)
							if os.WriteFile(path, pretty, 0644) == nil {
								found = true
							}
						}
					}
				}
			}
		}
	}

	// All host markdown trees ywai installs into (flat or one group subdir).
	for _, dir := range agentMarkdownSearchDirs() {
		mdPath := resolveAgentFile(dir, name)
		if mdPath == "" {
			// Also try basename for grouped keys (core/dev → dev).
			if base := filepath.Base(name); base != name {
				mdPath = resolveAgentFile(dir, base)
			}
		}
		if mdPath == "" {
			continue
		}
		mdContent, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}
		updated := setScalarFrontmatterField(string(mdContent), "model", model)
		_ = agents.WriteAgentBackup(mdPath, mdContent)
		if os.WriteFile(mdPath, []byte(updated), 0644) == nil {
			found = true
		}
	}

	return found
}

// ApplyActiveOrchestratorProfile writes the active userconfig model profile
// (balanced / fast / deep / custom) into every installed agent markdown on all
// hosts, and materializes the profile's orchestration policy into the
// orchestrator agent (generated section + edit/write permission flip). Returns
// how many agent roles were applied at least once (policy counts as one).
func ApplyActiveOrchestratorProfile() (int, error) {
	cfg, err := userconfig.LoadConfig()
	if err != nil {
		return 0, err
	}
	profile := cfg.GetActiveOrchestratorProfile()
	applied := 0
	for agentName, rd := range profile.Agents {
		// Empty model is inherit: clear any previous pin on the agent file.
		if applyAgentModel(agentName, rd.Model) {
			applied++
		}
	}
	if applyOrchestrationPolicy(profile.Orchestration) {
		applied++
	}
	if applyOmpModelRoles(profile) {
		applied++
	}
	return applied, nil
}

// ompModelRoleSources maps omp's modelRoles (config.yml) to ywai orchestrator
// agents. For each omp role, the first agent (priority order) with a non-empty
// model in the active profile supplies the value; roles without a source are
// left untouched.
var ompModelRoleSources = []struct {
	Role   string
	Agents []string
}{
	{"default", []string{"orchestrator", "dev"}},
	{"smol", []string{"qa", "ask", "finder"}},
	{"plan", []string{"architect", "planning"}},
	{"designer", []string{"designer"}},
	{"advisor", []string{"advisor"}},
	{"commit", []string{"dev", "orchestrator"}},
}

// ompModelRolesFor computes the omp modelRoles map from a profile: for each
// omp role, the first mapped ywai agent with a model wins. Explicit per-profile
// overrides (profile.OmpModelRoles) win over the derivation and are written
// verbatim — the user may pick an omp-specific provider id, or add roles the
// mapping table does not cover (vision, task, slow).
func ompModelRolesFor(profile userconfig.OrchestratorModelProfile) map[string]string {
	out := map[string]string{}
	for _, src := range ompModelRoleSources {
		if m := firstProfileModel(profile, src.Agents); m != "" {
			out[src.Role] = m
		}
	}
	for role, m := range profile.OmpModelRoles {
		if m = strings.TrimSpace(m); m != "" {
			out[role] = m
		}
	}
	return out
}

// OmpModelRolesFor is the exported form used by the profiles API so the UI can
// show what the active profile would write into omp's modelRoles.
func OmpModelRolesFor(profile userconfig.OrchestratorModelProfile) map[string]string {
	return ompModelRolesFor(profile)
}

// applyOmpModelRoles writes the active profile's models into omp's
// ~/.omp/agent/config.yml modelRoles block — the only model config omp honors
// (a model: line in agent markdown is inert for omp). Roles without a ywai
// source and unrelated config.yml keys stay untouched. No-op when omp is not
// installed or the file is unreadable. Returns true when the file changed.
func applyOmpModelRoles(profile userconfig.OrchestratorModelProfile) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	dir := filepath.Join(home, ".omp", "agent")
	if _, err := os.Stat(dir); err != nil {
		return false // omp not installed
	}
	path := filepath.Join(dir, "config.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false
	}
	roles, _ := root["modelRoles"].(map[string]any)
	if roles == nil {
		roles = map[string]any{}
	}
	changed := false
	for role, model := range ompModelRolesFor(profile) {
		if roles[role] != model {
			roles[role] = model
			changed = true
		}
	}
	if thinking := strings.TrimSpace(profile.OmpThinkingLevel); thinking != "" && root["defaultThinkingLevel"] != thinking {
		root["defaultThinkingLevel"] = thinking
		changed = true
	}
	if !changed {
		return false
	}
	root["modelRoles"] = roles
	out, err := yaml.Marshal(root)
	if err != nil {
		return false
	}
	_ = os.WriteFile(path+".bak", data, 0o600)
	return os.WriteFile(path, out, 0o600) == nil
}

// firstProfileModel returns the first non-empty model among the given ywai
// agent names in the profile, with the opencode provider prefix stripped
// ("opencode-admin/deepseek-v4-flash" → "deepseek-v4-flash"). omp resolves
// bare model ids by fuzzy match against its own providers; an opencode
// provider id would not resolve on omp.
func firstProfileModel(profile userconfig.OrchestratorModelProfile, agents []string) string {
	for _, name := range agents {
		m := strings.TrimSpace(profile.Agents[name].Model)
		if m == "" {
			continue
		}
		if i := strings.IndexByte(m, '/'); i >= 0 {
			m = m[i+1:]
		}
		return m
	}
	return ""
}

const (
	orchestrationPolicyMarkerStart = "<!-- ywai:orchestration-policy -->"
	orchestrationPolicyMarkerEnd   = "<!-- /ywai:orchestration-policy -->"
)

// orchestrationPolicySection renders the profile's orchestration policy as a
// self-contained markdown block (without the delimiting markers).
func orchestrationPolicySection(policy userconfig.OrchestrationPolicy) string {
	policy = policy.Normalize()
	soloWrite := "true"
	if !policy.SoloWriteAllowed() {
		soloWrite = "false"
	}
	hops := 1
	if policy.MaxHopsBeforeEscalate != nil {
		hops = *policy.MaxHopsBeforeEscalate
	}
	return fmt.Sprintf(`## Orchestration policy (installed from active profile)

This block is generated at install / profile-switch time — do not edit.

- **default_mode**: %s
- **allow_solo_write**: %s
- **max_hops_before_escalate**: %d
- **require_review**: %s
- **escalate_on**: %s

When the user gives no explicit signal, use **default_mode** as the triage
fallback (replacing the static table default). Solo/thin may edit directly
only when **allow_solo_write** is true; full never edits product code itself.
`,
		policy.DefaultMode, soloWrite, hops,
		policy.RequireReview, strings.Join(policy.EscalateOn, ", "))
}

// applyOrchestrationPolicy writes the active profile's orchestration policy
// into the installed orchestrator agent on every host: a generated policy
// section in the body, plus the edit/write permission flip (opencode
// permission: block; pi/omp/claude tools: list). Returns true if any host was
// updated.
func applyOrchestrationPolicy(policy userconfig.OrchestrationPolicy) bool {
	found := false
	section := orchestrationPolicySection(policy)
	for _, dir := range agentMarkdownSearchDirs() {
		mdPath := resolveAgentFile(dir, "orchestrator")
		if mdPath == "" {
			continue
		}
		content, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}
		updated := upsertPolicySection(string(content), section)
		updated = applySoloWritePermissions(updated, policy.SoloWriteAllowed())
		if updated == string(content) {
			continue
		}
		_ = agents.WriteAgentBackup(mdPath, content)
		if os.WriteFile(mdPath, []byte(updated), 0644) == nil {
			found = true
		}
	}
	if applyOrchestrationPolicyToOpenCodeJSON(policy.SoloWriteAllowed()) {
		found = true
	}
	return found
}

// upsertPolicySection replaces the previous generated policy block (marker
// delimited) or appends a new one at the end of the body.
func upsertPolicySection(content, section string) string {
	block := orchestrationPolicyMarkerStart + "\n" + section + orchestrationPolicyMarkerEnd + "\n"
	start := strings.Index(content, orchestrationPolicyMarkerStart)
	end := strings.Index(content, orchestrationPolicyMarkerEnd)
	if start >= 0 && end > start {
		end += len(orchestrationPolicyMarkerEnd)
		return content[:start] + block + content[end:]
	}
	return strings.TrimRight(content, "\n") + "\n\n" + block
}

// applySoloWritePermissions flips the orchestrator's write surface on the
// installed markdown: the v2 rules' broad edit/shell entries (and the pi/omp/
// claude tools: list). The flip is symmetric — a deny→allow cycle restores
// edit and shell to the allow posture with the guardrail denials intact.
// Verify allowlist patterns are dropped on deny (they would keep granting
// shell under a broad deny) and are not reconstructed on re-allow.
func applySoloWritePermissions(content string, allow bool) string {
	fm, _ := parseFrontmatter(content)
	if fm == "" {
		return content
	}
	rules, hasRules := agents.ParsePermissionRulesYAML(fm)
	if !hasRules {
		// pi/omp/claude-style file: no v2 rules block, sync the tools list.
		return syncToolsList(content, allow)
	}
	effect := "deny"
	if allow {
		effect = "allow"
	}
	var out []agents.PermissionRule
	broadEdit, broadShell := false, false
	for _, r := range rules {
		switch {
		case r.Action == "edit" && r.Resource == "*" && !broadEdit:
			r.Effect = effect
			broadEdit = true
		case r.Action == "shell" && r.Resource == "*" && !broadShell:
			r.Effect = effect
			broadShell = true
		case r.Action == "shell" && !allow && r.Effect == "allow":
			// Verify-posture allowlist: dead weight under a broad deny.
			continue
		}
		out = append(out, r)
	}
	return replacePermissionsBlock(content, out)
}

// syncToolsList makes a pi/omp/claude-style "tools:" frontmatter list match the
// solo-write policy: deny removes edit/write tokens, allow re-adds them (they
// are the install baseline). Reversible in both directions.
func syncToolsList(content string, allow bool) string {
	fm, body := parseFrontmatter(content)
	if fm == "" {
		return content
	}
	lines := strings.Split(fm, "\n")
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if line != trimmed || !strings.HasPrefix(trimmed, "tools:") {
			continue
		}
		var kept []string
		hasEdit, hasWrite := false, false
		for _, t := range strings.Split(strings.TrimSpace(strings.TrimPrefix(trimmed, "tools:")), ",") {
			t = strings.TrimSpace(t)
			// Claude-style lists capitalize tool names (Edit, Write); pi/omp
			// use lowercase. Compare case-insensitively.
			low := strings.ToLower(t)
			if low == "edit" {
				hasEdit = true
			}
			if low == "write" {
				hasWrite = true
			}
			if t != "" && (allow || (low != "edit" && low != "write")) {
				kept = append(kept, t)
			}
		}
		if allow {
			// Re-add the install baseline (edit/write) using the list's casing
			// so claude-style capitalized lists stay capitalized, inserted right
			// after the read token to mirror the install ordering.
			editName, writeName := "edit", "write"
			for _, t := range kept {
				if t != "" && t[0] >= 'A' && t[0] <= 'Z' {
					editName, writeName = "Edit", "Write"
					break
				}
			}
			inserted := false
			for idx, t := range kept {
				if strings.EqualFold(t, "read") {
					out := append([]string(nil), kept[:idx+1]...)
					if !hasEdit {
						out = append(out, editName)
					}
					if !hasWrite {
						out = append(out, writeName)
					}
					kept = append(out, kept[idx+1:]...)
					inserted = true
					break
				}
			}
			if !inserted {
				if !hasEdit {
					kept = append(kept, editName)
				}
				if !hasWrite {
					kept = append(kept, writeName)
				}
			}
		}
		lines[i] = "tools: " + strings.Join(kept, ",")
		changed = true
	}
	if !changed {
		return content
	}
	return "---\n" + strings.Join(lines, "\n") + "\n---\n\n" + body
}

// applyOrchestrationPolicyToOpenCodeJSON mirrors the edit/shell flip into the
// opencode.json agent.orchestrator entry when it exists (markdown-only installs
// have no such entry). Always writes a v2 `permissions` rule array. A leftover
// v1 `permission` map is converted at this boundary and not written back.
func applyOrchestrationPolicyToOpenCodeJSON(allow bool) bool {
	path, err := opencodeConfigPath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var config map[string]json.RawMessage
	if json.Unmarshal(data, &config) != nil {
		return false
	}
	var agentsMap map[string]json.RawMessage
	if raw, ok := config["agent"]; !ok || json.Unmarshal(raw, &agentsMap) != nil {
		return false
	}
	orchestratorRaw, ok := agentsMap["orchestrator"]
	if !ok {
		return false
	}
	var agentCfg map[string]json.RawMessage
	if json.Unmarshal(orchestratorRaw, &agentCfg) != nil {
		return false
	}
	effect := "deny"
	if allow {
		effect = "allow"
	}

	var rules []agents.PermissionRule
	if rulesRaw, ok := agentCfg["permissions"]; ok {
		if json.Unmarshal(rulesRaw, &rules) != nil {
			return false
		}
	} else {
		permRaw, ok := agentCfg["permission"]
		if !ok {
			return false
		}
		var perms map[string]any
		if json.Unmarshal(permRaw, &perms) != nil {
			return false
		}
		internal := map[string]string{}
		var task map[string]string
		for k, v := range perms {
			if k == "task" {
				if m, ok := v.(map[string]any); ok {
					task = make(map[string]string, len(m))
					for id, ev := range m {
						if s, ok := ev.(string); ok {
							task[id] = s
						}
					}
				} else if s, ok := v.(string); ok && s != "" {
					task = map[string]string{"*": s}
				}
				continue
			}
			if s, ok := v.(string); ok && s != "" {
				internal[k] = s
				continue
			}
			if k == "bash" {
				internal[k] = "verify"
			}
		}
		rules = agents.RulesFromPermissionMap("orchestrator", internal)
		if len(task) > 0 {
			rules = agents.ReplaceSubagentRules(rules, task)
		}
	}
	var out []agents.PermissionRule
	broadEdit, broadShell := false, false
	for _, r := range rules {
		switch {
		case r.Action == "edit" && r.Resource == "*" && !broadEdit:
			r.Effect = effect
			broadEdit = true
		case r.Action == "shell" && r.Resource == "*" && !broadShell:
			r.Effect = effect
			broadShell = true
		case r.Action == "shell" && !allow && r.Effect == "allow":
			continue
		}
		out = append(out, r)
	}
	updated, _ := json.Marshal(out)
	agentCfg["permissions"] = updated
	delete(agentCfg, "permission")

	agentJSON, _ := json.Marshal(agentCfg)
	agentsMap["orchestrator"] = agentJSON
	agentsJSON, _ := json.Marshal(agentsMap)
	config["agent"] = agentsJSON
	pretty, _ := json.MarshalIndent(config, "", "  ")
	_ = os.WriteFile(path+".bak", data, 0644)
	return os.WriteFile(path, pretty, 0644) == nil
}

// GET /api/config/agents/graph
//
// Returns the static delegation graph derived from each agent's
// permission.task map: nodes are agents (from opencode.json + the agents dir),
// edges run source->target for every task key whose value is allow/ask (except
// "*", which is a catch-all surfaced as a node attribute). Targets referenced
// by a task map but not themselves defined become "ghost" nodes so the diagram
// never shows dangling edges. This is the capability graph (what an agent MAY
// delegate), distinct from the runtime delegation DAG at
// GET /api/sessions/{id}/graph.
func (h *Handlers) GetAgentGraph(w http.ResponseWriter, r *http.Request) {
	names := collectAgentNames()

	// Config bytes are read once and reused for field/task lookups. When there
	// is no opencode.json we still build the graph from the agents dir alone
	// (with empty per-agent delegation).
	var configData []byte
	if path, err := opencodeConfigPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			configData = data
		}
	}

	agentsDirPath, _ := agentsDir()

	// Deterministic node order.
	ordered := make([]string, 0, len(names))
	for n := range names {
		ordered = append(ordered, n)
	}
	sort.Strings(ordered)

	// Index of which nodes exist (by name) so we can flag ghost targets.
	existing := make(map[string]bool, len(ordered))
	for _, n := range ordered {
		existing[n] = true
	}

	nodes := make([]agentGraphNode, 0, len(ordered))
	edges := make([]agentGraphEdge, 0)

	for _, name := range ordered {
		node := agentGraphNode{ID: name, Name: name}

		// mode: prefer opencode.json, fall back to markdown frontmatter.
		if raw := lookupAgentField(configData, name, "mode"); len(raw) > 0 {
			_ = json.Unmarshal(raw, &node.Mode)
		}
		if node.Mode == "" {
			if mdPath := resolveAgentFile(agentsDirPath, name); mdPath != "" {
				if data, err := os.ReadFile(mdPath); err == nil {
					fm, _ := parseFrontmatter(string(data))
					node.Mode = extractModeFromFrontmatter(fm)
					node.Group = extractFrontmatterField(data, "group")
				}
			}
		}
		if node.Group == "" {
			node.Group = resolveTeam(name, agentsDirPath)
		}

		// model: opencode.json first, then markdown frontmatter.
		if raw := lookupAgentField(configData, name, "model"); len(raw) > 0 {
			_ = json.Unmarshal(raw, &node.Model)
		}
		if node.Model == "" {
			if mdPath := resolveAgentFile(agentsDirPath, name); mdPath != "" {
				if data, err := os.ReadFile(mdPath); err == nil {
					node.Model = getScalarFrontmatterField(string(data), "model")
				}
			}
		}

		// task delegation map -> edges + wildcard attribute. Prefer opencode.json;
		// fall back to the agent markdown's permission.task. Workflow-exported
		// agents live only as .md files and never touch opencode.json, so without
		// this fallback their delegation edges would be missing from the graph.
		var taskMap map[string]string
		if m, ok := lookupAgentTaskMap(configData, name); ok {
			taskMap = m
		}
		if taskMap == nil && !node.HasWildcard {
			if mdPath := resolveAgentFile(agentsDirPath, name); mdPath != "" {
				taskMap = taskMapFromMarkdown(mdPath)
			}
		}
		for target, val := range taskMap {
			if target == "*" {
				node.HasWildcard = true
				node.WildcardValue = val
				continue
			}
			if val == "allow" || val == "ask" {
				edges = append(edges, agentGraphEdge{
					ID:     name + "->" + target,
					Source: name,
					Target: target,
					Value:  val,
				})
			}
		}

		nodes = append(nodes, node)
	}

	// Ghost nodes: targets referenced by an edge but not defined as an agent.
	seenGhost := map[string]bool{}
	for _, e := range edges {
		if existing[e.Target] || seenGhost[e.Target] {
			continue
		}
		seenGhost[e.Target] = true
		nodes = append(nodes, agentGraphNode{
			ID:    e.Target,
			Name:  e.Target,
			Ghost: true,
		})
	}

	writeJSON(w, http.StatusOK, agentGraphResp{Nodes: nodes, Edges: edges})
}

// taskMapFromMarkdown extracts an agent's permission.task delegation map from
// its markdown frontmatter. Used as a fallback for agents that exist only as
// .md files (e.g. workflow exports) and are absent from opencode.json. Returns
// nil when the file is unreadable, has no frontmatter, or task is not a map.
func taskMapFromMarkdown(mdPath string) map[string]string {
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return nil
	}
	fm, _ := parseFrontmatter(string(data))
	if fm == "" {
		return nil
	}
	if rules, ok := agents.ParsePermissionRulesYAML(fm); ok {
		out := map[string]string{}
		for _, r := range rules {
			if r.Action == "subagent" {
				out[r.Resource] = r.Effect
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	var doc struct {
		Permission struct {
			Task map[string]string `yaml:"task"`
		} `yaml:"permission"`
	}
	if err := yaml.Unmarshal([]byte(fm), &doc); err != nil {
		return nil
	}
	return doc.Permission.Task
}

// agentGraphNode is a single agent (or a ghost target) in the delegation graph.
type agentGraphNode struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Mode          string `json:"mode,omitempty"`
	Model         string `json:"model,omitempty"`
	Group         string `json:"group,omitempty"`
	HasWildcard   bool   `json:"hasWildcard,omitempty"`
	WildcardValue string `json:"wildcardValue,omitempty"`
	Ghost         bool   `json:"ghost,omitempty"` // referenced by an edge but not defined
}

// agentGraphEdge is a delegation permission source -> target.
type agentGraphEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Value  string `json:"value"` // allow | ask
}

type agentGraphResp struct {
	Nodes []agentGraphNode `json:"nodes"`
	Edges []agentGraphEdge `json:"edges"`
}

// lookupAgentField returns the raw JSON value of agent.<name>.<key> from
// opencode.json config bytes, or nil if any level is missing.
func lookupAgentField(configData []byte, name, key string) json.RawMessage {
	var config map[string]json.RawMessage
	if json.Unmarshal(configData, &config) != nil {
		return nil
	}
	agentRaw, ok := config["agent"]
	if !ok {
		return nil
	}
	var agents map[string]json.RawMessage
	if json.Unmarshal(agentRaw, &agents) != nil {
		return nil
	}
	agentData, ok := agents[name]
	if !ok {
		return nil
	}
	var agent map[string]json.RawMessage
	if json.Unmarshal(agentData, &agent) != nil {
		return nil
	}
	return agent[key]
}

// lookupAgentTaskMap returns the delegation map for agent.<name> from
// opencode.json: v2 subagent rules (agent.<name>.permissions) first, the
// legacy v1 permission.task map/scalar as fallback. ok is false when neither
// form is present.
func lookupAgentTaskMap(configData []byte, name string) (map[string]string, bool) {
	var config map[string]json.RawMessage
	if json.Unmarshal(configData, &config) != nil {
		return nil, false
	}
	agentRaw, ok := config["agent"]
	if !ok {
		return nil, false
	}
	var agentsMap map[string]json.RawMessage
	if json.Unmarshal(agentRaw, &agentsMap) != nil {
		return nil, false
	}
	agentData, ok := agentsMap[name]
	if !ok {
		return nil, false
	}
	var agent map[string]json.RawMessage
	if json.Unmarshal(agentData, &agent) != nil {
		return nil, false
	}
	if rulesRaw, ok := agent["permissions"]; ok {
		var rules []struct {
			Action   string `json:"action"`
			Resource string `json:"resource"`
			Effect   string `json:"effect"`
		}
		if json.Unmarshal(rulesRaw, &rules) == nil {
			out := map[string]string{}
			for _, r := range rules {
				if r.Action == "subagent" {
					out[r.Resource] = r.Effect
				}
			}
			if len(out) > 0 {
				return out, true
			}
		}
	}
	// Legacy v1: permission.task object or scalar.
	permRaw, ok := agent["permission"]
	if !ok {
		return nil, false
	}
	var perm map[string]json.RawMessage
	if json.Unmarshal(permRaw, &perm) != nil {
		return nil, false
	}
	taskRaw, ok := perm["task"]
	if !ok {
		return nil, false
	}
	var asMap map[string]string
	if json.Unmarshal(taskRaw, &asMap) == nil {
		return asMap, true
	}
	var asStr string
	if json.Unmarshal(taskRaw, &asStr) == nil && asStr != "" {
		return map[string]string{"*": asStr}, true
	}
	return nil, false
}

// --- Delegation Rules (structured JSON sidecar) ---
//
// The "Delegation Rules" table + "Mandatory Delegation Triggers" are stored as
// structured JSON in a sidecar file (agentsDir/delegations.json) — NOT parsed
// from markdown. The .md is GENERATED from the JSON (by agents.ApplyDelegations
// on install, and re-rendered on PUT here). The JSON is the single source of
// truth, so there is no fragile markdown parser to break.

// delegationRule is one row of the "Delegation Rules" table.
type delegationRule struct {
	Action   string `json:"action"`
	Inline   string `json:"inline"`
	Delegate string `json:"delegate"`
}

// delegationTrigger is one item of the "Mandatory Delegation Triggers" list.
type delegationTrigger struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// delegationRulesResp is the GET /delegation-rules payload.
type delegationRulesResp struct {
	Rules    []delegationRule    `json:"rules"`
	Triggers []delegationTrigger `json:"triggers"`
	HasRules bool                `json:"hasRules"`
}

// delegationSidecar is the on-disk shape of agentsDir/delegations.json. It
// mirrors agents.DelegationsDoc but lives in the configapi package to avoid an
// import cycle, and only carries the fields the UI needs.
type delegationSidecar struct {
	Defaults struct {
		Rules    []delegationRule    `json:"rules"`
		Triggers []delegationTrigger `json:"triggers"`
	} `json:"defaults"`
	Agents map[string]struct {
		Rules     []delegationRule    `json:"rules,omitempty"`
		Triggers  []delegationTrigger `json:"triggers,omitempty"`
		SkipRules bool                `json:"skip_rules,omitempty"`
	} `json:"agents"`
}

// loadDelegationSidecar reads agentsDir/delegations.json. Returns an empty doc
// (hasRules=false) when absent so the UI offers to seed via Enable.
func loadDelegationSidecar() (*delegationSidecar, bool) {
	dir, err := agentsDir()
	if err != nil {
		return &delegationSidecar{Agents: map[string]struct {
			Rules     []delegationRule    `json:"rules,omitempty"`
			Triggers  []delegationTrigger `json:"triggers,omitempty"`
			SkipRules bool                `json:"skip_rules,omitempty"`
		}{}}, false
	}
	data, err := os.ReadFile(filepath.Join(dir, "delegations.json"))
	if err != nil {
		return &delegationSidecar{Agents: map[string]struct {
			Rules     []delegationRule    `json:"rules,omitempty"`
			Triggers  []delegationTrigger `json:"triggers,omitempty"`
			SkipRules bool                `json:"skip_rules,omitempty"`
		}{}}, false
	}
	var sc delegationSidecar
	if json.Unmarshal(data, &sc) != nil {
		return &delegationSidecar{Agents: map[string]struct {
			Rules     []delegationRule    `json:"rules,omitempty"`
			Triggers  []delegationTrigger `json:"triggers,omitempty"`
			SkipRules bool                `json:"skip_rules,omitempty"`
		}{}}, false
	}
	return &sc, true
}

// GET /api/config/agents/{name}/delegation-rules
//
// Returns the rules + triggers for an agent from the sidecar JSON. Falls back
// to defaults when the agent has no override, and reports hasRules=false when
// the agent opts out (skip_rules) or the sidecar is absent.
func (h *Handlers) GetDelegationRules(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	sc, _ := loadDelegationSidecar()
	resp := delegationRulesResp{Rules: []delegationRule{}, Triggers: []delegationTrigger{}}

	a, ok := sc.Agents[name]
	if ok && a.SkipRules {
		writeJSON(w, http.StatusOK, resp) // hasRules=false
		return
	}

	rules := a.Rules
	if len(rules) == 0 {
		rules = sc.Defaults.Rules
	}
	triggers := a.Triggers
	if len(triggers) == 0 {
		triggers = sc.Defaults.Triggers
	}

	if len(rules) > 0 || len(triggers) > 0 {
		resp.HasRules = true
		resp.Rules = rules
		resp.Triggers = triggers
	}
	writeJSON(w, http.StatusOK, resp)
}

// PUT /api/config/agents/{name}/delegation-rules
//
// Writes the rules + triggers for an agent into the sidecar JSON (creating the
// agent entry + overriding defaults) AND re-renders the markdown section into
// the agent's .md prompt body so the two stay in sync. The JSON is the source
// of truth; the .md is a generated artifact.
func (h *Handlers) PutDelegationRules(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body struct {
		Rules    []delegationRule    `json:"rules"`
		Triggers []delegationTrigger `json:"triggers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	dir, err := agentsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	sc, _ := loadDelegationSidecar()
	if sc.Agents == nil {
		sc.Agents = map[string]struct {
			Rules     []delegationRule    `json:"rules,omitempty"`
			Triggers  []delegationTrigger `json:"triggers,omitempty"`
			SkipRules bool                `json:"skip_rules,omitempty"`
		}{}
	}
	entry := sc.Agents[name]
	entry.SkipRules = false
	entry.Rules = body.Rules
	entry.Triggers = body.Triggers
	sc.Agents[name] = entry

	// Persist the sidecar JSON.
	sidecarData, _ := json.MarshalIndent(sc, "", "  ")
	sidecarPath := filepath.Join(dir, "delegations.json")
	if err := os.WriteFile(sidecarPath, append(sidecarData, '\n'), 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Re-render the markdown section so the .md the agent reads stays in sync.
	if mdPath := readAgentMarkdownPath(name); mdPath != "" {
		if mdContent, err := os.ReadFile(mdPath); err == nil {
			rendered := renderRulesMarkdown(body.Rules, body.Triggers)
			updated := replaceLocalMarkdownSection(string(mdContent), "Delegation Rules", "###", rendered, true)
			if updated != string(mdContent) {
				_ = agents.WriteAgentBackup(mdPath, mdContent)
				_ = os.WriteFile(mdPath, []byte(updated), 0o644)
			}
		}
	}

	writeJSON(w, http.StatusOK, delegationRulesResp{
		Rules: body.Rules, Triggers: body.Triggers, HasRules: true,
	})
}

// renderRulesMarkdown renders the rules table + triggers list as markdown body
// (the content that goes under the "### Delegation Rules" heading). Mirrors
// agents.renderRulesSection.
func renderRulesMarkdown(rules []delegationRule, triggers []delegationTrigger) string {
	var b strings.Builder
	b.WriteString("Core principle: **does this inflate my context without need?** If yes -> delegate. If no -> do it inline.\n\n")

	if len(rules) > 0 {
		b.WriteString("| Action | Inline | Delegate |\n")
		b.WriteString("| ------ | ------ | -------- |\n")
		for _, r := range rules {
			action := strings.ReplaceAll(r.Action, "|", "\\|")
			delegate := strings.ReplaceAll(r.Delegate, "|", "\\|")
			inline := r.Inline
			if inline == "" {
				inline = "No"
			}
			if delegate == "" {
				delegate = "No"
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", action, inline, delegate))
		}
		b.WriteString("\n")
		b.WriteString(agents.OpenCodeDelegateToolHint)
	}

	if len(triggers) > 0 {
		b.WriteString("\n#### Mandatory Delegation Triggers\n\n")
		b.WriteString("These gates are **non-skippable hard gates**, not recommendations.\n\n")
		b.WriteString(agents.OpenCodeDelegateSemanticGuard)
		for i, t := range triggers {
			n := strings.TrimSpace(t.Name)
			if n == "" {
				n = "Trigger"
			}
			b.WriteString(fmt.Sprintf("%d. **%s**: %s\n", i+1, n, strings.TrimSpace(t.Description)))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// replaceLocalMarkdownSection replaces the body content under a heading. Local
// copy (the configapi package already has extractMarkdownSection/replaceMarkdownSection
// in frontmatter.go; this is that same helper, kept here to avoid duplication
// confusion — it delegates to the frontmatter.go implementation).
func replaceLocalMarkdownSection(content, headerText, headingPrefix, newContent string, includeSubsections bool) string {
	return replaceMarkdownSection(content, headerText, headingPrefix, newContent, includeSubsections)
}
