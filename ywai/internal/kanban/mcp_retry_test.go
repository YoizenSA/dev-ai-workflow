package kanban

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func testServerPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// A control server hiccup (500s while it restarts) must not fail the tool call:
// doRequest retries until it succeeds.
func TestDoRequest_RetriesTransientFailures(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	// server != nil disables port re-resolution so the test stays hermetic.
	m := &MCPAdapter{server: &Server{}, port: testServerPort(t, srv), client: &http.Client{Timeout: 2 * time.Second}}
	body, err := m.doRequest("PATCH", "/x", []byte("{}"))
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if string(body) != `{"ok":true}` {
		t.Fatalf("unexpected body %q", body)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

// A 4xx is a real client error (e.g. delegation not found) — retrying cannot
// help, so it must return immediately without burning retries.
func TestDoRequest_NoRetryOn4xx(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer srv.Close()

	m := &MCPAdapter{server: &Server{}, port: testServerPort(t, srv), client: &http.Client{Timeout: 2 * time.Second}}
	if _, err := m.doRequest("PATCH", "/x", []byte("{}")); err == nil {
		t.Fatal("expected error on 404")
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Fatalf("404 must not retry, got %d attempts", got)
	}
}
