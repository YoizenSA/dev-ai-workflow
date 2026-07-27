package workflows

import (
	"strings"
	"testing"
)

// diamond: a fans out to b/c/d, which converge on e, reviewed by f, whose gate
// loops back to b. The shape every non-trivial workflow ends up with.
func diamondWorkflow() *Workflow {
	return &Workflow{
		Name: "wf",
		Nodes: []Node{
			{ID: "a", Type: NodeTypeSubAgent, Data: NodeData{Name: "scout", Prompt: "scout"}},
			{ID: "b", Type: NodeTypeSubAgent, Data: NodeData{Name: "fe", Prompt: "fe"}},
			{ID: "c", Type: NodeTypeSubAgent, Data: NodeData{Name: "be", Prompt: "be"}},
			{ID: "d", Type: NodeTypeSubAgent, Data: NodeData{Name: "infra", Prompt: "infra"}},
			{ID: "e", Type: NodeTypeSubAgent, Data: NodeData{Name: "qa", Prompt: "qa"}},
			{ID: "gate", Type: NodeTypeIfElse, Data: NodeData{Condition: "clean", Label: "gate"}},
			{ID: "end", Type: NodeTypeEnd, Data: NodeData{Label: "Done"}},
		},
		Connections: []Connection{
			{From: "a", To: "b"}, {From: "a", To: "c"}, {From: "a", To: "d"},
			{From: "b", To: "e"}, {From: "c", To: "e"}, {From: "d", To: "e"},
			{From: "e", To: "gate"},
			{From: "gate", To: "end", FromPort: "true"},
			{From: "gate", To: "b", FromPort: "false"}, // rework loop
		},
	}
}

func TestExecutionLayersGroupsIndependentNodes(t *testing.T) {
	wf := diamondWorkflow()
	layers := wf.executionLayers()

	// Every node must be placed exactly once.
	seen := map[string]int{}
	for _, l := range layers {
		for _, id := range l {
			seen[id]++
		}
	}
	if len(seen) != len(wf.Nodes) {
		t.Fatalf("placed %d nodes, want %d: %v", len(seen), len(wf.Nodes), layers)
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("node %s placed %d times", id, n)
		}
	}

	// b, c and d are the fan-out: independent, so they share a layer. The
	// rework edge gate→b must not push b into a later layer than its siblings.
	var fanout []string
	for _, l := range layers {
		if len(l) > 1 {
			fanout = l
		}
	}
	if strings.Join(fanout, ",") != "b,c,d" {
		t.Errorf("fan-out layer = %v, want [b c d]", fanout)
	}
}

// A cycle used to make topoOrder fail, which sent buildSteps to declaration
// order — so a node could be instructed before the node feeding it.
func TestBuildStepsOrdersCyclicGraphsByDependency(t *testing.T) {
	wf := diamondWorkflow()
	// Declare the reviewer BEFORE the work it reviews, so declaration order and
	// dependency order disagree and only the dependency one can be correct.
	wf.Nodes = append([]Node{{ID: "z", Type: NodeTypeSubAgent, Data: NodeData{Name: "late", Prompt: "late"}}}, wf.Nodes...)
	wf.Connections = append(wf.Connections, Connection{From: "e", To: "z"})

	ids := map[string]string{}
	for i := range wf.Nodes {
		if wf.Nodes[i].Type == NodeTypeSubAgent {
			ids[wf.Nodes[i].ID] = subAgentSlug(wf.Name, &wf.Nodes[i])
		}
	}
	steps := strings.Join(buildSteps(wf, ids), "\n---\n")
	qa, late := strings.Index(steps, "wf-qa"), strings.Index(steps, "wf-late")
	if qa < 0 || late < 0 {
		t.Fatalf("expected both agents in steps:\n%s", steps)
	}
	if qa > late {
		t.Errorf("qa must be instructed before the node consuming it:\n%s", steps)
	}
}

func TestBuildStepsAnnouncesFanOutAndRoutes(t *testing.T) {
	wf := diamondWorkflow()
	ids := map[string]string{}
	for i := range wf.Nodes {
		if wf.Nodes[i].Type == NodeTypeSubAgent {
			ids[wf.Nodes[i].ID] = subAgentSlug(wf.Name, &wf.Nodes[i])
		}
	}
	steps := strings.Join(buildSteps(wf, ids), "\n")

	// Fan-out has to be stated: a numbered list reads as an order.
	if !strings.Contains(steps, "Run these 3 in parallel") {
		t.Errorf("independent nodes not announced as parallel:\n%s", steps)
	}
	// The branch must name where each port goes, not just that a branch exists.
	if !strings.Contains(steps, "true → **Done**") {
		t.Errorf("true branch target not named:\n%s", steps)
	}
	if !strings.Contains(steps, "rework — loops back") {
		t.Errorf("rework edge not flagged:\n%s", steps)
	}
	// An uncapped loop is a budget leak.
	if !strings.Contains(steps, "at most 2 loop(s)") {
		t.Errorf("rework loop has no round cap:\n%s", steps)
	}
}

func TestMaxRoundsOverridesDefault(t *testing.T) {
	wf := diamondWorkflow()
	for i := range wf.Nodes {
		if wf.Nodes[i].ID == "gate" {
			wf.Nodes[i].Data.MaxRounds = 5
		}
	}
	steps := strings.Join(buildSteps(wf, map[string]string{}), "\n")
	if !strings.Contains(steps, "at most 5 loop(s)") {
		t.Errorf("node maxRounds ignored:\n%s", steps)
	}
}
