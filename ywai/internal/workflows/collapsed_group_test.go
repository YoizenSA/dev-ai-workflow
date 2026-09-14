package workflows

import "testing"

// TestCollapsedGroupIsVisualOnly guards the editor's collapse toggle: it flips
// data.collapsed on group nodes, and nothing downstream may read it. If the
// exported command/agent files change when a group is collapsed, a cosmetic
// canvas action has started altering what the orchestrator actually runs.
func TestCollapsedGroupIsVisualOnly(t *testing.T) {
	before := loadShipSeed(t)
	_, expanded, err := NewExporter().Plan(before)
	if err != nil {
		t.Fatalf("plan expanded: %v", err)
	}

	after := loadShipSeed(t)
	collapsed := 0
	for i := range after.Nodes {
		if after.Nodes[i].Type == NodeTypeGroup {
			after.Nodes[i].Data.Collapsed = true
			collapsed++
		}
	}
	if collapsed == 0 {
		t.Fatal("ship seed has no group nodes; this test would prove nothing")
	}

	_, got, err := NewExporter().Plan(after)
	if err != nil {
		t.Fatalf("plan collapsed: %v", err)
	}

	if len(got) != len(expanded) {
		t.Fatalf("file count changed: %d expanded vs %d collapsed", len(expanded), len(got))
	}
	for path, want := range expanded {
		if got[path] != want {
			t.Errorf("collapsing groups changed %s", path)
		}
	}
}
