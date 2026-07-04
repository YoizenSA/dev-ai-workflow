package control

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	subs     map[string][]chan message
}

var globalChatStore = &chatStore{
	sessions: make(map[string]*chatSession),
	subs:     make(map[string][]chan message),
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
	s, ok := cs.sessions[sessionID]
	if !ok {
		cs.mu.Unlock()
		return nil
	}
	m := message{
		ID:        newID(),
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}
	s.Messages = append(s.Messages, m)
	cs.mu.Unlock()

	// Only broadcast assistant messages via SSE; user messages are
	// already added optimistically by the frontend and emitting them
	// would cause duplicates.
	if role != "user" {
		cs.broadcast(sessionID, m)
	}
	return &m
}

func (cs *chatStore) subscribe(sessionID string) chan message {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	ch := make(chan message, 16)
	cs.subs[sessionID] = append(cs.subs[sessionID], ch)
	return ch
}

func (cs *chatStore) unsubscribe(sessionID string, ch chan message) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	subs := cs.subs[sessionID]
	for i, s := range subs {
		if s == ch {
			cs.subs[sessionID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(cs.subs[sessionID]) == 0 {
		delete(cs.subs, sessionID)
	}
}

func (cs *chatStore) broadcast(sessionID string, msg message) {
	cs.mu.RLock()
	subs := cs.subs[sessionID]
	cs.mu.RUnlock()
	for _, ch := range subs {
		select {
		case ch <- msg:
		default: // drop if subscriber buffer full
		}
	}
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

// handleChatSSEFallback streams in-memory echo events when no OpenCode server is available.
// GET /api/chat/events?sessionID=<id>
func (s *Server) handleChatSSEFallback(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionID")
	if sessionID == "" {
		http.Error(w, "missing sessionID", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	ch := globalChatStore.subscribe(sessionID)
	defer globalChatStore.unsubscribe(sessionID, ch)

	ctx := r.Context()
	for {
		select {
		case msg := <-ch:
			data, _ := json.Marshal(map[string]any{
				"params": map[string]any{
					"part": map[string]any{
						"kind":      "text",
						"text":      msg.Content,
						"messageID": msg.ID,
					},
				},
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			// Signal done so the frontend re-enables the send button.
			done, _ := json.Marshal(map[string]any{
				"params": map[string]any{
					"status": "done",
				},
			})
			fmt.Fprintf(w, "data: %s\n\n", done)
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}
