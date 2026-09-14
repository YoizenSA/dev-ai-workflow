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
	s := newBenchStoreAt(config.DataDir())
	// Runs persist across restarts by design; the goroutine driving them does
	// not. Anything still marked "running" at process start is an orphan that
	// would otherwise sit in the UI forever.
	if n := s.RecoverInterrupted("interrupted: the control server restarted while this run was in progress"); n > 0 {
		fmt.Printf("  Recovered %d interrupted benchmark run(s)\n", n)
	}
	return s
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

// benchActive maps the in-flight run id to its cancel func, so a stuck
// attempt (a hanging model request has a 30-minute ceiling and the whole run
// a 6-hour one) can be stopped instead of waited out. benchInFlight already
// serializes runs, so at most one id is ever registered here.
var (
	benchActiveMu sync.Mutex
	benchCancelID string
	benchCancelFn context.CancelFunc
)

func (s *Server) registerBenchRoutes() {
	s.mux.HandleFunc("GET /api/evals/tasks", s.handleEvalTasks)
	s.mux.HandleFunc("GET /api/evals/models-live", s.handleEvalModelsLive)
	s.mux.HandleFunc("GET /api/evals/summary", s.handleEvalSummary)
	s.mux.HandleFunc("POST /api/evals/runs", s.handleStartEvalRun)
	s.mux.HandleFunc("GET /api/evals/runs/{id}/live", s.handleEvalRunLive)
	s.mux.HandleFunc("POST /api/evals/runs/{id}/stop", s.handleStopEvalRun)
}

// handleStopEvalRun cancels the in-flight run, or closes out a persisted
// "running" record whose process died (an orphan that recovery missed only
// because the store was opened before it was written — rare, but the UI has
// no other way to clear it).
func (s *Server) handleStopEvalRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	benchActiveMu.Lock()
	cancel := benchCancelFn
	isActive := benchCancelID == id && cancel != nil
	benchActiveMu.Unlock()

	if isActive {
		cancel()
		writeJSON(w, http.StatusOK, map[string]any{"status": "stopping", "runId": id})
		return
	}

	run, err := benchRuns.GetRun(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "run not found"})
		return
	}
	if run.Status != "running" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "run is not running"})
		return
	}
	run.Status = "cancelled"
	run.Error = "cancelled by user"
	run.EndedAt = time.Now().UTC()
	if err := benchRuns.UpsertRun(run); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "cancelled", "runId": id})
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

	// The bench targets one environment: its server runs the attempts, its
	// database feeds metrics. Empty env preserves historical resolution.
	env := resolveEvalEnv(effectiveEvalEnvironments(), r.URL.Query().Get("env"))
	dbPath := defaultOpenCodeDBPath()
	if p := evalDBPath(env); p != "" {
		dbPath = p
	}
	db, err := openOpenCodeDB(dbPath)
	if err != nil {
		benchInFlight.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}

	run := evals.Run{
		ID:          fmt.Sprintf("run-%d", time.Now().UnixMilli()),
		Environment: strings.TrimSpace(r.URL.Query().Get("env")),
		TaskID:      task.ID,
		TaskName:    task.Name,
		Agent:       task.Agent,
		Provider:    req.Provider,
		Rounds:      req.Rounds,
		Models:      req.Models,
		Attempts:    []evals.Attempt{},
		Status:      "running",
		StartedAt:   time.Now().UTC(),
	}
	// Best effort like the persist it replaces: a failed write must never fail
	// or kill a benchmark that costs real model time.
	_ = benchRuns.UpsertRun(run)

	runner := &evals.Runner{
		BaseURL: evalServerURL(env),
		DB:      db,
		// Each attempt is a full agent session; the ceiling is per-request, not per-run.
		Client: &http.Client{Timeout: 30 * time.Minute},
	}
	// Publish each attempt's session the moment it exists so the UI can tail
	// the agent live (GET /api/evals/runs/{id}/live).
	baseURL := evalServerURL(env)
	runner.OnAttemptStart = func(model string, round int, sessionID string) {
		setBenchLive(&benchLiveAttempt{
			RunID:     run.ID,
			Model:     model,
			Round:     round,
			SessionID: sessionID,
			BaseURL:   baseURL,
		})
	}

	go func(run evals.Run) {
		// Own copy of the run: the handler below still reads and serializes
		// the original while this goroutine appends attempts to its local.
		defer benchInFlight.Unlock()
		defer db.Close()
		// Detached from the request: the browser must not have to stay open for a run
		// that takes tens of minutes.
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
		// Register the cancel func so handleStopEvalRun can stop a stuck run.
		benchActiveMu.Lock()
		benchCancelID = run.ID
		benchCancelFn = cancel
		benchActiveMu.Unlock()
		defer func() {
			benchActiveMu.Lock()
			if benchCancelID == run.ID {
				benchCancelID = ""
				benchCancelFn = nil
			}
			benchActiveMu.Unlock()
			cancel()
			// The live tail dies with the run.
			setBenchLive(nil)
		}()

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
		// A stop request surfaces here as the context's cancellation error:
		// record it as user intent, not as a failure.
		if ctx.Err() != nil {
			run.Status = "cancelled"
			run.Error = "cancelled by user"
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
