package workflows

import (
	"fmt"
	"regexp"
	"strings"
)

// Limits mirror the validation rules in workflow-definition.ts.
const (
	maxNodes            = 100
	descriptionMax      = 200   // subAgent description (frontmatter)
	agentDefinitionMax  = 10000 // subAgent system prompt
	promptMax           = 10000 // subAgent task prompt
	askOptionsMin       = 2
	askOptionsMax       = 4
	switchMaxBranches   = 10
	skillDescriptionMax = 1024
	argumentHintMax     = 200 // slashCommandOptions.argumentHint
)

var versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
var skillNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// ValidationIssue is a single finding. Severity distinguishes hard errors
// (block export) from warnings (best-effort, e.g. unreachable nodes).
type ValidationIssue struct {
	Severity string `json:"severity"` // "error" | "warning"
	NodeID   string `json:"nodeId,omitempty"`
	Message  string `json:"message"`
}

// ValidationResult is the structured report returned to the UI.
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationIssue `json:"errors"`
	Warnings []ValidationIssue `json:"warnings"`
}

// Validate checks a workflow against the structural rules. It returns a result
// even when there are errors (callers inspect Valid before exporting).
func Validate(wf *Workflow) ValidationResult {
	// Pre-initialize the slices as empty (not nil) so they JSON-serialize as
	// `[]` rather than `null`. The frontend maps over these unconditionally.
	res := ValidationResult{
		Errors:   []ValidationIssue{},
		Warnings: []ValidationIssue{},
	}
	if wf == nil {
		res.Errors = append(res.Errors, ValidationIssue{Severity: "error", Message: "workflow is nil"})
		res.Valid = false
		return res
	}

	addErr := func(nodeID, msg string) {
		res.Errors = append(res.Errors, ValidationIssue{Severity: "error", NodeID: nodeID, Message: msg})
	}
	addWarn := func(nodeID, msg string) {
		res.Warnings = append(res.Warnings, ValidationIssue{Severity: "warning", NodeID: nodeID, Message: msg})
	}

	// Top-level fields.
	if err := ValidateName(wf.Name); err != nil {
		addErr("", fmt.Sprintf("name: %v", err))
	}
	if wf.Version == "" {
		addErr("", "version is required (e.g. \"1.0.0\")")
	} else if !versionPattern.MatchString(wf.Version) {
		addErr("", "version must be semver-like (e.g. \"1.0.0\")")
	}
	if len(wf.Nodes) > maxNodes {
		addErr("", fmt.Sprintf("too many nodes: %d (max %d)", len(wf.Nodes), maxNodes))
	}

	// Top-level slash-command options (optional).
	if wf.SlashCommandOptions != nil {
		validateSlashCommandOptions(wf.SlashCommandOptions, addErr)
	}

	// Exactly one start and one end.
	counts := wf.countByType()
	if counts[NodeTypeStart] == 0 {
		addErr("", "workflow must have a start node")
	}
	if counts[NodeTypeStart] > 1 {
		addErr("", fmt.Sprintf("workflow must have exactly one start node (found %d)", counts[NodeTypeStart]))
	}
	if counts[NodeTypeEnd] == 0 {
		addErr("", "workflow must have an end node")
	}
	if counts[NodeTypeEnd] > 1 {
		addErr("", fmt.Sprintf("workflow must have exactly one end node (found %d)", counts[NodeTypeEnd]))
	}

	// Uniqueness + per-type rules.
	seenIDs := make(map[string]bool, len(wf.Nodes))
	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		if n.ID == "" {
			addErr("", fmt.Sprintf("node %d has no id", i))
			continue
		}
		if seenIDs[n.ID] {
			addErr(n.ID, "duplicate node id")
		}
		seenIDs[n.ID] = true

		validateNode(n, addErr, addWarn)
	}

	// Graph-level: cycles and reachability.
	// A cycle is a warning, not an error — a review→fix loop (e.g. gate false
	// → dev → qa → reviewer → gate) is a legitimate workflow pattern. The
	// orchestrator prompt handles it via its routing instructions.
	if wf.hasCycle() {
		addWarn("", "graph contains a cycle (loops like review→fix are valid; ensure the orchestrator can exit)")
	}
	if start := wf.findNode(NodeTypeStart); start != nil {
		reachable := wf.reachableFrom(start.ID)
		for i := range wf.Nodes {
			n := &wf.Nodes[i]
			if n.Type == NodeTypeStart {
				continue
			}
			if !reachable[n.ID] && n.Type != NodeTypeGroup {
				addWarn(n.ID, "node is not reachable from start")
			}
		}
	}

	res.Valid = len(res.Errors) == 0
	return res
}

// validateNode applies the per-type field rules.
func validateNode(n *Node, addErr, addWarn func(string, string)) {
	switch n.Type {
	case NodeTypeSubAgent:
		if n.Data.AgentDescription == "" {
			addErr(n.ID, "subAgent requires a description")
		} else if len(n.Data.AgentDescription) > descriptionMax {
			addErr(n.ID, fmt.Sprintf("description too long: %d chars (max %d)", len(n.Data.AgentDescription), descriptionMax))
		}
		if len(n.Data.AgentDefinition) > agentDefinitionMax {
			addErr(n.ID, fmt.Sprintf("agentDefinition too long: %d chars (max %d)", len(n.Data.AgentDefinition), agentDefinitionMax))
		}
		if len(n.Data.Prompt) > promptMax {
			addErr(n.ID, fmt.Sprintf("prompt too long: %d chars (max %d)", len(n.Data.Prompt), promptMax))
		}
	case NodeTypeAskUserQuestion:
		if n.Data.QuestionText == "" {
			addErr(n.ID, "askUserQuestion requires questionText")
		}
		if len(n.Data.Options) < askOptionsMin {
			addErr(n.ID, fmt.Sprintf("askUserQuestion needs at least %d options (found %d)", askOptionsMin, len(n.Data.Options)))
		}
		if len(n.Data.Options) > askOptionsMax {
			addErr(n.ID, fmt.Sprintf("askUserQuestion allows at most %d options (found %d)", askOptionsMax, len(n.Data.Options)))
		}
	case NodeTypeSwitch, NodeTypeBranch:
		if len(n.Data.Branches) > switchMaxBranches {
			addErr(n.ID, fmt.Sprintf("switch allows at most %d branches (found %d)", switchMaxBranches, len(n.Data.Branches)))
		}
	case NodeTypeSkill:
		if n.Data.Name != "" && !skillNamePattern.MatchString(n.Data.Name) {
			addErr(n.ID, "skill name must match [a-z0-9-]")
		}
		if len(n.Data.AgentDescription) > skillDescriptionMax {
			addErr(n.ID, fmt.Sprintf("skill description too long: %d chars (max %d)", len(n.Data.AgentDescription), skillDescriptionMax))
		}
	case NodeTypeMCP:
		validateMCPNode(n, addErr, addWarn)
	case NodeTypeStart, NodeTypeEnd, NodeTypePrompt, NodeTypeIfElse, NodeTypeSubAgentFlow, NodeTypeCodex, NodeTypeGroup:
		// no mandatory per-type checks beyond presence
	}
}

// validateMCPNode checks the MCP node's mode and the fields that mode requires.
// Empty Mode is allowed (treated as aiParameterConfig for backward compat).
func validateMCPNode(n *Node, addErr, addWarn func(string, string)) {
	switch n.Data.McpMode {
	case "", MCPModeManualParameterConfig, MCPModeAIParameterConfig, MCPModeAIToolSelection:
		// valid mode
	default:
		addErr(n.ID, fmt.Sprintf("mcp node has invalid mode %q", n.Data.McpMode))
		return
	}
	mode := n.Data.McpMode
	if mode == "" {
		mode = MCPModeAIParameterConfig
	}
	server := strings.TrimSpace(n.Data.Server)
	if server == "" {
		addErr(n.ID, "mcp node requires a server")
	}
	switch mode {
	case MCPModeManualParameterConfig, MCPModeAIParameterConfig:
		if strings.TrimSpace(n.Data.Tool) == "" {
			addWarn(n.ID, "mcp node in manual/aiParameterConfig mode should specify a tool")
		}
	case MCPModeAIToolSelection:
		if strings.TrimSpace(n.Data.TaskDescription) == "" {
			addWarn(n.ID, "mcp node in aiToolSelection mode should describe the task")
		}
	}
}

// validateSlashCommandOptions checks the model/context enums and field lengths
// of the optional slash-command options.
func validateSlashCommandOptions(opt *SlashCommandOptions, addErr func(string, string)) {
	switch m := strings.TrimSpace(opt.Model); m {
	case "", "default", "sonnet", "opus", "haiku", "inherit":
		// ok
	default:
		addErr("", fmt.Sprintf("slashCommandOptions.model %q is not a valid model", m))
	}
	switch c := strings.TrimSpace(opt.Context); c {
	case "", "default", "fork":
		// ok
	default:
		addErr("", fmt.Sprintf("slashCommandOptions.context %q is not valid (default|fork)", c))
	}
	if len(opt.ArgumentHint) > argumentHintMax {
		addErr("", fmt.Sprintf("slashCommandOptions.argumentHint too long: %d chars (max %d)", len(opt.ArgumentHint), argumentHintMax))
	}
	if opt.Hooks != nil {
		validateHooks(opt.Hooks, addErr)
	}
}

// validateHooks checks hook entries have actions with a valid type.
func validateHooks(h *WorkflowHooks, addErr func(string, string)) {
	validateHookBucket := func(name string, entries []HookEntry) {
		for _, e := range entries {
			if len(e.Hooks) == 0 {
				addErr("", fmt.Sprintf("hooks.%s entry has no actions", name))
			}
			for _, a := range e.Hooks {
				switch a.Type {
				case "command", "prompt":
					// ok
				default:
					addErr("", fmt.Sprintf("hooks.%s action has invalid type %q (command|prompt)", name, a.Type))
				}
			}
		}
	}
	validateHookBucket("PreToolUse", h.PreToolUse)
	validateHookBucket("PostToolUse", h.PostToolUse)
	validateHookBucket("Stop", h.Stop)
}

// reachableFrom returns the set of node ids reachable from start (excluding
// start itself), via BFS over the workflow adjacency.
func (wf *Workflow) reachableFrom(start string) map[string]bool {
	adj := wf.adjacency()
	seen := make(map[string]bool)
	queue := []string{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range adj[cur] {
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return seen
}
