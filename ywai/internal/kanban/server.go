package kanban

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// DefaultUIPort is the default port for the Kanban UI server.
const DefaultUIPort = 5768

// Server is the embedded HTTP server for the Kanban board.
type Server struct {
	port      int
	store     *Store
	hub       *Hub
	httpSrv   *http.Server
	mux       *http.ServeMux
	portReady chan struct{} // closed when port is assigned
}

// New creates a new Kanban server listening on the given port.
// Use port 0 to let the OS pick a free port.
func New(port int, dataDir string) *Server {
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			dataDir = filepath.Join(home, ".config", "opencode", "kanban-data")
		}
	}
	store := NewStore(dataDir)
	hub := NewHub()
	oc := opencode.DefaultClient(context.Background())
	handlers := &Handlers{store: store, hub: hub, opencodeClient: oc}

	mux := http.NewServeMux()

	// Session routes
	mux.HandleFunc("GET /api/sessions", handlers.ListSessions)
	mux.HandleFunc("POST /api/sessions", handlers.CreateSession)
	mux.HandleFunc("GET /api/sessions/{id}", handlers.GetSession)
	mux.HandleFunc("PATCH /api/sessions/{id}", handlers.UpdateSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", handlers.DeleteSession)
	mux.HandleFunc("PATCH /api/sessions", handlers.UpdateSessions)
	mux.HandleFunc("DELETE /api/sessions", handlers.DeleteSessions)

	// Board route
	mux.HandleFunc("GET /api/sessions/{id}/board", handlers.GetBoard)

	// Delegation routes
	mux.HandleFunc("POST /api/delegations", handlers.CreateDelegation)
	mux.HandleFunc("GET /api/delegations/{id}", handlers.GetDelegation)
	mux.HandleFunc("PATCH /api/delegations/{id}", handlers.UpdateDelegation)

	// Activity routes
	mux.HandleFunc("POST /api/delegations/{id}/activities", handlers.CreateActivity)
	mux.HandleFunc("GET /api/delegations/{id}/activities", handlers.GetActivities)
	mux.HandleFunc("PATCH /api/delegations/{id}/activities/{actId}", handlers.ResolveActivity)
	mux.HandleFunc("GET /api/sessions/{id}/decisions", handlers.GetPendingDecisions)

	// Dependency graph route
	mux.HandleFunc("GET /api/sessions/{id}/graph", handlers.GetGraph)

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
	mux.HandleFunc("GET /api/config/skills", handlers.ListSkills)
	mux.HandleFunc("GET /api/config/skills/{name}", handlers.GetSkill)
	mux.HandleFunc("PUT /api/config/skills/{name}", handlers.PutSkill)
	mux.HandleFunc("DELETE /api/config/skills/{name}", handlers.DeleteSkill)
	mux.HandleFunc("GET /api/config/mcp", handlers.ListMCP)
	mux.HandleFunc("PUT /api/config/mcp/{name}", handlers.PutMCP)
	mux.HandleFunc("DELETE /api/config/mcp/{name}", handlers.DeleteMCP)
	mux.HandleFunc("GET /api/config/providers", handlers.ListProviders)
	mux.HandleFunc("PUT /api/config/providers/{name}", handlers.PutProvider)
	mux.HandleFunc("DELETE /api/config/providers/{name}", handlers.DeleteProvider)
	mux.HandleFunc("GET /api/config/user", handlers.GetUserConfig)
	mux.HandleFunc("PUT /api/config/user", handlers.PutUserConfig)
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
		store:     store,
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
			return fmt.Errorf("kanban: failed to listen: %w", err)
		}
	}

	// Capture the actual port (useful when port 0 is used)
	s.port = ln.Addr().(*net.TCPAddr).Port

	// Signal that port is ready (for async starts)
	if s.portReady != nil {
		close(s.portReady)
	}

	if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("kanban: server error: %w", err)
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

// Store returns the kanban Store, for use by other components.
func (s *Server) Store() *Store {
	return s.store
}

// WaitForPort blocks until the server has a port assigned (for async starts).
// Returns the assigned port.
func (s *Server) WaitForPort() int {
	<-s.portReady
	return s.port
}

// HTTPHandler returns the HTTP handler for the kanban server.
// This allows the server to be mounted in other muxes.
func (s *Server) HTTPHandler() http.Handler {
	return s.mux
}

// Hub returns the WebSocket hub for the kanban server.
func (s *Server) Hub() *Hub {
	return s.hub
}

// Broadcast pushes a board update to all connected UI clients. It lets
// non-HTTP producers (e.g. the missions→kanban projector) drive live updates
// the same way the HTTP handlers do.
func (s *Server) Broadcast(updateType string, payload interface{}) {
	data, err := json.Marshal(BoardUpdate{Type: updateType, Payload: payload})
	if err != nil {
		log.Printf("kanban: marshal board update: %v", err)
		return
	}
	s.hub.Broadcast(data)
}

var (
	defaultServer   *Server
	defaultServerMu sync.Mutex
)

// GetOrStart returns the default kanban server, starting it if needed.
// If the server is already running, it returns the existing instance.
// port is the desired port (0 for random). If the port is in use, it falls back to random.
func GetOrStart(port int) (*Server, error) {
	defaultServerMu.Lock()
	defer defaultServerMu.Unlock()

	if defaultServer != nil {
		return defaultServer, nil
	}

	s := New(port, "")
	go func() {
		if err := s.Start(); err != nil {
			log.Printf("kanban: server error: %v", err)
		}
	}()
	s.WaitForPort() // wait for server to be ready
	defaultServer = s
	return s, nil
}
