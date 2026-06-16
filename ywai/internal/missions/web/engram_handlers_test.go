package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
)

// registerEngramTestRoutes builds a mux with ONLY the engram routes, so a
// minimal Handlers (no store/projectStore) can be exercised without panicking.
func registerEngramTestRoutes(h *Handlers) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/engram/status", h.EngramStatus)
	mux.HandleFunc("GET /api/engram/observations", h.ListObservations)
	mux.HandleFunc("GET /api/engram/observations/{id}", h.GetObservation)
	mux.HandleFunc("PATCH /api/engram/observations/{id}", h.UpdateObservation)
	mux.HandleFunc("DELETE /api/engram/observations/{id}", h.DeleteObservation)
	mux.HandleFunc("POST /api/engram/save", h.SaveObservation)
	mux.HandleFunc("GET /api/engram/search", h.SearchObservations)
	mux.HandleFunc("GET /api/engram/stats", h.EngramStats)
	mux.HandleFunc("GET /api/engram/sessions", h.ListEngramSessions)
	mux.HandleFunc("GET /api/engram/timeline", h.EngramTimeline)
	mux.HandleFunc("GET /api/engram/context", h.EngramContext)
	mux.HandleFunc("PUT /api/engram/context", h.UpdateEngramContext)
	mux.HandleFunc("POST /api/engram/consolidations", h.StartConsolidation)
	mux.HandleFunc("GET /api/engram/consolidations/{id}", h.GetConsolidation)
	mux.HandleFunc("POST /api/engram/consolidations/{id}/apply", h.ApplyConsolidation)
	mux.HandleFunc("POST /api/engram/consolidations/{id}/discard", h.DiscardConsolidation)
	return recoveryMiddleware(json405Middleware(mux))
}

func doReq(t *testing.T, handler http.Handler, method, target string, body string) *http.Response {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w.Result()
}

func TestHandler_EngramStatus_Connected(t *testing.T) {
	h := &Handlers{engramClient: &fakeEngramClient{}}
	resp := doReq(t, registerEngramTestRoutes(h), http.MethodGet, "/api/engram/status", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if body["connected"] != true {
		t.Fatalf("expected connected=true, got %v", body["connected"])
	}
}

func TestHandler_EngramStatus_NotConfigured(t *testing.T) {
	h := &Handlers{}
	resp := doReq(t, registerEngramTestRoutes(h), http.MethodGet, "/api/engram/status", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()
	if body["connected"] != false {
		t.Fatalf("expected connected=false, got %v", body["connected"])
	}
}

func TestHandler_SaveObservation(t *testing.T) {
	fe := &fakeEngramClient{}
	h := &Handlers{engramClient: fe}
	resp := doReq(t, registerEngramTestRoutes(h), http.MethodPost, "/api/engram/save",
		`{"type":"save","content":"hello","importance":5}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
	if len(fe.saved) != 1 || fe.saved[0].Content != "hello" {
		t.Fatalf("expected 1 save 'hello', got %+v", fe.saved)
	}
}

func TestHandler_SaveObservation_Validation(t *testing.T) {
	h := &Handlers{engramClient: &fakeEngramClient{}}
	resp := doReq(t, registerEngramTestRoutes(h), http.MethodPost, "/api/engram/save",
		`{"type":"save","content":""}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandler_DeleteObservation(t *testing.T) {
	fe := &fakeEngramClient{}
	h := &Handlers{engramClient: fe}
	resp := doReq(t, registerEngramTestRoutes(h), http.MethodDelete, "/api/engram/observations/obs_3", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
	if len(fe.deletedIDs) != 1 || fe.deletedIDs[0] != "obs_3" {
		t.Fatalf("expected delete obs_3, got %v", fe.deletedIDs)
	}
}

func TestHandler_Search(t *testing.T) {
	h := &Handlers{engramClient: &fakeEngramClient{}}
	resp := doReq(t, registerEngramTestRoutes(h), http.MethodGet, "/api/engram/search?q=react", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandler_Search_MissingQ(t *testing.T) {
	h := &Handlers{engramClient: &fakeEngramClient{}}
	resp := doReq(t, registerEngramTestRoutes(h), http.MethodGet, "/api/engram/search", "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandler_EngramNotConfigured(t *testing.T) {
	h := &Handlers{} // no engramClient
	resp := doReq(t, registerEngramTestRoutes(h), http.MethodGet, "/api/engram/observations", "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandler_GetConsolidation_NotFound(t *testing.T) {
	h := &Handlers{engramClient: &fakeEngramClient{},
		consolidations: NewConsolidationManager(&fakeEngramClient{}, nil, nil)}
	resp := doReq(t, registerEngramTestRoutes(h), http.MethodGet, "/api/engram/consolidations/nope", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHandler_StartConsolidation_NoOpencode(t *testing.T) {
	h := &Handlers{engramClient: &fakeEngramClient{},
		consolidations: NewConsolidationManager(&fakeEngramClient{}, nil, nil)}
	// opencodeClient is nil → sessions unavailable → 503
	resp := doReq(t, registerEngramTestRoutes(h), http.MethodPost, "/api/engram/consolidations",
		`{"model":"m","agent":"memory"}`)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// Compile-time guard: ensure engram import is used in this test file.
var _ = engram.SaveRequest{}
