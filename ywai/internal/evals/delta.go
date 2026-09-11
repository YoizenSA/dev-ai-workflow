package evals

// ModelDelta is one model's current-vs-baseline comparison for a task. A nil
// field means "no comparison possible" — the model has no baseline summary —
// never a computed zero, so an unchanged score (0) and an absent one (null)
// stay distinguishable in the API response.
type ModelDelta struct {
	Model         string   `json:"model"`
	WeightedDelta *float64 `json:"weightedDelta"`
	HardDelta     *float64 `json:"hardDelta"`
	TurnsDelta    *float64 `json:"turnsDelta"`
	CostDeltaUSD  *float64 `json:"costDeltaUsd"`
}

// DiffModels pairs each model in current with its baseline summary and reports
// current minus baseline: positive weighted/hard deltas mean the model improved,
// positive turns/cost deltas mean it got heavier. The baseline side is produced
// by aggregating only the baseline run, so both sides come from the same
// Aggregate math. Models missing from the baseline get nil deltas; baseline-only
// models are dropped so the table stays anchored to what exists now. Current's
// order is preserved and the function is pure: no I/O, no globals.
func DiffModels(current, baseline []ModelTaskSummary) []ModelDelta {
	base := make(map[string]ModelTaskSummary, len(baseline))
	for _, m := range baseline {
		base[m.Model] = m
	}
	out := make([]ModelDelta, 0, len(current))
	for _, cur := range current {
		d := ModelDelta{Model: cur.Model}
		if b, ok := base[cur.Model]; ok {
			d.WeightedDelta = floatPtr(cur.AvgWeighted - b.AvgWeighted)
			d.HardDelta = floatPtr(cur.HardRate - b.HardRate)
			d.TurnsDelta = floatPtr(cur.AvgTurns - b.AvgTurns)
			d.CostDeltaUSD = floatPtr(cur.TotalCostUSD - b.TotalCostUSD)
		}
		out = append(out, d)
	}
	return out
}

// floatPtr takes a delta's address so the struct can tell "compared and equal"
// (pointer to 0) apart from "no baseline to compare" (nil).
func floatPtr(f float64) *float64 { return &f }
