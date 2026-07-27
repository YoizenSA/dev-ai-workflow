package control

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// registerEvalsRoutes wires Agent Benchmarks stubs + Session Analytics API.
func (s *Server) registerEvalsRoutes() {
	s.mux.HandleFunc("GET /api/evals/runs", s.handleEvalRuns)
	s.mux.HandleFunc("GET /api/evals/session-analytics", s.handleSessionAnalytics)
}

// handleEvalRuns returns synthetic benchmark runs.
// The harness is not wired yet; keep an empty list so the UI does not 404.
func (s *Server) handleEvalRuns(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"runs": []any{}})
}

type analyticsCacheEntry struct {
	at   time.Time
	body *SessionAnalytics
}

var (
	analyticsCacheMu sync.Mutex
	analyticsCache   = map[string]analyticsCacheEntry{}
	analyticsTTL     = 45 * time.Second
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
