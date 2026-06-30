package control

import (
	"net/http"
	"sort"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/workflows"
)

// mcpTargetClaudeCode is the mcp.WriteAgentConfig target string for Claude
// Code (~/.claude.json, "mcpServers" key). Defined locally to avoid an import
// cycle with internal/workflows (which owns the export-target constants).
const mcpTargetClaudeCode = "claude-code"

// mcpSyncPreview is the result of previewing an opencode→claude-code MCP sync.
type mcpSyncPreview struct {
	ToSync   []string `json:"toSync"`   // servers present in opencode.json, missing in claude.json
	Existing []string `json:"existing"` // already present in both (will not be re-added)
	Missing  []string `json:"missing"`  // referenced by workflow nodes but not in opencode.json
}

// mcpSyncResult is the outcome of applying a sync.
type mcpSyncResult struct {
	Added   []string `json:"added"`
	Skipped []string `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

// handleMcpSyncPreview computes what an opencode→claude-code MCP sync would do
// for the MCP servers referenced by this workflow's nodes. Append-only: servers
// already present in claude.json are listed but not overwritten.
func (a *workflowsAPI) handleMcpSyncPreview(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	wf, err := a.store.Load(name)
	if err != nil {
		writeWorkflowsError(w, statusForWorkflowError(err), err)
		return
	}

	referenced := mcpServerIDs(wf)
	opencode, _ := readMcpConfig()
	claude, _ := mcp.ReadAgentConfig(mcpTargetClaudeCode)

	preview := mcpSyncPreview{
		ToSync:   []string{},
		Existing: []string{},
		Missing:  []string{},
	}
	for _, id := range referenced {
		if _, ok := opencode[id]; !ok {
			preview.Missing = append(preview.Missing, id)
			continue
		}
		if _, ok := claude[id]; ok {
			preview.Existing = append(preview.Existing, id)
		} else {
			preview.ToSync = append(preview.ToSync, id)
		}
	}
	sort.Strings(preview.ToSync)
	sort.Strings(preview.Existing)
	sort.Strings(preview.Missing)
	writeJSON(w, http.StatusOK, preview)
}

// handleMcpSyncApply replicates the workflow's referenced MCP servers from
// opencode.json (source of truth) into claude.json, converting each entry's
// shape to Claude's {command, args} form. Append-only: existing entries are
// skipped (never overwritten).
func (a *workflowsAPI) handleMcpSyncApply(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	wf, err := a.store.Load(name)
	if err != nil {
		writeWorkflowsError(w, statusForWorkflowError(err), err)
		return
	}

	referenced := mcpServerIDs(wf)
	opencode, _ := readMcpConfig()
	claude, _ := mcp.ReadAgentConfig(mcpTargetClaudeCode)

	res := mcpSyncResult{
		Added:   []string{},
		Skipped: []string{},
		Errors:  []string{},
	}
	for _, id := range referenced {
		src, ok := opencode[id]
		if !ok {
			// Missing from source; nothing to sync, not an error.
			continue
		}
		if _, exists := claude[id]; exists {
			res.Skipped = append(res.Skipped, id)
			continue
		}
		shape := opencodeShapeToClaude(src)
		if _, err := mcp.WriteAgentConfig(mcpTargetClaudeCode, id, shape); err != nil {
			res.Errors = append(res.Errors, id+": "+err.Error())
			continue
		}
		res.Added = append(res.Added, id)
	}
	sort.Strings(res.Added)
	sort.Strings(res.Skipped)
	if len(res.Errors) == 0 {
		res.Errors = nil
	}
	writeJSON(w, http.StatusOK, res)
}

// mcpServerIDs returns the distinct, non-empty server ids referenced by the
// workflow's MCP nodes.
func mcpServerIDs(wf *workflows.Workflow) []string {
	seen := map[string]bool{}
	var ids []string
	for _, n := range wf.Nodes {
		if n.Type != workflows.NodeTypeMCP {
			continue
		}
		id := strings.TrimSpace(n.Data.Server)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

// opencodeShapeToClaude converts an opencode.json MCP entry into the shape
// claude-code's ~/.claude.json expects under "mcpServers":
//   - opencode local:  {type:"local", command:[argv...], env, enabled}
//   - claude-code:     {command: argv[0], args: argv[1:], env, enabled} (no type)
//   - opencode remote: {type:"remote", url, enabled}
//   - claude-code:     {type:"http", url} (claude uses "http" for remote)
//
// env/enabled are preserved when present. Unknown fields pass through.
func opencodeShapeToClaude(src any) map[string]any {
	out := map[string]any{}
	server, ok := src.(map[string]any)
	if !ok {
		return out
	}
	// Remote server: claude uses type "http" + url (no command).
	if url, has := server["url"].(string); has && url != "" {
		out["type"] = "http"
		out["url"] = url
		if env, ok := server["env"].(map[string]any); ok {
			out["env"] = env
		}
		return out
	}
	// Local server: opencode stores command as an array; claude wants
	// {command: bin, args: [...]}. Also tolerate the {command:string, args:[...]}
	// shape some entries use.
	switch cmd := server["command"].(type) {
	case []any:
		if len(cmd) > 0 {
			out["command"] = cmd[0]
			if len(cmd) > 1 {
				out["args"] = cmd[1:]
			}
		}
	case string:
		out["command"] = cmd
		if args, ok := server["args"].([]any); ok {
			out["args"] = args
		}
	}
	if env, ok := server["env"].(map[string]any); ok {
		out["env"] = env
	}
	return out
}
