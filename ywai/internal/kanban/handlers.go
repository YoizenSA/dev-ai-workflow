package kanban

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// --- WebSocket Hub ---

// Client represents a single WebSocket connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Hub manages WebSocket connections and broadcasts messages to all clients.
type Hub struct {
	mu        sync.RWMutex
	clients   map[*Client]bool
	broadcast chan []byte
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:   make(map[*Client]bool),
		broadcast: make(chan []byte, 256),
	}
}

// Run starts the hub's event loop. Must be called as a goroutine.
func (h *Hub) Run() {
	for msg := range h.broadcast {
		h.mu.RLock()
		for client := range h.clients {
			select {
			case client.send <- msg:
			default:
				// Client's send buffer is full; drop it.
				close(client.send)
				delete(h.clients, client)
			}
		}
		h.mu.RUnlock()
	}
}

// Broadcast enqueues a message to be sent to all connected clients.
func (h *Hub) Broadcast(msg []byte) {
	h.broadcast <- msg
}

// Register adds a client to the hub.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = true
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
}

// --- Handlers ---

// Handlers holds references to the store and hub for HTTP handlers.
type Handlers struct {
	store *Store
	hub   *Hub
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for POC
	},
}

const wsWriteWait = 10 * time.Second
const wsPongWait = 60 * time.Second
const wsPingPeriod = (wsPongWait * 9) / 10
const maxMessageSize = 512

// readPump reads messages from the WebSocket connection (keeps it alive).
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// writePump writes messages from the hub to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// broadcastUpdate sends a BoardUpdate to all WebSocket clients.
func (h *Handlers) broadcastUpdate(updateType string, payload interface{}) {
	update := BoardUpdate{
		Type:    updateType,
		Payload: payload,
	}
	data, err := json.Marshal(update)
	if err != nil {
		log.Printf("Error marshaling board update: %v", err)
		return
	}
	h.hub.Broadcast(data)
}

// --- Session handlers ---

func (h *Handlers) ListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := h.store.ListSessions("")
	writeJSON(w, http.StatusOK, sessions)
}

func (h *Handlers) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Goal string `json:"goal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Goal == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "goal is required"})
		return
	}

	session := h.store.CreateSession(req.Goal)
	h.broadcastUpdate("session.created", session)
	writeJSON(w, http.StatusCreated, session)
}

func (h *Handlers) GetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, ok := h.store.GetSession(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handlers) UpdateSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	var err error
	switch req.Status {
	case "closed":
		err = h.store.CloseSession(id)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid status"})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	session, _ := h.store.GetSession(id)
	h.broadcastUpdate("session.status_changed", session)
	writeJSON(w, http.StatusOK, session)
}

// --- Board handler ---

func (h *Handlers) GetBoard(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	board, err := h.store.BoardView(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, board)
}

// --- Delegation handlers ---

func (h *Handlers) CreateDelegation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID    string   `json:"session_id"`
		Agent        string   `json:"agent"`
		TaskSummary  string   `json:"task_summary"`
		Dependencies []string `json:"dependencies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.SessionID == "" || req.Agent == "" || req.TaskSummary == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session_id, agent, and task_summary are required"})
		return
	}

	validAgents := map[string]bool{"dev": true, "qa": true, "reviewer": true, "architect": true, "devops": true}
	if !validAgents[req.Agent] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent: must be dev, qa, reviewer, architect, or devops"})
		return
	}

	d, err := h.store.CreateDelegation(req.SessionID, req.Agent, req.TaskSummary, req.Dependencies)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	h.broadcastUpdate("delegation.created", d)
	writeJSON(w, http.StatusCreated, d)
}

func (h *Handlers) GetDelegation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, ok := h.store.GetDelegation(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "delegation not found"})
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (h *Handlers) UpdateDelegation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Status         *string `json:"status"`
		Column         *string `json:"column"`
		HandoffPreview *string `json:"handoff_preview"`
		Blocker        *string `json:"blocker"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	validColumns := map[string]bool{"backlog": true, "ready": true, "in_progress": true, "review": true, "done": true}
	if req.Column != nil && *req.Column != "" && !validColumns[*req.Column] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid column"})
		return
	}

	d, err := h.store.UpdateDelegation(id, req.Status, req.Column, req.HandoffPreview, req.Blocker)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	h.broadcastUpdate("delegation.status_changed", d)
	writeJSON(w, http.StatusOK, d)
}

// --- WebSocket handler ---

func (h *Handlers) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  h.hub,
		conn: conn,
		send: make(chan []byte, 256),
	}
	h.hub.Register(client)

	go client.writePump()
	go client.readPump()
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Error writing JSON response: %v", err)
	}
}
