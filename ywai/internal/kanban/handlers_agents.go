package kanban

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

	// Try opencode.json first
	path, err := opencodeConfigPath()
	if err == nil {
		if data, err := os.ReadFile(path); err == nil {
			var config map[string]json.RawMessage
			if err := json.Unmarshal(data, &config); err == nil {
				if agentRaw, ok := config["agent"]; ok {
					var agents map[string]json.RawMessage
					if err := json.Unmarshal(agentRaw, &agents); err == nil {
						if agentData, ok := agents[name]; ok {
							var agent map[string]json.RawMessage
							if err := json.Unmarshal(agentData, &agent); err == nil {
								if permRaw, ok := agent["permission"]; ok {
									// permission.task may be a nested object (per-subagent
									// map) that does not fit map[string]string. Decode into
									// raw values and keep only scalar-string entries; the
									// task object is surfaced via the task-permissions endpoint.
									var rawPerm map[string]json.RawMessage
									if err := json.Unmarshal(permRaw, &rawPerm); err == nil {
										permission := make(map[string]string, len(rawPerm))
										for k, v := range rawPerm {
											var s string
											if json.Unmarshal(v, &s) == nil {
												permission[k] = s
											}
										}
										writeJSON(w, http.StatusOK, permission)
										return
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Fallback: read from markdown frontmatter
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
var ValidPermissionKeys = map[string]bool{
	// File & code tools
	"read":        true,
	"edit":        true,
	"write":       true,
	"bash":        true,
	"glob":        true,
	"grep":        true,
	"lsp":         true,
	"ast_grep":    true,
	"websearch":   true,
	"code_search": true,
	"webfetch":    true,
	// Task & orchestration (consolidated: task=full todo, delegate=full subagent)
	"task":     true,
	"delegate": true,
	"question": true,
	"skill":    true,
	// Extended categories (plugins, MCP, integrations)
	"memory":   true,
	"intercom": true,
	"ado":      true,
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

	// Kanban MCP tools (from ywai-kanban MCP server)
	"add_activity":          true,
	"create_delegation":     true,
	"create_session":        true,
	"delete_session":        true,
	"get_activities":        true,
	"get_board":             true,
	"get_graph":             true,
	"get_pending_decisions": true,
	"get_ui_url":            true,
	"list_sessions":         true,
	"resolve_activity":      true,
	"update_delegation":     true,
}

// ValidPermissionValues are the only accepted permission values.
var ValidPermissionValues = map[string]bool{
	"allow": true,
	"ask":   true,
	"deny":  true,
}

// triggerLineRegex matches a numbered delegation trigger item, capturing the
// bold name and its description. Accepts ":", " -", or " —" separators.
// Examples matched:
//
//	1. **4-file rule**: if understanding requires reading 4+ files...
//	2. **PR rule** - before commit, push...
var triggerLineRegex = regexp.MustCompile(`^\d+\.\s+\*\*(.+?)\*\*\s*[:\-—]\s*(.+)$`)

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

	// Write to opencode.json (primary source for GetAgentPermissions)
	if path, err := opencodeConfigPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			var config map[string]json.RawMessage
			if err := json.Unmarshal(data, &config); err == nil {
				var agents map[string]json.RawMessage
				if agentRaw, ok := config["agent"]; ok {
					if err := json.Unmarshal(agentRaw, &agents); err == nil {
						if existingRaw, exists := agents[name]; exists {
							var agentCfg map[string]json.RawMessage
							if err := json.Unmarshal(existingRaw, &agentCfg); err == nil {
								// Build the new permission block from the incoming
								// scalar map, but preserve any existing object-valued
								// entries (e.g. permission.task as a per-subagent map)
								// that the flat editor cannot represent and must not clobber.
								merged := make(map[string]json.RawMessage, len(body))
								for k, v := range body {
									vJSON, _ := json.Marshal(v)
									merged[k] = vJSON
								}
								if prevRaw, ok := agentCfg["permission"]; ok {
									var prev map[string]json.RawMessage
									if json.Unmarshal(prevRaw, &prev) == nil {
										for k, v := range prev {
											if _, sent := merged[k]; sent {
												continue
											}
											var s string
											if json.Unmarshal(v, &s) != nil {
												// Non-scalar (object) entry — preserve it.
												merged[k] = v
											}
										}
									}
								}
								permJSON, _ := json.Marshal(merged)
								agentCfg["permission"] = permJSON
								agentJSON, _ := json.Marshal(agentCfg)
								agents[name] = agentJSON
								agentsJSON, _ := json.Marshal(agents)
								config["agent"] = agentsJSON

								pretty, _ := json.MarshalIndent(config, "", "  ")
								_ = os.WriteFile(path+".bak", data, 0644)
								if err := os.WriteFile(path, pretty, 0644); err == nil {
									found = true
								}
							}
						}
					}
				}
			}
		}
	}

	// Also update markdown if it exists (backward compat)
	if mdPath := readAgentMarkdownPath(name); mdPath != "" {
		found = true
		mdContent, err := os.ReadFile(mdPath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		updated := updatePermissionsInFrontmatter(string(mdContent), body)
		_ = os.WriteFile(mdPath+".bak", mdContent, 0644)
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

	path, err := opencodeConfigPath()
	if err == nil {
		if data, err := os.ReadFile(path); err == nil {
			taskRaw := lookupAgentPermissionKey(data, name, "task")
			if len(taskRaw) > 0 {
				// Object form: {"*": "deny", "sub-agent": "allow"}.
				var asMap map[string]string
				if json.Unmarshal(taskRaw, &asMap) == nil {
					result = asMap
				} else {
					// Scalar form: "allow"/"ask"/"deny".
					var asStr string
					if json.Unmarshal(taskRaw, &asStr) == nil && asStr != "" {
						result["*"] = asStr
					}
				}
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

	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "opencode.json not found"})
		return
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var agents map[string]json.RawMessage
	if agentRaw, ok := config["agent"]; !ok || json.Unmarshal(agentRaw, &agents) != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no agents in opencode.json"})
		return
	}
	existingRaw, ok := agents[name]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found in opencode.json"})
		return
	}

	var agentCfg map[string]json.RawMessage
	if err := json.Unmarshal(existingRaw, &agentCfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Merge into the existing permission block so scalar tool permissions are kept.
	var perm map[string]json.RawMessage
	if permRaw, ok := agentCfg["permission"]; ok {
		_ = json.Unmarshal(permRaw, &perm)
	}
	if perm == nil {
		perm = map[string]json.RawMessage{}
	}
	if len(body) == 0 {
		// Empty map clears the delegation restriction.
		delete(perm, "task")
	} else {
		taskJSON, _ := json.Marshal(body)
		perm["task"] = taskJSON
	}
	permJSON, _ := json.Marshal(perm)
	agentCfg["permission"] = permJSON
	agentJSON, _ := json.Marshal(agentCfg)
	agents[name] = agentJSON
	agentsJSON, _ := json.Marshal(agents)
	config["agent"] = agentsJSON

	pretty, _ := json.MarshalIndent(config, "", "  ")
	_ = os.WriteFile(path+".bak", data, 0644)
	if err := os.WriteFile(path, pretty, 0644); err != nil {
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

	found := false

	// Write to opencode.json if the agent is configured there.
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

	// Also update markdown frontmatter (backward compat / source of truth on disk).
	if mdPath := readAgentMarkdownPath(name); mdPath != "" {
		if mdContent, err := os.ReadFile(mdPath); err == nil {
			updated := setScalarFrontmatterField(string(mdContent), "model", model)
			_ = os.WriteFile(mdPath+".bak", mdContent, 0644)
			if err := os.WriteFile(mdPath, []byte(updated), 0644); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			found = true
		}
	}

	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found in opencode.json or markdown files"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"model": model})
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

		// task delegation map -> edges + wildcard attribute.
		taskRaw := lookupAgentPermissionKey(configData, name, "task")
		if len(taskRaw) > 0 {
			var asMap map[string]string
			if json.Unmarshal(taskRaw, &asMap) == nil {
				for target, val := range asMap {
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
			} else {
				// Scalar form: "allow"/"ask"/"deny" applies to all targets.
				var asStr string
				if json.Unmarshal(taskRaw, &asStr) == nil && asStr != "" {
					node.HasWildcard = true
					node.WildcardValue = asStr
				}
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

// lookupAgentPermissionKey returns the raw JSON value of agent.<name>.permission.<key>
// from opencode.json config bytes, or nil if any level is missing.
func lookupAgentPermissionKey(configData []byte, name, key string) json.RawMessage {
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
	permRaw, ok := agent["permission"]
	if !ok {
		return nil
	}
	var perm map[string]json.RawMessage
	if json.Unmarshal(permRaw, &perm) != nil {
		return nil
	}
	return perm[key]
}

// --- Delegation Rules (prompt-body section editor) ---
//
// "Delegation Rules" is a prose section of an orchestrator agent's prompt that
// lives as a markdown table (Action | Inline | Delegate) plus a numbered list
// of "Mandatory Delegation Triggers". It is distinct from the permission.task
// map (which is structured config). These handlers parse/serialize the section
// so the UI can edit it as structured data without the user hand-writing
// markdown tables.

// delegationRule is one row of the "Delegation Rules" table.
type delegationRule struct {
	Action   string `json:"action"`
	Inline   string `json:"inline"`   // "Yes" | "No"
	Delegate string `json:"delegate"` // free text, e.g. "Yes" or "Yes, together with the write"
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
	HasRules bool                `json:"hasRules"` // false when the section is absent
}

const (
	delegationRulesHeader   = "Delegation Rules"
	delegationTriggersHeader = "Mandatory Delegation Triggers"
)

// GET /api/config/agents/{name}/delegation-rules
//
// Extracts the "Delegation Rules" table and the "Mandatory Delegation
// Triggers" list from the agent's prompt body. Returns hasRules=false when the
// section is absent (the UI then offers to seed it).
func (h *Handlers) GetDelegationRules(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	mdPath := readAgentMarkdownPath(name)
	if mdPath == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}
	data, err := os.ReadFile(mdPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	_, body := parseFrontmatter(string(data))
	rulesSection, hasRules := extractMarkdownSection(body, delegationRulesHeader, false)

	resp := delegationRulesResp{Rules: []delegationRule{}, Triggers: []delegationTrigger{}}
	if hasRules {
		resp.HasRules = true
		resp.Rules = parseDelegationRulesTable(rulesSection)

		// Triggers live as a nested sub-heading under Delegation Rules.
		triggersSection, hasTriggers := extractMarkdownSection(body, delegationTriggersHeader, true)
		if hasTriggers {
			resp.Triggers = parseDelegationTriggers(triggersSection)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// PUT /api/config/agents/{name}/delegation-rules
//
// Serializes the rules table + triggers list back into the prompt body,
// replacing the existing section (or appending if absent). Frontmatter is left
// untouched.
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

	mdPath := readAgentMarkdownPath(name)
	if mdPath == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}
	data, err := os.ReadFile(mdPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	fm, mdBody := parseFrontmatter(string(data))

	// Rebuild the Delegation Rules section content (table + optional triggers).
	rulesMarkdown := serializeDelegationRulesTable(body.Rules)
	if len(body.Triggers) > 0 {
		rulesMarkdown += "\n\n#### " + delegationTriggersHeader + "\n\n" + serializeDelegationTriggers(body.Triggers)
	}

	newBody := replaceMarkdownSection(mdBody, delegationRulesHeader, "###", rulesMarkdown, true)

	// Re-join frontmatter (untouched) with the modified body.
	var out string
	if fm == "" {
		out = newBody
	} else {
		out = "---\n" + fm + "\n---\n\n" + newBody
	}

	_ = os.WriteFile(mdPath+".bak", data, 0644)
	if err := os.WriteFile(mdPath, []byte(out), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, delegationRulesResp{Rules: body.Rules, Triggers: body.Triggers, HasRules: true})
}

// parseDelegationRulesTable extracts rows from a markdown table of the form
//
//	| Action | Inline | Delegate |
//	| ------ | ------ | -------- |
//	| Read to decide | Yes | No |
//
// Header and separator rows are skipped.
func parseDelegationRulesTable(section string) []delegationRule {
	var rules []delegationRule
	seenSeparator := false
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
			continue
		}
		inner := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		// Skip the separator row (---, :---, etc.).
		if !seenSeparator {
			if isTableSeparator(inner) {
				seenSeparator = true
			}
			continue // skip header row + separator
		}
		cols := splitTableRow(inner)
		if len(cols) >= 3 {
			rules = append(rules, delegationRule{
				Action:   strings.TrimSpace(cols[0]),
				Inline:   strings.TrimSpace(cols[1]),
				Delegate: strings.TrimSpace(cols[2]),
			})
		}
	}
	return rules
}

// isTableSeparator reports whether a table inner line is the markdown
// alignment row (only dashes, colons, pipes, spaces).
func isTableSeparator(inner string) bool {
	for _, r := range inner {
		if r != '-' && r != ':' && r != '|' && r != ' ' {
			return false
		}
	}
	return strings.Contains(inner, "-")
}

// splitTableRow splits "a | b | c" into ["a"," b"," c"] respecting the pipe as
// a column delimiter (cells may contain escaped pipes "\|").
func splitTableRow(inner string) []string {
	var cols []string
	var cur strings.Builder
	for i := 0; i < len(inner); i++ {
		if i+1 < len(inner) && inner[i] == '\\' && inner[i+1] == '|' {
			cur.WriteByte('|')
			i++
			continue
		}
		if inner[i] == '|' {
			cols = append(cols, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(inner[i])
	}
	cols = append(cols, cur.String())
	return cols
}

// serializeDelegationRulesTable renders the rules as a markdown table.
func serializeDelegationRulesTable(rules []delegationRule) string {
	var b strings.Builder
	b.WriteString("| Action | Inline | Delegate |\n")
	b.WriteString("| ------ | ------ | -------- |\n")
	for _, r := range rules {
		action := strings.ReplaceAll(r.Action, "|", "\\|")
		delegate := strings.ReplaceAll(r.Delegate, "|", "\\|")
		b.WriteString("| ")
		b.WriteString(action)
		b.WriteString(" | ")
		b.WriteString(orDefault(r.Inline, "No"))
		b.WriteString(" | ")
		b.WriteString(orDefault(delegate, "No"))
		b.WriteString(" |\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// parseDelegationTriggers extracts the numbered trigger list, each item being
// "**Name**: description." (bold name, colon, description, until the next
// numbered item or blank).
func parseDelegationTriggers(section string) []delegationTrigger {
	var triggers []delegationTrigger
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		// Match "N. **Name**: description" or "N. **Name** - description".
		m := triggerLineRegex.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		triggers = append(triggers, delegationTrigger{
			Name:        strings.TrimSpace(m[1]),
			Description: strings.TrimSpace(m[2]),
		})
	}
	return triggers
}

// serializeDelegationTriggers renders triggers as a numbered list.
func serializeDelegationTriggers(triggers []delegationTrigger) string {
	var b strings.Builder
	for i, t := range triggers {
		name := strings.TrimSpace(t.Name)
		desc := strings.TrimSpace(t.Description)
		if name == "" && desc == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("%d. **%s**: %s\n", i+1, orDefault(name, "Trigger"), desc))
	}
	return strings.TrimRight(b.String(), "\n")
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
