package kanban

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
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
func New(port int) *Server {
	store := NewStore()
	hub := NewHub()
	handlers := &Handlers{store: store, hub: hub}

	mux := http.NewServeMux()

	// Session routes
	mux.HandleFunc("GET /api/sessions", handlers.ListSessions)
	mux.HandleFunc("POST /api/sessions", handlers.CreateSession)
	mux.HandleFunc("GET /api/sessions/{id}", handlers.GetSession)
	mux.HandleFunc("PATCH /api/sessions/{id}", handlers.UpdateSession)

	// Board route
	mux.HandleFunc("GET /api/sessions/{id}/board", handlers.GetBoard)

	// Delegation routes
	mux.HandleFunc("POST /api/delegations", handlers.CreateDelegation)
	mux.HandleFunc("GET /api/delegations/{id}", handlers.GetDelegation)
	mux.HandleFunc("PATCH /api/delegations/{id}", handlers.UpdateDelegation)

	// WebSocket route
	mux.HandleFunc("GET /api/events", handlers.HandleWebSocket)

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
