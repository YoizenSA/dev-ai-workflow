package workflows

import (
	"fmt"
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
		if def := strings.TrimSpace(s.Data.AgentDefinition); def != "" {
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
// Node ids are sanitized for Mermaid (no spaces/special chars).
func renderMermaid(wf *Workflow, subAgentIDs map[string]string) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")

	byID := wf.nodeByID()
	mermaidID := make(map[string]string, len(wf.Nodes))

	// Emit node declarations with labels.
	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		mid := mermaidName(n.ID, i)
		mermaidID[n.ID] = mid
		label := mermaidLabel(n, subAgentIDs)
		shape := mermaidShape(n.Type)
		fmt.Fprintf(&b, "  %s%s%s\n", mid, shape.open, quoteMermaid(label, shape.open, shape.close))
		_ = byID
	}

	// Emit edges.
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
	order, err := wf.topoOrder()
	if err != nil {
		// Cyclic: fall back to node declaration order.
		order = make([]string, len(wf.Nodes))
		for i := range wf.Nodes {
			order[i] = wf.Nodes[i].ID
		}
	}
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

	steps := make([]string, 0, len(order))
	for _, id := range order {
		n, ok := byID[id]
		if !ok {
			continue
		}
		if s := stepForNode(n, subAgentIDs, outByPort[id], byID); s != "" {
			steps = append(steps, s)
		}
	}
	return steps
}

func stepForNode(n *Node, subAgentIDs map[string]string, outs map[string][]string, byID map[string]*Node) string {
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
		return b.String()
	case NodeTypeIfElse:
		return fmt.Sprintf("**Branch (if/else)** on condition: %s. Follow the matching outgoing edge.", quoteInline(n.Data.Condition))
	case NodeTypeSwitch, NodeTypeBranch:
		var b strings.Builder
		fmt.Fprintf(&b, "**Switch** on: %s", quoteInline(n.Data.Expression))
		for _, br := range n.Data.Branches {
			b.WriteString("\n   - " + br.Label + " → " + br.Value)
		}
		return b.String()
	case NodeTypeSkill:
		mode := n.Data.ExecutionMode
		if mode == "" {
			mode = "load"
		}
		return fmt.Sprintf("**%s skill `%s`** using the `skill` tool.", strings.Title(mode), n.Data.Name)
	case NodeTypeMCP:
		s := fmt.Sprintf("**Call MCP tool** `%s/%s`.", n.Data.Server, n.Data.Tool)
		if p := strings.TrimSpace(n.Data.AIParams); p != "" {
			s += " Infer its parameters from: " + p
		}
		return s
	case NodeTypeSubAgentFlow:
		// A sub-workflow is invoked as its exported slash command.
		return fmt.Sprintf("**Run the `/%s` sub-workflow** and wait for it to finish.", n.Data.FlowID)
	case NodeTypeGroup:
		return "" // visual only
	}
	return ""
}

// quoteInline wraps a free-text instruction in quotes for readability in the
// generated prompt, collapsing newlines.
func quoteInline(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	return "\"" + s + "\""
}
