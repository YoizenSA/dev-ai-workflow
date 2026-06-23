// mcp_store_test.go — TDD slice 6: refactor of handleMcpInstall to async.
//
// Pinned symbols (all must be added to support the async install flow):
//
//	s.jobs (*mcp.JobManager)            — Server must hold a JobManager
//	s.handleMcpInstallStatus (handler)  — GET /api/mcp/install/{id}
//	mcp.WithInstallFn (test seam)       — exported wrapper for the
//	                                       package-private installFn var,
//	                                       so tests in other packages
//	                                       can swap the install pipeline.
//
// Pinned behavior (POST /api/mcp/install):
//   - 202 Accepted with
//     {"install_id": <mcp-job-N>,
//      "status_url": "/api/mcp/install/<id>",
//      "ws_channel": "mcp-install",
//      "entry_id": <id>,
//      "target_agent": <target>}
//   - 400 on invalid JSON body
//   - 400 on unknown / unsupported target_agent
//   - 404 on unknown id
//   - 409 on conflict (active job for same (id, target)) with
//     {"error": "install_in_progress", "existing_id": "<id>"}
//   - 422 on missing required credentials with
//     {"error": "missing_credentials", "required": ["GITHUB_PERSONAL_ACCESS_TOKEN"]}
//
// Pinned behavior (GET /api/mcp/install/{id}):
//   - 404 when id is unknown
//   - 200 with the serialized *mcp.Job otherwise
//
// RED state: this file references symbols that do not exist yet. The
// expected compile errors include:
//
//	undefined: mcp.WithInstallFn
//	Server has no field or method jobs
//	undefined: s.handleMcpInstallStatus
//
// Once @dev adds the pinned symbols + the async handler logic, the
// tests will compile and run, and will FAIL (not compile error) on
// every assertion that depends on the new behavior (202, conflict
// detection, status polling, etc.).
//
// This file is stdlib-only. It depends on github.com/Yoizen/.../mcp
// for the public types (CatalogEntry, InstallOptions, State*,
// Step*, Err*). It does NOT depend on github.com/Yoizen/.../mcp
// internals — installFn is accessed only via the exported seam
// mcp.WithInstallFn.

package control

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
)

// wantWSChannel pins the WS channel name returned in the 202 response.
// The brief pins this exact string; the dev must use the same value.
const wantWSChannel = "mcp-install"

// ─── helpers ─────────────────────────────────────────────────────────────

// captureHub is a minimal mcp.Broadcaster used to wire a JobManager
// without dragging in the kanban / missions hubs. It records every
// Broadcast call for optional assertion, but the tests in this file
// do not inspect the recordings — they exist so the JobManager
// doesn't no-op forever waiting for a nil hub.
type captureHub struct {
	mu   sync.Mutex
	msgs [][]byte
}

func (h *captureHub) Broadcast(b []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.msgs = append(h.msgs, append([]byte(nil), b...))
}

func newCaptureHub() *captureHub { return &captureHub{} }

// defaultInstallMock is a quick-OK install function used by tests
// that don't care about install behavior. It drives the progress
// callback through the last two steps (prereq + finalizing) and
// returns a single tool. Mirrors the pattern in mcp/job_test.go.
func defaultInstallMock(ctx context.Context, entry mcp.CatalogEntry, opts mcp.InstallOptions) ([]string, error) {
	if opts.Progress != nil {
		opts.Progress(mcp.StepPrereq, 10, "prereq ok")
		opts.Progress(mcp.StepFinalizing, 100, "done")
	}
	return []string{"tool1"}, nil
}

// newTestMcpServer builds a minimal *Server wired with a fresh
// JobManager whose installFn is swapped to mockInstall for the
// duration of the test. If mockInstall is nil, a quick-OK default
// is used. The returned *mcp.JobManager lets tests inspect job
// state directly (e.g. wait for StateDone / StateFailed).
//
// Pinned test seam: mcp.WithInstallFn must exist for this helper
// to compile. @dev adds it to internal/mcp/job.go.
func newTestMcpServer(t *testing.T, mockInstall func(ctx context.Context, entry mcp.CatalogEntry, opts mcp.InstallOptions) ([]string, error)) (*Server, *mcp.JobManager) {
	t.Helper()
	if mockInstall == nil {
		mockInstall = defaultInstallMock
	}
	mcp.WithInstallFn(t, mockInstall)

	hub := newCaptureHub()
	jobs := mcp.NewJobManager(hub)

	s := &Server{
		mux:  http.NewServeMux(),
		jobs: jobs,
	}
	// Register the existing routes (the dev will extend
	// registerMcpStoreRoutes in slice 6; the tests mostly call
	// handlers directly so this is here for completeness).
	s.registerMcpStoreRoutes()
	return s, jobs
}

// waitForJobState polls the job until it reaches want or timeout
// elapses. Reads the job's state via Snapshot() so it is race-free
// under `go test -race`.
func waitForJobState(t *testing.T, jobs *mcp.JobManager, id string, want mcp.State, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last mcp.State
	for time.Now().Before(deadline) {
		if job, ok := jobs.Get(id); ok {
			last = job.Snapshot()
			if last == want {
				return
			}
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("waitForJobState: job %s did not reach state %q within %s (last state: %q)", id, want, timeout, last)
}

// decodeJSON decodes the recorder body into a generic map for
// assertion. Failures here are fatal — the test must be able to
// parse its own response to assert anything.
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v (body: %q)", err, w.Body.String())
	}
	return out
}

// callPost invokes a handler with a JSON POST body.
func callPost(handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/mcp/install", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

// callGet invokes a handler with a path value. The handler is
// expected to use r.PathValue("id") to read it.
func callGet(handler http.HandlerFunc, pathValue string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/mcp/install/"+pathValue, nil)
	req.SetPathValue("id", pathValue)
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

// ─── Test 1: handleMcpInstall with no required creds → 202 ───────────────

// TestHandleMcpInstall_NoCredsEntry_OK pins the async happy path for an
// entry that does not require credentials (playwright). The handler
// must enqueue the job, return 202, and include the install_id,
// status_url, ws_channel, entry_id, and target_agent in the body.
func TestHandleMcpInstall_NoCredsEntry_OK(t *testing.T) {
	// Mock that takes 50ms before returning — pins that the response
	// is sent BEFORE the install completes (the whole point of the
	// async refactor).
	delayed := func(ctx context.Context, entry mcp.CatalogEntry, opts mcp.InstallOptions) ([]string, error) {
		if opts.Progress != nil {
			opts.Progress(mcp.StepPrereq, 10, "prereq ok")
		}
		// Yield long enough that a synchronous handler would be
		// observably slower than 202.
		select {
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return []string{"tool1", "tool2"}, nil
	}
	s, _ := newTestMcpServer(t, delayed)

	body := `{"id": "playwright", "target_agent": "opencode"}`
	w := callPost(s.handleMcpInstall, body)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusAccepted, w.Body.String())
	}

	resp := decodeJSON(t, w)
	installID, _ := resp["install_id"].(string)
	if !strings.HasPrefix(installID, "mcp-job-") {
		t.Errorf("install_id = %q, want prefix %q", installID, "mcp-job-")
	}
	if got := resp["entry_id"]; got != "playwright" {
		t.Errorf("entry_id = %v, want %q", got, "playwright")
	}
	if got := resp["target_agent"]; got != "opencode" {
		t.Errorf("target_agent = %v, want %q", got, "opencode")
	}
	if got, want := resp["status_url"], "/api/mcp/install/"+installID; got != want {
		t.Errorf("status_url = %v, want %q", got, want)
	}
	if got := resp["ws_channel"]; got != wantWSChannel {
		t.Errorf("ws_channel = %v, want %q", got, wantWSChannel)
	}
}

// ─── Test 2: handleMcpInstall with creds → 202 ───────────────────────────

// TestHandleMcpInstall_WithCreds_OK pins the async happy path for an
// entry that DOES require credentials (github). The handler must
// accept the credentials, enqueue the job, and return 202.
func TestHandleMcpInstall_WithCreds_OK(t *testing.T) {
	s, _ := newTestMcpServer(t, nil)

	body := `{"id": "github", "target_agent": "opencode", "credentials": {"GITHUB_PERSONAL_ACCESS_TOKEN": "ghp_test"}}`
	w := callPost(s.handleMcpInstall, body)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusAccepted, w.Body.String())
	}
	resp := decodeJSON(t, w)
	installID, _ := resp["install_id"].(string)
	if !strings.HasPrefix(installID, "mcp-job-") {
		t.Errorf("install_id = %q, want prefix %q", installID, "mcp-job-")
	}
}

// ─── Test 3: handleMcpInstall with missing required creds → 422 ──────────

// TestHandleMcpInstall_MissingRequiredCreds_422 pins that the handler
// rejects an entry whose required_env list is not satisfied. The
// response must include the names of the missing required env vars.
func TestHandleMcpInstall_MissingRequiredCreds_422(t *testing.T) {
	s, _ := newTestMcpServer(t, nil)

	body := `{"id": "github", "target_agent": "opencode"}`
	w := callPost(s.handleMcpInstall, body)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
	resp := decodeJSON(t, w)
	required, _ := resp["required"].([]interface{})
	found := false
	for _, r := range required {
		if r == "GITHUB_PERSONAL_ACCESS_TOKEN" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("required = %v, want to contain GITHUB_PERSONAL_ACCESS_TOKEN", required)
	}
}

// ─── Test 4: handleMcpInstall with empty credentials object → 422 ────────

// TestHandleMcpInstall_PartialCreds_422 pins that an explicit empty
// credentials object is treated the same as no credentials — missing
// required env vars still fail validation.
func TestHandleMcpInstall_PartialCreds_422(t *testing.T) {
	s, _ := newTestMcpServer(t, nil)

	body := `{"id": "github", "target_agent": "opencode", "credentials": {}}`
	w := callPost(s.handleMcpInstall, body)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
}

// ─── Test 5: handleMcpInstall with empty-string cred → 422 ──────────────

// TestHandleMcpInstall_EmptyStringCred_422 pins that an empty string
// does not satisfy a required env var. (mcp.ValidateCreds treats
// "present but empty" the same as missing.)
func TestHandleMcpInstall_EmptyStringCred_422(t *testing.T) {
	s, _ := newTestMcpServer(t, nil)

	body := `{"id": "github", "target_agent": "opencode", "credentials": {"GITHUB_PERSONAL_ACCESS_TOKEN": ""}}`
	w := callPost(s.handleMcpInstall, body)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
}

// ─── Test 6: handleMcpInstall with unknown id → 404 ──────────────────────

// TestHandleMcpInstall_UnknownId_404 pins the catalog lookup: an id
// that is not in the catalog must produce 404, not 500.
func TestHandleMcpInstall_UnknownId_404(t *testing.T) {
	s, _ := newTestMcpServer(t, nil)

	body := `{"id": "no-existe", "target_agent": "opencode"}`
	w := callPost(s.handleMcpInstall, body)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusNotFound, w.Body.String())
	}
}

// ─── Test 7: handleMcpInstall with invalid JSON body → 400 ───────────────

// TestHandleMcpInstall_InvalidBody_400 pins the JSON parsing contract.
func TestHandleMcpInstall_InvalidBody_400(t *testing.T) {
	s, _ := newTestMcpServer(t, nil)

	w := callPost(s.handleMcpInstall, "not json at all")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ─── Test 8: handleMcpInstall with invalid target_agent → 400 ────────────

// TestHandleMcpInstall_InvalidTarget_400 pins that target_agent must
// be one of the supported agents. "vim" is not — the handler must
// reject it before reaching the install pipeline.
func TestHandleMcpInstall_InvalidTarget_400(t *testing.T) {
	s, _ := newTestMcpServer(t, nil)

	body := `{"id": "github", "target_agent": "vim"}`
	w := callPost(s.handleMcpInstall, body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

// ─── Test 9: handleMcpInstall defaults target_agent to opencode ──────────

// TestHandleMcpInstall_DefaultTargetOpencode pins that an omitted
// target_agent defaults to "opencode" (the primary supported agent).
// The brief's "Si opencode está disponible, usa opencode" leaves the
// exact fallback policy to @dev; this test asserts the response says
// "opencode", which holds whether the dev checks PATH or not — as
// long as the host actually has opencode available.
func TestHandleMcpInstall_DefaultTargetOpencode(t *testing.T) {
	s, _ := newTestMcpServer(t, nil)

	body := `{"id": "playwright"}`
	w := callPost(s.handleMcpInstall, body)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusAccepted, w.Body.String())
	}
	resp := decodeJSON(t, w)
	if got := resp["target_agent"]; got != "opencode" {
		t.Errorf("target_agent = %v, want %q", got, "opencode")
	}
}

// ─── Test 10: handleMcpInstall with active job for same (id, target) → 409

// TestHandleMcpInstall_Conflict_409 pins that a second Start for the
// same (entryID, target) while the first job is still active must
// return 409 with the in-progress job's id in existing_id.
func TestHandleMcpInstall_Conflict_409(t *testing.T) {
	release := make(chan struct{})
	defer close(release) // ensure the blocking mock unblocks on test exit

	blocking := func(ctx context.Context, entry mcp.CatalogEntry, opts mcp.InstallOptions) ([]string, error) {
		if opts.Progress != nil {
			opts.Progress(mcp.StepPrereq, 10, "prereq ok")
		}
		<-release // keep the first job active until the test ends
		return []string{"tool1"}, nil
	}
	s, _ := newTestMcpServer(t, blocking)

	body := `{"id": "playwright", "target_agent": "opencode"}`

	// First request — must succeed with 202 and a fresh install_id.
	w1 := callPost(s.handleMcpInstall, body)
	if w1.Code != http.StatusAccepted {
		t.Fatalf("first request: status = %d, want %d (body: %s)", w1.Code, http.StatusAccepted, w1.Body.String())
	}
	resp1 := decodeJSON(t, w1)
	firstID, _ := resp1["install_id"].(string)
	if firstID == "" {
		t.Fatal("first request: install_id is empty")
	}

	// Second request — same (id, target) — must return 409 and point
	// existing_id at the first job.
	w2 := callPost(s.handleMcpInstall, body)
	if w2.Code != http.StatusConflict {
		t.Fatalf("second request: status = %d, want %d (body: %s)", w2.Code, http.StatusConflict, w2.Body.String())
	}
	resp2 := decodeJSON(t, w2)
	if got := resp2["existing_id"]; got != firstID {
		t.Errorf("existing_id = %v, want %q", got, firstID)
	}
}

// ─── Test 11: install error after 202 is observed via GET status ─────────

// TestHandleMcpInstall_JobError_StillReturns202 pins the central
// invariant of the async refactor: an install failure is NOT
// returned synchronously. The POST response is 202 (the job is
// enqueued); the error surfaces only when the client polls the
// status endpoint or subscribes to the WS channel.
func TestHandleMcpInstall_JobError_StillReturns202(t *testing.T) {
	failing := func(ctx context.Context, entry mcp.CatalogEntry, opts mcp.InstallOptions) ([]string, error) {
		if opts.Progress != nil {
			opts.Progress(mcp.StepPrereq, 10, "prereq failed")
		}
		return nil, mcp.ErrPrereqMissing
	}
	s, jobs := newTestMcpServer(t, failing)

	body := `{"id": "playwright", "target_agent": "opencode"}`

	// Initial response must be 202 — not 4xx/5xx.
	w := callPost(s.handleMcpInstall, body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusAccepted, w.Body.String())
	}
	resp := decodeJSON(t, w)
	installID, _ := resp["install_id"].(string)
	if installID == "" {
		t.Fatal("install_id is empty")
	}

	// Wait for the job to reach StateFailed.
	waitForJobState(t, jobs, installID, mcp.StateFailed, 2*time.Second)

	// GET status must reflect the failure with error.code mapped from
	// the sentinel. The brief pins "prereq_missing" as the public
	// code for ErrPrereqMissing (see mcp.codeFromErr).
	w2 := callGet(s.handleMcpInstallStatus, installID)
	if w2.Code != http.StatusOK {
		t.Fatalf("status endpoint: status = %d, want %d (body: %s)", w2.Code, http.StatusOK, w2.Body.String())
	}
	status := decodeJSON(t, w2)
	if got, want := status["state"], string(mcp.StateFailed); got != want {
		t.Errorf("state = %v, want %q", got, want)
	}
	errObj, _ := status["error"].(map[string]interface{})
	if errObj == nil {
		t.Fatal("error object is missing from status response")
	}
	if got := errObj["code"]; got != "prereq_missing" {
		t.Errorf("error.code = %v, want %q", got, "prereq_missing")
	}
}

// ─── Test 12: handler passes credentials through to the install fn ───────

// TestHandleMcpInstall_PassesCredsToInstallFn pins that the handler
// forwards the request credentials to the install pipeline unchanged.
// The mock captures opts.Credentials; the test asserts the captured
// map matches the body.
func TestHandleMcpInstall_PassesCredsToInstallFn(t *testing.T) {
	var capturedCreds map[string]string
	capturing := func(ctx context.Context, entry mcp.CatalogEntry, opts mcp.InstallOptions) ([]string, error) {
		capturedCreds = opts.Credentials
		return []string{"tool1"}, nil
	}
	s, jobs := newTestMcpServer(t, capturing)

	body := `{"id": "github", "target_agent": "opencode", "credentials": {"GITHUB_PERSONAL_ACCESS_TOKEN": "ghp_capture"}}`
	w := callPost(s.handleMcpInstall, body)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusAccepted, w.Body.String())
	}
	resp := decodeJSON(t, w)
	installID, _ := resp["install_id"].(string)
	if installID == "" {
		t.Fatal("install_id is empty")
	}

	// Wait for the job to reach StateDone. The job's run goroutine
	// writes capturedCreds before transitioning to StateDone, so
	// waiting on Snapshot() is a happens-before edge that makes the
	// subsequent read race-free.
	waitForJobState(t, jobs, installID, mcp.StateDone, 2*time.Second)

	if capturedCreds == nil {
		t.Fatal("installFn was not invoked within 2s")
	}
	if got, want := capturedCreds["GITHUB_PERSONAL_ACCESS_TOKEN"], "ghp_capture"; got != want {
		t.Errorf("captured GITHUB_PERSONAL_ACCESS_TOKEN = %q, want %q", got, want)
	}
}

// ─── Test 13: handleMcpInstallStatus with unknown id → 404 ───────────────

// TestHandleMcpInstallStatus_NotFound_404 pins the polling-fallback
// GET handler's miss path. A bogus id must return 404, not 500 or
// an empty 200.
func TestHandleMcpInstallStatus_NotFound_404(t *testing.T) {
	s, _ := newTestMcpServer(t, nil)

	w := callGet(s.handleMcpInstallStatus, "nope-does-not-exist")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusNotFound, w.Body.String())
	}
}

// ─── Test 14: handleMcpInstallStatus returns the done job ────────────────

// TestHandleMcpInstallStatus_OK pins the polling-fallback GET happy
// path. After a job finishes successfully, GET status must return
// 200 with state=done and a non-empty result.tools.
func TestHandleMcpInstallStatus_OK(t *testing.T) {
	s, jobs := newTestMcpServer(t, nil)

	// Start a job.
	body := `{"id": "playwright", "target_agent": "opencode"}`
	w1 := callPost(s.handleMcpInstall, body)
	if w1.Code != http.StatusAccepted {
		t.Fatalf("install: status = %d, want %d (body: %s)", w1.Code, http.StatusAccepted, w1.Body.String())
	}
	resp1 := decodeJSON(t, w1)
	installID, _ := resp1["install_id"].(string)
	if installID == "" {
		t.Fatal("install_id is empty")
	}

	// Wait for the job to finish.
	waitForJobState(t, jobs, installID, mcp.StateDone, 2*time.Second)

	// GET status must return the serialized job.
	w2 := callGet(s.handleMcpInstallStatus, installID)
	if w2.Code != http.StatusOK {
		t.Fatalf("status: status = %d, want %d (body: %s)", w2.Code, http.StatusOK, w2.Body.String())
	}
	status := decodeJSON(t, w2)
	if got, want := status["state"], string(mcp.StateDone); got != want {
		t.Errorf("state = %v, want %q", got, want)
	}
	result, _ := status["result"].(map[string]interface{})
	if result == nil {
		t.Fatal("result object is missing from status response")
	}
	tools, _ := result["tools"].([]interface{})
	if len(tools) == 0 {
		t.Errorf("result.tools is empty; want at least one tool")
	}
}

// ─── Test 15: handleMcpInstallStatus returns the failed job ─────────────

// TestHandleMcpInstallStatus_Failed pins that a failed job's status
// response includes state=failed and an error.code matching the
// sentinel that was returned by the install pipeline.
func TestHandleMcpInstallStatus_Failed(t *testing.T) {
	failing := func(ctx context.Context, entry mcp.CatalogEntry, opts mcp.InstallOptions) ([]string, error) {
		if opts.Progress != nil {
			opts.Progress(mcp.StepPrereq, 10, "prereq failed")
		}
		return nil, mcp.ErrPrereqMissing
	}
	s, jobs := newTestMcpServer(t, failing)

	body := `{"id": "playwright", "target_agent": "opencode"}`
	w1 := callPost(s.handleMcpInstall, body)
	if w1.Code != http.StatusAccepted {
		t.Fatalf("install: status = %d, want %d (body: %s)", w1.Code, http.StatusAccepted, w1.Body.String())
	}
	resp1 := decodeJSON(t, w1)
	installID, _ := resp1["install_id"].(string)
	if installID == "" {
		t.Fatal("install_id is empty")
	}

	waitForJobState(t, jobs, installID, mcp.StateFailed, 2*time.Second)

	w2 := callGet(s.handleMcpInstallStatus, installID)
	if w2.Code != http.StatusOK {
		t.Fatalf("status: status = %d, want %d (body: %s)", w2.Code, http.StatusOK, w2.Body.String())
	}
	status := decodeJSON(t, w2)
	if got, want := status["state"], string(mcp.StateFailed); got != want {
		t.Errorf("state = %v, want %q", got, want)
	}
	errObj, _ := status["error"].(map[string]interface{})
	if errObj == nil {
		t.Fatal("error object is missing from status response")
	}
	if got := errObj["code"]; got != "prereq_missing" {
		t.Errorf("error.code = %v, want %q", got, "prereq_missing")
	}
}

// ─── Test 16: handleMcpInstall with empty id → 400 ──────────────────────

// TestHandleMcpInstall_EmptyID_400 pins the explicit empty-string id
// branch (mcp_store.go:342-345). The handler must reject the request
// with 400 and the dedicated "id is required" message before reaching
// the catalog lookup — a missing or empty id is a different failure
// mode than a non-empty unknown id (which returns 404 "unknown MCP
// id: …"). JSON `{}` and `{"id": ""}` both decode to req.ID == "" and
// must take this branch.
func TestHandleMcpInstall_EmptyID_400(t *testing.T) {
	s, _ := newTestMcpServer(t, nil)

	for _, body := range []string{`{}`, `{"id": ""}`} {
		w := callPost(s.handleMcpInstall, body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body=%s: status = %d, want %d (body: %s)", body, w.Code, http.StatusBadRequest, w.Body.String())
			continue
		}
		resp := decodeJSON(t, w)
		if got, want := resp["error"], "id is required"; got != want {
			t.Errorf("body=%s: error = %v, want %q", body, got, want)
		}
	}
}

// ─── Test 17: handleMcpInstall with missing id field → 400 ──────────────

// TestHandleMcpInstall_MissingIDField_400 pins that the JSON decoder
// leaves req.ID at its zero value ("") when the field is absent, so
// the empty-id branch is reached. This is the same code path as
// TestHandleMcpInstall_EmptyID_400 but with a different body shape
// to confirm the contract holds for both absent and explicit-empty.
func TestHandleMcpInstall_MissingIDField_400(t *testing.T) {
	s, _ := newTestMcpServer(t, nil)

	// Body has target_agent and credentials but no id.
	body := `{"target_agent": "opencode"}`
	w := callPost(s.handleMcpInstall, body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, http.StatusBadRequest, w.Body.String())
	}
	resp := decodeJSON(t, w)
	if got, want := resp["error"], "id is required"; got != want {
		t.Errorf("error = %v, want %q", got, want)
	}
}
