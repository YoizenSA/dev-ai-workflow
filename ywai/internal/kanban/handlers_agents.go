package kanban

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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
									var permission map[string]string
									if err := json.Unmarshal(permRaw, &permission); err == nil {
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
								permJSON, _ := json.Marshal(body)
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
