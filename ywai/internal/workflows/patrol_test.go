package workflows

import (
	"encoding/json"
	"os"
	"testing"
)

func TestValidatePatrolSeed(t *testing.T) {
	data, err := os.ReadFile("../../workflows/patrol.json")
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	var wf Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	res := Validate(&wf)
	if !res.Valid {
		t.Fatalf("patrol seed is INVALID:\n%+v", res)
	}
	for _, w := range res.Warnings {
		t.Logf("warning: [%s] %s", w.NodeID, w.Message)
	}

	// The patrol loop: gate true -> feedback, gate false -> straight to end.
	gate := wf.findNode(NodeTypeIfElse)
	if gate == nil {
		t.Fatal("patrol seed has no ifElse gate")
	}
	var trueToFeedback, falseToEnd bool
	for _, c := range wf.Connections {
		if c.From != gate.ID {
			continue
		}
		if c.FromPort == "true" && c.To == "feedback" {
			trueToFeedback = true
		}
		if c.FromPort == "false" && c.To == "end" {
			falseToEnd = true
		}
	}
	if !trueToFeedback || !falseToEnd {
		t.Errorf("gate wiring wrong: true->feedback=%v, false->end=%v", trueToFeedback, falseToEnd)
	}
}
