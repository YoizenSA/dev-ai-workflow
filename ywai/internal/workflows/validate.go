package workflows

import (
	"fmt"
	"regexp"
)

// Limits mirror cc-wf-studio's validation rules (workflow-definition.ts).
const (
	maxNodes            = 100
	descriptionMax      = 200   // subAgent description (frontmatter)
	agentDefinitionMax  = 10000 // subAgent system prompt
	promptMax           = 10000 // subAgent task prompt
	askOptionsMin       = 2
	askOptionsMax       = 4
	switchMaxBranches   = 10
	skillDescriptionMax = 1024
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
	if wf.hasCycle() {
		addErr("", "graph contains a cycle")
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
	case NodeTypeStart, NodeTypeEnd, NodeTypePrompt, NodeTypeIfElse, NodeTypeMCP, NodeTypeSubAgentFlow, NodeTypeCodex, NodeTypeGroup:
		// no mandatory per-type checks beyond presence
	}
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
