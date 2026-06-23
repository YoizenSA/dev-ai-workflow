package mcp

// job_test.go — TDD slice 5 of the "Real MCP Install" plan.
//
// Pinned symbols (all must be added to ywai/internal/mcp/job.go by @dev):
//
//	State, StatePending, StatePrereq, StateInstalling, StateProbing,
//	StateConfiguring, StateDone, StateFailed
//	Progress, Result, JobError
//	Job
//	Broadcaster interface
//	ErrJobInProgress sentinel
//	JobManager, NewJobManager, (*JobManager).Start, Get, List
//	(*JobManager).gc  (unexported; pinned for the GC test)
//
// Pinned behavior:
//   - install_id is prefixed with "mcp-job-" (pin the prefix for testability).
//   - Each state change emits a WS event via hub.Broadcast(msg).
//   - Progress event shape: {"type":"mcp-install.progress","ts":<RFC3339>,
//     "install_id":<id>,"data":{"step":<state>,"percent":<int>,"message":<str>}}
//   - Completed event shape: {"type":"mcp-install.completed","ts":<RFC3339>,
//     "install_id":<id>,"data":{"tools":[...],"duration_ms":<int64>,"config_path":<str>}}
//   - Failed event shape: {"type":"mcp-install.failed","ts":<RFC3339>,
//     "install_id":<id>,"data":{"error":{"code":<str>,"message":<str>,
//     "step":<str>,"details":<str>}}}
//
// Mocking strategy: this test file uses Option A from the brief — it
// references a package-level `installFn` variable that JobManager.Start
// must call instead of calling Install directly. @dev must add to job.go:
//
//	var installFn func(ctx context.Context, entry CatalogEntry,
//	    opts InstallOptions) ([]string, error) = Install
//
// Tests assign to installFn to override the pipeline with a fake that
// drives Progress callbacks synchronously. The default value (= Install)
// is only used in production builds; tests always override it.
//
// RED state: this file references symbols that do not exist yet. The
// expected compile errors are (one per pinned symbol):
//
//	undefined: installFn
//	undefined: Broadcaster
//	undefined: State
//	undefined: StatePending
//	undefined: StatePrereq
//	undefined: StateInstalling
//	undefined: StateProbing
//	undefined: StateConfiguring
//	undefined: StateDone
//	undefined: StateFailed
//	undefined: Progress
//	undefined: Result
//	undefined: JobError
//	undefined: Job
//	undefined: ErrJobInProgress
//	undefined: JobManager
//	undefined: NewJobManager
//	undefined: (*JobManager).Start
//	undefined: (*JobManager).Get
//	undefined: (*JobManager).List
//	undefined: (*JobManager).gc
//
// All test bodies in this file are pinned to the public contract only
// (Start/Get/List, conflict detection, broadcast shape, retention) — not
// to internal data structures or synchronization primitives.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// ─── helpers ──────────────────────────────────────────────────────────────

// jobMessage is the top-level shape of a broadcast message. The "data"
// field is left as RawMessage because its shape varies by event type
// (progress / completed / failed).
type jobMessage struct {
	Type      string          `json:"type"`
	Ts        string          `json:"ts"`
	InstallID string          `json:"install_id"`
	Data      json.RawMessage `json:"data"`
}

// progressData is the shape of the "data" field for progress events.
type progressData struct {
	Step    string `json:"step"`
	Percent int    `json:"percent"`
	Message string `json:"message"`
}

// completedData is the shape of the "data" field for completed events.
type completedData struct {
	Tools      []string `json:"tools"`
	DurationMs int64    `json:"duration_ms"`
	ConfigPath string   `json:"config_path"`
}

// failedData is the shape of the "data" field for failed events.
type failedData struct {
	Error JobError `json:"error"`
}

// captureHub is a test Broadcaster that records every Broadcast call. It
// captures both the raw bytes (for shape/encoding inspection) and the
// decoded messages (for ordered inspection). The mock is safe for
// concurrent use because JobManager.Start launches a goroutine that
// emits broadcasts from a different goroutine than the test.
type captureHub struct {
	mu   sync.Mutex
	raw  [][]byte
	msgs []jobMessage
}

func newCaptureHub() *captureHub { return &captureHub{} }

// Broadcast implements the Broadcaster interface.
func (h *captureHub) Broadcast(b []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.raw = append(h.raw, append([]byte(nil), b...))
	var m jobMessage
	if err := json.Unmarshal(b, &m); err == nil {
		h.msgs = append(h.msgs, m)
	}
}

// snapshotMsgs returns a copy of the decoded messages so the caller can
// iterate without holding the lock.
func (h *captureHub) snapshotMsgs() []jobMessage {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]jobMessage, len(h.msgs))
	copy(out, h.msgs)
	return out
}

// progressSteps extracts the step names from the progress events in the
// order they were emitted, excluding the terminal "completed" / "failed"
// event. Used by the transitions test to assert the sequence.
func (h *captureHub) progressSteps() []string {
	var out []string
	for _, m := range h.snapshotMsgs() {
		if m.Type != "mcp-install.progress" {
			continue
		}
		var d progressData
		if err := json.Unmarshal(m.Data, &d); err != nil {
			continue
		}
		out = append(out, d.Step)
	}
	return out
}

// withInstallFn temporarily replaces the package-level installFn and
// returns a restore function (intended for use with defer). The dev's
// job.go must declare installFn at package scope so this assignment
// mutates the same variable the production JobManager uses.
func withInstallFn(t *testing.T, fn func(ctx context.Context, entry CatalogEntry, opts InstallOptions) ([]string, error)) func() {
	t.Helper()
	orig := installFn
	installFn = fn
	return func() { installFn = orig }
}

// fakeEntry returns a minimal CatalogEntry for tests. The Command and
// Type fields are not used because installFn is mocked; the entry exists
// only so JobManager can build a Job with an ID and an entryID.
func fakeEntry(id string) CatalogEntry {
	return CatalogEntry{
		ID:      id,
		Name:    id,
		Type:    "local",
		Command: []string{"true"},
	}
}

// fakeInstallOK is a quick-OK installFn that drives the Progress
// callback through the full pipeline (all five steps) and returns the
// given tool list. The JobManager's Progress bridge observes each call
// and updates the state + broadcasts.
func fakeInstallOK(tools []string) func(ctx context.Context, entry CatalogEntry, opts InstallOptions) ([]string, error) {
	return func(ctx context.Context, entry CatalogEntry, opts InstallOptions) ([]string, error) {
		if opts.Progress != nil {
			opts.Progress(StepPrereq, 10, "prereq ok")
			opts.Progress(StepInstalling, 20, "install ok")
			opts.Progress(StepProbing, 70, "probe ok")
			opts.Progress(StepConfiguring, 95, "config ok")
			opts.Progress(StepFinalizing, 100, "done")
		}
		return tools, nil
	}
}

// fakeInstallErr is a quick-FAIL installFn that returns the given error
// after a single Progress call (prereq). The JobManager must translate
// the error into a StateFailed transition and a "mcp-install.failed"
// broadcast.
func fakeInstallErr(sentinel error) func(ctx context.Context, entry CatalogEntry, opts InstallOptions) ([]string, error) {
	return func(ctx context.Context, entry CatalogEntry, opts InstallOptions) ([]string, error) {
		if opts.Progress != nil {
			opts.Progress(StepPrereq, 10, "prereq failed")
		}
		return nil, sentinel
	}
}

// fakeInstallBlock returns an installFn that blocks until release is
// closed or ctx is cancelled. Used by the conflict test to keep the
// first job "active" while the second Start is issued.
func fakeInstallBlock(release <-chan struct{}) func(ctx context.Context, entry CatalogEntry, opts InstallOptions) ([]string, error) {
	return func(ctx context.Context, entry CatalogEntry, opts InstallOptions) ([]string, error) {
		if opts.Progress != nil {
			opts.Progress(StepPrereq, 10, "prereq ok")
		}
		select {
		case <-release:
			return []string{"tool1"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// waitForState polls Get until the job reaches want or timeout elapses.
// Returns the final *Job. The test treats this as the synchronization
// point between the Start call (which returns immediately) and the
// goroutine that performs the install + state transitions.
func waitForState(t *testing.T, m *JobManager, id string, want State, timeout time.Duration) *Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last *Job
	for time.Now().Before(deadline) {
		if job, ok := m.Get(id); ok {
			last = job
			job.mu.RLock()
			state := job.State
			job.mu.RUnlock()
			if state == want {
				return job
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	state := State("<unknown>")
	if last != nil {
		last.mu.RLock()
		state = last.State
		last.mu.RUnlock()
	}
	t.Fatalf("waitForState: job %s did not reach state %q within %s; final state = %q", id, want, timeout, state)
	return nil
}

// isValidTimestamp asserts the ts field is parseable as RFC3339. The
// brief pins RFC3339 (no sub-second precision). If @dev's broadcast
// uses time.RFC3339Nano, the test will surface the mismatch and the
// pin can be relaxed.
func isValidTimestamp(t *testing.T, ts string) {
	t.Helper()
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("broadcast ts %q is not a valid RFC3339 timestamp: %v", ts, err)
	}
}

// ─── Test 1: TestNewJobManager_StartsEmpty ───────────────────────────────

// TestNewJobManager_StartsEmpty pins the trivial constructor contract:
// a freshly built JobManager has no jobs in its in-memory store. List()
// returns an empty slice (not nil) and the length is zero.
func TestNewJobManager_StartsEmpty(t *testing.T) {
	hub := newCaptureHub()
	m := NewJobManager(hub)
	if m == nil {
		t.Fatal("NewJobManager returned nil")
	}
	jobs := m.List()
	if got := len(jobs); got != 0 {
		t.Errorf("NewJobManager List() len = %d, want 0", got)
	}
}

// ─── Test 2: TestJobManager_Start_Transitions ────────────────────────────

// TestJobManager_Start_Transitions pins the full happy-path state
// machine. The job must visit StatePending → StatePrereq → StateInstalling
// → StateProbing → StateConfiguring → StateDone, and each transition
// must emit exactly one WS event with the correct "step" value (the
// state's string form). The terminal event is "mcp-install.completed".
func TestJobManager_Start_Transitions(t *testing.T) {
	defer withInstallFn(t, fakeInstallOK([]string{"tool1", "tool2"}))()

	hub := newCaptureHub()
	m := NewJobManager(hub)

	entry := fakeEntry("playwright")
	job, err := m.Start(context.Background(), entry, "opencode", nil)
	if err != nil {
		t.Fatalf("Start(happy) unexpected error: %v", err)
	}
	if job == nil {
		t.Fatal("Start(happy) returned nil job")
	}
	if !strings.HasPrefix(job.ID, "mcp-job-") {
		t.Errorf("Start(happy) job.ID = %q, want prefix %q", job.ID, "mcp-job-")
	}
	job.mu.RLock()
	initialState := job.State
	job.mu.RUnlock()
	if initialState != StatePending {
		t.Errorf("Start(happy) initial State = %q, want %q", initialState, StatePending)
	}
	if job.EntryID != "playwright" {
		t.Errorf("Start(happy) EntryID = %q, want %q", job.EntryID, "playwright")
	}
	if job.TargetAgent != "opencode" {
		t.Errorf("Start(happy) TargetAgent = %q, want %q", job.TargetAgent, "opencode")
	}

	// Wait for the goroutine to drive the job to StateDone.
	waitForState(t, m, job.ID, StateDone, 2*time.Second)

	// Assert the broadcast sequence: progress events emit the state name
	// in order. The brief pins the visible sequence as
	// pending → prereq_check → installing → probing → configuring → done.
	wantSteps := []string{
		string(StatePending),
		StepPrereq,      // "prereq_check"
		StepInstalling,  // "installing"
		StepProbing,     // "probing"
		StepConfiguring, // "configuring"
	}
	gotSteps := hub.progressSteps()

	// gotSteps may include the finalizing/done step in addition. We
	// check that the wanted steps appear as a prefix in the right order
	// (allowing the dev to insert extra progress events before "done").
	if len(gotSteps) < len(wantSteps) {
		t.Fatalf("Start(happy) emitted %d progress steps, want at least %d (sequence: %v)",
			len(gotSteps), len(wantSteps), gotSteps)
	}
	for i, want := range wantSteps {
		if gotSteps[i] != want {
			t.Errorf("Start(happy) progress step[%d] = %q, want %q (full sequence: %v)",
				i, gotSteps[i], want, gotSteps)
		}
	}

	// Last broadcast must be the "completed" event.
	msgs := hub.snapshotMsgs()
	if len(msgs) == 0 {
		t.Fatal("Start(happy) emitted no broadcasts; want at least one")
	}
	last := msgs[len(msgs)-1]
	if last.Type != "mcp-install.completed" {
		t.Errorf("Start(happy) last broadcast Type = %q, want %q", last.Type, "mcp-install.completed")
	}
	if last.InstallID != job.ID {
		t.Errorf("Start(happy) last broadcast InstallID = %q, want %q", last.InstallID, job.ID)
	}
	var cd completedData
	if err := json.Unmarshal(last.Data, &cd); err != nil {
		t.Errorf("Start(happy) completed Data = %q is not valid JSON: %v", last.Data, err)
	} else {
		// Tools order is not guaranteed; check set membership.
		if len(cd.Tools) != 2 || cd.Tools[0] != "tool1" || cd.Tools[1] != "tool2" {
			t.Errorf("Start(happy) completed Tools = %v, want [tool1 tool2]", cd.Tools)
		}
		if cd.DurationMs < 0 {
			t.Errorf("Start(happy) completed DurationMs = %d, want >= 0", cd.DurationMs)
		}
	}
}

// ─── Test 3: TestJobManager_Start_Conflict ───────────────────────────────

// TestJobManager_Start_Conflict pins the conflict-detection contract:
// starting the same (entryID, target) twice while the first job is
// still active must fail with errors.Is(err, ErrJobInProgress). The
// first job must continue running unaffected. After releasing the first
// job's blocker, both jobs reach StateDone (or the first reaches
// StateDone; the second was never started).
func TestJobManager_Start_Conflict(t *testing.T) {
	release := make(chan struct{})
	defer withInstallFn(t, fakeInstallBlock(release))()

	hub := newCaptureHub()
	m := NewJobManager(hub)

	entry := fakeEntry("playwright")

	// First Start: blocks in the installFn until release is closed.
	first, err := m.Start(context.Background(), entry, "opencode", nil)
	if err != nil {
		t.Fatalf("Start(first) unexpected error: %v", err)
	}
	if first == nil {
		t.Fatal("Start(first) returned nil job")
	}

	// Give the goroutine a moment to enter installFn and call Progress.
	// 50ms is generous for a synchronous Progress call on a fast machine.
	waitForState(t, m, first.ID, StatePrereq, 2*time.Second)

	// Second Start with the same (entryID, target) must return
	// ErrJobInProgress. The first job must still be running.
	second, err := m.Start(context.Background(), entry, "opencode", nil)
	if !errors.Is(err, ErrJobInProgress) {
		t.Errorf("Start(second) err = %v, want errors.Is(err, ErrJobInProgress)", err)
	}
	if second != nil {
		t.Errorf("Start(second) returned non-nil job on conflict: %+v", second)
	}

	// First job must still be present and in a non-terminal state.
	if job, ok := m.Get(first.ID); !ok {
		t.Errorf("Get(first.ID=%q) ok = false, want true (first job must still be tracked after a conflict)", first.ID)
	} else {
		job.mu.RLock()
		state := job.State
		job.mu.RUnlock()
		if state == StateDone || state == StateFailed {
			t.Errorf("Get(first.ID=%q) State = %q; first job must still be running after a conflict", first.ID, state)
		}
	}

	// Release the first job; it should reach StateDone.
	close(release)
	waitForState(t, m, first.ID, StateDone, 2*time.Second)

	// After the first job completes, a fresh Start with the same
	// (entryID, target) should succeed: a finished job is not "active".
	// (This is the natural consequence of "active" = not in a terminal
	// state; we don't pin the exact definition, but the behavior is
	// important to verify the conflict map is cleaned up on completion.)
	third, err := m.Start(context.Background(), entry, "opencode", nil)
	if err != nil {
		t.Errorf("Start(third, after first done) unexpected error: %v; want nil "+
			"(a completed job must not block a fresh install for the same entry/target)", err)
	}
	if third == nil {
		t.Error("Start(third, after first done) returned nil job; want a fresh job")
	}
	if third != nil && third.ID == first.ID {
		t.Errorf("Start(third) ID = %q, same as first.ID; want a unique ID", third.ID)
	}
}

// ─── Test 4: TestJobManager_Start_Failure ────────────────────────────────

// TestJobManager_Start_Failure pins the failure path. When installFn
// returns a wrapped sentinel error, the job must transition to
// StateFailed, populate job.Error with a non-empty Code, and emit a
// "mcp-install.failed" broadcast whose data.error.code matches.
func TestJobManager_Start_Failure(t *testing.T) {
	defer withInstallFn(t, fakeInstallErr(ErrPrereqMissing))()

	hub := newCaptureHub()
	m := NewJobManager(hub)

	entry := fakeEntry("playwright")
	job, err := m.Start(context.Background(), entry, "opencode", nil)
	if err != nil {
		t.Fatalf("Start(failure) unexpected error: %v; Start should not return the install error synchronously", err)
	}
	if job == nil {
		t.Fatal("Start(failure) returned nil job")
	}

	waitForState(t, m, job.ID, StateFailed, 2*time.Second)

	// Job must have a populated Error with a non-empty Code.
	job.mu.RLock()
	hasErr := job.Error != nil
	var errCode, errStep string
	if hasErr {
		errCode = job.Error.Code
		errStep = job.Error.Step
	}
	job.mu.RUnlock()

	if !hasErr {
		t.Fatal("Start(failure) job.Error = nil, want a non-nil *JobError")
	}
	if errCode == "" {
		t.Errorf("Start(failure) job.Error.Code = %q, want non-empty (e.g. %q)", errCode, "prereq_missing")
	}
	if errStep == "" {
		t.Errorf("Start(failure) job.Error.Step = %q, want non-empty (e.g. %q)", errStep, StepPrereq)
	}

	// A "mcp-install.failed" broadcast must have been emitted.
	msgs := hub.snapshotMsgs()
	var failed *jobMessage
	for i := range msgs {
		if msgs[i].Type == "mcp-install.failed" {
			failed = &msgs[i]
			break
		}
	}
	if failed == nil {
		t.Fatalf("Start(failure) did not emit a mcp-install.failed broadcast; "+
			"emitted types: %v", messageTypes(msgs))
	}
	if failed.InstallID != job.ID {
		t.Errorf("Start(failure) failed.InstallID = %q, want %q", failed.InstallID, job.ID)
	}
	var fd failedData
	if err := json.Unmarshal(failed.Data, &fd); err != nil {
		t.Errorf("Start(failure) failed Data = %q is not valid JSON: %v", failed.Data, err)
	} else {
		job.mu.RLock()
		jobErrCode := ""
		if job.Error != nil {
			jobErrCode = job.Error.Code
		}
		job.mu.RUnlock()
		if fd.Error.Code != jobErrCode {
			t.Errorf("Start(failure) failed Data error.code = %q, job.Error.Code = %q (they must match)",
				fd.Error.Code, jobErrCode)
		}
	}
}

// ─── Test 5: TestJobManager_Get_OK ───────────────────────────────────────

// TestJobManager_Get_OK pins the Get lookup happy path. After Start
// returns a job, Get(job.ID) must return the same job (same ID, same
// entryID) and ok=true.
func TestJobManager_Get_OK(t *testing.T) {
	defer withInstallFn(t, fakeInstallOK([]string{"tool1"}))()

	hub := newCaptureHub()
	m := NewJobManager(hub)

	entry := fakeEntry("playwright")
	job, err := m.Start(context.Background(), entry, "opencode", nil)
	if err != nil {
		t.Fatalf("Start unexpected error: %v", err)
	}

	got, ok := m.Get(job.ID)
	if !ok {
		t.Fatalf("Get(%q) ok = false, want true", job.ID)
	}
	if got == nil {
		t.Fatalf("Get(%q) returned nil job, want non-nil", job.ID)
	}
	if got.ID != job.ID {
		t.Errorf("Get(%q) returned job.ID = %q, want %q", job.ID, got.ID, job.ID)
	}
	if got.EntryID != "playwright" {
		t.Errorf("Get(%q) returned job.EntryID = %q, want %q", job.ID, got.EntryID, "playwright")
	}
}

// ─── Test 6: TestJobManager_Get_NotFound ─────────────────────────────────

// TestJobManager_Get_NotFound pins the Get lookup miss path. Get on a
// non-existent ID must return (nil, false), not panic.
func TestJobManager_Get_NotFound(t *testing.T) {
	hub := newCaptureHub()
	m := NewJobManager(hub)

	got, ok := m.Get("nope-does-not-exist")
	if ok {
		t.Errorf("Get(\"nope\") ok = true, want false")
	}
	if got != nil {
		t.Errorf("Get(\"nope\") returned non-nil job: %+v, want nil", got)
	}
}

// ─── Test 7: TestJobManager_List ─────────────────────────────────────────

// TestJobManager_List pins the List snapshot contract. After starting
// two jobs with different entryIDs, List() must return exactly two
// jobs. The order of List() is not pinned; we check set membership.
func TestJobManager_List(t *testing.T) {
	defer withInstallFn(t, fakeInstallOK([]string{"tool1"}))()

	hub := newCaptureHub()
	m := NewJobManager(hub)

	entryA := fakeEntry("playwright")
	entryB := fakeEntry("github")
	if _, err := m.Start(context.Background(), entryA, "opencode", nil); err != nil {
		t.Fatalf("Start(A) unexpected error: %v", err)
	}
	if _, err := m.Start(context.Background(), entryB, "opencode", nil); err != nil {
		t.Fatalf("Start(B) unexpected error: %v", err)
	}

	jobs := m.List()
	if got := len(jobs); got != 2 {
		t.Errorf("List() len = %d, want 2", got)
	}
	seen := map[string]bool{}
	for _, j := range jobs {
		seen[j.EntryID] = true
	}
	if !seen["playwright"] || !seen["github"] {
		t.Errorf("List() entryIDs = %v, want to contain both %q and %q",
			entryIDs(jobs), "playwright", "github")
	}
}

// ─── Test 8: TestJobManager_Broadcasts ────────────────────────────────────

// TestJobManager_Broadcasts pins the wire format of every broadcast the
// JobManager emits during a happy-path install. The captured messages
// must satisfy, for every message:
//
//   - Type is one of "mcp-install.progress" | "mcp-install.completed" | "mcp-install.failed".
//   - Ts is a valid RFC3339 timestamp.
//   - InstallID matches the job's ID.
//
// The test also pins the count: a happy path emits at least 4 messages
// (one initial + at least one progress + one completed = 3; the brief
// says "at least 4" to allow for a "started" event in addition to the
// initial pending progress event — the dev may emit them as separate
// frames or merge them).
func TestJobManager_Broadcasts(t *testing.T) {
	defer withInstallFn(t, fakeInstallOK([]string{"tool1"}))()

	hub := newCaptureHub()
	m := NewJobManager(hub)

	entry := fakeEntry("playwright")
	job, err := m.Start(context.Background(), entry, "opencode", nil)
	if err != nil {
		t.Fatalf("Start unexpected error: %v", err)
	}
	waitForState(t, m, job.ID, StateDone, 2*time.Second)

	msgs := hub.snapshotMsgs()
	if len(msgs) < 4 {
		t.Errorf("Start(happy) emitted %d broadcasts, want at least 4 (sequence types: %v)",
			len(msgs), messageTypes(msgs))
	}

	allowedTypes := map[string]bool{
		"mcp-install.progress":  true,
		"mcp-install.completed": true,
		"mcp-install.failed":    true,
	}
	for i, m := range msgs {
		if !allowedTypes[m.Type] {
			t.Errorf("broadcast[%d] Type = %q, want one of mcp-install.progress|completed|failed", i, m.Type)
		}
		isValidTimestamp(t, m.Ts)
		if m.InstallID != job.ID {
			t.Errorf("broadcast[%d] InstallID = %q, want %q", i, m.InstallID, job.ID)
		}
		// Data must be a non-null JSON object.
		if len(m.Data) == 0 || string(m.Data) == "null" {
			t.Errorf("broadcast[%d] Data = %q, want a JSON object", i, m.Data)
		}
	}

	// Must include at least one progress event and exactly one terminal
	// completed event. Multiple completed events are a bug (idempotency).
	progressCount := 0
	completedCount := 0
	for _, m := range msgs {
		switch m.Type {
		case "mcp-install.progress":
			progressCount++
		case "mcp-install.completed":
			completedCount++
		}
	}
	if progressCount == 0 {
		t.Error("Start(happy) emitted zero progress broadcasts; want at least one")
	}
	if completedCount != 1 {
		t.Errorf("Start(happy) emitted %d completed broadcasts, want exactly 1", completedCount)
	}
}

// ─── Test 9: TestJobManager_ConcurrentStart ──────────────────────────────

// TestJobManager_ConcurrentStart pins the concurrency contract. 10
// goroutines call Start with distinct entryIDs and the same target
// agent. All 10 must succeed (no spurious conflicts), all 10 jobs must
// appear in List(), and all 10 IDs must be unique.
func TestJobManager_ConcurrentStart(t *testing.T) {
	defer withInstallFn(t, fakeInstallOK([]string{"tool1"}))()

	hub := newCaptureHub()
	m := NewJobManager(hub)

	const N = 10
	ids := make([]string, N)
	errs := make([]error, N)

	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			entry := fakeEntry(fmt.Sprintf("entry-%d", i))
			job, err := m.Start(context.Background(), entry, "opencode", nil)
			if err != nil {
				errs[i] = err
				return
			}
			ids[i] = job.ID
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("Start[%d] unexpected error: %v", i, err)
		}
	}

	// All 10 IDs must be non-empty and unique.
	seen := map[string]int{}
	for i, id := range ids {
		if id == "" {
			t.Errorf("Start[%d] returned empty ID", i)
			continue
		}
		if !strings.HasPrefix(id, "mcp-job-") {
			t.Errorf("Start[%d] ID = %q, want prefix %q", i, id, "mcp-job-")
		}
		seen[id]++
	}
	if len(seen) != N {
		t.Errorf("Start returned %d unique IDs, want %d (counts: %v)", len(seen), N, seen)
	}

	// List() must contain all 10 jobs.
	jobs := m.List()
	if got := len(jobs); got != N {
		t.Errorf("List() len = %d, want %d", got, N)
	}
}

// ─── Test 10: TestJobManager_GC_RemovesOld ───────────────────────────────

// TestJobManager_GC_RemovesOld pins the retention policy. After
// starting a job that completes, mutating its UpdatedAt to 2 hours
// ago, and calling gc(), the job must be removed and gc() must return
// 1. List() must be empty afterwards.
//
// The dev is free to choose the retention duration; this test pins the
// observable consequence (a 2-hour-old completed job is GC'd) without
// constraining the value. The default retention must be < 2 hours for
// the install-use case (jobs older than a few minutes are stale by
// the time the user looks at them again).
func TestJobManager_GC_RemovesOld(t *testing.T) {
	defer withInstallFn(t, fakeInstallOK([]string{"tool1"}))()

	hub := newCaptureHub()
	m := NewJobManager(hub)

	entry := fakeEntry("playwright")
	job, err := m.Start(context.Background(), entry, "opencode", nil)
	if err != nil {
		t.Fatalf("Start unexpected error: %v", err)
	}

	// Wait for the job to reach a terminal state (StateDone) so the GC
	// considers it eligible for removal. If GC only removes terminal
	// jobs, this is required; if it removes any job past the retention
	// window, it is also correct.
	waitForState(t, m, job.ID, StateDone, 2*time.Second)

	// Mutate UpdatedAt to 2 hours ago. UpdatedAt is an exported field
	// on the Job struct; the test mutates it through the pointer
	// returned by Get. This is a deliberate "white-box" pin: the test
	// is in `package mcp` and depends on UpdatedAt being writable from
	// outside the production type. The dev's Job struct must expose
	// UpdatedAt (or re-expose a helper) for this test to compile and
	// pass.
	job.mu.Lock()
	job.UpdatedAt = time.Now().Add(-2 * time.Hour)
	job.mu.Unlock()

	removed := m.gc()
	if removed != 1 {
		t.Errorf("gc() returned %d, want 1 (one job older than retention should be removed)", removed)
	}

	jobs := m.List()
	if got := len(jobs); got != 0 {
		t.Errorf("List() after gc() len = %d, want 0", got)
	}
}

// ─── misc helpers ────────────────────────────────────────────────────────

// messageTypes returns the Type field of every message, for nicer error
// messages in the test output.
func messageTypes(msgs []jobMessage) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Type
	}
	return out
}

// entryIDs returns the EntryID field of every job, for nicer error
// messages in the test output.
func entryIDs(jobs []*Job) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = j.EntryID
	}
	return out
}
