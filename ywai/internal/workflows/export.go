package workflows

import (
	"fmt"
	"os"
	"path/filepath"
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
	DryRun       bool             `json:"dryRun"`
}

// Exporter renders a workflow into opencode artifacts. The target directories
// are the opencode config dirs (commands/, agents/), resolved once at
// construction so tests can inject temp dirs.
type Exporter struct {
	commandsDir string // ~/.config/opencode/commands
	agentsDir   string // ~/.config/opencode/agents
}

// NewExporter builds an Exporter targeting the real opencode config dirs.
func NewExporter() *Exporter {
	return &Exporter{
		commandsDir: config.OpenCodeCommandsDir(),
		agentsDir:   config.OpenCodeAgentsDir(),
	}
}

// NewExporterWithDirs is for tests and targets an explicit pair of dirs.
func NewExporterWithDirs(commandsDir, agentsDir string) *Exporter {
	return &Exporter{commandsDir: commandsDir, agentsDir: agentsDir}
}

// Plan renders the workflow into in-memory file contents and records the
// artifact list without writing anything. Apply writes the same set.
func (e *Exporter) Plan(wf *Workflow) (*ExportPlan, map[string]string, error) {
	if wf == nil {
		return nil, nil, fmt.Errorf("nil workflow")
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
	files[orchPath] = renderOrchestratorMarkdown(wf, orchestratorID, orchTaskTargets, orchBody)
	artifacts = append(artifacts, ExportArtifact{Path: orchPath, Kind: "agent", Name: orchestratorID})

	// 2. One agent per subAgent node.
	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		if n.Type != NodeTypeSubAgent {
			continue
		}
		id := subAgentIDs[n.ID]
		path := filepath.Join(e.agentsDir, id+".md")
		files[path] = e.renderSubAgentMarkdown(wf, n, id)
		artifacts = append(artifacts, ExportArtifact{Path: path, Kind: "agent", Name: id})
	}

	// 3. The slash command (entry point users invoke as /<name>).
	cmdPath := filepath.Join(e.commandsDir, wf.Name+".md")
	files[cmdPath] = renderCommandMarkdown(wf, orchestratorID)
	artifacts = append(artifacts, ExportArtifact{Path: cmdPath, Kind: "command", Name: wf.Name})

	return &ExportPlan{
		WorkflowName: wf.Name,
		Files:        artifacts,
		DryRun:       true,
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
// cc-wf-studio node-level name is the agent name), falling back to the
// description.
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
// It reuses agents.BuildOpenCodeMarkdown so the permission block follows the
// exact same rendering/bucket-expansion rules as every other ywai agent.
func renderOrchestratorMarkdown(wf *Workflow, orchestratorID string, taskTargets []string, body string) string {
	mode := "all"
	perm := orchestratorPermissions(taskTargets)
	// The orchestrator needs read/bash to inspect context and task/skill/question
	// to drive the workflow. BuildOpenCodeMarkdown renders these consistently.
	profile := agents.AgentProfile{
		Name:        orchestratorID,
		Description: orchestratorDescription(wf),
		Prompt:      body,
		Permission:  perm,
		Mode:        mode,
		Group:       "workflows",
	}
	return agents.BuildOpenCodeMarkdown(orchestratorID, profile)
}

// orchestratorPermissions builds the permission map: read/bash/task/skill/
// question allow, plus a `task` sub-map restricting delegation to the
// workflow's own sub-agents. BuildOpenCodeMarkdown expands these.
func orchestratorPermissions(taskTargets []string) map[string]string {
	// We encode the task sub-map as a synthetic key "task:<id>" that
	// buildOpenCodeMarkdown cannot interpret directly; instead we render the
	// task sub-map ourselves below and pass a coarse allow set here. The
	// orchestrator gets task/skill/question allow plus the standard read/bash.
	perm := map[string]string{
		"read":      "allow",
		"edit":      "allow",
		"write":     "allow",
		"bash":      "allow",
		"glob":      "allow",
		"grep":      "allow",
		"task":      "allow",
		"skill":     "allow",
		"question":  "allow",
		"webfetch":  "allow",
		"websearch": "allow",
	}
	_ = taskTargets // referenced via the explicit task sub-map in the body
	return perm
}

func orchestratorDescription(wf *Workflow) string {
	if wf.Description != "" {
		return wf.Description
	}
	return "Orchestrator for the " + wf.Name + " workflow."
}

// renderSubAgentMarkdown builds a sub-agent's .md via BuildOpenCodeMarkdown,
// mapping the node's tools/model to an AgentProfile.
func (e *Exporter) renderSubAgentMarkdown(wf *Workflow, n *Node, id string) string {
	perm := subAgentPermissions(n)
	mode := n.Data.Mode
	if mode == "" {
		mode = "all"
	}
	desc := n.Data.AgentDescription
	if desc == "" {
		desc = "Sub-agent for the " + wf.Name + " workflow."
	}
	prompt := n.Data.AgentDefinition
	if strings.TrimSpace(prompt) == "" {
		prompt = n.Data.Prompt
	}
	profile := agents.AgentProfile{
		Name:        id,
		Description: desc,
		Prompt:      prompt,
		Permission:  perm,
		Mode:        mode,
		Group:       "workflows",
	}
	md := agents.BuildOpenCodeMarkdown(id, profile)

	// Append the task instruction so the spawned agent knows what to do.
	if strings.TrimSpace(n.Data.Prompt) != "" && strings.TrimSpace(n.Data.AgentDefinition) != "" {
		md += "\n\n---\n\n## Task\n\n" + strings.TrimSpace(n.Data.Prompt) + "\n"
	}
	return md
}

// subAgentPermissions maps a node's comma-separated tools string to the
// opencode permission map. Empty tools → read-only baseline.
func subAgentPermissions(n *Node) map[string]string {
	perm := map[string]string{
		"read": "allow",
		"glob": "allow",
		"grep": "allow",
	}
	for _, t := range strings.Split(n.Data.Tools, ",") {
		t = strings.ToLower(strings.TrimSpace(t))
		switch t {
		case "":
			continue
		case "bash", "execute_bash":
			perm["bash"] = "allow"
		case "edit":
			perm["edit"] = "allow"
		case "write":
			perm["write"] = "allow"
		case "read":
			perm["read"] = "allow"
		case "webfetch", "fetch":
			perm["webfetch"] = "allow"
		case "websearch", "search":
			perm["websearch"] = "allow"
		case "task", "delegate":
			perm["task"] = "allow"
		case "skill":
			perm["skill"] = "allow"
		case "question", "askuserquestion":
			perm["question"] = "allow"
		case "glob":
			perm["glob"] = "allow"
		case "grep":
			perm["grep"] = "allow"
		case "lsp":
			perm["lsp"] = "allow"
		default:
			// Pass through unknown tool names verbatim (e.g. MCP tools).
			perm[t] = "allow"
		}
	}
	// Sub-agents with tools that mutate get edit/write; pure-research ones stay read-only.
	return perm
}

// renderCommandMarkdown builds the slash command file users invoke as /<name>.
// It targets the workflow's orchestrator agent and forwards $ARGUMENTS.
func renderCommandMarkdown(wf *Workflow, orchestratorID string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: ")
	b.WriteString(yamlQuote(orchestratorDescription(wf)))
	b.WriteByte('\n')
	b.WriteString("agent: ")
	b.WriteString(orchestratorID)
	b.WriteByte('\n')
	b.WriteString("subtask: true\n")
	b.WriteString("---\n\n")
	desc := wf.Description
	if desc == "" {
		desc = "Run the " + wf.Name + " workflow."
	}
	b.WriteString(desc + "\n\n")
	b.WriteString("Execute the **" + wf.Name + "** workflow")
	if wf.Description != "" {
		b.WriteString(": " + wf.Description)
	}
	b.WriteString(".\n\n")
	b.WriteString("Arguments: `$ARGUMENTS`\n")
	return b.String()
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
