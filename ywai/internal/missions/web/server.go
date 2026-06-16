package web

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

const DefaultPort = 5769

// Server is the Missions Web UI server.
type Server struct {
	port      int
	store     *missions.MissionsStore
	hub       *Hub
	httpSrv   *http.Server
	mux       *http.ServeMux
	portReady chan struct{}
	startedAt time.Time
	mu        sync.Mutex
	handlers  *Handlers
	eventSink func(evtType string, payload interface{})
}

// New creates a new Missions Web UI server.
func New(port int, store *missions.MissionsStore) *Server {
	s := &Server{
		port:      port,
		store:     store,
		hub:       NewHub(),
		portReady: make(chan struct{}),
		startedAt: time.Now(),
	}

	// Create project store (persists to the missions base directory)
	projectStore, err := missions.NewProjectStore(store.BaseDir())
	if err != nil {
		log.Printf("WARNING: failed to create project store: %v", err)
		projectStore, _ = missions.NewProjectStore("") // Fallback to in-memory
	}

	mux := http.NewServeMux()
	oc := opencode.DefaultClient(context.Background())
	h := &Handlers{
		store:           store,
		projectStore:    projectStore,
		hub:             s.hub,
		startTime:       s.startedAt,
		opencodeClient:  oc,
		runningMissions: make(map[string]struct{}),
	}
	// Engram memory client + consolidation manager (in-memory runs).
	engramClient := engram.DefaultClient()
	h.consolidations = NewConsolidationManager(
		engramClient,
		func() opencode.SessionAPI { return h.opencodeClient.Sessions() },
		func(et string, payload any) { s.hub.BroadcastEvent(et, payload) },
	)
	h.engramClient = engramClient
	s.handlers = h

	registerRoutes(mux, h)

	s.mux = mux
	return s
}

// registerRoutes wires all HTTP routes onto the given mux using the handlers.
// Extracted from New() so tests can build a server from a custom Handlers
// (e.g. with injected planner/engineRunner) without duplicating the route table.
func registerRoutes(mux *http.ServeMux, h *Handlers) {
	// API routes
	mux.HandleFunc("GET /api/health", h.HealthCheck)
	mux.HandleFunc("GET /api/missions", h.ListMissions)
	mux.HandleFunc("GET /api/missions/{id}", h.GetMission)
	mux.HandleFunc("GET /api/missions/{id}/features", h.ListFeatures)
	mux.HandleFunc("GET /api/missions/{id}/validation", h.GetValidation)
	mux.HandleFunc("GET /api/missions/{id}/features/{featureId}/logs", h.GetFeatureLogs)
	mux.HandleFunc("POST /api/missions/{id}/pause", h.PauseMission)
	mux.HandleFunc("POST /api/missions/{id}/resume", h.ResumeMission)
	mux.HandleFunc("POST /api/missions/{id}/cancel", h.CancelMission)
	mux.HandleFunc("DELETE /api/missions/{id}", h.DeleteMission)
	mux.HandleFunc("POST /api/missions/{id}/features/{featureId}/retry", h.RetryFeature)

	// Mission artifacts
	mux.HandleFunc("GET /api/missions/{id}/artifacts/{type}", h.GetMissionArtifact)
	mux.HandleFunc("POST /api/missions/{id}/validate-contract", h.ValidateContract)

	// Mission creation + execution
	mux.HandleFunc("POST /api/missions", h.CreateMission)
	mux.HandleFunc("POST /api/missions/approve", h.ApprovePlan)
	mux.HandleFunc("POST /api/missions/{id}/run", h.RunMission)
	mux.HandleFunc("POST /api/missions/auto", h.AutoMission)

	// Project routes
	mux.HandleFunc("GET /api/projects", h.ListProjects)
	mux.HandleFunc("POST /api/projects", h.CreateProject)
	mux.HandleFunc("DELETE /api/projects/{name}", h.DeleteProject)
	mux.HandleFunc("GET /api/projects/{name}/git-info", h.GetProjectGitInfo)
	mux.HandleFunc("POST /api/projects/{name}/init-git", h.InitProjectGit)

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

	// WebSocket
	mux.HandleFunc("GET /ws", h.HandleWebSocket)

	// UI
	mux.Handle("GET /", uiHandler())
}

// Handler returns the full middleware-wrapped handler for testing.
func (s *Server) Handler() http.Handler {
	// Chain middleware (outermost to innermost):
	// 1. recoveryMiddleware - catch panics
	// 2. json405Middleware - intercept 405 responses to return JSON (VAL-WEB-047)
	// 3. validateMissionIDMiddleware - reject empty/null mission IDs (VAL-WEB-010)
	handler := validateMissionIDMiddleware(s.mux)
	handler = json405Middleware(handler)
	handler = recoveryMiddleware(handler)
	return handler
}

// SetEventSink sets an optional callback for mission events (e.g., kanban projection).
func (s *Server) SetEventSink(fn func(evtType string, payload interface{})) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventSink = fn
	if s.handlers != nil {
		s.handlers.eventSink = fn
	}
}

// Start starts the HTTP server on the configured port.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	// Determine actual port (useful if port was 0)
	s.mu.Lock()
	s.port = listener.Addr().(*net.TCPAddr).Port
	s.mu.Unlock()

	close(s.portReady)

	s.httpSrv = &http.Server{
		Handler:      s.Handler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := s.httpSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.hub.Shutdown()
	if s.httpSrv != nil {
		_ = s.httpSrv.Shutdown(ctx)
	}
}

// Port returns the port the server is listening on.
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// WaitForPort blocks until the server port is assigned.
func (s *Server) WaitForPort() int {
	<-s.portReady
	return s.Port()
}

// Hub returns the WebSocket hub for the missions server.
func (s *Server) Hub() *Hub {
	return s.hub
}

// GetServerState returns a snapshot of server state for health checks.
func (s *Server) GetServerState() ServerState {
	return ServerState{
		Uptime:  time.Since(s.startedAt).String(),
		Started: s.startedAt,
	}
}

// ServerState holds server metadata for health check responses.
type ServerState struct {
	Uptime  string    `json:"uptime"`
	Started time.Time `json:"started"`
}

// ─── Recovery Middleware ───────────────────────────────────────────────────

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

// ─── 405 JSON Middleware (VAL-WEB-047) ─────────────────────────────────────
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

// ─── Mission ID Validation Middleware (VAL-WEB-010) ────────────────────────
// Returns 400 for empty mission IDs (e.g., /api/missions/) or
// null byte in mission IDs (e.g., /api/missions/%00).

func validateMissionIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for empty mission ID: /api/missions/ (trailing slash, nothing after)
		if strings.HasPrefix(r.URL.Path, "/api/missions/") {
			rest := strings.TrimPrefix(r.URL.Path, "/api/missions/")
			// Extract the first path segment (the mission ID)
			idx := strings.Index(rest, "/")
			var missionID string
			if idx >= 0 {
				missionID = rest[:idx]
			} else {
				missionID = rest
			}

			if missionID == "" {
				writeError(w, http.StatusBadRequest, "mission id is required")
				return
			}

			// Check for null byte in mission ID (from %00 encoding)
			if strings.ContainsRune(missionID, 0) {
				writeError(w, http.StatusBadRequest, "mission id contains invalid characters")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// ─── JSON Helpers ──────────────────────────────────────────────────────────

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

// ─── Started singleton ─────────────────────────────────────────────────────

var (
	defaultServer   *Server
	defaultServerMu sync.Mutex
)

// GetOrStart gets the existing server or starts a new one on the given port.
func GetOrStart(port int, store *missions.MissionsStore) (*Server, error) {
	defaultServerMu.Lock()
	defer defaultServerMu.Unlock()

	if defaultServer != nil {
		return defaultServer, nil
	}

	s := New(port, store)
	errCh := make(chan error, 1)
	go func() {
		if err := s.Start(); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return nil, err
	case <-time.After(100 * time.Millisecond):
		// Give server a moment to start
	}

	// Wait for port to be assigned
	port = s.WaitForPort()
	log.Printf("Missions Web UI started on port %d", port)

	defaultServer = s
	return s, nil
}
