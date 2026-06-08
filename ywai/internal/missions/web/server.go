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

	mux := http.NewServeMux()
	h := &Handlers{store: store, hub: s.hub, startTime: s.startedAt}

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
	mux.HandleFunc("POST /api/missions/{id}/features/{featureId}/retry", h.RetryFeature)

	// WebSocket
	mux.HandleFunc("GET /ws", h.HandleWebSocket)

	// UI
	mux.Handle("GET /", uiHandler())

	s.mux = mux
	return s
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

	log.Printf("Missions Web UI running on http://localhost:%d", s.port)
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
