// Package toolsapi serves the shared tool endpoints that outlive the missions
// feature: OpenCode models/agents/status/start, filesystem browse/mkdir, goal
// refinement, and the whole Engram memory API (REST + WebSocket).
//
// The control server mounts this handler under /missions/api/ and
// /missions/engram/ws. Those URL prefixes are a frozen contract with the UI;
// the missions name in them is historical.
package toolsapi

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// Handlers holds the collaborators the tool endpoints need.
type Handlers struct {
	hub            *Hub
	startTime      time.Time
	opencodeClient opencode.Client
	engramClient   engram.Client
	consolidations *ConsolidationManager

	// modelCache memoizes the slow opencode model lookup (see model_cache.go).
	modelCache modelCache
}

// Server wraps the tool API mux for mounting into a parent server.
type Server struct {
	mux      *http.ServeMux
	handlers *Handlers
	hub      *Hub
}

// New creates the tool API server: routes, hub, consolidation manager, and a
// warm-up of the model cache so the first Settings open never blocks on the
// multi-second `opencode models` CLI.
func New() *Server {
	hub := NewHub()
	oc := opencode.DefaultClient(context.Background())
	engramClient := engram.DefaultClient()

	h := &Handlers{
		hub:            hub,
		startTime:      time.Now(),
		opencodeClient: oc,
		engramClient:   engramClient,
	}
	h.consolidations = NewConsolidationManager(
		engramClient,
		func() opencode.SessionAPI { return h.opencodeClient.Sessions() },
		func(et string, payload any) { hub.BroadcastEvent(et, payload) },
	)
	h.WarmModels()

	mux := http.NewServeMux()
	registerRoutes(mux, h)

	return &Server{
		mux:      mux,
		handlers: h,
		hub:      hub,
	}
}

// registerRoutes wires the tool API routes. Every route here must be reachable
// through the control server's proxy prefixes (/missions/api/,
// /missions/engram/ws) — toolsapi_routes_test.go enforces it.
func registerRoutes(mux *http.ServeMux, h *Handlers) {
	// Health check
	mux.HandleFunc("GET /api/health", h.HealthCheck)

	// Filesystem browser
	mux.HandleFunc("GET /api/fs/browse", h.BrowseFS)
	mux.HandleFunc("POST /api/fs/mkdir", h.MkdirFS)

	// OpenCode config
	mux.HandleFunc("GET /api/opencode/models", h.ListModels)
	mux.HandleFunc("GET /api/opencode/agents", h.ListAgents)
	mux.HandleFunc("GET /api/opencode/status", h.OpenCodeStatus)
	mux.HandleFunc("POST /api/opencode/start", h.StartOpencode)

	// Engram memory API
	mux.HandleFunc("GET /api/engram/status", h.EngramStatus)
	mux.HandleFunc("GET /api/engram/observations", h.ListObservations)
	mux.HandleFunc("GET /api/engram/observations/{id}", h.GetObservation)
	mux.HandleFunc("PATCH /api/engram/observations/{id}", h.UpdateObservation)
	mux.HandleFunc("DELETE /api/engram/observations/{id}", h.DeleteObservation)
	mux.HandleFunc("POST /api/engram/save", h.SaveObservation)
	mux.HandleFunc("GET /api/engram/search", h.SearchObservations)
	mux.HandleFunc("GET /api/engram/stats", h.EngramStats)
	mux.HandleFunc("GET /api/engram/sessions", h.ListEngramSessions)
	mux.HandleFunc("DELETE /api/engram/sessions/{id}", h.DeleteEngramSession)
	mux.HandleFunc("GET /api/engram/prompts", h.ListEngramPrompts)
	mux.HandleFunc("DELETE /api/engram/prompts/{id}", h.DeleteEngramPrompt)
	mux.HandleFunc("GET /api/engram/timeline", h.EngramTimeline)
	mux.HandleFunc("GET /api/engram/context", h.EngramContext)
	mux.HandleFunc("PUT /api/engram/context", h.UpdateEngramContext)
	mux.HandleFunc("GET /api/engram/export", h.ExportEngram)
	mux.HandleFunc("POST /api/engram/import", h.ImportEngram)
	mux.HandleFunc("POST /api/engram/projects/merge", h.MergeEngramProjects)
	mux.HandleFunc("POST /api/engram/memory-evals", h.RunMemoryEval)

	// Consolidations
	mux.HandleFunc("POST /api/engram/consolidations", h.StartConsolidation)
	mux.HandleFunc("GET /api/engram/consolidations/{id}", h.GetConsolidation)
	mux.HandleFunc("POST /api/engram/consolidations/{id}/apply", h.ApplyConsolidation)
	mux.HandleFunc("POST /api/engram/consolidations/{id}/discard", h.DiscardConsolidation)

	// Engram WebSocket
	mux.HandleFunc("GET /engram/ws", h.HandleEngramWebSocket)

	// AI refinement
	mux.HandleFunc("POST /api/refine", h.RefineGoal)
}

// Handler returns the middleware-wrapped handler for mounting.
func (s *Server) Handler() http.Handler {
	// Chain middleware (outermost to innermost):
	// 1. recoveryMiddleware - catch panics
	// 2. json405Middleware - intercept 405 responses to return JSON, and keep
	//    WebSocket upgrades working through the chain (Hijacker passthrough).
	handler := json405Middleware(s.mux)
	handler = recoveryMiddleware(handler)
	return handler
}

// Hub returns the WebSocket hub (already running; see NewHub).
func (s *Server) Hub() *Hub {
	return s.hub
}

// —— Health Check ———————————————————————————————————————————————

// HealthCheck returns server health status.
func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"version": "dev",
		"uptime":  time.Since(h.startTime).String(),
	})
}

// —— Recovery Middleware ————————————————————————————————————————

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC recovered: %v", rec)
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"error": "internal server error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// —— 405 JSON Middleware ————————————————————————————————————————
// Intercepts 405 Method Not Allowed responses from Go's http.ServeMux
// and returns JSON format instead of Go's default plain text.

type json405Writer struct {
	http.ResponseWriter
	statusCode int
	wroteBody  bool
}

func (w *json405Writer) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	if statusCode == http.StatusMethodNotAllowed {
		// Replace 405 body with JSON
		w.ResponseWriter.Header().Set("Content-Type", "application/json")
		w.ResponseWriter.WriteHeader(statusCode)
		_, _ = w.ResponseWriter.Write([]byte(`{"error":"method not allowed"}`))
		w.wroteBody = true
		return
	}
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *json405Writer) Write(b []byte) (int, error) {
	if w.wroteBody {
		return len(b), nil
	}
	if w.statusCode == http.StatusMethodNotAllowed {
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}

// Hijack implements http.Hijacker so WebSocket upgrades work through
// the json405Middleware chain. It delegates to the underlying ResponseWriter's
// Hijacker if available.
func (w *json405Writer) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported")
}

func json405Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jw := &json405Writer{ResponseWriter: w}
		next.ServeHTTP(jw, r)
	})
}

// —— JSON Helpers ———————————————————————————————————————————————

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
