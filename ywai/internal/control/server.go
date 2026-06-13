package control

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/kanban"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions/web"
)

const DefaultPort = 5768

var (
	embeddedUI   func() fs.FS
	defaultServer   *Server
	defaultServerMu sync.Mutex
)

// RegisterEmbeddedUI registers the embedded UI filesystem.
func RegisterEmbeddedUI(ui func() fs.FS) {
	embeddedUI = ui
}

// Server is the unified ywai control server combining Kanban and Missions.
type Server struct {
	port      int
	kanban    *kanban.Server
	missions  *web.Server
	httpSrv   *http.Server
	mux       *http.ServeMux
	portReady chan struct{}
	startedAt time.Time
	mu        sync.Mutex
}

// New creates a new control server.
func New(port int) (*Server, error) {
	if port == 0 {
		port = DefaultPort
	}

	kanbanDataDir := ""
	if home, err := os.UserHomeDir(); err == nil {
		kanbanDataDir = filepath.Join(home, ".config", "opencode", "kanban-data")
	}
	kServer := kanban.New(port, kanbanDataDir)

	missionsStore, err := missions.OpenStore()
	if err != nil {
		return nil, fmt.Errorf("failed to open missions store: %w", err)
	}
	mServer := web.New(port, missionsStore)

	s := &Server{
		port:      port,
		kanban:    kServer,
		missions:  mServer,
		portReady: make(chan struct{}),
		startedAt: time.Now(),
	}

	s.buildRoutes()
	return s, nil
}

// buildRoutes registers all routes: explicit APIs first, then SPA catch-all.
func (s *Server) buildRoutes() {
	s.mux = http.NewServeMux()

	// Health check
	s.mux.HandleFunc("GET /health", s.healthHandler)

	// ─── Kanban API ──────────────────────────────────────────────
	s.mux.HandleFunc("/api/", s.kanbanHandler)

	// ─── Missions API ────────────────────────────────────────────
	s.mux.HandleFunc("/missions/api/", s.missionsHandler)
	s.mux.HandleFunc("/missions/ws", s.missionsHandler)

	// ─── React SPA ──────────────────────────────────────────────
	// Everything else (/, /missions, /settings, /app.js, etc.)
	s.mux.HandleFunc("/", s.serveSPA)
}

// kanbanHandler forwards requests to the kanban HTTP handler.
func (s *Server) kanbanHandler(w http.ResponseWriter, r *http.Request) {
	s.kanban.HTTPHandler().ServeHTTP(w, r)
}

// missionsHandler strips the /missions prefix and forwards to missions handler.
func (s *Server) missionsHandler(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = strings.TrimPrefix(r.URL.Path, "/missions")
	if r.URL.Path == "" {
		r.URL.Path = "/"
	}
	s.missions.Handler().ServeHTTP(w, r)
}

// healthHandler returns server health status.
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","uptime":"%s","started":"%s"}`,
		time.Since(s.startedAt).String(),
		s.startedAt.Format(time.RFC3339))
}

// serveSPA serves all non-API requests: static assets or SPA index.html.
func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Root or known SPA routes → index.html
	if path == "/" || path == "/missions" || path == "/settings" {
		s.serveSPAIndex(w, r)
		return
	}

	// Static assets: files with extensions
	if strings.Contains(path, ".") {
		assetPath := strings.TrimPrefix(path, "/")
		contentType := guessContentType(assetPath)
		w.Header().Set("Content-Type", contentType)

		// Try embedded first
		if embeddedUI != nil {
			uiFS := embeddedUI()
			content, err := fs.ReadFile(uiFS, assetPath)
			if err == nil {
				w.Write(content)
				return
			}
		}

		// Fallback to filesystem (dev mode)
		fullPath := filepath.Join("internal", "control", "web", "dist", assetPath)
		content, err := os.ReadFile(fullPath)
		if err == nil {
			w.Write(content)
			return
		}

		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Everything else → SPA fallback (client-side routing)
	s.serveSPAIndex(w, r)
}

// serveSPAIndex serves the React SPA index.html.
func (s *Server) serveSPAIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if embeddedUI != nil {
		uiFS := embeddedUI()
		content, err := fs.ReadFile(uiFS, "index.html")
		if err == nil {
			w.Write(content)
			return
		}
	}

	fullPath := filepath.Join("internal", "control", "web", "dist", "index.html")
	content, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, "UI not found", http.StatusNotFound)
		return
	}
	w.Write(content)
}

// guessContentType returns a content type based on file extension.
func guessContentType(path string) string {
	switch {
	case strings.HasSuffix(path, ".js"):
		return "application/javascript"
	case strings.HasSuffix(path, ".css"):
		return "text/css"
	case strings.HasSuffix(path, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".json"):
		return "application/json"
	case strings.HasSuffix(path, ".ico"):
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

// Start starts the control server.
func (s *Server) Start() error {
	log.Printf("Starting control server on port %d", s.port)

	// Start kanban hub in background
	go s.kanban.Hub().Run()

	// Start missions hub in background
	go s.missions.Hub().Run()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", s.port, err)
	}

	s.httpSrv = &http.Server{Handler: s.mux}

	// Signal port is ready
	close(s.portReady)

	return s.httpSrv.Serve(listener)
}

// Stop shuts down the control server.
func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpSrv.Shutdown(ctx)
	}
}

// Port returns the server port.
func (s *Server) Port() int {
	return s.port
}

// WaitForPort blocks until the server is ready, returning the port.
func (s *Server) WaitForPort() int {
	<-s.portReady
	return s.port
}

// GetOrStart creates and starts a control server if one isn't already running.
func GetOrStart(port int) (*Server, error) {
	defaultServerMu.Lock()
	defer defaultServerMu.Unlock()

	if defaultServer != nil {
		return defaultServer, nil
	}

	s, err := New(port)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := s.Start(); err != nil {
			log.Printf("control server error: %v", err)
		}
	}()

	s.WaitForPort()
	defaultServer = s
	return s, nil
}

// IsRunning checks if the control server is currently running.
func IsRunning() bool {
	defaultServerMu.Lock()
	defer defaultServerMu.Unlock()
	return defaultServer != nil
}

// GetPort returns the port of the running control server, or 0 if not running.
func GetPort() int {
	defaultServerMu.Lock()
	defer defaultServerMu.Unlock()
	if defaultServer == nil {
		return 0
	}
	return defaultServer.Port()
}
