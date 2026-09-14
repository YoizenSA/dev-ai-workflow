package workflows

import (
	"strings"
	"testing"
)

// The implementation stage fans out and the ship-ready gate loops back into all
// three of its nodes. Classifying only one of those loops as a back edge pushed
// the other two into a layer after the gate, so the exported prompt instructed
// backend and infrastructure after the review that was supposed to gate them.
func TestShipGateLoopsBackIntoWholeFanOut(t *testing.T) {
	wf := loadShipSeed(t)
	back := wf.backEdges()

	for _, e := range []string{"gate->fe", "gate->be", "gate->infra"} {
		if !back[e] {
			t.Errorf("%s is a rework loop and must be a back edge", e)
		}
	}
	// The edges leading into the gate are the forward pass, not the loop.
	for _, e := range []string{"ask->fe", "ask->be", "ask->infra", "fe->qaflow", "be->qaflow", "infra->qaflow", "qaflow->rev", "rev->gate", "gate->end"} {
		if back[e] {
			t.Errorf("%s is a forward edge and must not be a back edge", e)
		}
	}

	layerOf := map[string]int{}
	for i, layer := range wf.executionLayers() {
		for _, id := range layer {
			layerOf[id] = i
		}
	}
	if layerOf["fe"] != layerOf["be"] || layerOf["be"] != layerOf["infra"] {
		t.Errorf("implementation stage split across layers: fe=%d be=%d infra=%d",
			layerOf["fe"], layerOf["be"], layerOf["infra"])
	}
	if layerOf["fe"] >= layerOf["gate"] {
		t.Errorf("implementation (%d) must be laid out before the gate (%d)", layerOf["fe"], layerOf["gate"])
	}

	// And it has to survive into the prompt the orchestrator actually reads.
	ids := map[string]string{}
	for i := range wf.Nodes {
		ids[wf.Nodes[i].ID] = subAgentSlug(wf.Name, &wf.Nodes[i])
	}
	steps := buildSteps(wf, ids)
	fanout := -1
	for i, s := range steps {
		if strings.Contains(s, "Run these 3 in parallel") &&
			strings.Contains(s, "ship-frontend") && strings.Contains(s, "ship-backend") && strings.Contains(s, "ship-infrastructure") {
			fanout = i
		}
	}
	if fanout < 0 {
		t.Fatalf("the three implementation agents are not announced as one fan-out:\n%s", strings.Join(steps, "\n---\n"))
	}
	for i, s := range steps {
		if strings.Contains(s, "Branch (if/else)") && i < fanout {
			t.Errorf("step %d gates work that is only dispatched at step %d", i+1, fanout+1)
		}
	}
}
