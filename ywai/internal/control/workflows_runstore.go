package control

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// runRecord tracks one in-progress or finished workflow run. Minimal: status +
// accumulated output + timing. Persisted as JSONL under the workflow's data dir
// so a reload can show the last run's log.
type runRecord struct {
	Workflow  string    `json:"workflow"`
	RunID     string    `json:"runId"`
	Status    string    `json:"status"` // running | done | error | cancelled
	ExitCode  int       `json:"exitCode,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt,omitempty"`
	Output    string    `json:"output"` // accumulated stdout/stderr
	// cancel kills the running opencode process. Not serialized.
	cancel context.CancelFunc `json:"-"`
}

// runStore tracks in-memory run records per workflow and is the source of truth
// for the idempotency guard (one active run per workflow, like missions'
// runningMissions). Log lines are appended to the record's Output.
type runStore struct {
	mu     sync.Mutex
	runs   map[string]*runRecord // key: workflow name -> active run
	logDir string                // where run logs are persisted (optional)
}

func newRunStore(logDir string) *runStore {
	return &runStore{
		runs:   make(map[string]*runRecord),
		logDir: logDir,
	}
}

// start records a new run and refuses if one is already active for the workflow.
// Returns the record, or an error when a run is already in progress. The cancel
// func kills the opencode process when the user stops the run.
func (rs *runStore) start(workflow, runID string, cancel context.CancelFunc) (*runRecord, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if existing, ok := rs.runs[workflow]; ok && existing.Status == "running" {
		return existing, fmt.Errorf("workflow %q is already running (run %s)", workflow, existing.RunID)
	}
	rec := &runRecord{
		Workflow:  workflow,
		RunID:     runID,
		Status:    "running",
		StartedAt: time.Now(),
		cancel:    cancel,
	}
	rs.runs[workflow] = rec
	return rec, nil
}

// cancel stops the active run for a workflow by invoking its cancel func and
// marking it cancelled. Returns false if there's no active run to cancel.
func (rs *runStore) cancel(workflow string) bool {
	rs.mu.Lock()
	rec, ok := rs.runs[workflow]
	rs.mu.Unlock()
	if !ok || rec.Status != "running" {
		return false
	}
	if rec.cancel != nil {
		rec.cancel()
	}
	return true
}

// appendOutput adds a chunk to the active run's accumulated log.
func (rs *runStore) appendOutput(workflow, chunk string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rec, ok := rs.runs[workflow]; ok {
		rec.Output += chunk
	}
}

// finish marks the run as done (or errored) and persists the log to disk.
func (rs *runStore) finish(workflow string, exitCode int, runErr error) {
	rs.mu.Lock()
	rec, ok := rs.runs[workflow]
	if !ok {
		rs.mu.Unlock()
		return
	}
	rec.Status = "done"
	rec.ExitCode = exitCode
	rec.EndedAt = time.Now()
	if runErr != nil {
		rec.Status = "error"
		rec.Error = runErr.Error()
	}
	if exitCode != 0 && rec.Status == "done" {
		rec.Status = "error"
	}
	rs.mu.Unlock()
	rs.persist(rec)
}

// isRunning reports whether a workflow has an active run.
func (rs *runStore) isRunning(workflow string) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rec, ok := rs.runs[workflow]
	return ok && rec.Status == "running"
}

// persist writes a finished run's log to <logDir>/<workflow>-<runId>.log so the
// output survives a server restart. Best-effort: a missing/unwritable logDir is
// silently ignored (the in-memory record is still kept for the session).
func (rs *runStore) persist(rec *runRecord) {
	if rs.logDir == "" {
		return
	}
	if err := os.MkdirAll(rs.logDir, 0o755); err != nil {
		return
	}
	path := filepath.Join(rs.logDir, fmt.Sprintf("%s-%s.log", rec.Workflow, rec.RunID))
	_ = os.WriteFile(path, []byte(rec.Output), 0o644)
}
