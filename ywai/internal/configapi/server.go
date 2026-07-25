package configapi

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// DefaultUIPort is the default port for the UI server.
const DefaultUIPort = 5768

// Server is the embedded HTTP server for the config/UI API.
type Server struct {
	port      int
	hub       *Hub
	httpSrv   *http.Server
	mux       *http.ServeMux
	portReady chan struct{} // closed when port is assigned
}

// New creates a new config/UI API server listening on the given port.
// Use port 0 to let the OS pick a free port.
func New(port int) *Server {
	hub := NewHub()
	oc := opencode.DefaultClient(context.Background())
	handlers := &Handlers{hub: hub, opencodeClient: oc}

	mux := http.NewServeMux()

	// WebSocket route
	mux.HandleFunc("GET /api/events", handlers.HandleWebSocket)

	// Config API
	mux.HandleFunc("GET /api/config/opencode", handlers.GetOpenCodeConfig)
	mux.HandleFunc("PUT /api/config/opencode", handlers.PutOpenCodeConfig)
	mux.HandleFunc("GET /api/config/agents", handlers.ListAgents)
	// Register the literal "graph" path BEFORE the {name} routes so Go 1.22's
	// ServeMux matches it as the static segment rather than an agent named "graph".
	mux.HandleFunc("GET /api/config/agents/graph", handlers.GetAgentGraph)
	mux.HandleFunc("GET /api/config/agents/{name}", handlers.GetAgent)
	mux.HandleFunc("PUT /api/config/agents/{name}", handlers.PutAgent)
	mux.HandleFunc("POST /api/config/agents", handlers.CreateAgent)
	mux.HandleFunc("DELETE /api/config/agents/{name}", handlers.DeleteAgent)
	mux.HandleFunc("GET /api/config/agents/{name}/permissions", handlers.GetAgentPermissions)
	mux.HandleFunc("PUT /api/config/agents/{name}/permissions", handlers.PutAgentPermissions)
	mux.HandleFunc("GET /api/config/agents/{name}/task-permissions", handlers.GetAgentTaskPermissions)
	mux.HandleFunc("PUT /api/config/agents/{name}/task-permissions", handlers.PutAgentTaskPermissions)
	mux.HandleFunc("GET /api/config/agents/{name}/model", handlers.GetAgentModel)
	mux.HandleFunc("PUT /api/config/agents/{name}/model", handlers.PutAgentModel)
	mux.HandleFunc("GET /api/config/agents/{name}/delegation-rules", handlers.GetDelegationRules)
	mux.HandleFunc("PUT /api/config/agents/{name}/delegation-rules", handlers.PutDelegationRules)
	mux.HandleFunc("GET /api/config/tools", handlers.ListTools)
	// Skills are served by the control server (internal/control/skills_handlers.go),
	// which registers the same paths on the same mux and wins. A second set here
	// looked live and answered nothing — reading it instead of the real one is
	// how a cwd-dependent skills bug stayed hidden for an afternoon.
	mux.HandleFunc("GET /api/config/mcp", handlers.ListMCP)
	mux.HandleFunc("PUT /api/config/mcp/{name}", handlers.PutMCP)
	mux.HandleFunc("DELETE /api/config/mcp/{name}", handlers.DeleteMCP)
	mux.HandleFunc("GET /api/config/providers", handlers.ListProviders)
	mux.HandleFunc("PUT /api/config/providers/{name}", handlers.PutProvider)
	mux.HandleFunc("DELETE /api/config/providers/{name}", handlers.DeleteProvider)
	mux.HandleFunc("GET /api/config/user", handlers.GetUserConfig)
	mux.HandleFunc("PUT /api/config/user", handlers.PutUserConfig)
	mux.HandleFunc("GET /api/config/vision-models", handlers.ListVisionModels)
	mux.HandleFunc("GET /api/config/user/role-defaults", handlers.GetRoleDefaults)
	mux.HandleFunc("GET /api/config/user/orchestrator-profiles", handlers.GetOrchestratorProfiles)
	mux.HandleFunc("PUT /api/config/user/orchestrator-profiles/active", handlers.SetActiveOrchestratorProfile)
	mux.HandleFunc("POST /api/config/user/orchestrator-profiles/resync", handlers.ResyncOrchestratorProfiles)
	// Literal "active"/"resync" segments above take precedence over this wildcard.
	mux.HandleFunc("PUT /api/config/user/orchestrator-profiles/{name}", handlers.UpdateOrchestratorProfile)

	// Native directory picker
	mux.HandleFunc("POST /api/browse-directory", handlers.BrowseDirectory)

	// UI (frontend)
	mux.Handle("GET /", uiHandler())

	return &Server{
		port:      port,
		hub:       hub,
		mux:       mux,
		portReady: make(chan struct{}),
	}
}

// Start starts the HTTP server and begins serving requests.
// This call blocks until the server is stopped or fails to start.
func (s *Server) Start() error {
	s.httpSrv = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the WebSocket hub
	go s.hub.Run()

	ln, err := net.Listen("tcp", s.httpSrv.Addr)
	if err != nil {
		if s.port != 0 {
			log.Printf("Port %d in use, falling back to random port", s.port)
			s.port = 0
			s.httpSrv.Addr = ":0"
			ln, err = net.Listen("tcp", ":0")
		}
		if err != nil {
			return fmt.Errorf("configapi: failed to listen: %w", err)
		}
	}

	// Capture the actual port (useful when port 0 is used)
	s.port = ln.Addr().(*net.TCPAddr).Port

	// Signal that port is ready (for async starts)
	if s.portReady != nil {
		close(s.portReady)
	}

	if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("configapi: server error: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop() {
	if s.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.httpSrv.Shutdown(ctx)
	}
}

// Port returns the actual port the server is listening on.
func (s *Server) Port() int {
	return s.port
}

// WaitForPort blocks until the server has a port assigned (for async starts).
// Returns the assigned port.
func (s *Server) WaitForPort() int {
	<-s.portReady
	return s.port
}

// HTTPHandler returns the HTTP handler for the config API server.
// This allows the server to be mounted in other muxes.
func (s *Server) HTTPHandler() http.Handler {
	return s.mux
}

// Hub returns the WebSocket hub for the config API server.
func (s *Server) Hub() *Hub {
	return s.hub
}

var (
	defaultServer   *Server
	defaultServerMu sync.Mutex
)

// GetOrStart returns the default config API server, starting it if needed.
// If the server is already running, it returns the existing instance.
// port is the desired port (0 for random). If the port is in use, it falls back to random.
func GetOrStart(port int) (*Server, error) {
	defaultServerMu.Lock()
	defer defaultServerMu.Unlock()

	if defaultServer != nil {
		return defaultServer, nil
	}

	s := New(port)
	go func() {
		if err := s.Start(); err != nil {
			log.Printf("configapi: server error: %v", err)
		}
	}()
	s.WaitForPort() // wait for server to be ready
	defaultServer = s
	return s, nil
}
