package control

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/evals"
)

// registerEvalsRoutes wires Agent Benchmarks + Session Analytics API.
func (s *Server) registerEvalsRoutes() {
	s.mux.HandleFunc("GET /api/evals/runs", s.handleEvalRuns)
	s.mux.HandleFunc("GET /api/evals/session-analytics", s.handleSessionAnalytics)
	s.mux.HandleFunc("POST /api/evals/baselines", s.handleSetEvalBaseline)
	s.mux.HandleFunc("DELETE /api/evals/baselines", s.handleClearEvalBaseline)
	s.registerBenchRoutes()
}

// handleEvalRuns returns benchmark runs, newest first.
func (s *Server) handleEvalRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := benchRuns.ListRuns()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if runs == nil {
		runs = []evals.Run{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

// handleEvalSummary aggregates every stored run of one task into per-model
// summaries, best model first. With ?baseline=auto|<runId> it also diffs each
// summary against that baseline run; "none" (or no param) keeps the Phase-1
// response shape exactly. The math lives in evals.Aggregate and
// evals.DiffModels so it stays unit-testable without a server.
func (s *Server) handleEvalSummary(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(r.URL.Query().Get("taskId"))
	runs, err := benchRuns.ListRuns()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	current := evals.Aggregate(runs, taskID)
	body := map[string]any{
		"taskId":    taskID,
		"summaries": current,
	}

	// A baseline that cannot resolve (none stored, or its run was purged) must
	// not take the summary down: the response just carries no baseline key.
	if mode := strings.TrimSpace(r.URL.Query().Get("baseline")); mode != "" && mode != "none" {
		runID := mode
		if mode == "auto" {
			runID, _ = benchRuns.GetBaseline(taskID)
		}
		if runID != "" {
			if baseRun, err := benchRuns.GetRun(runID); err == nil {
				body["baseline"] = map[string]any{
					"runId":  runID,
					"deltas": evals.DiffModels(current, evals.Aggregate([]evals.Run{baseRun}, taskID)),
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, body)
}

// handleSetEvalBaseline pins one stored run as the comparison anchor for a task.
func (s *Server) handleSetEvalBaseline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID string `json:"taskId"`
		RunID  string `json:"runId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body: " + err.Error()})
		return
	}
	req.TaskID = strings.TrimSpace(req.TaskID)
	req.RunID = strings.TrimSpace(req.RunID)
	if req.TaskID == "" || req.RunID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "taskId and runId are required"})
		return
	}
	// GetRun fails when the run is not addressable (never stored, or dropped by
	// retention): 404 tells the client the anchor does not exist rather than
	// pinning a ghost reference.
	if _, err := benchRuns.GetRun(req.RunID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "unknown run " + req.RunID})
		return
	}
	if err := benchRuns.SetBaseline(req.TaskID, req.RunID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"taskId": req.TaskID, "baselineRunId": req.RunID})
}

// handleClearEvalBaseline unpins a task's baseline. Clearing an unset baseline
// is still a success, so DELETE is idempotent.
func (s *Server) handleClearEvalBaseline(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimSpace(r.URL.Query().Get("taskId"))
	if taskID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "taskId is required"})
		return
	}
	if err := benchRuns.ClearBaseline(taskID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type analyticsCacheEntry struct {
	at   time.Time
	body *SessionAnalytics
}

var (
	analyticsCacheMu sync.Mutex
	analyticsCache   = map[string]analyticsCacheEntry{}
	// A full scan reads multi-gigabyte OpenCode databases and takes tens of seconds, so
	// a short TTL just guarantees every visit pays for it again. Session history changes
	// slowly enough that a stale-by-minutes view is fine, and the UI offers an explicit
	// regenerate for when it is not.
	analyticsTTL = 30 * time.Minute
)

func (s *Server) handleSessionAnalytics(w http.ResponseWriter, r *http.Request) {
	q := AnalyticsQuery{
		ProjectID:   strings.TrimSpace(r.URL.Query().Get("projectId")),
		Worktree:    strings.TrimSpace(r.URL.Query().Get("worktree")),
		Days:        queryInt(r, "days", 30),
		ToolsLimit:  queryInt(r, "toolsLimit", 30),
		SkillsLimit: queryInt(r, "skillsLimit", 50),
	}
	// days=0 means all time; negative coerced in LoadSessionAnalytics.
	if raw := r.URL.Query().Get("days"); raw == "0" || raw == "all" {
		q.Days = 0
	}

	cacheKey := strings.Join([]string{
		q.ProjectID, q.Worktree,
		strconv.Itoa(q.Days), strconv.Itoa(q.ToolsLimit), strconv.Itoa(q.SkillsLimit),
	}, "|")

	forceRefresh := r.URL.Query().Get("refresh") == "1" || r.URL.Query().Get("refresh") == "true"

	analyticsCacheMu.Lock()
	if !forceRefresh {
		if ent, ok := analyticsCache[cacheKey]; ok && time.Since(ent.at) < analyticsTTL {
			body := ent.body
			analyticsCacheMu.Unlock()
			writeJSON(w, http.StatusOK, body)
			return
		}
	}
	analyticsCacheMu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	// Always use the local OpenCode DB (OPENCODE_DB / XDG_DATA_HOME override).
	result, err := LoadSessionAnalytics(ctx, "", q)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": err.Error(),
		})
		return
	}

	analyticsCacheMu.Lock()
	analyticsCache[cacheKey] = analyticsCacheEntry{at: time.Now(), body: result}
	// prevent unbounded growth
	if len(analyticsCache) > 64 {
		for k, ent := range analyticsCache {
			if time.Since(ent.at) > analyticsTTL {
				delete(analyticsCache, k)
			}
		}
	}
	analyticsCacheMu.Unlock()

	writeJSON(w, http.StatusOK, result)
}

func queryInt(r *http.Request, key string, def int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}
