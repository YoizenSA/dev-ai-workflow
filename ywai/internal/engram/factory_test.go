package engram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbeServer_Reachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if !ProbeServer(context.Background(), srv.URL) {
		t.Fatal("Expected ProbeServer=true")
	}
}

func TestProbeServer_Unreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	if ProbeServer(context.Background(), srv.URL) {
		t.Fatal("Expected ProbeServer=false")
	}
}

func TestDefaultClient_EnvOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("ENGRAM_URL", srv.URL)
	c := DefaultClient()
	if c.baseURL != srv.URL {
		t.Fatalf("Expected baseURL=%s, got %s", srv.URL, c.baseURL)
	}
}
