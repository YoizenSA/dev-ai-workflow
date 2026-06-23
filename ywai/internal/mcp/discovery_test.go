package mcp

// discovery_test.go — TDD slice 1 of the "Real MCP Install" plan.
//
// These tests pin the contract of the two discovery functions that will be
// exported from ywai/internal/mcp/discovery.go:
//
//	DiscoverStdio(ctx context.Context, command []string, env map[string]string) ([]string, error)
//	DiscoverHTTP (ctx context.Context, url string) ([]string, error)
//
// Both functions are extractions of the probe code already living in
// ywai/internal/kanban/tool_discovery.go (discoverStdioMCPTools at lines
// 69-199 and discoverMCPTools at lines 18-57). That code WORKS; our job
// here is to pin the *exported contract* so the extraction can land green
// without re-deciding shape.
//
// RED: the file does not compile right now because DiscoverStdio and
// DiscoverHTTP are not defined. `go test ./internal/mcp/...` will fail with
// `undefined: DiscoverStdio` and `undefined: DiscoverHTTP`. That is the
// intended signal that the tests are pinned and waiting for @dev.
//
// These tests use stdlib only (no testify), live in `package mcp` so they
// can reach any unexported helpers @dev chooses to add, and follow the
// conventions in internal/missions/worker_test.go and the existing
// credentials_test.go in this same package.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

// ─── DiscoverStdio ─────────────────────────────────────────────────────────

// TestDiscoverStdio_OK pins the happy path: a stdio MCP server that responds
// to initialize + tools/list with two tool names. The function must return
// both names, no error.
//
// We pin tool names as a *set* (sorted equal) rather than a fixed order so
// the implementation is free to choose iteration order internally.
func TestDiscoverStdio_OK(t *testing.T) {
	binDir := writeFakeMCPBin(t, fakeMCPSpec{
		Mode:  "ok",
		Tools: []string{"tool1", "tool2"},
	})
	prependFakeMCPPath(t, binDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := DiscoverStdio(ctx, []string{"mcpfake"}, nil)
	if err != nil {
		t.Fatalf("DiscoverStdio OK: unexpected error: %v", err)
	}
	slices.Sort(got)
	want := []string{"tool1", "tool2"}
	if !slices.Equal(got, want) {
		t.Errorf("DiscoverStdio OK = %v, want %v", got, want)
	}
}

// TestDiscoverStdio_InitializeFails pins the failure path where the server
// does not reply to the initialize request. The function MUST return a
// non-nil error and MUST NOT panic.
//
// Assumption: the implementer surfaces the failure as a wrapped error from
// the read/write pipeline (any non-nil error is acceptable; we do not pin
// the exact wrap text because that would couple the test to internal
// fmt.Errorf formatting).
func TestDiscoverStdio_InitializeFails(t *testing.T) {
	binDir := writeFakeMCPBin(t, fakeMCPSpec{Mode: "no_init_response"})
	prependFakeMCPPath(t, binDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := DiscoverStdio(ctx, []string{"mcpfake"}, nil)
	if err == nil {
		t.Fatalf("DiscoverStdio(initialize fails) err = nil, tools = %v, want error", got)
	}
}

// TestDiscoverStdio_ToolsListEmpty pins the contract for a server that
// completes the handshake but advertises no tools. The function returns a
// zero-length result (nil or []string{} are both acceptable) with no error:
// the server responded validly, the discovery just found nothing.
//
// Assumption: an empty tools list is NOT an error — the install proceeds
// with no tools to probe. This matches the existing kanban probe behavior,
// which returns (nil, nil) in this case.
func TestDiscoverStdio_ToolsListEmpty(t *testing.T) {
	binDir := writeFakeMCPBin(t, fakeMCPSpec{
		Mode:  "ok",
		Tools: []string{}, // explicit: zero tools
	})
	prependFakeMCPPath(t, binDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := DiscoverStdio(ctx, []string{"mcpfake"}, nil)
	if err != nil {
		t.Fatalf("DiscoverStdio(empty tools) err = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("DiscoverStdio(empty tools) = %v, want empty (len 0)", got)
	}
}

// TestDiscoverStdio_Timeout pins that the function respects the context's
// deadline: when ctx times out, the function returns a context.DeadlineExceeded
// error (or wraps it) within a small grace period after the deadline.
//
// The fake binary blocks forever (mode=hang). The test gives the ctx a
// 500ms deadline and a 2s safety guard: if the function takes longer than
// 2s to return, the test fails with "did not respect context timeout".
//
// Assumption: errors.Is(err, context.DeadlineExceeded) is a stable contract.
// The existing kanban probe uses context.WithTimeout internally; the
// extracted version must surface that timeout to the caller via the same
// error type.
func TestDiscoverStdio_Timeout(t *testing.T) {
	binDir := writeFakeMCPBin(t, fakeMCPSpec{Mode: "hang"})
	prependFakeMCPPath(t, binDir)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	type result struct {
		tools []string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		tools, err := DiscoverStdio(ctx, []string{"mcpfake"}, nil)
		done <- result{tools, err}
	}()

	select {
	case r := <-done:
		if !errors.Is(r.err, context.DeadlineExceeded) {
			t.Errorf("DiscoverStdio(timeout) err = %v, want context.DeadlineExceeded (or wrap)", r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("DiscoverStdio did not return within 2s after ctx timeout; ctx is being ignored")
	}
}

// TestDiscoverStdio_EnvInjection pins that the env map passed to DiscoverStdio
// is actually applied to the child process's environment. The fake binary
// is configured with EnvEcho:["MCP_TEST_TOKEN"]; if the env var is set on
// the child, it appears as a tool with that value. We assert the tool name
// equals the value we passed in.
//
// This is a behavior pin: it does NOT assume the implementation strategy
// (os.Environ() + append, or a fresh env with overrides). It only asserts
// the externally visible consequence — the env var reaches the child.
func TestDiscoverStdio_EnvInjection(t *testing.T) {
	binDir := writeFakeMCPBin(t, fakeMCPSpec{
		Mode:    "ok",
		Tools:   []string{"always-present"},
		EnvEcho: []string{"MCP_TEST_TOKEN"},
	})
	prependFakeMCPPath(t, binDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const secret = "abc123"
	got, err := DiscoverStdio(ctx, []string{"mcpfake"}, map[string]string{
		"MCP_TEST_TOKEN": secret,
	})
	if err != nil {
		t.Fatalf("DiscoverStdio(env injection) unexpected error: %v", err)
	}
	if !slices.Contains(got, secret) {
		t.Errorf("DiscoverStdio(env injection) tools = %v, want to contain %q "+
			"(the env var must be applied to the child process)", got, secret)
	}
}

// TestDiscoverStdio_CtxCancel pins that a pre-canceled context aborts the
// call without hanging. The fake binary blocks forever, so the only way
// the test can return is if the function respects the already-canceled ctx.
//
// We assert errors.Is(err, context.Canceled) — the standard Go contract
// for a context that was canceled before any work was done.
func TestDiscoverStdio_CtxCancel(t *testing.T) {
	binDir := writeFakeMCPBin(t, fakeMCPSpec{Mode: "hang"})
	prependFakeMCPPath(t, binDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	type result struct {
		tools []string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		tools, err := DiscoverStdio(ctx, []string{"mcpfake"}, nil)
		done <- result{tools, err}
	}()

	select {
	case r := <-done:
		if !errors.Is(r.err, context.Canceled) {
			t.Errorf("DiscoverStdio(canceled) err = %v, want context.Canceled (or wrap)", r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("DiscoverStdio did not return within 2s on canceled ctx; ctx is being ignored")
	}
}

// ─── DiscoverHTTP ──────────────────────────────────────────────────────────

// TestDiscoverHTTP_OK pins the happy path: an HTTP MCP endpoint that
// responds to tools/list with two tool names. The function returns both
// names, no error.
//
// We pin a *set* of tool names (sorted equal) so iteration order in the
// implementation is unconstrained.
func TestDiscoverHTTP_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond only to tools/list; any other request returns 404 so a
		// probe implementation that mistakenly issues initialize over HTTP
		// would surface as a test failure rather than a silent pass.
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Method != "tools/list" {
			http.Error(w, "unexpected method: "+body.Method, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"tools": []map[string]string{
					{"name": "fetch"},
					{"name": "search"},
				},
			},
		})
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	got, err := DiscoverHTTP(ctx, srv.URL)
	if err != nil {
		t.Fatalf("DiscoverHTTP OK: unexpected error: %v", err)
	}
	slices.Sort(got)
	want := []string{"fetch", "search"}
	if !slices.Equal(got, want) {
		t.Errorf("DiscoverHTTP OK = %v, want %v", got, want)
	}
}

// TestDiscoverHTTP_NonOK pins the failure path for an HTTP error status.
// The function must return a non-nil error.
//
// Assumption: any non-2xx response is an error. The exact wrap text is not
// pinned; we only assert err != nil so the test is not coupled to internal
// formatting.
func TestDiscoverHTTP_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal", http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := DiscoverHTTP(ctx, srv.URL)
	if err == nil {
		t.Fatalf("DiscoverHTTP(500) err = nil, want error")
	}
}

// TestDiscoverHTTP_Timeout pins that the function respects the context's
// deadline: when the server hangs and ctx times out, the function returns
// a context.DeadlineExceeded error (or wraps it) within a small grace
// period after the deadline.
//
// Assumption: the implementation uses http.NewRequestWithContext and
// respects the ctx's deadline. errors.Is(err, context.DeadlineExceeded)
// is the standard Go contract for this case (the underlying *url.Error
// wraps the timeout).
func TestDiscoverHTTP_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a hanging server. The handler must return in finite
		// time so httptest.Server.Close() can shut down cleanly within
		// its 5s grace window. We do NOT rely on r.Context().Done()
		// alone: when the *client* closes the connection (the case we
		// are actually testing), the server-side r.Context() is not
		// guaranteed to fire, and the deferred srv.Close() would hang
		// and panic with "httptest.Server blocked in Close after 5
		// seconds" even though the test assertion already passed.
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	type result struct {
		tools []string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		tools, err := DiscoverHTTP(ctx, srv.URL)
		done <- result{tools, err}
	}()

	select {
	case r := <-done:
		if !errors.Is(r.err, context.DeadlineExceeded) {
			t.Errorf("DiscoverHTTP(timeout) err = %v, want context.DeadlineExceeded (or wrap)", r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("DiscoverHTTP did not return within 2s after ctx timeout; ctx is being ignored")
	}
}

// TestDiscoverHTTP_MalformedJSON pins the failure path for a server that
// returns a non-JSON body. The function must return a non-nil error.
//
// We send HTML because it is the most common "wrong content type" the
// real probe will hit in the wild (e.g. a corporate proxy returning a
// login page on a private endpoint).
//
// Assumption: any decode failure is an error. The exact wrap text is not
// pinned.
func TestDiscoverHTTP_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><html>login</html>"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := DiscoverHTTP(ctx, srv.URL)
	if err == nil {
		t.Fatalf("DiscoverHTTP(malformed JSON) err = nil, want error")
	}
	// Sanity: the error must mention the decode failure somewhere, so a
	// silent 200-OK-on-bad-body bug is caught. We accept any of the common
	// markers because the wrap text is implementation-defined.
	msg := strings.ToLower(err.Error())
	hasMarker := strings.Contains(msg, "decode") ||
		strings.Contains(msg, "json") ||
		strings.Contains(msg, "invalid") ||
		strings.Contains(msg, "unmarshal")
	if !hasMarker {
		t.Errorf("DiscoverHTTP(malformed JSON) err = %q, want an error mentioning decode/json/invalid", err)
	}
}
