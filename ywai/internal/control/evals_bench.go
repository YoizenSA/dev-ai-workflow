package control

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/evals"
)

// benchStore keeps benchmark runs. Runs are worth minutes of real model time each,
// so they survive a control-server restart rather than living only in memory.
type benchStore struct {
	mu   sync.RWMutex
	path string
	runs []evals.Run
}

const benchHistoryLimit = 50

func newBenchStore() *benchStore {
	return newBenchStoreAt(config.DataDir())
}

// newBenchStoreAt builds a store rooted at dir (tests use a temp dir so real
// runs on the machine never leak into assertions).
func newBenchStoreAt(dir string) *benchStore {
	_ = os.MkdirAll(dir, 0755)
	s := &benchStore{path: filepath.Join(dir, "eval-runs.json")}
	if data, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(data, &s.runs)
	}
	if s.runs == nil {
		s.runs = []evals.Run{}
	}
	return s
}

func (s *benchStore) list() []evals.Run {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]evals.Run, len(s.runs))
	copy(out, s.runs)
	return out
}

func (s *benchStore) upsert(run evals.Run) {
	s.mu.Lock()
	replaced := false
	for i := range s.runs {
		if s.runs[i].ID == run.ID {
			s.runs[i] = run
			replaced = true
			break
		}
	}
	if !replaced {
		s.runs = append([]evals.Run{run}, s.runs...)
		if len(s.runs) > benchHistoryLimit {
			s.runs = s.runs[:benchHistoryLimit]
		}
	}
	data, _ := json.MarshalIndent(s.runs, "", "  ")
	path := s.path
	s.mu.Unlock()
	// Written outside the lock: a slow disk must not block an in-flight run's updates.
	_ = os.WriteFile(path, data, 0644)
}

var benchRuns = newBenchStore()

// benchInFlight guards against a second run starting while one is going: they would
// contend for the same CodeGraph index and provider, inflating the very timings the
// benchmark compares.
var benchInFlight sync.Mutex

func (s *Server) registerBenchRoutes() {
	s.mux.HandleFunc("GET /api/evals/tasks", s.handleEvalTasks)
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
	benchRuns.upsert(run)

	runner := &evals.Runner{
		BaseURL: opencodeURLForBench(),
		DB:      db,
		// Each attempt is a full agent session; the ceiling is per-request, not per-run.
		Client: &http.Client{Timeout: 30 * time.Minute},
	}

	go func() {
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
			benchRuns.upsert(run)
			return
		}

		_, err := runner.Execute(ctx, task, req, func(a evals.Attempt) {
			// Persist as each attempt lands so a long run is observable while it goes.
			a.Response = truncateResponse(a.Response)
			run.Attempts = append(run.Attempts, a)
			benchRuns.upsert(run)
		})
		run.Status = "done"
		if err != nil {
			run.Status = "failed"
			run.Error = err.Error()
		}
		run.EndedAt = time.Now().UTC()
		benchRuns.upsert(run)
	}()

	writeJSON(w, http.StatusAccepted, run)
}

// truncateResponse keeps enough of an answer to audit a score without letting the
// run history grow into megabytes of transcript.
func truncateResponse(s string) string {
	const max = 4000
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…[truncated]"
}

func opencodeURLForBench() string {
	if u := strings.TrimSpace(os.Getenv("OPENCODE_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	// Same discovery as the chat proxy: probe with credentials so a server that
	// requires auth (opencode2) is found, not just an unauthenticated v1.
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
