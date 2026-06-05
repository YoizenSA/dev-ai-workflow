package kanban

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Server is the embedded HTTP server for the Kanban board.
type Server struct {
	port    int
	store   *Store
	hub     *Hub
	httpSrv *http.Server
	mux     *http.ServeMux
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
	handlers := &Handlers{store: store, hub: hub}

	mux := http.NewServeMux()

	// Session routes
	mux.HandleFunc("GET /api/sessions", handlers.ListSessions)
	mux.HandleFunc("POST /api/sessions", handlers.CreateSession)
	mux.HandleFunc("GET /api/sessions/{id}", handlers.GetSession)
	mux.HandleFunc("PATCH /api/sessions/{id}", handlers.UpdateSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", handlers.DeleteSession)

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
	mux.HandleFunc("GET /api/config/agents/{name}", handlers.GetAgent)
	mux.HandleFunc("PUT /api/config/agents/{name}", handlers.PutAgent)
	mux.HandleFunc("POST /api/config/agents", handlers.CreateAgent)
	mux.HandleFunc("DELETE /api/config/agents/{name}", handlers.DeleteAgent)
	mux.HandleFunc("GET /api/config/agents/{name}/permissions", handlers.GetAgentPermissions)
	mux.HandleFunc("PUT /api/config/agents/{name}/permissions", handlers.PutAgentPermissions)
	mux.HandleFunc("GET /api/config/tools", handlers.ListTools)
	mux.HandleFunc("GET /api/config/skills", handlers.ListSkills)
	mux.HandleFunc("GET /api/config/skills/{name}", handlers.GetSkill)
	mux.HandleFunc("DELETE /api/config/skills/{name}", handlers.DeleteSkill)
	mux.HandleFunc("GET /api/config/mcp", handlers.ListMCP)
	mux.HandleFunc("PUT /api/config/mcp/{name}", handlers.PutMCP)
	mux.HandleFunc("GET /api/config/providers", handlers.ListProviders)
	mux.HandleFunc("PUT /api/config/providers/{name}", handlers.PutProvider)
	mux.HandleFunc("DELETE /api/config/providers/{name}", handlers.DeleteProvider)

	// UI (frontend)
	mux.Handle("GET /", uiHandler())

	return &Server{
		port:  port,
		store: store,
		hub:   hub,
		mux:   mux,
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
		return fmt.Errorf("kanban: failed to listen on %s: %w", s.httpSrv.Addr, err)
	}

	// Capture the actual port (useful when port 0 is used)
	s.port = ln.Addr().(*net.TCPAddr).Port
	log.Printf("ywai Kanban server running on http://localhost:%d", s.port)

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
