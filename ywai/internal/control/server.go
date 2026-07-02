package control

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/kanban"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions/web"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/selfupdate"
)

const DefaultPort = 5768

var (
	embeddedUI      func() fs.FS
	defaultServer   *Server
	defaultServerMu sync.Mutex

	// AppVersion is the ywai binary version, set by the CLI at startup.
	AppVersion = "dev"
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
	startErr  error
	jobs      *mcp.JobManager
	workflows *workflowsAPI
	push      *PushAPI
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

	// Reuse the kanban WebSocket hub as the install-job broadcaster. The
	// JobManager only calls Hub.Broadcast, so a nil hub is a safe no-op;
	// wiring the kanban hub here means a single WS endpoint fans out
	// both kanban and install events to the UI.
	s.jobs = mcp.NewJobManager(s.kanban.Hub())

	// Wire the missions→kanban event bridge
	projector := NewKanbanProjector(kServer, missionsStore)
	mServer.SetEventSink(projector.Project)

	// Push notification setup
	pushStore, _ := NewPushStore()
	if pushStore != nil {
		s.push = NewPushAPI(pushStore)
		projector.OnComplete(s.push.sender.Send)
	}

	s.buildRoutes()
	return s, nil
}

// buildRoutes registers all routes: explicit APIs first, then SPA catch-all.
func (s *Server) buildRoutes() {
	s.mux = http.NewServeMux()

	// Health check
	s.mux.HandleFunc("GET /health", s.healthHandler)

	// Version check
	s.mux.HandleFunc("GET /api/version", s.versionHandler)

	// Self-update trigger: spawns a detached `ywai update` process.
	s.mux.HandleFunc("POST /api/update", s.updateHandler)

	// ─── Kanban API ──────────────────────────────────────────────
	s.mux.HandleFunc("/api/", s.kanbanHandler)

	// ─── Missions API ────────────────────────────────────────────
	s.mux.HandleFunc("/missions/api/", s.missionsHandler)
	s.mux.HandleFunc("/missions/ws", s.missionsHandler)

	// ─── MCP Store API ──────────────────────────────────────────
	s.registerMcpStoreRoutes()

	// ─── Azure DevOps Config API ────────────────────────────────
	s.registerAdoConfigRoutes()

	// ─── Agents.md API ──────────────────────────────────────────
	s.registerAgentsMdRoutes()

	// ─── Workflows API (Workflow Studio) ────────────────────────
	s.registerWorkflowsRoutes()

	// ─── Settings maintenance API (SDD cleanup, etc.) ───────────
	s.registerSettingsRoutes()

	// ─── Git status API ─────────────────────────────────────────
	s.registerGitRoutes()

	// ─── Push notifications API ─────────────────────────────────
	s.registerPushRoutes()

	// ─── Skills CRUD API ────────────────────────────────────────
	s.registerSkillsRoutes()

	// ─── React SPA ──────────────────────────────────────────────
	// Chat proxy (SSE to OpenCode server)
	s.registerChatRoutes()

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

// versionHandler returns the current and latest ywai version.
func (s *Server) versionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	current := AppVersion
	latest, err := selfupdate.LatestVersion()
	if err != nil {
		// Can't check — still return current version
		fmt.Fprintf(w, `{"current":%q,"latest":null,"updateAvailable":false,"error":%q}`,
			current, err.Error())
		return
	}

	// Only offer an update when the latest published release is strictly NEWER
	// than the running version. A plain inequality check wrongly flagged an
	// update when GitHub's "latest" release lagged behind a newer local build
	// (e.g. running v8.8.8 while the latest published release is still v8.8.6).
	updateAvail := !strings.HasPrefix(current, "dev") && isNewerVersion(latest, current)
	fmt.Fprintf(w, `{"current":%q,"latest":%q,"updateAvailable":%t}`,
		current, latest, updateAvail)
}

// isNewerVersion reports whether semver `latest` is strictly greater than
// `current`. Both may carry a leading "v" and a pre-release/build suffix, which
// are ignored; only MAJOR.MINOR.PATCH are compared.
func isNewerVersion(latest, current string) bool {
	l := parseSemver(latest)
	c := parseSemver(current)
	for i := 0; i < 3; i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	var out [3]int
	for i, part := range strings.Split(v, ".") {
		if i >= 3 {
			break
		}
		out[i], _ = strconv.Atoi(strings.TrimSpace(part))
	}
	return out
}

// updateHandler spawns a detached `ywai update` process and returns
// immediately. The update pipeline self-replaces this binary and, in its
// final step, kills and relaunches this very server — so we must NOT run it
// in-process. The child is detached (Setsid on Unix) so it survives the
// parent server being killed mid-update. The frontend polls /health until
// the relaunched server answers, then reloads.
func (s *Server) updateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if strings.HasPrefix(AppVersion, "dev") {
		w.WriteHeader(http.StatusConflict)
		fmt.Fprintf(w, `{"started":false,"error":%q}`,
			"self-update is disabled for dev builds")
		return
	}

	exe, err := selfupdate.ResolvedExecutable()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"started":false,"error":%q}`, err.Error())
		return
	}

	cmd := exec.Command(exe, "update")
	cmd.SysProcAttr = detachedSysProcAttr()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"started":false,"error":%q}`, err.Error())
		return
	}
	// Release so the child is not reaped when this process exits during the
	// update's restart step.
	_ = cmd.Process.Release()

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, `{"started":true,"pid":%d}`, cmd.Process.Pid)
}

// serveSPA serves all non-API requests: static assets or SPA index.html.
func (s *Server) serveSPA(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Root or known SPA routes → index.html
	if path == "/" || path == "/missions" || path == "/settings" || path == "/memories" || path == "/workflows" {
		s.serveSPAIndex(w, r)
		return
	}

	// Static assets: files with extensions
	if strings.Contains(path, ".") {
		assetPath := strings.TrimPrefix(path, "/")
		contentType := guessContentType(assetPath)
		w.Header().Set("Content-Type", contentType)

		// Content-hashed assets (under /assets/) are immutable — cache forever.
		if strings.HasPrefix(assetPath, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		// Try embedded first
		if embeddedUI != nil {
			uiFS := embeddedUI()
			content, err := fs.ReadFile(uiFS, assetPath)
			if err == nil {
				_, _ = w.Write(content)
				return
			}
		}

		// Fallback to filesystem (dev mode)
		fullPath := filepath.Join("internal", "control", "web", "dist", assetPath)
		content, err := os.ReadFile(fullPath)
		if err == nil {
			_, _ = w.Write(content)
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
	// Never cache the HTML shell: it references content-hashed assets, so it must
	// always be revalidated to pick up a new build.
	w.Header().Set("Cache-Control", "no-cache")

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
		s.startErr = fmt.Errorf("failed to listen on port %d: %w", s.port, err)
		close(s.portReady)
		return s.startErr
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
	if s.startErr != nil {
		return nil, s.startErr
	}
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
