package workflows

import (
	"fmt"
	"sort"
	"strings"
)

// orchestratorBody builds the prompt body for the orchestrator agent: a Mermaid
// diagram of the workflow graph followed by step-by-step execution instructions.
// This is what the LLM reads to know how to drive the workflow.
func orchestratorBody(wf *Workflow, subAgentIDs map[string]string) string {
	var b strings.Builder

	b.WriteString("# " + wf.Name + " Workflow\n\n")
	if wf.Description != "" {
		b.WriteString(wf.Description + "\n\n")
	}

	// The START node configures the orchestrator's own identity: its system
	// prompt (agentDefinition) is prepended as the parent agent's persona.
	if s := wf.findNode(NodeTypeStart); s != nil {
		// Same resolution as sub-agent nodes: the START node may link to a real
		// orchestrator under agents/ instead of embedding a copy of its prompt.
		if def := strings.TrimSpace(resolveAgentDefinition(s)); def != "" {
			b.WriteString(def + "\n\n")
		}
	}

	// Mermaid diagram.
	b.WriteString("## Flow\n\n```mermaid\n")
	b.WriteString(renderMermaid(wf, subAgentIDs))
	b.WriteString("```\n\n")

	// Step-by-step instructions.
	b.WriteString("## Execution steps\n\n")
	b.WriteString("Follow these steps in order. Use the `task` tool to delegate to sub-agents, ")
	b.WriteString("the `skill` tool to load referenced skills, and the `question` tool to ask ")
	b.WriteString("the user when a choice is required. Do not skip steps.\n\n")

	steps := buildSteps(wf, subAgentIDs)
	for i, step := range steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, step)
	}

	return b.String()
}

// renderMermaid produces a Mermaid flowchart (LR) of the workflow graph.
// Group nodes render as Mermaid `subgraph` blocks containing their children
// (nodes whose ParentID points at the group); top-level nodes render at the
// flowchart root. Node ids are sanitized for Mermaid (no spaces/special chars).
func renderMermaid(wf *Workflow, subAgentIDs map[string]string) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")

	mermaidID := make(map[string]string, len(wf.Nodes))
	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		mermaidID[n.ID] = mermaidName(n.ID, i)
	}

	// emitNode writes a non-group node declaration.
	emitNode := func(n *Node, indent string) {
		mid := mermaidID[n.ID]
		label := mermaidLabel(n, subAgentIDs)
		shape := mermaidShape(n.Type)
		fmt.Fprintf(&b, "%s%s%s%s\n", indent, mid, shape.open, quoteMermaid(label, shape.open, shape.close))
	}

	hasEmptyGroup := false

	// Top-level nodes (no parent, not a group themselves).
	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		if n.ParentID != "" || n.Type == NodeTypeGroup {
			continue
		}
		emitNode(n, "  ")
	}

	// Each group as a subgraph with its children nested inside.
	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		if n.Type != NodeTypeGroup {
			continue
		}
		gid := mermaidID[n.ID]
		glabel := mermaidLabel(n, subAgentIDs)
		// subgraph header: `subgraph <id> ["<label>"]`. Escape quotes in label.
		glabel = strings.ReplaceAll(glabel, `"`, `'`)
		fmt.Fprintf(&b, "  subgraph %s [\"%s\"]\n", gid, glabel)
		children := 0
		for j := range wf.Nodes {
			c := &wf.Nodes[j]
			if c.ParentID == n.ID && c.Type != NodeTypeGroup {
				emitNode(c, "    ")
				children++
			}
		}
		if children == 0 {
			// Mermaid needs a member to render the subgraph box.
			fmt.Fprintf(&b, "    %s_empty[ ]:::hidden\n", gid)
			hasEmptyGroup = true
		}
		b.WriteString("  end\n")
	}

	if hasEmptyGroup {
		b.WriteString("  classDef hidden display:none;\n")
	}

	// Edges last (Mermaid resolves ids declared anywhere).
	for _, c := range wf.Connections {
		from, ok1 := mermaidID[c.From]
		to, ok2 := mermaidID[c.To]
		if !ok1 || !ok2 {
			continue
		}
		fmt.Fprintf(&b, "  %s --> %s\n", from, to)
	}
	return b.String()
}

type mermaidShapeDef struct {
	open  string
	close string
}

func mermaidShape(t string) mermaidShapeDef {
	switch t {
	case NodeTypeStart:
		return mermaidShapeDef{"([", "])"} // stadium
	case NodeTypeEnd:
		return mermaidShapeDef{"{", "}"} // rhombus-ish
	case NodeTypeAskUserQuestion, NodeTypeIfElse, NodeTypeSwitch, NodeTypeBranch:
		return mermaidShapeDef{"{", "}"} // decision
	case NodeTypeGroup:
		return mermaidShapeDef{"subgraph ", " end"} // handled below
	default:
		return mermaidShapeDef{"[", "]"} // rectangle
	}
}

func mermaidName(id string, idx int) string {
	// Use a stable sanitized id; fall back to N<idx>.
	s := sanitizeSlug(id)
	if s == "" {
		return fmt.Sprintf("N%d", idx)
	}
	// Ensure it doesn't start with a digit.
	if s[0] >= '0' && s[0] <= '9' {
		s = "N" + s
	}
	return s
}

func mermaidLabel(n *Node, subAgentIDs map[string]string) string {
	switch n.Type {
	case NodeTypeStart:
		return "Start"
	case NodeTypeEnd:
		return "End"
	case NodeTypeSubAgent:
		name := n.Data.Name
		if name == "" {
			name = n.Data.AgentDescription
		}
		if name == "" {
			name = subAgentIDs[n.ID]
		}
		return "SubAgent: " + name
	case NodeTypeAskUserQuestion:
		q := n.Data.QuestionText
		if q == "" {
			q = "Ask user"
		}
		return "Ask: " + q
	case NodeTypeIfElse:
		return "If: " + ellipsize(n.Data.Condition, 40)
	case NodeTypeSwitch:
		return "Switch: " + ellipsize(n.Data.Expression, 40)
	case NodeTypePrompt:
		l := n.Data.Label
		if l == "" {
			l = n.Data.Prompt
		}
		return "Prompt: " + ellipsize(l, 40)
	case NodeTypeSkill:
		return "Skill: " + n.Data.Name
	case NodeTypeMCP:
		return "MCP: " + n.Data.Server + "/" + n.Data.Tool
	case NodeTypeSubAgentFlow:
		return "SubFlow: " + n.Data.FlowID
	case NodeTypeGroup:
		return n.Data.Label
	}
	return n.Type
}

func ellipsize(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// quoteMermaid wraps a label for a Mermaid node shape, escaping pipe chars and
// surrounding with the appropriate delimiters.
func quoteMermaid(label, openShape, closeShape string) string {
	// For subgraph we return the label as-is (caller handles structure).
	if openShape == "subgraph " {
		return label
	}
	// Escape pipes and quotes inside the label.
	label = strings.ReplaceAll(label, "|", "\\|")
	label = strings.ReplaceAll(label, "[", "\\[")
	label = strings.ReplaceAll(label, "]", "\\]")
	label = strings.ReplaceAll(label, "{", "\\{")
	label = strings.ReplaceAll(label, "}", "\\}")
	return label + closeShape
}

// buildSteps produces one human-readable instruction per graph node in
// topological order. Branching nodes (askUserQuestion/ifElse/switch) describe
// their options/conditions so the LLM knows how to route.
func buildSteps(wf *Workflow, subAgentIDs map[string]string) []string {
	layers := wf.executionLayers()
	byID := wf.nodeByID()
	// Outgoing edges grouped by source, keyed by port for branching nodes.
	outByPort := make(map[string]map[string][]string) // nodeID -> port -> []targetID
	for _, c := range wf.Connections {
		ports := outByPort[c.From]
		if ports == nil {
			ports = make(map[string][]string)
		}
		ports[c.FromPort] = append(ports[c.FromPort], c.To)
		outByPort[c.From] = ports
	}

	back := wf.backEdges()
	steps := make([]string, 0, len(wf.Nodes))
	for _, layer := range layers {
		// Render the layer first: group nodes and unsupported types produce no
		// step, so a layer of three can still collapse to a single instruction.
		rendered := make([]string, 0, len(layer))
		for _, id := range layer {
			n, ok := byID[id]
			if !ok {
				continue
			}
			if s := stepForNode(n, subAgentIDs, outByPort[id], byID, back); s != "" {
				rendered = append(rendered, s)
			}
		}
		switch len(rendered) {
		case 0:
			continue
		case 1:
			steps = append(steps, rendered[0])
		default:
			steps = append(steps, parallelStep(rendered))
		}
	}
	return steps
}

// parallelStep renders a layer of independent nodes as one fan-out instruction.
//
// Nothing flows between these nodes, so running them in sequence buys nothing
// and costs the sum of their wall time instead of the slowest one. The exported
// prompt has to say so explicitly: an orchestrator reading a numbered list
// assumes the numbers are an order.
func parallelStep(rendered []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**Run these %d in parallel.** They are independent — no result "+
		"flows between them. Dispatch every one of them before waiting on any, then "+
		"collect all the handoffs before moving on.", len(rendered))
	for _, s := range rendered {
		// Indent continuation lines so nested bullets stay under their item.
		b.WriteString("\n   - " + strings.ReplaceAll(s, "\n   ", "\n     "))
	}
	return b.String()
}

func stepForNode(n *Node, subAgentIDs map[string]string, outs map[string][]string, byID map[string]*Node, back map[string]bool) string {
	switch n.Type {
	case NodeTypeStart:
		return "**Start.** Begin the workflow."
	case NodeTypeEnd:
		return "**End.** The workflow is complete. Summarize what was done."
	case NodeTypeSubAgent:
		id := subAgentIDs[n.ID]
		task := strings.TrimSpace(n.Data.Prompt)
		if task == "" {
			task = strings.TrimSpace(n.Data.AgentDescription)
		}
		if task == "" {
			task = "Perform the agent's role."
		}
		return fmt.Sprintf("**Delegate to sub-agent `%s`** using the `task` tool with: %s", id, quoteInline(task))
	case NodeTypePrompt:
		p := strings.TrimSpace(n.Data.Prompt)
		if p == "" {
			p = strings.TrimSpace(n.Data.Label)
		}
		return "**Prompt:** " + p
	case NodeTypeAskUserQuestion:
		var b strings.Builder
		fmt.Fprintf(&b, "**Ask the user** (via the `question` tool): %s", quoteInline(n.Data.QuestionText))
		for _, opt := range n.Data.Options {
			desc := opt.Label
			if opt.Description != "" {
				desc += " — " + opt.Description
			}
			b.WriteString("\n   - " + desc)
		}
		b.WriteString("\n   Route to the branch matching the user's choice.")
		b.WriteString(routingLines(n, outs, byID, subAgentIDs, back))
		return b.String()
	case NodeTypeIfElse:
		var b strings.Builder
		fmt.Fprintf(&b, "**Branch (if/else)** on condition: %s", quoteInline(n.Data.Condition))
		b.WriteString(routingLines(n, outs, byID, subAgentIDs, back))
		return b.String()
	case NodeTypeSwitch, NodeTypeBranch:
		var b strings.Builder
		fmt.Fprintf(&b, "**Switch** on: %s", quoteInline(n.Data.Expression))
		for _, br := range n.Data.Branches {
			b.WriteString("\n   - " + br.Label + " → " + br.Value)
		}
		b.WriteString(routingLines(n, outs, byID, subAgentIDs, back))
		return b.String()
	case NodeTypeSkill:
		mode := n.Data.ExecutionMode
		if mode == "" {
			mode = "load"
		}
		return fmt.Sprintf("**%s skill `%s`** using the `skill` tool.", strings.Title(mode), n.Data.Name)
	case NodeTypeMCP:
		return mcpStep(n)
	case NodeTypeSubAgentFlow:
		// A sub-workflow is invoked as its exported slash command.
		return fmt.Sprintf("**Run the `/%s` sub-workflow** and wait for it to finish.", n.Data.FlowID)
	case NodeTypeGroup:
		return "" // visual only
	}
	return ""
}

// DefaultMaxRounds caps a rework loop when the node does not set its own.
//
// A loop with no cap is a budget leak: "review → fix → review" repeats until
// something external stops it. Two rounds is the point where a third pass
// stops being a fix and starts being a disagreement for a human to settle.
const DefaultMaxRounds = 2

// routingLines renders where each outgoing port of a branching node goes.
//
// Naming the targets is the whole point: "follow the matching outgoing edge"
// tells an orchestrator that a choice exists but not what the choices are, so
// it guesses. Back edges are called out as rework and carry the round cap.
func routingLines(n *Node, outs map[string][]string, byID map[string]*Node, subAgentIDs map[string]string, back map[string]bool) string {
	ports := make([]string, 0, len(outs))
	for p := range outs {
		ports = append(ports, p)
	}
	sort.Slice(ports, func(i, j int) bool {
		rank := func(p string) int {
			switch p {
			case "true":
				return 0
			case "false":
				return 1
			default:
				return 2
			}
		}
		if rank(ports[i]) != rank(ports[j]) {
			return rank(ports[i]) < rank(ports[j])
		}
		return ports[i] < ports[j]
	})

	var b strings.Builder
	hasRework := false
	for _, port := range ports {
		for _, target := range outs[port] {
			t, ok := byID[target]
			if !ok {
				continue
			}
			label := port
			if label == "" {
				label = "→"
			}
			b.WriteString("\n   - " + label + " → " + stepTargetName(t, subAgentIDs))
			if back[n.ID+"->"+target] {
				hasRework = true
				b.WriteString(" *(rework — loops back)*")
			}
		}
	}
	if hasRework {
		max := n.Data.MaxRounds
		if max <= 0 {
			max = DefaultMaxRounds
		}
		fmt.Fprintf(&b, "\n   - Rework limit: at most %d loop(s). Send only the nodes the "+
			"review actually flagged, not the whole stage. If it still fails after %d, stop "+
			"and report what is blocking instead of looping again.", max, max)
	}
	return b.String()
}

// stepTargetName names a node the way the execution steps refer to it: a
// sub-agent by the id the orchestrator delegates to, anything else by label.
func stepTargetName(n *Node, subAgentIDs map[string]string) string {
	if n.Type == NodeTypeSubAgent {
		if id := subAgentIDs[n.ID]; id != "" {
			return "sub-agent `" + id + "`"
		}
	}
	label := strings.TrimSpace(n.Data.Label)
	if label == "" {
		label = strings.TrimSpace(n.Name)
	}
	if label == "" {
		label = n.ID
	}
	return "**" + label + "**"
}

// quoteInline wraps a free-text instruction in quotes for readability in the
// generated prompt, collapsing newlines.
func quoteInline(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	return "\"" + s + "\""
}

// mcpStep renders a single MCP node's execution instruction based on its mode,
// in one of three forms:
//   - manualParameterConfig / aiParameterConfig: a specific tool is pinned; the
//     agent calls it directly (params inferred from AIParams when present).
//   - aiToolSelection: only the server is pinned; the agent must query the
//     server at runtime (tools/list), pick the best tool for the task, and fill
//     its parameters from the natural-language TaskDescription.
//
// Empty Mode defaults to aiParameterConfig so existing workflows keep working.
func mcpStep(n *Node) string {
	mode := n.Data.McpMode
	if mode == "" {
		mode = MCPModeAIParameterConfig
	}
	server := strings.TrimSpace(n.Data.Server)
	if server == "" {
		server = "<unspecified>"
	}
	switch mode {
	case MCPModeAIToolSelection:
		task := strings.TrimSpace(n.Data.TaskDescription)
		if task == "" {
			task = "(no task description provided)"
		}
		return fmt.Sprintf(
			"**MCP (AI tool selection) — server `%s`.** At runtime, query the `%s` MCP "+
				"server via `tools/list`, select the tool that best matches the task, and "+
				"fill its parameters from this task description:\n\n    %s",
			server, server, quoteInline(task),
		)
	default: // manualParameterConfig | aiParameterConfig
		tool := strings.TrimSpace(n.Data.Tool)
		if tool == "" {
			tool = "<unspecified>"
		}
		s := fmt.Sprintf("**Call MCP tool** `%s/%s`.", server, tool)
		if p := strings.TrimSpace(n.Data.AIParams); p != "" {
			s += " Infer its parameters from: " + p
		}
		return s
	}
}
