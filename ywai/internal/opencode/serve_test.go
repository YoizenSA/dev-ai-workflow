package opencode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStartPortFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want int
	}{
		{"", DefaultPort},
		{"http://127.0.0.1:4096", 4096},
		{"http://127.0.0.1:5757", 5757},
		{"https://127.0.0.1:8443", 8443},
		{"http://127.0.0.1", DefaultPort},
		{"not a url", DefaultPort},
	}
	for _, tt := range tests {
		if got := StartPortFromURL(tt.url); got != tt.want {
			t.Errorf("StartPortFromURL(%q) = %d, want %d", tt.url, got, tt.want)
		}
	}
}

func TestFindRunningURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	port := StartPortFromURL(srv.URL)
	got, ok := FindRunningURL(context.Background(), port, 1, nil)
	if !ok || got != srv.URL {
		t.Fatalf("FindRunningURL = (%q, %v), want (%q, true)", got, ok, srv.URL)
	}
}

func TestFindRunningURL_ExtraValidator(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	port := StartPortFromURL(srv.URL)

	// A rejecting validator makes the walk find nothing.
	if _, ok := FindRunningURL(context.Background(), port, 1,
		func(context.Context, string) bool { return false }); ok {
		t.Fatal("FindRunningURL accepted a candidate the validator rejected")
	}

	got, ok := FindRunningURL(context.Background(), port, 1,
		func(_ context.Context, u string) bool { return u == srv.URL })
	if !ok || got != srv.URL {
		t.Fatalf("FindRunningURL = (%q, %v), want (%q, true)", got, ok, srv.URL)
	}
}

func TestFindRunningURL_NoneUp(t *testing.T) {
	// Port 1 (tcpmux) is never a healthy opencode server in CI.
	if _, ok := FindRunningURL(context.Background(), 1, 1, nil); ok {
		t.Fatal("FindRunningURL reported a running server on a dead port")
	}
}
