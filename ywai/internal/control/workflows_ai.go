package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/workflows"
)

// aiEditRequest is the body of POST /api/workflows/{name}/ai-edit.
type aiEditRequest struct {
	Instruction string `json:"instruction"`
	Model       string `json:"model,omitempty"`
	// History carries the recent refinement turns so the AI has conversational
	// context (last few messages). Optional; omit for single-turn edits.
	History []workflows.ConversationMessage `json:"history,omitempty"`
}

// aiQuestionError is returned when the AI asks a clarifying question instead
// of directly editing the workflow. The handler converts this to a {"question":...} JSON response.
type aiQuestionError struct {
	Question string
}
func (e *aiQuestionError) Error() string { return "AI question: " + e.Question }

// handleAIEdit applies a natural-language edit to a workflow via the opencode
// CLI and returns the proposed workflow plus its validation. It does NOT save —
// the frontend applies the result into the editor (undoable) so the user can
// review and Save explicitly.
//
// When History is provided, the prompt includes the last few turns so the AI
// can refine the workflow conversationally (multi-turn Edit-with-AI).
func (a *workflowsAPI) handleAIEdit(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	wf, err := a.store.Load(name)
	if err != nil {
		writeWorkflowsError(w, statusForWorkflowError(err), err)
		return
	}

	var req aiEditRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeWorkflowsError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Instruction) == "" {
		writeWorkflowsError(w, http.StatusBadRequest, errors.New("instruction is required"))
		return
	}

	edited, err := aiEditWorkflow(r.Context(), wf, req.Instruction, req.Model, req.History)
	if err != nil {
		// If the AI asked a question, return it as a conversational response.
		var qErr *aiQuestionError
		if errors.As(err, &qErr) {
			writeJSON(w, http.StatusOK, map[string]any{
				"question": qErr.Question,
			})
			return
		}
		writeWorkflowsError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"workflow":   edited,
		"validation": workflows.Validate(edited),
	})
}

// aiEditWorkflow drives the opencode CLI (mirrors missions.RefineGoalWithOpencode:
// the HTTP session API has known issues processing prompts via REST) to rewrite
// the workflow JSON per the instruction. Identity fields are preserved from the
// source so the AI cannot rename or re-id the workflow.
//
// history carries recent turns (optional) so the AI can refine the workflow
// conversationally; only the last few are passed to stay within prompt limits.
func aiEditWorkflow(ctx context.Context, wf *workflows.Workflow, instruction, model string, history []workflows.ConversationMessage) (*workflows.Workflow, error) {
	opencodePath, err := missions.DetectOpencode()
	if err != nil {
		return nil, fmt.Errorf("opencode is not available: %w", err)
	}

	cur, err := json.MarshalIndent(wf, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal workflow: %w", err)
	}
	prompt := buildAIEditPrompt(string(cur), instruction, history)

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	args := []string{"run"}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, opencodePath, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("opencode run failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	jsonStr := extractJSONObject(string(out))
	if jsonStr == "" {
		return nil, errors.New("AI returned no JSON")
	}

	// Check if the AI asked a clarifying question instead of editing.
	var maybeQ struct{ Question string `json:"question"` }
	if err := json.Unmarshal([]byte(jsonStr), &maybeQ); err == nil && maybeQ.Question != "" {
		return nil, &aiQuestionError{Question: maybeQ.Question}
	}

	var edited workflows.Workflow
	if err := json.Unmarshal([]byte(jsonStr), &edited); err != nil {
		return nil, fmt.Errorf("AI output is not a valid workflow: %w", err)
	}

	// Preserve identity — the edit may change nodes/connections, never the id/name.
	edited.ID = wf.ID
	edited.Name = wf.Name
	if edited.Version == "" {
		edited.Version = wf.Version
	}
	return &edited, nil
}

// buildAIEditPrompt frames the workflow-editing task so opencode returns ONLY a
// JSON workflow object. When history is non-empty, the last few turns are
// included so the AI can refine the workflow conversationally.
func buildAIEditPrompt(currentJSON, instruction string, history []workflows.ConversationMessage) string {
	var b strings.Builder
	b.WriteString("You are a workflow editor. You are given a workflow as JSON and an instruction. ")
	b.WriteString("Apply the instruction and return the COMPLETE updated workflow as a single JSON object.\n\n")
	b.WriteString("Rules:\n")
	b.WriteString("- Output ONLY the JSON object. No prose, no markdown fences, no explanation.\n")
	b.WriteString("- Keep the same top-level \"id\", \"name\" and \"version\".\n")
	b.WriteString("- Node types must be one of: start, end, prompt, subAgent, askUserQuestion, ifElse, switch, branch, skill, mcp, subAgentFlow, codex, group.\n")
	b.WriteString("- Keep exactly one start node and one end node. Every node needs a unique \"id\", a \"type\", a \"name\", a \"position\" {x,y}, and a \"data\" object.\n")
	b.WriteString("- Connections are {\"from\": nodeId, \"to\": nodeId} with optional \"fromPort\"/\"toPort\".\n")
	b.WriteString("- Preserve existing node ids and positions unless the instruction requires changing them.\n\n")
	// Conversational context: include the last few turns (cap to ~6 messages =
	// 3 rounds). This lets the user say "now make the reviewer stricter"
	// without re-explaining the workflow.
	if len(history) > 0 {
		// Trim to the most recent turns and drop any loading/error placeholders.
		recent := history
		if len(recent) > 6 {
			recent = recent[len(recent)-6:]
		}
		b.WriteString("Recent conversation (for context):\n")
		for _, m := range recent {
			if m.IsLoading || m.IsError {
				continue
			}
			label := "User"
			if m.Sender == "ai" {
				label = "Assistant"
			}
			b.WriteString(label + ": " + strings.TrimSpace(m.Content) + "\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("Current workflow:\n")
	b.WriteString(currentJSON)
	b.WriteString("\n\nInstruction:\n")
	b.WriteString(instruction)
	b.WriteString("\n\nReturn the updated workflow JSON now:")
	return b.String()
}

// extractJSONObject pulls the first balanced-looking JSON object out of CLI
// output, tolerating markdown fences and opencode status/ANSI noise around it.
func extractJSONObject(s string) string {
	// Drop ```json ... ``` fences if present.
	if i := strings.Index(s, "```"); i >= 0 {
		s = strings.TrimPrefix(s[i+3:], "json")
		if j := strings.LastIndex(s, "```"); j >= 0 {
			s = s[:j]
		}
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return strings.TrimSpace(s[start : end+1])
}

// decodeJSONBody decodes a JSON request body into v.
func decodeJSONBody(r *http.Request, v any) error {
	defer func() { _ = r.Body.Close() }()
	return json.NewDecoder(r.Body).Decode(v)
}

// skillInfo is one installed Agent Skill the Skill node can reference.
type skillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// handleSkillsList lists the Agent Skills installed under the opencode skills
// dir (one directory per skill, each with a SKILL.md). Returns [] when none.
func (a *workflowsAPI) handleSkillsList(w http.ResponseWriter, r *http.Request) {
	dir := config.OpenCodeSkillsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeJSON(w, http.StatusOK, []skillInfo{})
		return
	}
	skills := []skillInfo{}
	for _, e := range entries {
		// Skip non-dirs and internal/hidden dirs (e.g. _shared, .git).
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info := skillInfo{Name: e.Name()}
		if data, err := os.ReadFile(filepath.Join(dir, e.Name(), "SKILL.md")); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(line, "description:") {
					info.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
					break
				}
			}
		}
		skills = append(skills, info)
	}
	writeJSON(w, http.StatusOK, skills)
}

// mcpServerInfo is one MCP server configured in opencode.json.
type mcpServerInfo struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

// handleMcpServersList lists the MCP servers actually configured in
// opencode.json (the real installed ones, not the static catalog).
func (a *workflowsAPI) handleMcpServersList(w http.ResponseWriter, r *http.Request) {
	cfg, err := readMcpConfig()
	if err != nil {
		writeJSON(w, http.StatusOK, []mcpServerInfo{})
		return
	}
	servers := []mcpServerInfo{}
	for id, raw := range cfg {
		info := mcpServerInfo{ID: id, Enabled: true}
		if m, ok := raw.(map[string]interface{}); ok {
			if v, has := m["enabled"]; has {
				if b, ok := v.(bool); ok {
					info.Enabled = b
				}
			}
		}
		servers = append(servers, info)
	}
	sort.Slice(servers, func(i, j int) bool { return servers[i].ID < servers[j].ID })
	writeJSON(w, http.StatusOK, servers)
}

// mcpToolInfo is one tool exposed by an MCP server (discovered via live handshake).
type mcpToolInfo struct {
	Name string `json:"name"`
}

// handleMcpServerTools discovers the tools a given MCP server exposes by doing
// a live handshake (stdio for local, HTTP for remote). Used by the MCP node
// editor to populate the tool dropdown in manual/aiParameterConfig mode.
//
// Returns an empty list (200) when discovery fails: the editor falls back to a
// free-text input so the user can still type a tool name. This mirrors how
// kanban's tool discovery degrades gracefully.
func (a *workflowsAPI) handleMcpServerTools(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("server")
	if serverID == "" {
		writeJSON(w, http.StatusOK, []mcpToolInfo{})
		return
	}
	cfg, err := readMcpConfig()
	if err != nil {
		writeJSON(w, http.StatusOK, []mcpToolInfo{})
		return
	}
	raw, ok := cfg[serverID]
	if !ok {
		writeJSON(w, http.StatusOK, []mcpToolInfo{})
		return
	}
	server, ok := raw.(map[string]interface{})
	if !ok {
		writeJSON(w, http.StatusOK, []mcpToolInfo{})
		return
	}

	tools := discoverServerTools(r.Context(), server)
	out := make([]mcpToolInfo, 0, len(tools))
	for _, t := range tools {
		out = append(out, mcpToolInfo{Name: t})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, http.StatusOK, out)
}

// discoverServerTools runs a live MCP probe against a server config entry (as
// parsed from opencode.json) and returns the discovered tool names. It tries
// HTTP first when a url is present, then falls back to a stdio spawn. Errors
// yield an empty slice — discovery is best-effort.
func discoverServerTools(ctx context.Context, server map[string]interface{}) []string {
	// Remote server: POST tools/list to its url.
	if urlStr, ok := server["url"].(string); ok && urlStr != "" {
		if tools, err := mcp.DiscoverHTTP(ctx, urlStr); err == nil {
			return tools
		}
	}
	// Local stdio server: spawn command (+args) and handshake.
	command := mcpServerCommand(server)
	if len(command) == 0 {
		return nil
	}
	env := map[string]string{}
	if envRaw, ok := server["env"].(map[string]interface{}); ok {
		for k, v := range envRaw {
			if s, ok := v.(string); ok {
				env[k] = s
			}
		}
	}
	tools, _ := mcp.DiscoverStdio(ctx, command, env)
	return tools
}

// mcpServerCommand extracts the command argv from an opencode.json MCP entry.
// opencode local servers use {command: [argv...]} (array); some entries use the
// {command: "bin", args: [...]} shape instead. Both are handled.
func mcpServerCommand(server map[string]interface{}) []string {
	cmdRaw, ok := server["command"]
	if !ok {
		return nil
	}
	switch v := cmdRaw.(type) {
	case []interface{}:
		command := make([]string, 0, len(v))
		for _, arg := range v {
			if s, ok := arg.(string); ok {
				command = append(command, s)
			}
		}
		return command
	case string:
		command := strings.Fields(v)
		if argsRaw, ok := server["args"].([]interface{}); ok {
			for _, arg := range argsRaw {
				if s, ok := arg.(string); ok {
					command = append(command, s)
				}
			}
		}
		return command
	}
	return nil
}
