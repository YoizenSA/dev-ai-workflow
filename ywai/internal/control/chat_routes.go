package control

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// chatSession represents an in-memory chat session.
type chatSession struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	Messages  []message `json:"messages,omitempty"`
}

type message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// chatStore is a simple in-memory session store.
type chatStore struct {
	mu       sync.RWMutex
	sessions map[string]*chatSession
}

var globalChatStore = &chatStore{
	sessions: make(map[string]*chatSession),
}

func (cs *chatStore) create(title string) *chatSession {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	s := &chatSession{
		ID:        newID(),
		Title:     title,
		CreatedAt: time.Now(),
		Messages:  make([]message, 0),
	}
	cs.sessions[s.ID] = s
	return s
}

func (cs *chatStore) list() []*chatSession {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	out := make([]*chatSession, 0, len(cs.sessions))
	for _, s := range cs.sessions {
		out = append(out, s)
	}
	return out
}

func (cs *chatStore) get(id string) *chatSession {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.sessions[id]
}

func (cs *chatStore) addMessage(sessionID, role, content string) *message {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	s, ok := cs.sessions[sessionID]
	if !ok {
		return nil
	}
	m := message{
		ID:        newID(),
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}
	s.Messages = append(s.Messages, m)
	return &m
}

// registerChatRoutes registers chat session API endpoints.
func (s *Server) registerChatRoutes() {
	log.Printf("[chat] using in-memory session store")

	s.mux.HandleFunc("POST /api/chat/sessions", s.handleCreateSession)
	s.mux.HandleFunc("GET /api/chat/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /api/chat/sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("POST /api/chat/sessions/{id}/messages", s.handleSendMessage)
}

// handleCreateSession creates a new chat session.
// POST /api/chat/sessions
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req) // ignore decode errors
	}
	if req.Title == "" {
		req.Title = "New Chat"
	}
	session := globalChatStore.create(req.Title)
	writeJSON(w, http.StatusCreated, session)
}

// handleListSessions returns all chat sessions.
// GET /api/chat/sessions
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := globalChatStore.list()
	writeJSON(w, http.StatusOK, sessions)
}

// handleGetSession returns a single session with its messages.
// GET /api/chat/sessions/{id}
func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session := globalChatStore.get(id)
	if session == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// handleSendMessage adds a user message and returns the session with assistant echo.
// POST /api/chat/sessions/{id}/messages
func (s *Server) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session := globalChatStore.get(id)
	if session == nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Store user message
	globalChatStore.addMessage(id, "user", req.Content)

	// Echo as assistant response
	globalChatStore.addMessage(id, "assistant", "Echo: "+req.Content)

	writeJSON(w, http.StatusCreated, session)
}
