package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type State string

const (
	StatePending     State = "pending"
	StatePrereq      State = "prereq_check"
	StateInstalling  State = "installing"
	StateProbing     State = "probing"
	StateConfiguring State = "configuring"
	StateDone        State = "done"
	StateFailed      State = "failed"
)

type Progress struct {
	Step    string `json:"step"`
	Percent int    `json:"percent"`
	Message string `json:"message"`
}

type Result struct {
	Tools      []string `json:"tools"`
	DurationMs int64    `json:"duration_ms"`
	ConfigPath string   `json:"config_path"`
}

type JobError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Step    string `json:"step"`
	Details string `json:"details,omitempty"`
}

var ErrJobInProgress = errors.New("install_in_progress")

// installFn is a package-level seam used by tests to substitute the
// production Install pipeline with a fake. Tests assign to it via
// withInstallFn; production code never mutates it. installFnMu guards
// concurrent reads (in the install goroutine) and writes (in test
// setup / teardown) so the install pipeline pointer is race-free.
var (
	installFnMu sync.RWMutex
	installFn   func(context.Context, CatalogEntry, InstallOptions) ([]string, error) = Install
)

// loadInstallFn returns the current installFn under the read lock. The
// returned function value is a value copy; calling it does not touch
// the package variable, so the call is race-free even if a writer
// swaps installFn concurrently.
func loadInstallFn() func(context.Context, CatalogEntry, InstallOptions) ([]string, error) {
	installFnMu.RLock()
	defer installFnMu.RUnlock()
	return installFn
}

// setInstallFn atomically replaces installFn (used by tests).
func setInstallFn(fn func(context.Context, CatalogEntry, InstallOptions) ([]string, error)) {
	installFnMu.Lock()
	defer installFnMu.Unlock()
	installFn = fn
}

// JobInProgressError is returned by JobManager.Start when another install
// for the same (entryID, target) is still active. JobID is the id of the
// in-progress job, so callers can point the user at the existing run.
// errors.Is(err, ErrJobInProgress) still matches because Unwrap returns
// the base sentinel.
type JobInProgressError struct {
	JobID string
}

func (e *JobInProgressError) Error() string {
	return fmt.Sprintf("install_in_progress: job %s is already active", e.JobID)
}

func (e *JobInProgressError) Unwrap() error {
	return ErrJobInProgress
}

// WithInstallFn temporarily replaces the package installFn and restores
// it on test cleanup. Intended for tests in other packages that need to
// stub the install pipeline; the in-package withInstallFn in job_test.go
// uses the defer-restore pattern instead.
func WithInstallFn(t *testing.T, fn func(ctx context.Context, entry CatalogEntry, opts InstallOptions) ([]string, error)) {
	t.Helper()
	orig := loadInstallFn()
	setInstallFn(fn)
	t.Cleanup(func() { setInstallFn(orig) })
}

type Broadcaster interface {
	Broadcast(msg []byte)
}

type Job struct {
	ID          string    `json:"install_id"`
	EntryID     string    `json:"entry_id"`
	TargetAgent string    `json:"target_agent"`
	State       State     `json:"state"`
	StartedAt   time.Time `json:"started_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Progress    Progress  `json:"progress"`
	Result      *Result   `json:"result,omitempty"`
	Error       *JobError `json:"error,omitempty"`

	mu     sync.RWMutex
	cancel context.CancelFunc
}

// Snapshot returns the job's current state under the job's read lock.
// It exists so polling / status callers can read state without racing
// the runJob goroutine that mutates it. Returns the empty State if the
// job has been GC'd between the Get() and the Snapshot() call.
func (j *Job) Snapshot() State {
	if j == nil {
		return ""
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.State
}

type JobManager struct {
	hub       Broadcaster
	mu        sync.Mutex
	jobs      map[string]*Job
	byKey     map[string]string
	seq       atomic.Uint64
	retention time.Duration
}

func NewJobManager(hub Broadcaster) *JobManager {
	return &JobManager{
		hub:       hub,
		jobs:      map[string]*Job{},
		byKey:     map[string]string{},
		retention: 1 * time.Hour,
	}
}

// Start kicks off an async install. Returns the freshly-created Job
// (State=pending) or ErrJobInProgress if another install for the same
// (entryID, target) is still active. The returned job's goroutine drives
// the state machine and emits progress / completed / failed broadcasts.
func (m *JobManager) Start(ctx context.Context, entry CatalogEntry, target string, creds map[string]string) (*Job, error) {
	key := entry.ID + "|" + target

	m.mu.Lock()
	if existingID, ok := m.byKey[key]; ok {
		m.mu.Unlock()
		if existing, ok2 := m.jobs[existingID]; ok2 {
			existing.mu.RLock()
			st := existing.State
			existing.mu.RUnlock()
			if st != StateDone && st != StateFailed {
				return nil, &JobInProgressError{JobID: existingID}
			}
		}
	} else {
		// nothing to do; fall through to the create path
	}

	m.seq.Add(1)
	id := fmt.Sprintf("mcp-job-%d", m.seq.Load())
	jobCtx, cancel := context.WithCancel(ctx)
	now := time.Now()
	job := &Job{
		ID:          id,
		EntryID:     entry.ID,
		TargetAgent: target,
		State:       StatePending,
		StartedAt:   now,
		UpdatedAt:   now,
		cancel:      cancel,
	}
	m.jobs[id] = job
	m.byKey[key] = id
	m.mu.Unlock()

	// Emit the initial "pending" state so subscribers see the job before
	// the install goroutine even starts. Without this, the first
	// observable step would be "prereq_check" and the pending phase
	// would be invisible on the wire.
	m.broadcast(id, "mcp-install.progress", map[string]any{
		"step":    string(StatePending),
		"percent": 0,
		"message": "install queued",
	})

	go func() {
		defer cancel()
		m.runJob(jobCtx, job, entry, target, creds, key)
	}()
	return job, nil
}

// Get returns the job by id, or (nil, false) if no such job exists.
func (m *JobManager) Get(id string) (*Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	return j, ok
}

// List returns a snapshot of every tracked job. Order is unspecified.
func (m *JobManager) List() []*Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, j)
	}
	return out
}

// gc drops terminal jobs whose UpdatedAt is older than the retention
// window and prunes byKey entries that no longer point at a live job.
// Returns the number of jobs removed.
func (m *JobManager) gc() int {
	cutoff := time.Now().Add(-m.retention)
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, j := range m.jobs {
		j.mu.RLock()
		terminal := j.State == StateDone || j.State == StateFailed
		old := j.UpdatedAt.Before(cutoff)
		j.mu.RUnlock()
		if terminal && old {
			delete(m.jobs, id)
			n++
		}
	}
	// Sweep byKey for any entry pointing at a job we just removed (or
	// that was removed by some other path). Keeps the conflict map
	// honest so a fresh Start after GC isn't blocked by a stale key.
	for key, jid := range m.byKey {
		if _, ok := m.jobs[jid]; !ok {
			delete(m.byKey, key)
		}
	}
	return n
}

// runJob executes the install pipeline under the given context, routing
// every Progress callback through the JobManager's progressFn bridge so
// state transitions and broadcasts stay consistent.
func (m *JobManager) runJob(ctx context.Context, job *Job, entry CatalogEntry, target string, creds map[string]string, key string) {
	startedAt := time.Now()
	var lastStep string

	progressFn := func(step string, percent int, message string) {
		var st State
		switch step {
		case StepPrereq:
			st = StatePrereq
		case StepInstalling:
			st = StateInstalling
		case StepProbing:
			st = StateProbing
		case StepConfiguring, StepFinalizing:
			// Finalizing is the terminal "we're done" step; the job
			// enters StateConfiguring until the install return sets
			// StateDone, so subscribers see a coherent finalization.
			st = StateConfiguring
		default:
			st = StatePending
		}
		job.mu.Lock()
		job.State = st
		job.Progress = Progress{Step: step, Percent: percent, Message: message}
		job.UpdatedAt = time.Now()
		lastStep = step
		job.mu.Unlock()

		m.broadcast(job.ID, "mcp-install.progress", map[string]any{
			"step":    step,
			"percent": percent,
			"message": message,
		})
	}

	opts := InstallOptions{
		Credentials: creds,
		Progress:    progressFn,
		Target:      target,
		EntryID:     entry.ID,
	}
	tools, err := loadInstallFn()(ctx, entry, opts)

	job.mu.Lock()
	job.UpdatedAt = time.Now()
	if err != nil {
		step := lastStep
		if step == "" {
			step = string(StatePending)
		}
		job.State = StateFailed
		job.Error = &JobError{
			Code:    codeFromErr(err),
			Message: err.Error(),
			Step:    step,
		}
		job.mu.Unlock()
		m.broadcast(job.ID, "mcp-install.failed", map[string]any{
			"error": job.Error,
		})
	} else {
		durationMs := time.Since(startedAt).Milliseconds()
		job.State = StateDone
		job.Result = &Result{
			Tools:      tools,
			DurationMs: durationMs,
			ConfigPath: "",
		}
		job.mu.Unlock()
		m.broadcast(job.ID, "mcp-install.completed", map[string]any{
			"tools":       tools,
			"duration_ms": durationMs,
			"config_path": "",
		})
	}

	// Release the conflict slot so a new install for the same
	// (entry, target) can start without being rejected as in-progress.
	m.mu.Lock()
	if m.byKey[key] == job.ID {
		delete(m.byKey, key)
	}
	m.mu.Unlock()
}

// broadcast serializes msgType + data into the WS wire format and
// dispatches it through the hub. Silently no-ops if no hub is wired.
func (m *JobManager) broadcast(installID, msgType string, data any) {
	if m.hub == nil {
		return
	}
	msg := map[string]any{
		"type":       msgType,
		"ts":         time.Now().Format(time.RFC3339),
		"install_id": installID,
		"data":       data,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	m.hub.Broadcast(b)
}

// codeFromErr maps a sentinel install error to the public API error
// code. Unrecognized errors collapse to "internal" so the API surface
// stays a closed enum of known failure modes.
func codeFromErr(err error) string {
	switch {
	case errors.Is(err, ErrPrereqMissing):
		return "prereq_missing"
	case errors.Is(err, ErrInstallTimeout):
		return "install_timeout"
	case errors.Is(err, ErrInstallNonZero):
		return "install_nonzero"
	case errors.Is(err, ErrProbeTimeout):
		return "probe_timeout"
	case errors.Is(err, ErrProbeNoTools):
		return "probe_no_tools"
	case errors.Is(err, ErrProbeUnreachable):
		return "probe_unreachable"
	case errors.Is(err, ErrConfigWriteFailed):
		return "config_write_failed"
	default:
		return "internal"
	}
}
