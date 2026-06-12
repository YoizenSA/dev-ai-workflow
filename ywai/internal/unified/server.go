package unified

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

var embeddedUI func() fs.FS

// RegisterEmbeddedUI registers the embedded UI filesystem.
func RegisterEmbeddedUI(ui func() fs.FS) {
	embeddedUI = ui
}

// Server is the unified ywai server combining Kanban and Missions.
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

// New creates a new unified server.
func New(port int) (*Server, error) {
	if port == 0 {
		port = DefaultPort
	}

	// Initialize kanban server
	kanbanDataDir := ""
	if home, err := os.UserHomeDir(); err == nil {
		kanbanDataDir = filepath.Join(home, ".config", "opencode", "kanban-data")
	}
	kServer := kanban.New(port, kanbanDataDir)

	// Initialize missions server
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

	// Build unified routes
	s.buildRoutes()

	return s, nil
}

// buildRoutes combines routes from both servers by registering their handlers directly.
func (s *Server) buildRoutes() {
	s.mux = http.NewServeMux()

	// Use a custom handler to route between kanban and missions
	s.mux.HandleFunc("/", s.routeHandler)
}

// routeHandler routes requests to the appropriate server based on path.
func (s *Server) routeHandler(w http.ResponseWriter, r *http.Request) {
	// Health check
	if r.URL.Path == "/health" {
		s.healthHandler(w, r)
		return
	}

	// Unified UI assets
	if strings.HasPrefix(r.URL.Path, "/ui/") {
		s.serveUIAsset(w, r)
		return
	}

	// Unified UI root
	if r.URL.Path == "/ui" {
		s.serveUnifiedUI(w, r)
		return
	}

	// Missions routes
	if r.URL.Path == "/missions" || strings.HasPrefix(r.URL.Path, "/missions/") {
		// Strip /missions prefix and forward to missions handler
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/missions")
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		s.missions.Handler().ServeHTTP(w, r)
		return
	}

	// Default to kanban
	s.kanban.HTTPHandler().ServeHTTP(w, r)
}

// healthHandler returns health status of the unified server.
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"ok","uptime":"%s","started":"%s"}`,
		time.Since(s.startedAt).String(),
		s.startedAt.Format(time.RFC3339))
}

// serveUnifiedUI serves the unified dashboard UI with sidebar navigation.
func (s *Server) serveUnifiedUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// Try embedded data first
	if embeddedUI != nil {
		uiFS := embeddedUI()
		content, err := fs.ReadFile(uiFS, "index.html")
		if err == nil {
			w.Write(content)
			return
		}
	}

	// Fallback to filesystem
	uiPath := filepath.Join("internal", "unified", "ui", "index.html")
	content, err := os.ReadFile(uiPath)
	if err != nil {
		// Fallback: serve a simple HTML if file not found
		html := `<!DOCTYPE html>
<html>
<head>
    <title>ywai Unified Dashboard</title>
    <style>
        body { font-family: system-ui; background: #272a35; color: #f3f5fb; padding: 2rem; }
        .nav { display: flex; gap: 1rem; margin-bottom: 2rem; }
        .nav a { color: #6ea3ff; text-decoration: none; padding: 0.5rem 1rem; border: 1px solid #4a3abf; border-radius: 8px; }
        .nav a:hover { background: #4a3abf; }
        .nav a.active { background: #1a66ff; }
        iframe { width: 100%; height: 70vh; border: 1px solid #4a3abf; border-radius: 8px; background: #1a1f2e; }
    </style>
</head>
<body>
    <div class="nav">
        <a href="#" onclick="loadKanban()" class="active">Kanban</a>
        <a href="#" onclick="loadMissions()">Missions</a>
    </div>
    <iframe id="content" src="/"></iframe>
    <script>
        function loadKanban() {
            document.getElementById('content').src = '/';
            document.querySelectorAll('.nav a').forEach(a => a.classList.remove('active'));
            event.target.classList.add('active');
        }
        function loadMissions() {
            document.getElementById('content').src = '/missions/';
            document.querySelectorAll('.nav a').forEach(a => a.classList.remove('active'));
            event.target.classList.add('active');
        }
    </script>
</body>
</html>`
		w.Write([]byte(html))
		return
	}

	w.Write(content)
}

// serveUIAsset serves static assets from the unified UI directory.
func (s *Server) serveUIAsset(w http.ResponseWriter, r *http.Request) {
	// Strip /ui/ prefix
	assetPath := strings.TrimPrefix(r.URL.Path, "/ui/")
	
	// Security: prevent directory traversal
	if strings.Contains(assetPath, "..") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Determine content type
	ext := filepath.Ext(assetPath)
	var contentType string
	switch ext {
	case ".css":
		contentType = "text/css; charset=utf-8"
	case ".js":
		contentType = "application/javascript; charset=utf-8"
	case ".svg":
		contentType = "image/svg+xml"
	case ".png":
		contentType = "image/png"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".html":
		contentType = "text/html; charset=utf-8"
	default:
		contentType = "application/octet-stream"
	}

	// Try embedded data first
	if embeddedUI != nil {
		uiFS := embeddedUI()
		content, err := fs.ReadFile(uiFS, assetPath)
		if err == nil {
			w.Header().Set("Content-Type", contentType)
			w.Write(content)
			return
		}
	}

	// Fallback to filesystem
	fullPath := filepath.Join("internal", "unified", "ui", assetPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Write(content)
}

// Start starts the unified server.
func (s *Server) Start() error {
	// Start WebSocket hubs
	go s.kanban.Hub().Run()
	go s.missions.Hub().Run()

	addr := fmt.Sprintf(":%d", s.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		if s.port != 0 {
			log.Printf("Port %d in use, falling back to random port", s.port)
			s.port = 0
			listener, err = net.Listen("tcp", ":0")
		}
		if err != nil {
			return fmt.Errorf("failed to listen: %w", err)
		}
	}

	s.mu.Lock()
	s.port = listener.Addr().(*net.TCPAddr).Port
	s.mu.Unlock()

	close(s.portReady)

	s.httpSrv = &http.Server{
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if err := s.httpSrv.Serve(listener); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("serve error: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() {
	if s.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
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

// Singleton instance
var (
	defaultServer   *Server
	defaultServerMu sync.Mutex
)

// GetOrStart returns the default unified server, starting it if needed.
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
			log.Printf("unified server error: %v", err)
		}
	}()

	s.WaitForPort()
	defaultServer = s
	return s, nil
}

// IsRunning checks if the unified server is currently running.
func IsRunning() bool {
	defaultServerMu.Lock()
	defer defaultServerMu.Unlock()
	return defaultServer != nil
}

// GetPort returns the port of the running unified server, or 0 if not running.
func GetPort() int {
	defaultServerMu.Lock()
	defer defaultServerMu.Unlock()
	if defaultServer == nil {
		return 0
	}
	return defaultServer.Port()
}
