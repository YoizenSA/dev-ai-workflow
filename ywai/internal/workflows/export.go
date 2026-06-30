package workflows

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// Export artifact produced by Plan/Apply. Files is the list of opencode paths
// that would be (or were) written.
type ExportArtifact struct {
	Path string `json:"path"` // absolute path under ~/.config/opencode
	Kind string `json:"kind"` // "command" | "agent" | "skill"
	Name string `json:"name"` // agent id / command name / skill name
}

// ExportPlan is the result of planning an export. Apply writes exactly these
// files (and only these), so the UI can show a preview before committing.
type ExportPlan struct {
	WorkflowName string           `json:"workflowName"`
	Files        []ExportArtifact `json:"files"`
	// EstimatedTokens is a rough size hint for the exported orchestrator prompt,
	// using the chars/4 heuristic. Useful to flag oversized workflows in the UI
	// before exporting.
	EstimatedTokens int           `json:"estimatedTokens"`
	DryRun          bool          `json:"dryRun"`
}

// Export targets. opencode renders agents with opencode permission blocks under
// ~/.config/opencode; claude-code renders Claude-native frontmatter under
// ~/.claude. Both share the orchestrator + sub-agents + slash-command structure.
const (
	TargetOpenCode   = "opencode"
	TargetClaudeCode = "claude-code"
)

// Exporter renders a workflow into a target's artifacts. The target directories
// (commands/, agents/) are resolved once at construction so tests can inject
// temp dirs.
type Exporter struct {
	commandsDir string
	agentsDir   string
	target      string
}

// NewExporter builds an Exporter targeting the real opencode config dirs.
func NewExporter() *Exporter {
	return &Exporter{
		commandsDir: config.OpenCodeCommandsDir(),
		agentsDir:   config.OpenCodeAgentsDir(),
		target:      TargetOpenCode,
	}
}

// NewExporterForTarget builds an Exporter for the given target, pointing at that
// runtime's real config dirs. Unknown targets fall back to opencode.
func NewExporterForTarget(target string) *Exporter {
	switch target {
	case TargetClaudeCode:
		return &Exporter{
			commandsDir: config.ClaudeCommandsDir(),
			agentsDir:   config.ClaudeAgentsDir(),
			target:      TargetClaudeCode,
		}
	default:
		return NewExporter()
	}
}

// NewExporterWithDirs is for tests and targets an explicit pair of dirs (opencode).
func NewExporterWithDirs(commandsDir, agentsDir string) *Exporter {
	return &Exporter{commandsDir: commandsDir, agentsDir: agentsDir, target: TargetOpenCode}
}

// NewExporterWithDirsForTarget is for tests: explicit dirs + target dialect.
func NewExporterWithDirsForTarget(commandsDir, agentsDir, target string) *Exporter {
	return &Exporter{commandsDir: commandsDir, agentsDir: agentsDir, target: target}
}

// Plan renders the workflow into in-memory file contents and records the
// artifact list without writing anything. Apply writes the same set.
func (e *Exporter) Plan(wf *Workflow) (*ExportPlan, map[string]string, error) {
	if wf == nil {
		return nil, nil, fmt.Errorf("nil workflow")
	}

	// Drop duplicate connections so the exported Mermaid + steps stay clean even
	// when the source workflow carries them (e.g. a branch with two outcomes
	// routed to the same target). Works on a shallow copy to avoid mutating the
	// caller's workflow.
	wf = &Workflow{
		ID:                  wf.ID,
		Name:                wf.Name,
		Description:         wf.Description,
		Version:             wf.Version,
		Nodes:               wf.Nodes,
		Connections:         wf.dedupConnections(),
		SlashCommandOptions: wf.SlashCommandOptions,
		ConversationHistory: wf.ConversationHistory,
		CreatedAt:           wf.CreatedAt,
		UpdatedAt:           wf.UpdatedAt,
	}

	// Build the agent id for each subAgent node. The slug is the workflow name
	// plus a per-node suffix derived from the node's Name/description, so a
	// workflow never collides with the user's own agents and its sub-agents
	// don't collide with each other.
	subAgentIDs := make(map[string]string, len(wf.Nodes))
	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		if n.Type != NodeTypeSubAgent {
			continue
		}
		subAgentIDs[n.ID] = subAgentSlug(wf.Name, n)
	}
	orchestratorID := wf.Name + "-orchestrator"

	// The orchestrator may delegate to every subAgent node (via the native
	// `task` tool), so its permission.task whitelist is the set of sub-agent ids.
	orchTaskTargets := make([]string, 0, len(subAgentIDs))
	for _, id := range subAgentIDs {
		orchTaskTargets = append(orchTaskTargets, id)
	}

	files := make(map[string]string) // path → content
	var artifacts []ExportArtifact

	// 1. Orchestrator agent.
	orchPath := filepath.Join(e.agentsDir, orchestratorID+".md")
	orchBody := orchestratorBody(wf, subAgentIDs)
	files[orchPath] = e.renderOrchestratorMarkdown(wf, orchestratorID, orchTaskTargets, orchBody)
	artifacts = append(artifacts, ExportArtifact{Path: orchPath, Kind: "agent", Name: orchestratorID})

	// 2. One agent per subAgent node.
	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		if n.Type != NodeTypeSubAgent {
			continue
		}
		id := subAgentIDs[n.ID]
		path := filepath.Join(e.agentsDir, id+".md")
		files[path] = e.renderSubAgentMarkdown(wf, n, id, subAgentIDs)
		artifacts = append(artifacts, ExportArtifact{Path: path, Kind: "agent", Name: id})
	}

	// 3. The slash command (entry point users invoke as /<name>).
	cmdPath := filepath.Join(e.commandsDir, wf.Name+".md")
	files[cmdPath] = e.renderCommandMarkdown(wf, orchestratorID)
	artifacts = append(artifacts, ExportArtifact{Path: cmdPath, Kind: "command", Name: wf.Name})

	// Token estimate for the orchestrator prompt body (chars/4 heuristic). Used
	// by the UI to flag oversized workflows; not authoritative.
	estimatedTokens := estimateTokens(orchBody)

	return &ExportPlan{
		WorkflowName:    wf.Name,
		Files:           artifacts,
		EstimatedTokens: estimatedTokens,
		DryRun:          true,
	}, files, nil
}

// Apply writes the planned artifacts to disk, creating the target dirs.
func (e *Exporter) Apply(wf *Workflow) (*ExportPlan, error) {
	plan, files, err := e.Plan(wf)
	if err != nil {
		return nil, err
	}
	// Ensure the opencode commands/ and agents/ dirs exist before writing.
	for _, dir := range []string{e.commandsDir, e.agentsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	for path, content := range files {
		if err := atomicWrite(path, []byte(content)); err != nil {
			return nil, fmt.Errorf("write %s: %w", path, err)
		}
	}
	plan.DryRun = false
	return plan, nil
}

// ─── slug & model helpers ──────────────────────────────────────────────────

// subAgentSlug derives a unique, filesystem-safe agent id for a subAgent node.
// Format: <workflow>-<suffix> where suffix comes from the node's Name (the
// node-level Name is the agent name), falling back to the description.
func subAgentSlug(workflow string, n *Node) string {
	base := strings.ToLower(strings.TrimSpace(n.Name))
	if base == "" {
		base = strings.ToLower(strings.TrimSpace(n.Data.Name))
	}
	if base == "" {
		base = strings.ToLower(strings.TrimSpace(n.Data.AgentDescription))
	}
	base = sanitizeSlug(base)
	if base == "" {
		base = "agent"
	}
	return workflow + "-" + base
}

// sanitizeSlug keeps only [a-z0-9-_] and collapses runs. Any character outside
// that set (spaces, punctuation, slashes) acts as a separator so multi-word
// names stay readable ("News Briefing Agent" → "news-briefing-agent",
// "Weird/Chars!" → "weird-chars").
func sanitizeSlug(s string) string {
	var b strings.Builder
	prevDash := false
	emitDash := func() {
		if !prevDash && b.Len() > 0 {
			b.WriteRune('-')
			prevDash = true
		}
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			prevDash = false
		case r == '-' || r == '_' || r == ' ':
			emitDash()
		default:
			// Any other char (punctuation, slash, etc.) acts as a separator.
			emitDash()
		}
	}
	return strings.Trim(b.String(), "-")
}

// ─── markdown rendering ────────────────────────────────────────────────────

// renderOrchestratorMarkdown builds the orchestrator agent frontmatter + body.
// The orchestrator's tools come from the START node's `tools` field (CSV). When
// empty, coordinator defaults are used (read/glob/grep/task/skill/question/
// delegate). The permission.task sub-map restricts delegation to this
// workflow's sub-agents only (one allow per subAgent node), matching the
// preconfigured orchestrator's model.
func (e *Exporter) renderOrchestratorMarkdown(wf *Workflow, orchestratorID string, taskTargets []string, body string) string {
	orchTools := ""
	if s := wf.findNode(NodeTypeStart); s != nil {
		orchTools = s.Data.Tools
	}
	perm := toolsToPermissions(orchTools, defaultOrchestratorTools)
	if e.target == TargetClaudeCode {
		claudeTools := csvFromPermissions(perm)
		return renderClaudeAgentMarkdown(orchestratorID, orchestratorDescription(wf), claudeTools, orchestratorModel(wf), body)
	}
	profile := agents.AgentProfile{
		Name:        orchestratorID,
		Description: orchestratorDescription(wf),
		Prompt:      body,
		Permission:  perm,
		Mode:        "all",
		Group:       wf.Name,
	}
	md := agents.BuildOpenCodeMarkdown(orchestratorID, profile)
	md = injectModel(md, orchestratorModel(wf))

	// Delegation: orchestrator may delegate to every subAgent in the workflow.
	taskMap := map[string]string{"*": "deny"}
	for _, id := range taskTargets {
		taskMap[id] = "allow"
	}
	if injected, ok := agents.InjectTaskPermission(md, taskMap); ok {
		md = injected
	}
	return md
}

func orchestratorDescription(wf *Workflow) string {
	if s := wf.findNode(NodeTypeStart); s != nil && strings.TrimSpace(s.Data.AgentDescription) != "" {
		return s.Data.AgentDescription
	}
	if wf.Description != "" {
		return wf.Description
	}
	return "Orchestrator for the " + wf.Name + " workflow."
}

func orchestratorModel(wf *Workflow) string {
	if s := wf.findNode(NodeTypeStart); s != nil {
		if m := strings.TrimSpace(s.Data.Model); m != "" && m != "inherit" {
			return m
		}
	}
	return ""
}

// renderSubAgentMarkdown builds a sub-agent's .md. Its tools come from the
// node's `tools` field; when empty, dev defaults apply (read/edit/write/bash/
// glob/grep/skill). Delegation is derived from outgoing edges: a subAgent may
// delegate to any subAgent node it has a direct connection to.
func (e *Exporter) renderSubAgentMarkdown(wf *Workflow, n *Node, id string, subAgentIDs map[string]string) string {
	perm := toolsToPermissions(n.Data.Tools, defaultSubAgentTools)
	// Sub-agents default to "subagent" mode: they don't appear in the agent
	// selector (only invocable via `task` by the orchestrator). The user can
	// override with "all" if they want the agent selectable.
	mode := n.Data.Mode
	if mode == "" {
		mode = "subagent"
	}
	desc := n.Data.AgentDescription
	if desc == "" {
		desc = "Sub-agent for the " + wf.Name + " workflow."
	}
	prompt := n.Data.AgentDefinition
	if strings.TrimSpace(prompt) == "" {
		prompt = n.Data.Prompt
	}
	if e.target == TargetClaudeCode {
		claudeTools := csvFromPermissions(perm)
		md := renderClaudeAgentMarkdown(id, desc, claudeTools, n.Data.Model, prompt)
		if strings.TrimSpace(n.Data.Prompt) != "" && strings.TrimSpace(n.Data.AgentDefinition) != "" {
			md += "\n\n---\n\n## Task\n\n" + strings.TrimSpace(n.Data.Prompt) + "\n"
		}
		return md
	}
	profile := agents.AgentProfile{
		Name:        id,
		Description: desc,
		Prompt:      prompt,
		Permission:  perm,
		Mode:        mode,
		Group:       wf.Name,
	}
	md := agents.BuildOpenCodeMarkdown(id, profile)
	md = injectModel(md, n.Data.Model)

	// Delegation: derived from the sub-agent's outgoing edges to other sub-agents.
	taskMap := delegationMapFromOutgoing(wf, n.ID, subAgentIDs)
	if injected, ok := agents.InjectTaskPermission(md, taskMap); ok {
		md = injected
	}

	if strings.TrimSpace(n.Data.Prompt) != "" && strings.TrimSpace(n.Data.AgentDefinition) != "" {
		md += "\n\n---\n\n## Task\n\n" + strings.TrimSpace(n.Data.Prompt) + "\n"
	}
	return md
}

// ─── tools → permissions ───────────────────────────────────────────────────

// defaultOrchestratorTools is applied when the START node has no tools set.
// Coordinator-only: read + ask + skill + context (mcp bucket). No edit/write/
// bash — the orchestrator never touches code directly. (delegate/delegation_*
// are added by BuildOpenCodeMarkdown's AlwaysAllowed path when needed.)
const defaultOrchestratorTools = "read,glob,grep,task,skill,question,mcp"

// defaultSubAgentTools is applied when a subAgent node has no tools set.
// Full implementer: read + write + edit + bash + skill + context (mcp bucket).
const defaultSubAgentTools = "read,edit,write,bash,glob,grep,skill,task,mcp"

// toolsToPermissions converts a comma-separated tools string into a permission
// map suitable for BuildOpenCodeMarkdown. If csv is empty, defaults are used.
// Each entry may carry a ":deny" suffix to block a tool (e.g. "bash:deny").
// Coarse buckets (mcp, memory, delegate, codegraph, context7, ywai-kanban) are
// expanded to opencode-native wildcards via ExpandPermissionBuckets so they
// actually gate the underlying tools.
func toolsToPermissions(csv, defaults string) map[string]string {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		csv = defaults
	}
	perm := map[string]string{}
	for _, entry := range strings.Split(csv, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		val := "allow"
		// ":deny" suffix → this tool is explicitly blocked.
		if idx := strings.Index(entry, ":deny"); idx >= 0 {
			entry = strings.TrimSpace(entry[:idx])
			val = "deny"
		}
		if entry == "" {
			continue
		}
		perm[entry] = val
	}
	return agents.ExpandPermissionBuckets(perm)
}

// csvFromPermissions renders a permission map as a comma-separated string of
// allowed tool names (for the claude-code `tools:` frontmatter key).
func csvFromPermissions(perm map[string]string) string {
	var allowed []string
	for k, v := range perm {
		if v == "allow" {
			allowed = append(allowed, k)
		}
	}
	sort.Strings(allowed)
	return strings.Join(allowed, ",")
}

// delegationMapFromOutgoing builds the permission.task map for a subAgent node.
// Delegation targets come from TWO sources:
//  1. Outgoing edges to other subAgent nodes in the graph (the visible flow).
//  2. The node's `delegateTo` field — explicit agent ids (comma-separated) this
//     agent may delegate to, even without a visible edge. Use this for utility
//     agents like "finder" that several sub-agents need but aren't part of the
//     main execution flow.
//
// delegateTo entries resolve against subAgentIDs first (so "finder" → the
// workflow's finder slug), then pass through verbatim (so external agent names
// like "memory" or "ask" also work). Everything not listed is denied.
func delegationMapFromOutgoing(wf *Workflow, nodeID string, subAgentIDs map[string]string) map[string]string {
	taskMap := map[string]string{"*": "deny"}

	// 1. Outgoing edges to subAgents.
	for _, c := range wf.Connections {
		if c.From != nodeID {
			continue
		}
		if slug, ok := subAgentIDs[c.To]; ok {
			taskMap[slug] = "allow"
		}
	}

	// 2. Explicit delegateTo field (utility agents without visible edges).
	byID := wf.nodeByID()
	if n, ok := byID[nodeID]; ok {
		for _, entry := range strings.Split(n.Data.DelegateTo, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			// Resolve to the workflow-scoped slug if it's a subAgent node id.
			if slug, ok := subAgentIDs[entry]; ok {
				taskMap[slug] = "allow"
			} else {
				// External agent name (e.g. "finder", "memory", "ask") — pass
				// through verbatim.
				taskMap[entry] = "allow"
			}
		}
	}

	return taskMap
}

// injectModel writes a `model:` key into the agent markdown frontmatter after
// `description:`. No-op when model is empty or already present.
func injectModel(md, model string) string {
	model = strings.TrimSpace(model)
	if model == "" || model == "inherit" {
		return md
	}
	lines := strings.Split(md, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return md
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "model:") {
			return md // already present
		}
	}
	var out []string
	inserted := false
	for i, line := range lines {
		if !inserted && i > 0 && strings.HasPrefix(strings.TrimSpace(line), "description:") {
			out = append(out, line)
			out = append(out, "model: "+model)
			inserted = true
			continue
		}
		out = append(out, line)
	}
	if !inserted {
		return md
	}
	return strings.Join(out, "\n")
}

// renderClaudeAgentMarkdown builds a Claude Code agent .md: native frontmatter
// (name/description/tools/model) followed by the system prompt body. Unlike the
// opencode renderer there is no permission block — Claude scopes tools via the
// comma-separated `tools` frontmatter key.
func renderClaudeAgentMarkdown(id, description, tools, model, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: ")
	b.WriteString(id)
	b.WriteByte('\n')
	b.WriteString("description: ")
	b.WriteString(yamlQuote(description))
	b.WriteByte('\n')
	if t := strings.TrimSpace(tools); t != "" {
		b.WriteString("tools: ")
		b.WriteString(t)
		b.WriteByte('\n')
	}
	if m := strings.TrimSpace(model); m != "" && m != "inherit" {
		b.WriteString("model: ")
		b.WriteString(m)
		b.WriteByte('\n')
	}
	b.WriteString("---\n\n")
	b.WriteString(strings.TrimSpace(body))
	b.WriteByte('\n')
	return b.String()
}

// renderCommandMarkdown builds the slash command file users invoke as /<name>.
// It targets the workflow's orchestrator agent and forwards $ARGUMENTS. When
// the workflow carries SlashCommandOptions, the advanced frontmatter fields
// (allowed-tools, model, context, disable-model-invocation, argument-hint,
// hooks) are emitted.
func (e *Exporter) renderCommandMarkdown(wf *Workflow, orchestratorID string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: ")
	b.WriteString(yamlQuote(orchestratorDescription(wf)))
	b.WriteByte('\n')
	// Advanced slash-command options (optional). Only emitted when set so the
	// default output stays unchanged for workflows that don't use them.
	if opt := wf.SlashCommandOptions; opt != nil {
		if v := strings.TrimSpace(opt.AllowedTools); v != "" {
			b.WriteString("allowed-tools: ")
			b.WriteString(v)
			b.WriteByte('\n')
		}
		// model only when set and not "default" (default is implicit).
		if m := strings.TrimSpace(opt.Model); m != "" && m != "default" {
			b.WriteString("model: ")
			b.WriteString(m)
			b.WriteByte('\n')
		}
		if c := strings.TrimSpace(opt.Context); c != "" && c != "default" {
			b.WriteString("context: ")
			b.WriteString(c)
			b.WriteByte('\n')
		}
		if opt.DisableModelInvocation {
			b.WriteString("disable-model-invocation: true\n")
		}
		if ah := strings.TrimSpace(opt.ArgumentHint); ah != "" {
			b.WriteString("argument-hint: ")
			b.WriteString(yamlQuote(ah))
			b.WriteByte('\n')
		}
		if opt.Hooks != nil {
			renderHooksFrontmatter(&b, opt.Hooks)
		}
	}
	// Claude Code slash commands have no `agent:`/`subtask:` keys; the body just
	// drives the conversation. opencode binds the command to its orchestrator agent.
	if e.target != TargetClaudeCode {
		b.WriteString("agent: ")
		b.WriteString(orchestratorID)
		b.WriteByte('\n')
		b.WriteString("subtask: true\n")
	}
	b.WriteString("---\n\n")
	b.WriteString("Execute the **" + wf.Name + "** workflow")
	if wf.Description != "" {
		b.WriteString(": " + wf.Description)
	} else {
		b.WriteString(".")
	}
	b.WriteString("\n\nArguments: `$ARGUMENTS`\n")
	return b.String()
}

// renderHooksFrontmatter writes the `hooks:` YAML block for a slash command.
// Mirrors Claude Code's hook frontmatter shape: PreToolUse/PostToolUse/Stop,
// each a list of {matcher?, hooks: [{type, command, once?}]}.
func renderHooksFrontmatter(b *strings.Builder, h *WorkflowHooks) {
	b.WriteString("hooks:\n")
	renderHookBucket(b, "PreToolUse", h.PreToolUse)
	renderHookBucket(b, "PostToolUse", h.PostToolUse)
	renderHookBucket(b, "Stop", h.Stop)
}

func renderHookBucket(b *strings.Builder, name string, entries []HookEntry) {
	if len(entries) == 0 {
		return
	}
	b.WriteString("  ")
	b.WriteString(name)
	b.WriteString(":\n")
	for _, e := range entries {
		b.WriteString("    - matcher: ")
		if m := strings.TrimSpace(e.Matcher); m != "" {
			b.WriteString(yamlQuote(m))
		} else {
			b.WriteString(`""`)
		}
		b.WriteByte('\n')
		b.WriteString("      hooks:\n")
		for _, a := range e.Hooks {
			b.WriteString("        - type: ")
			b.WriteString(a.Type)
			b.WriteByte('\n')
			if c := strings.TrimSpace(a.Command); c != "" {
				b.WriteString("          command: ")
				b.WriteString(yamlQuote(c))
				b.WriteByte('\n')
			}
			if a.Once {
				b.WriteString("          once: true\n")
			}
		}
	}
}

// estimateTokens is a coarse prompt-size heuristic: 1 token ≈ 4 characters.
// It exists to surface oversized exports in the UI, not to be authoritative.
func estimateTokens(s string) int {
	return (len(s) + 3) / 4 // ceil(chars/4) without float math
}

// yamlQuote quotes a string for a YAML scalar value when needed.
func yamlQuote(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return `""`
	}
	// Quote if it contains characters that would confuse YAML scalars.
	needsQuote := strings.ContainsAny(s, ":#{}[]&*!|>'\"%@`") || strings.Contains(s, "\n")
	if needsQuote {
		// Escape backslashes and double quotes, wrap in double quotes.
		escaped := strings.ReplaceAll(s, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return s
}
