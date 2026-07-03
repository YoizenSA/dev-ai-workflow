package kanban

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
	_ = os.WriteFile(mdPath+".bak", mdContent, 0644)
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

// applyAgentModel writes an agent's model into opencode.json (agent.<name>.model)
// and its markdown frontmatter (the source of truth opencode enforces). An empty
// model clears the override. Returns true if the agent was found in either
// location. Shared by PutAgentModel and orchestrator-profile activation.
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

	if mdPath := readAgentMarkdownPath(name); mdPath != "" {
		if mdContent, err := os.ReadFile(mdPath); err == nil {
			updated := setScalarFrontmatterField(string(mdContent), "model", model)
			_ = os.WriteFile(mdPath+".bak", mdContent, 0644)
			if os.WriteFile(mdPath, []byte(updated), 0644) == nil {
				found = true
			}
		}
	}

	return found
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
		taskRaw := lookupAgentPermissionKey(configData, name, "task")
		if len(taskRaw) > 0 {
			if json.Unmarshal(taskRaw, &taskMap) != nil {
				taskMap = nil
				// Scalar form: "allow"/"ask"/"deny" applies to all targets.
				var asStr string
				if json.Unmarshal(taskRaw, &asStr) == nil && asStr != "" {
					node.HasWildcard = true
					node.WildcardValue = asStr
				}
			}
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
// mirrors agents.DelegationsDoc but lives in the kanban package to avoid an
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
			updated := replaceKanbanMarkdownSection(string(mdContent), "Delegation Rules", "###", rendered, true)
			if updated != string(mdContent) {
				_ = os.WriteFile(mdPath+".bak", mdContent, 0o644)
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
		b.WriteString("\nUse OpenCode's native `task` tool for delegated work.\n")
	}

	if len(triggers) > 0 {
		b.WriteString("\n#### Mandatory Delegation Triggers\n\n")
		b.WriteString("These gates are **non-skippable hard gates**, not recommendations.\n\n")
		b.WriteString("Semantic guard: **delegate** means using OpenCode's native `task` tool to invoke a configured sub-agent. Running local scripts, Python, or Bash inline is execution, not delegation.\n\n")
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

// replaceKanbanMarkdownSection replaces the body content under a heading. Local
// copy (the kanban package already has extractMarkdownSection/replaceMarkdownSection
// in frontmatter.go; this is that same helper, kept here to avoid duplication
// confusion — it delegates to the frontmatter.go implementation).
func replaceKanbanMarkdownSection(content, headerText, headingPrefix, newContent string, includeSubsections bool) string {
	return replaceMarkdownSection(content, headerText, headingPrefix, newContent, includeSubsections)
}
