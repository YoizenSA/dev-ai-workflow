package control

import (
	"net/http"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/evals"
)

// registerLeaderboardRoutes wires the read-only leaderboard + run detail API.
func (s *Server) registerLeaderboardRoutes() {
	s.mux.HandleFunc("GET /api/evals/leaderboard", s.handleEvalLeaderboard)
	s.mux.HandleFunc("GET /api/evals/runs/{id}", s.handleEvalRunDetail)
}

// handleEvalLeaderboard ranks the models over the stored runs of one task,
// scoped to the last days days (absent or <=0 means all time). Read-only over
// the run store, so it is safe to call while a benchmark run is in flight.
func (s *Server) handleEvalLeaderboard(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(r.URL.Query().Get("taskId"))
	days := queryInt(r, "days", 0)
	runs, err := benchRuns.ListRuns()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	rows := evals.BuildLeaderboard(runs, taskID, days, time.Now().UTC())
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
}

// handleEvalRunDetail returns one full run, attempts and all. A missing id is
// a 404, not a 500: the client asked for something that is not there.
func (s *Server) handleEvalRunDetail(w http.ResponseWriter, r *http.Request) {
	run, err := benchRuns.GetRun(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, run)
}
