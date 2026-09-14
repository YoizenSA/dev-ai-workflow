package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/evals"
)

// Baseline route tests run against the frozen evals.Store contract: the temp
// dir keeps one test's baselines from leaking into another's, and the package
// global benchRuns is swapped for the duration of each test.

func newBaselineTestServer(t *testing.T) *Server {
	t.Helper()
	store, err := evals.OpenStore(t.TempDir())
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	benchRuns = store
	s := &Server{mux: http.NewServeMux()}
	s.registerEvalsRoutes()
	return s
}

// baselineTestRun is one completed run with a single answered attempt, so
// Aggregate yields exactly one summary per model with exact binary floats.
func baselineTestRun(id, taskID, model string, weighted float64, turns int, cost float64) evals.Run {
	return evals.Run{
		ID: id, TaskID: taskID, Status: "done",
		StartedAt: time.Now().UTC(),
		Attempts: []evals.Attempt{{
			Model:   model,
			Score:   evals.Score{Answered: true, Weighted: weighted, GotHard: true},
			Metrics: evals.Metrics{Turns: turns},
			CostUSD: cost, CostKnown: true,
		}},
	}
}

func postBaseline(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/evals/baselines", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	return rr
}

func TestSetEvalBaseline_PinRun(t *testing.T) {
	s := newBaselineTestServer(t)
	if err := benchRuns.UpsertRun(baselineTestRun("run-1", "task-a", "m/one", 0.75, 3, 0.5)); err != nil {
		t.Fatal(err)
	}

	rr := postBaseline(t, s, `{"taskId":"task-a","runId":"run-1"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		TaskID        string `json:"taskId"`
		BaselineRunID string `json:"baselineRunId"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.TaskID != "task-a" || body.BaselineRunID != "run-1" {
		t.Fatalf("body=%+v want taskId=task-a baselineRunId=run-1", body)
	}
	got, err := benchRuns.GetBaseline("task-a")
	if err != nil {
		t.Fatal(err)
	}
	if got != "run-1" {
		t.Fatalf("stored baseline=%q want %q", got, "run-1")
	}
}

func TestSetEvalBaseline_UnknownRun(t *testing.T) {
	s := newBaselineTestServer(t)
	rr := postBaseline(t, s, `{"taskId":"task-a","runId":"run-nope"}`)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", rr.Code, rr.Body.String())
	}
	got, err := benchRuns.GetBaseline("task-a")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("baseline=%q want unset after rejected pin", got)
	}
}

func TestSetEvalBaseline_MissingFields(t *testing.T) {
	s := newBaselineTestServer(t)
	for _, body := range []string{`{"runId":"run-1"}`, `{"taskId":"task-a"}`, `{}`, `not json`} {
		if rr := postBaseline(t, s, body); rr.Code != http.StatusBadRequest {
			t.Errorf("body %q: status=%d want 400", body, rr.Code)
		}
	}
}

func TestClearEvalBaseline_Unpins(t *testing.T) {
	s := newBaselineTestServer(t)
	if err := benchRuns.UpsertRun(baselineTestRun("run-1", "task-a", "m/one", 0.75, 3, 0.5)); err != nil {
		t.Fatal(err)
	}
	if err := benchRuns.SetBaseline("task-a", "run-1"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/evals/baselines?taskId=task-a", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want 204", rr.Code)
	}
	if got, _ := benchRuns.GetBaseline("task-a"); got != "" {
		t.Fatalf("baseline=%q want unset", got)
	}

	// Idempotent: clearing an already-unset baseline is still a success.
	rr2 := httptest.NewRecorder()
	s.mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodDelete, "/api/evals/baselines?taskId=task-a", nil))
	if rr2.Code != http.StatusNoContent {
		t.Fatalf("second DELETE status=%d want 204", rr2.Code)
	}
}

func TestClearEvalBaseline_MissingTaskId(t *testing.T) {
	s := newBaselineTestServer(t)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/evals/baselines", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rr.Code)
	}
}

// summaryResponse mirrors the summary endpoint's shape; Baseline being nil
// proves the response carried no baseline key.
type summaryResponse struct {
	TaskID    string                   `json:"taskId"`
	Summaries []evals.ModelTaskSummary `json:"summaries"`
	Baseline  *struct {
		RunID  string             `json:"runId"`
		Deltas []evals.ModelDelta `json:"deltas"`
	} `json:"baseline"`
}

func getSummary(t *testing.T, s *Server, query string) (*httptest.ResponseRecorder, summaryResponse) {
	t.Helper()
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/evals/summary"+query, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body summaryResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return rr, body
}

// seedTwoRuns stores an older and a newer run of task-a, both model m/one, so
// current aggregates average the two runs while a baseline over run-old alone
// yields exact binary-float deltas: weighted 0.625-0.5=0.125, hard 1-1=0
// (compared-and-equal, so a non-nil pointer to 0), turns 4-3=1, cost 0.75-0.25=0.5.
func seedTwoRuns(t *testing.T) *Server {
	t.Helper()
	s := newBaselineTestServer(t)
	seed := []evals.Run{
		baselineTestRun("run-old", "task-a", "m/one", 0.5, 3, 0.25),
		baselineTestRun("run-new", "task-a", "m/one", 0.75, 5, 0.5),
	}
	for _, run := range seed {
		if err := benchRuns.UpsertRun(run); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestEvalSummary_NoBaselineParamKeepsPhase1Shape(t *testing.T) {
	s := seedTwoRuns(t)
	rr, body := getSummary(t, s, "?taskId=task-a")
	// Phase-1 shape, byte-identical: exactly the two original top-level keys,
	// no baseline key sneaking in.
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys["taskId"] == nil || keys["summaries"] == nil {
		t.Fatalf("top-level keys %v want exactly [summaries taskId]", keys)
	}
	if body.Baseline != nil {
		t.Fatalf("baseline=%+v want absent", body.Baseline)
	}
	if len(body.Summaries) != 1 || body.Summaries[0].Model != "m/one" {
		t.Fatalf("summaries=%+v", body.Summaries)
	}
}

func TestEvalSummary_BaselineAuto(t *testing.T) {
	s := seedTwoRuns(t)
	if err := benchRuns.SetBaseline("task-a", "run-old"); err != nil {
		t.Fatal(err)
	}
	_, body := getSummary(t, s, "?taskId=task-a&baseline=auto")
	if body.Baseline == nil {
		t.Fatal("baseline key absent, want resolved auto baseline")
	}
	if body.Baseline.RunID != "run-old" {
		t.Fatalf("baseline.runId=%q want run-old", body.Baseline.RunID)
	}
	if len(body.Baseline.Deltas) != 1 {
		t.Fatalf("deltas=%+v want 1 entry", body.Baseline.Deltas)
	}
	d := body.Baseline.Deltas[0]
	if d.Model != "m/one" {
		t.Fatalf("delta.model=%q want m/one", d.Model)
	}
	for name, got := range map[string]*float64{
		"weightedDelta": d.WeightedDelta,
		"hardDelta":     d.HardDelta,
		"turnsDelta":    d.TurnsDelta,
		"costDeltaUsd":  d.CostDeltaUSD,
	} {
		if got == nil {
			t.Fatalf("%s=nil, want a computed value (model exists on both sides)", name)
		}
	}
	if *d.WeightedDelta != 0.125 {
		t.Errorf("weightedDelta=%v want 0.125", *d.WeightedDelta)
	}
	// Hard rate is unchanged: compared-and-equal must be a pointer to 0.
	if *d.HardDelta != 0 {
		t.Errorf("hardDelta=%v want 0", *d.HardDelta)
	}
	if *d.TurnsDelta != 1 {
		t.Errorf("turnsDelta=%v want 1", *d.TurnsDelta)
	}
	if *d.CostDeltaUSD != 0.5 {
		t.Errorf("costDeltaUsd=%v want 0.5", *d.CostDeltaUSD)
	}
}

func TestEvalSummary_AutoWithoutStoredBaseline(t *testing.T) {
	s := seedTwoRuns(t)
	_, body := getSummary(t, s, "?taskId=task-a&baseline=auto")
	if body.Baseline != nil {
		t.Fatalf("baseline=%+v want absent when no baseline is stored", body.Baseline)
	}
}

func TestEvalSummary_ExplicitRunId(t *testing.T) {
	s := seedTwoRuns(t)
	_, body := getSummary(t, s, "?taskId=task-a&baseline=run-old")
	if body.Baseline == nil || body.Baseline.RunID != "run-old" {
		t.Fatalf("baseline=%+v want run-old", body.Baseline)
	}
	if len(body.Baseline.Deltas) != 1 || body.Baseline.Deltas[0].WeightedDelta == nil || *body.Baseline.Deltas[0].WeightedDelta != 0.125 {
		t.Fatalf("deltas=%+v want m/one weightedDelta 0.125", body.Baseline.Deltas)
	}
}

// An explicit runId that no longer resolves degrades gracefully: "only when a
// baseline resolves" — the summary still serves, without a baseline key.
func TestEvalSummary_ExplicitUnknownRunIdGraceful(t *testing.T) {
	s := seedTwoRuns(t)
	_, body := getSummary(t, s, "?taskId=task-a&baseline=run-nope")
	if body.Baseline != nil {
		t.Fatalf("baseline=%+v want absent for unknown runId", body.Baseline)
	}
}

func TestEvalSummary_NoneSuppressesStoredBaseline(t *testing.T) {
	s := seedTwoRuns(t)
	if err := benchRuns.SetBaseline("task-a", "run-old"); err != nil {
		t.Fatal(err)
	}
	_, body := getSummary(t, s, "?taskId=task-a&baseline=none")
	if body.Baseline != nil {
		t.Fatalf("baseline=%+v want absent for baseline=none", body.Baseline)
	}
}
