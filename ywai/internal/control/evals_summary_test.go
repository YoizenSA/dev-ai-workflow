package control

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/evals"
)

// TestHandleEvalSummary_AggregatesSortsAndFilters seeds a temp-dir bench store
// with two runs of one task, four models total, laid out so the summary endpoint
// must aggregate across runs, drop unanswered attempts, and order on all three
// sort keys: weighted score desc, then hard rate desc, then turns asc.
func TestHandleEvalSummary_AggregatesSortsAndFilters(t *testing.T) {
	// The bench store is a package global that persists to $HOME/.ywai; real
	// runs from this machine would leak into the test and make it order-
	// dependent. Swap in a temp-dir store for the duration of the test.
	benchRuns = newBenchStoreAt(t.TempDir())

	run1 := evals.Run{
		ID: "run-1", TaskID: "trace-answer", Status: "done",
		StartedAt: time.Now().UTC(),
		Attempts: []evals.Attempt{
			{Model: "m/beta", Score: evals.Score{Answered: true, Weighted: 1, GotHard: true},
				Metrics: evals.Metrics{Turns: 4, TokensIn: 100, TokensOut: 200}, Seconds: 10, CostUSD: 0.5, CostKnown: true},
			{Model: "m/gamma", Score: evals.Score{Answered: true, Weighted: 1, GotHard: true},
				Metrics: evals.Metrics{Turns: 5, TokensIn: 30, TokensOut: 40}, Seconds: 12, CostUSD: 0.3, CostKnown: true},
			{Model: "m/alpha", Score: evals.Score{Answered: true, Weighted: 0.5, GotHard: true},
				Metrics: evals.Metrics{Turns: 6, TokensIn: 50, TokensOut: 60}, Seconds: 20, CostUSD: 0.2, CostKnown: true},
			{Model: "m/delta", Score: evals.Score{Answered: true, Weighted: 0.5, GotHard: true},
				Metrics: evals.Metrics{Turns: 3, TokensIn: 20, TokensOut: 30}, Seconds: 8, CostUSD: 0.1, CostKnown: true},
		},
	}
	run2 := evals.Run{
		ID: "run-2", TaskID: "trace-answer", Status: "done",
		StartedAt: time.Now().UTC(),
		Attempts: []evals.Attempt{
			// Second counted attempt for beta; its model is unpriced, so the
			// summary's total cost becomes partial (CostKnown false).
			{Model: "m/beta", Score: evals.Score{Answered: true, Weighted: 0.75, GotHard: true},
				Metrics: evals.Metrics{Turns: 8, TokensIn: 40, TokensOut: 80}, Seconds: 30},
			// Gamma misses the hard expectation: it ties beta on weighted
			// average no more, but its lower hard rate must rank it after beta.
			{Model: "m/gamma", Score: evals.Score{Answered: true, Weighted: 0.5},
				Metrics: evals.Metrics{Turns: 7, TokensIn: 25, TokensOut: 35}, Seconds: 16, CostUSD: 0.1, CostKnown: true},
			// Unanswered attempts are missing measurements, not zeros: they must
			// not dilute averages, add cost, or poison CostKnown (delta stays
			// CostKnown true even though its only unknown-cost attempt is here).
			{Model: "m/alpha", Score: evals.Score{Answered: false},
				Metrics: evals.Metrics{Turns: 99, TokensIn: 999, TokensOut: 999}, Seconds: 5, CostUSD: 9.9},
			{Model: "m/delta", Score: evals.Score{Answered: false},
				Metrics: evals.Metrics{Turns: 99, TokensIn: 999, TokensOut: 999}, Seconds: 5, CostUSD: 9.9},
		},
	}
	if err := benchRuns.UpsertRun(run1); err != nil {
		t.Fatal(err)
	}
	if err := benchRuns.UpsertRun(run2); err != nil {
		t.Fatal(err)
	}

	s := &Server{mux: http.NewServeMux()}
	s.registerEvalsRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/evals/summary?taskId=trace-answer", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		TaskID    string                   `json:"taskId"`
		Summaries []evals.ModelTaskSummary `json:"summaries"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TaskID != "trace-answer" {
		t.Fatalf("taskId=%q want %q", body.TaskID, "trace-answer")
	}

	// Order proves all three sort keys: beta/gamma outscore delta/alpha,
	// beta outranks gamma on hard rate, delta outranks alpha on turns.
	want := []evals.ModelTaskSummary{
		{Model: "m/beta", Attempts: 2, AvgWeighted: 0.875, HardRate: 1, AvgTurns: 6, AvgSeconds: 20,
			TokensIn: 140, TokensOut: 280, TotalCostUSD: 0.5, CostKnown: false},
		{Model: "m/gamma", Attempts: 2, AvgWeighted: 0.75, HardRate: 0.5, AvgTurns: 6, AvgSeconds: 14,
			TokensIn: 55, TokensOut: 75, TotalCostUSD: 0.4, CostKnown: true},
		{Model: "m/delta", Attempts: 1, AvgWeighted: 0.5, HardRate: 1, AvgTurns: 3, AvgSeconds: 8,
			TokensIn: 20, TokensOut: 30, TotalCostUSD: 0.1, CostKnown: true},
		{Model: "m/alpha", Attempts: 1, AvgWeighted: 0.5, HardRate: 1, AvgTurns: 6, AvgSeconds: 20,
			TokensIn: 50, TokensOut: 60, TotalCostUSD: 0.2, CostKnown: true},
	}
	if len(body.Summaries) != len(want) {
		t.Fatalf("got %d summaries, want %d: %+v", len(body.Summaries), len(want), body.Summaries)
	}
	for i, w := range want {
		compareSummary(t, i, body.Summaries[i], w)
	}

	// A task nobody ran must come back empty, not as another task's numbers.
	req2 := httptest.NewRequest(http.MethodGet, "/api/evals/summary?taskId=no-such-task", nil)
	rr2 := httptest.NewRecorder()
	s.mux.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("unknown-task status=%d", rr2.Code)
	}
	var empty struct {
		TaskID    string                   `json:"taskId"`
		Summaries []evals.ModelTaskSummary `json:"summaries"`
	}
	if err := json.Unmarshal(rr2.Body.Bytes(), &empty); err != nil {
		t.Fatal(err)
	}
	if empty.TaskID != "no-such-task" || len(empty.Summaries) != 0 {
		t.Fatalf("taskId=%q summaries=%+v, want empty for no-such-task", empty.TaskID, empty.Summaries)
	}
}

// compareSummary asserts one summary cell by cell, so a mismatch names the
// field instead of dumping two structs.
func compareSummary(t *testing.T, i int, got, want evals.ModelTaskSummary) {
	t.Helper()
	if got.Model != want.Model {
		t.Fatalf("summary[%d].model=%q want %q (order broken)", i, got.Model, want.Model)
	}
	if got.Attempts != want.Attempts {
		t.Errorf("summary[%d] (%s): attempts=%d want %d", i, got.Model, got.Attempts, want.Attempts)
	}
	for _, c := range []struct {
		name      string
		got, want float64
	}{
		{"avgWeighted", got.AvgWeighted, want.AvgWeighted},
		{"hardRate", got.HardRate, want.HardRate},
		{"avgTurns", got.AvgTurns, want.AvgTurns},
		{"avgSeconds", got.AvgSeconds, want.AvgSeconds},
		{"totalCostUsd", got.TotalCostUSD, want.TotalCostUSD},
	} {
		if math.Abs(c.got-c.want) > 1e-9 {
			t.Errorf("summary[%d] (%s): %s=%v want %v", i, got.Model, c.name, c.got, c.want)
		}
	}
	if got.TokensIn != want.TokensIn || got.TokensOut != want.TokensOut {
		t.Errorf("summary[%d] (%s): tokens=(%d,%d) want (%d,%d)",
			i, got.Model, got.TokensIn, got.TokensOut, want.TokensIn, want.TokensOut)
	}
	if got.CostKnown != want.CostKnown {
		t.Errorf("summary[%d] (%s): costKnown=%v want %v", i, got.Model, got.CostKnown, want.CostKnown)
	}
}
