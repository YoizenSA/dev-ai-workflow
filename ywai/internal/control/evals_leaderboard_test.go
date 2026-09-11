package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/evals"
)

// swapLeaderboardStore points benchRuns at a fresh temp-dir store so real runs
// on this machine never leak into assertions (same pattern as the evals route
// tests), then seeds the given runs into it.
func swapLeaderboardStore(t *testing.T, runs ...evals.Run) {
	t.Helper()
	st, err := evals.OpenStore(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	benchRuns = st
	for _, run := range runs {
		if err := st.UpsertRun(run); err != nil {
			t.Fatalf("upsert run: %v", err)
		}
	}
}

func lbSeedRun() evals.Run {
	return evals.Run{
		ID:        "run-42",
		TaskID:    "t1",
		TaskName:  "Task One",
		Agent:     "dev",
		Status:    "done",
		StartedAt: time.Now().UTC().Add(-time.Hour),
		Attempts: []evals.Attempt{
			{Model: "m1", Score: evals.Score{Answered: true, Weighted: 0.9, GotHard: true}, CostKnown: true},
			{Model: "m2", Score: evals.Score{Answered: true, Weighted: 0.5}, CostKnown: false},
		},
	}
}

func TestHandleEvalLeaderboard_ReturnsRankedRows(t *testing.T) {
	swapLeaderboardStore(t, lbSeedRun())
	s := &Server{mux: http.NewServeMux()}
	s.registerLeaderboardRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/evals/leaderboard?taskId=t1&days=30", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Rows []evals.LeaderRow `json:"rows"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Rows) != 2 {
		t.Fatalf("rows=%d want 2 (body=%s)", len(body.Rows), rr.Body.String())
	}
	if body.Rows[0].Model != "m1" || body.Rows[0].AvgWeighted != 0.9 {
		t.Fatalf("top row = %+v, want m1 at 0.9", body.Rows[0])
	}
	if body.Rows[0].CostKnown != true || body.Rows[1].CostKnown != false {
		t.Fatalf("costKnown flags wrong: %+v", body.Rows)
	}
}

func TestHandleEvalLeaderboard_Empty(t *testing.T) {
	swapLeaderboardStore(t)
	s := &Server{mux: http.NewServeMux()}
	s.registerLeaderboardRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/evals/leaderboard", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body struct {
		Rows []evals.LeaderRow `json:"rows"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Rows) != 0 {
		t.Fatalf("rows=%d want 0 on an empty store", len(body.Rows))
	}
}

func TestHandleEvalRunDetail_FoundAndMissing(t *testing.T) {
	swapLeaderboardStore(t, lbSeedRun())
	s := &Server{mux: http.NewServeMux()}
	s.registerLeaderboardRoutes()

	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/evals/runs/run-42", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var run evals.Run
	if err := json.Unmarshal(rr.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if run.ID != "run-42" || run.TaskID != "t1" || len(run.Attempts) != 2 {
		t.Fatalf("run = %+v, want the seeded run-42 with 2 attempts", run)
	}

	rr = httptest.NewRecorder()
	s.mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/evals/runs/nope", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing run: status=%d want 404", rr.Code)
	}
}
