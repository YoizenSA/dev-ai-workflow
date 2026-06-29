package workflows

import (
	"strings"
	"testing"
)

func simpleValidWorkflow() *Workflow {
	return &Workflow{
		Name:    "w",
		Version: "1.0.0",
		Nodes: []Node{
			{ID: "s", Type: NodeTypeStart, Name: "s"},
			{ID: "a", Type: NodeTypeSubAgent, Name: "a", Data: NodeData{AgentDescription: "does a"}},
			{ID: "e", Type: NodeTypeEnd, Name: "e"},
		},
		Connections: []Connection{
			{From: "s", To: "a"},
			{From: "a", To: "e"},
		},
	}
}

func TestValidateValid(t *testing.T) {
	res := Validate(simpleValidWorkflow())
	if !res.Valid {
		t.Fatalf("expected valid, got errors: %+v", res.Errors)
	}
}

func TestValidateMissingEndpoints(t *testing.T) {
	wf := simpleValidWorkflow()
	wf.Nodes = wf.Nodes[1:] // drop start
	res := Validate(wf)
	if res.Valid {
		t.Fatal("expected invalid when start missing")
	}
	if !hasIssueContaining(res.Errors, "start node") {
		t.Fatalf("expected start-node error, got: %+v", res.Errors)
	}
}

func TestValidateDuplicateEndpoints(t *testing.T) {
	wf := simpleValidWorkflow()
	wf.Nodes = append(wf.Nodes, Node{ID: "s2", Type: NodeTypeStart, Name: "s2"})
	res := Validate(wf)
	if res.Valid || !hasIssueContaining(res.Errors, "exactly one start") {
		t.Fatalf("expected duplicate-start error, got: %+v", res.Errors)
	}
}

func TestValidateCycle(t *testing.T) {
	wf := simpleValidWorkflow()
	// Create a cycle a -> e -> a.
	wf.Connections = append(wf.Connections, Connection{From: "e", To: "a"})
	res := Validate(wf)
	if res.Valid {
		t.Fatal("expected invalid for cyclic graph")
	}
	if !hasIssueContaining(res.Errors, "cycle") {
		t.Fatalf("expected cycle error, got: %+v", res.Errors)
	}
}

func TestValidateAskUserOptions(t *testing.T) {
	wf := simpleValidWorkflow()
	wf.Nodes = append(wf.Nodes, Node{
		ID:   "q",
		Type: NodeTypeAskUserQuestion,
		Name: "q",
		Data: NodeData{QuestionText: "which?", Options: []QuestionOption{{Label: "one"}}}, // too few
	})
	wf.Connections = append(wf.Connections, Connection{From: "a", To: "q"}, Connection{From: "q", To: "e"})
	res := Validate(wf)
	if res.Valid || !hasIssueContaining(res.Errors, "options") {
		t.Fatalf("expected options error, got: %+v", res.Errors)
	}
}

func TestValidateUnreachableWarns(t *testing.T) {
	wf := simpleValidWorkflow()
	// Add a disconnected node.
	wf.Nodes = append(wf.Nodes, Node{ID: "lonely", Type: NodeTypePrompt, Name: "lonely"})
	res := Validate(wf)
	// Warnings don't make it invalid...
	if !res.Valid {
		t.Fatalf("expected valid despite warnings, errors: %+v", res.Errors)
	}
	if !hasIssueContaining(res.Warnings, "reachable") {
		t.Fatalf("expected unreachable warning, got: %+v", res.Warnings)
	}
}

func TestHasCycle(t *testing.T) {
	// Line: s -> a -> e (no cycle).
	if simpleValidWorkflow().hasCycle() {
		t.Fatal("linear graph should not have a cycle")
	}
	// Self-loop.
	wf := &Workflow{
		Nodes:       []Node{{ID: "x", Type: NodeTypePrompt, Name: "x"}},
		Connections: []Connection{{From: "x", To: "x"}},
	}
	if !wf.hasCycle() {
		t.Fatal("self-loop should be a cycle")
	}
}

func TestTopoOrder(t *testing.T) {
	wf := simpleValidWorkflow()
	order, err := wf.topoOrder()
	if err != nil {
		t.Fatalf("topoOrder: %v", err)
	}
	// start must come before a, a before end.
	pos := func(id string) int {
		for i, n := range order {
			if n == id {
				return i
			}
		}
		return -1
	}
	if pos("s") >= pos("a") || pos("a") >= pos("e") {
		t.Fatalf("topo order wrong: %v", order)
	}
}

func hasIssueContaining(issues []ValidationIssue, substr string) bool {
	for _, iss := range issues {
		if strings.Contains(iss.Message, substr) {
			return true
		}
	}
	return false
}
