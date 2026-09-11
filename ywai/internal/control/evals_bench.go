package control

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/evals"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// benchRuns keeps benchmark runs on disk under the shared data dir. Runs are
// worth minutes of real model time each, so they survive a control-server
// restart rather than living only in memory.
var benchRuns = newBenchStore()

// newBenchStore opens the persistent run store at the shared data dir. The
// server has nothing to serve without it, so an open failure is fatal at
// startup.
func newBenchStore() *evals.Store {
	return newBenchStoreAt(config.DataDir())
}

// newBenchStoreAt builds a store rooted at dir (tests use a temp dir so real
// runs on the machine never leak into assertions).
func newBenchStoreAt(dir string) *evals.Store {
	s, err := evals.OpenStore(dir)
	if err != nil {
		panic(fmt.Sprintf("evals: open run store at %s: %v", dir, err))
	}
	return s
}

// benchInFlight guards against a second run starting while one is going: they would
// contend for the same CodeGraph index and provider, inflating the very timings the
// benchmark compares.
var benchInFlight sync.Mutex

func (s *Server) registerBenchRoutes() {
	s.mux.HandleFunc("GET /api/evals/tasks", s.handleEvalTasks)
	s.mux.HandleFunc("GET /api/evals/summary", s.handleEvalSummary)
	s.mux.HandleFunc("POST /api/evals/runs", s.handleStartEvalRun)
}

func (s *Server) handleEvalTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := evals.LoadTasks(projectRoot())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (s *Server) handleStartEvalRun(w http.ResponseWriter, r *http.Request) {
	var req evals.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body: " + err.Error()})
		return
	}
	if len(req.Models) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "pick at least one model"})
		return
	}
	if strings.TrimSpace(req.Provider) == "" {
		req.Provider = "opencode-admin"
	}
	if req.Rounds < 1 {
		req.Rounds = 1
	}

	task, err := evals.FindTask(projectRoot(), req.TaskID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}

	if !benchInFlight.TryLock() {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "a benchmark is already running; concurrent runs would distort each other's timings",
		})
		return
	}

	db, err := openOpenCodeDB(defaultOpenCodeDBPath())
	if err != nil {
		benchInFlight.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	run := evals.Run{
		ID:        fmt.Sprintf("run-%d", time.Now().UnixMilli()),
		TaskID:    task.ID,
		TaskName:  task.Name,
		Agent:     task.Agent,
		Provider:  req.Provider,
		Rounds:    req.Rounds,
		Models:    req.Models,
		Attempts:  []evals.Attempt{},
		Status:    "running",
		StartedAt: time.Now().UTC(),
	}
	// Best effort like the persist it replaces: a failed write must never fail
	// or kill a benchmark that costs real model time.
	_ = benchRuns.UpsertRun(run)

	runner := &evals.Runner{
		BaseURL: opencodeURLForBench(),
		DB:      db,
		// Each attempt is a full agent session; the ceiling is per-request, not per-run.
		Client: &http.Client{Timeout: 30 * time.Minute},
	}

	go func(run evals.Run) {
		// Own copy of the run: the handler below still reads and serializes
		// the original while this goroutine appends attempts to its local.
		defer benchInFlight.Unlock()
		defer db.Close()
		// Detached from the request: the browser must not have to stay open for a run
		// that takes tens of minutes.
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
		defer cancel()

		if err := runner.Preflight(ctx, task.Agent, req.Models[0], req.Provider); err != nil {
			run.Status = "failed"
			run.Error = err.Error()
			run.EndedAt = time.Now().UTC()
			_ = benchRuns.UpsertRun(run)
			return
		}

		_, err := runner.Execute(ctx, task, req, func(a evals.Attempt) {
			// Persist as each attempt lands so a long run is observable while it goes.
			a.Response = truncateResponse(a.Response)
			run.Attempts = append(run.Attempts, a)
			_ = benchRuns.UpsertRun(run)
		})
		run.Status = "done"
		if err != nil {
			run.Status = "failed"
			run.Error = err.Error()
		}
		run.EndedAt = time.Now().UTC()
		_ = benchRuns.UpsertRun(run)
	}(run)

	writeJSON(w, http.StatusAccepted, run)
}

// truncateResponse keeps enough of an answer to audit a score without letting the
// run history grow into megabytes of transcript. The cut backs up to a rune
// boundary so a multi-byte character is never split into invalid UTF-8.
func truncateResponse(s string) string {
	max := 4000
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max] + "\n…[truncated]"
}

// detectOpenCodeURL tries to find a running OpenCode server.
func detectOpenCodeURL() string {
	if url := strings.TrimSpace(os.Getenv("OPENCODE_URL")); url != "" {
		return strings.TrimRight(url, "/")
	}
	// Try common ports. OpenCode's default is 4096.
	ports := []string{"4096", "3000", "3001"}
	for _, port := range ports {
		url := fmt.Sprintf("http://localhost:%s", port)
		client := &http.Client{Timeout: 1 * time.Second}
		req, err := http.NewRequest(http.MethodGet, url+"/app", nil)
		if err != nil {
			continue
		}
		opencode.ApplyServerAuth(req)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return url
			}
		}
	}
	return ""
}

func opencodeURLForBench() string {
	if u := strings.TrimSpace(os.Getenv("OPENCODE_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	// Probe with credentials so a server that requires auth (opencode2) is
	// found, not just an unauthenticated v1.
	if u := detectOpenCodeURL(); u != "" {
		return u
	}
	return "http://localhost:4096"
}

func projectRoot() string {
	if d, err := os.Getwd(); err == nil {
		return d
	}
	return ""
}
