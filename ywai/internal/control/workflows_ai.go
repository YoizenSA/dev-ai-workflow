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
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/workflows"
)

// aiEditRequest is the body of POST /api/workflows/{name}/ai-edit.
type aiEditRequest struct {
	Instruction string `json:"instruction"`
	Model       string `json:"model,omitempty"`
}

// handleAIEdit applies a natural-language edit to a workflow via the opencode
// CLI and returns the proposed workflow plus its validation. It does NOT save —
// the frontend applies the result into the editor (undoable) so the user can
// review and Save explicitly.
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

	edited, err := aiEditWorkflow(r.Context(), wf, req.Instruction, req.Model)
	if err != nil {
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
func aiEditWorkflow(ctx context.Context, wf *workflows.Workflow, instruction, model string) (*workflows.Workflow, error) {
	opencodePath, err := missions.DetectOpencode()
	if err != nil {
		return nil, fmt.Errorf("opencode is not available: %w", err)
	}

	cur, err := json.MarshalIndent(wf, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal workflow: %w", err)
	}
	prompt := buildAIEditPrompt(string(cur), instruction)

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
		return nil, errors.New("AI returned no JSON workflow")
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
// JSON workflow object.
func buildAIEditPrompt(currentJSON, instruction string) string {
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
		if !e.IsDir() {
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
// opencode.json (the real installed ones, not the static catalog). Tools are
// not enumerated — that needs a live MCP handshake — so the editor lets the user
// type the tool name.
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
