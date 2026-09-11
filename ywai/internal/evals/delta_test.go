package evals

import (
	"reflect"
	"testing"
)

// All expected deltas below use values that are exact in binary floating point
// (0.25, 0.5, 1), so plain DeepEqual works without a tolerance.

func TestDiffModels_ExactDeltas(t *testing.T) {
	current := []ModelTaskSummary{
		{Model: "m/one", AvgWeighted: 0.75, HardRate: 1, AvgTurns: 4.5, TotalCostUSD: 0.75},
		{Model: "m/two", AvgWeighted: 0.5, HardRate: 0.5, AvgTurns: 3.5, TotalCostUSD: 0.25},
	}
	baseline := []ModelTaskSummary{
		{Model: "m/one", AvgWeighted: 0.5, HardRate: 0.5, AvgTurns: 3.5, TotalCostUSD: 0.25},
		{Model: "m/two", AvgWeighted: 0.5, HardRate: 1, AvgTurns: 4.5, TotalCostUSD: 0.75},
	}
	got := DiffModels(current, baseline)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(got), got)
	}
	// m/two is unchanged on weighted score: a computed 0 must surface as a
	// non-nil pointer, never as the null that means "no comparison".
	want := []ModelDelta{
		{Model: "m/one", WeightedDelta: floatPtr(0.25), HardDelta: floatPtr(0.5), TurnsDelta: floatPtr(1), CostDeltaUSD: floatPtr(0.5)},
		{Model: "m/two", WeightedDelta: floatPtr(0), HardDelta: floatPtr(-0.5), TurnsDelta: floatPtr(-1), CostDeltaUSD: floatPtr(-0.5)},
	}
	for i, w := range want {
		if !reflect.DeepEqual(got[i], w) {
			t.Errorf("delta[%d]=%+v want %+v", i, got[i], w)
		}
	}
}

func TestDiffModels_MissingFromBaselineGetsNils(t *testing.T) {
	current := []ModelTaskSummary{
		{Model: "m/new", AvgWeighted: 0.9, HardRate: 1, AvgTurns: 2, TotalCostUSD: 0.5},
	}
	baseline := []ModelTaskSummary{
		{Model: "m/other", AvgWeighted: 0.1},
	}
	got := DiffModels(current, baseline)
	want := ModelDelta{Model: "m/new"} // every delta nil: null in the API, not 0
	if !reflect.DeepEqual(got[0], want) {
		t.Fatalf("got %+v want %+v", got[0], want)
	}
}

func TestDiffModels_EmptyBaselineAllNils(t *testing.T) {
	current := []ModelTaskSummary{
		{Model: "m/a", AvgWeighted: 0.5},
		{Model: "m/b", AvgWeighted: 0.25},
	}
	got := DiffModels(current, nil)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	for i, d := range got {
		if d.WeightedDelta != nil || d.HardDelta != nil || d.TurnsDelta != nil || d.CostDeltaUSD != nil {
			t.Errorf("delta[%d] (%s) has non-nil deltas against an empty baseline: %+v", i, d.Model, d)
		}
	}
}

func TestDiffModels_EmptyCurrent(t *testing.T) {
	baseline := []ModelTaskSummary{{Model: "m/a", AvgWeighted: 0.5}}
	got := DiffModels(nil, baseline)
	if len(got) != 0 {
		t.Fatalf("len=%d want 0: %+v", len(got), got)
	}
}

func TestDiffModels_KeepsCurrentOrder(t *testing.T) {
	// Aggregate returns summaries best-first; the delta table must keep that
	// order rather than the baseline's.
	current := []ModelTaskSummary{
		{Model: "m/beta", AvgWeighted: 1},
		{Model: "m/alpha", AvgWeighted: 0.5},
	}
	baseline := []ModelTaskSummary{
		{Model: "m/alpha", AvgWeighted: 0.25},
		{Model: "m/beta", AvgWeighted: 0.75},
	}
	got := DiffModels(current, baseline)
	if len(got) != 2 || got[0].Model != "m/beta" || got[1].Model != "m/alpha" {
		t.Fatalf("order broken: %+v", got)
	}
}
