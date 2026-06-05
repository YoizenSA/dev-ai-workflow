package kanban

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
		Project string `json:"project"`
		Goal    string `json:"goal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Goal == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "goal is required"})
		return
	}

	session := h.store.CreateSession(req.Project, req.Goal)
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

func (h *Handlers) DeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteSession(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	h.broadcastUpdate("session.deleted", map[string]string{"session_id": id})
	writeJSON(w, http.StatusOK, map[string]string{"message": "session deleted"})
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

// --- Activity handlers ---

// CreateActivity adds a new activity event to a delegation.
func (h *Handlers) CreateActivity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Type    string   `json:"type"`
		Content string   `json:"content"`
		Options []string `json:"options,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if req.Type == "" || req.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "type and content are required"})
		return
	}

	activity := &ActivityEvent{
		Type:    ActivityType(req.Type),
		Content: html.EscapeString(req.Content),
		Options: req.Options,
	}

	if err := h.store.AddActivity(id, activity); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	h.broadcastUpdate("activity.created", activity)
	writeJSON(w, http.StatusCreated, activity)
}

// GetActivities returns all activity events for a delegation.
func (h *Handlers) GetActivities(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.store.DelegationExists(id) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "delegation not found"})
		return
	}
	activities, err := h.store.GetActivities(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, activities)
}

// ResolveActivity sets the resolution on a pending activity.
func (h *Handlers) ResolveActivity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	actID := r.PathValue("actId")

	var req struct {
		Resolution string `json:"resolution"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if req.Resolution == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "resolution is required"})
		return
	}

	if err := h.store.ResolveActivity(id, actID, req.Resolution); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	h.broadcastUpdate("activity.resolved", map[string]string{
		"delegation_id": id,
		"activity_id":   actID,
		"resolution":    req.Resolution,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

// GetPendingDecisions returns all unresolved decision/question/blocked activities for a session.
func (h *Handlers) GetPendingDecisions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	pending, err := h.store.GetPendingDecisions(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, pending)
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

func isValidName(name string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	return matched
}

// --- Config Handlers ---

func opencodeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode", "opencode.json"), nil
}

func agentsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode", "agents"), nil
}

func skillsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode", "skills"), nil
}

// GET /api/config/opencode
func (h *Handlers) GetOpenCodeConfig(w http.ResponseWriter, r *http.Request) {
	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// PUT /api/config/opencode
func (h *Handlers) PutOpenCodeConfig(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit

	var body json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Validate the JSON is an object/map, not an array or primitive
	if len(body) > 0 && body[0] != '{' {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected JSON object"})
		return
	}

	// Backup existing config
	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if existing, err := os.ReadFile(path); err == nil {
		_ = os.WriteFile(path+".bak", existing, 0644)
	}

	// Write new config
	pretty, _ := json.MarshalIndent(body, "", "  ")
	if err := os.WriteFile(path, pretty, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// GET /api/config/agents
func (h *Handlers) ListAgents(w http.ResponseWriter, r *http.Request) {
	dir, err := agentsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	type agentInfo struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	}
	var agents []agentInfo
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			info, _ := e.Info()
			name := strings.TrimSuffix(e.Name(), ".md")
			agents = append(agents, agentInfo{Name: name, Size: info.Size()})
		}
	}
	writeJSON(w, http.StatusOK, agents)
}

// GET /api/config/agents/{name}
func (h *Handlers) GetAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	agentsDirPath, err := agentsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	path := filepath.Join(agentsDirPath, name+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"name": name, "content": string(data)})
}

// PUT /api/config/agents/{name}
func (h *Handlers) PutAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	agentsDirPath, err := agentsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	path := filepath.Join(agentsDirPath, name+".md")

	// Prevent path traversal
	absPath, err := filepath.Abs(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	baseDir, err := filepath.Abs(agentsDirPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !strings.HasPrefix(absPath, baseDir) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "path outside allowed directory"})
		return
	}

	if err := os.WriteFile(path, []byte(body.Content), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// POST /api/config/agents
func (h *Handlers) CreateAgent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if body.Name == "" || !isValidName(body.Name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	agentsDirPath, err := agentsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	path := filepath.Join(agentsDirPath, body.Name+".md")

	// Prevent path traversal
	absPath, err := filepath.Abs(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	baseDir, err := filepath.Abs(agentsDirPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !strings.HasPrefix(absPath, baseDir) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "path outside allowed directory"})
		return
	}

	if _, err := os.Stat(path); err == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "agent already exists"})
		return
	}

	if err := os.WriteFile(path, []byte(body.Content), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

// DELETE /api/config/agents/{name}
func (h *Handlers) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	agentsDirPath, err := agentsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	path := filepath.Join(agentsDirPath, name+".md")

	// Prevent path traversal
	absPath, err := filepath.Abs(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	baseDir, err := filepath.Abs(agentsDirPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !strings.HasPrefix(absPath, baseDir) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "path outside allowed directory"})
		return
	}

	if err := os.Remove(path); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GET /api/config/agents/{name}/permissions
func (h *Handlers) GetAgentPermissions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var agents map[string]json.RawMessage
	if agentRaw, ok := config["agent"]; ok {
		if err := json.Unmarshal(agentRaw, &agents); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	} else {
		writeJSON(w, http.StatusOK, map[string]string{})
		return
	}

	agentRaw, ok := agents[name]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}

	var agent map[string]json.RawMessage
	if err := json.Unmarshal(agentRaw, &agent); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var permission map[string]string
	if permRaw, ok := agent["permission"]; ok {
		if err := json.Unmarshal(permRaw, &permission); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusOK, permission)
}

// PUT /api/config/agents/{name}/permissions
func (h *Handlers) PutAgentPermissions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit

	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Validate permission values
	validValues := map[string]bool{"allow": true, "ask": true, "deny": true}
	for k, v := range body {
		if !validValues[v] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid value %q for key %q: must be allow, ask, or deny", v, k)})
			return
		}
		_ = k // keys can be any tool name (including custom/MCP tools)
	}

	// Read current config
	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Get agent section
	var agents map[string]json.RawMessage
	if agentRaw, ok := config["agent"]; ok {
		if err := json.Unmarshal(agentRaw, &agents); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	} else {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}

	agentRaw, ok := agents[name]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}

	var agent map[string]json.RawMessage
	if err := json.Unmarshal(agentRaw, &agent); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Update permission
	permJSON, _ := json.Marshal(body)
	agent["permission"] = permJSON

	// Write back agent
	agentRaw, _ = json.Marshal(agent)
	agents[name] = agentRaw
	agentsJSON, _ := json.Marshal(agents)
	config["agent"] = agentsJSON

	// Backup and write
	_ = os.WriteFile(path+".bak", data, 0644)
	pretty, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(path, pretty, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, body)
}

// GET /api/config/tools
func (h *Handlers) ListTools(w http.ResponseWriter, r *http.Request) {
	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Built-in opencode tools
	builtIn := []string{
		"read", "edit", "write", "bash", "glob", "grep", "lsp",
		"ast_grep", "websearch", "code_search", "webfetch",
		"task", "delegate", "delegation_list", "delegation_read",
		"question", "todowrite", "todoread", "skill",
	}

	// Collect all known tool names in a set
	toolSet := map[string]bool{}
	for _, t := range builtIn {
		toolSet[t] = true
	}

	// Also collect tools already referenced in agent permissions
	var agents map[string]json.RawMessage
	if agentRaw, ok := config["agent"]; ok {
		_ = json.Unmarshal(agentRaw, &agents)
		for _, agentRaw := range agents {
			var agent map[string]json.RawMessage
			if err := json.Unmarshal(agentRaw, &agent); err != nil {
				continue
			}
			var perm map[string]string
			if permRaw, ok := agent["permission"]; ok {
				if err := json.Unmarshal(permRaw, &perm); err == nil {
					for k := range perm {
						toolSet[k] = true
					}
				}
			}
		}
	}

	// MCP discovery — best effort for HTTP/SSE MCPs
	mcpTools := map[string][]string{}
	var mcpServers map[string]json.RawMessage
	if mcpRaw, ok := config["mcp"]; ok {
		_ = json.Unmarshal(mcpRaw, &mcpServers)
		for name, serverRaw := range mcpServers {
			var server map[string]interface{}
			if err := json.Unmarshal(serverRaw, &server); err != nil {
				continue
			}
			// Skip disabled MCPs
			if disabled, ok := server["disabled"].(bool); ok && disabled {
				continue
			}

			// Try HTTP/SSE discovery
			urlStr := ""
			if u, ok := server["url"].(string); ok && u != "" {
				urlStr = u
			}
			if urlStr == "" {
				// Try command+args to detect stdio (skip for now)
				continue
			}

			tools, err := discoverMCPTools(urlStr)
			if err == nil && len(tools) > 0 {
				mcpTools[name] = tools
				for _, t := range tools {
					toolSet[t] = true
				}
			}
		}
	}

	// Convert set to sorted slice
	var allTools []string
	for t := range toolSet {
		allTools = append(allTools, t)
	}
	sortStrings(allTools)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"built_in":  builtIn,
		"all":       allTools,
		"mcp_tools": mcpTools,
	})
}

func discoverMCPTools(urlStr string) ([]string, error) {
	// JSON-RPC request for tools/list
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}
	payload, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post(urlStr, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var rpcResp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, err
	}

	var names []string
	for _, t := range rpcResp.Result.Tools {
		if t.Name != "" {
			names = append(names, t.Name)
		}
	}
	return names, nil
}

func sortStrings(a []string) {
	for i := 0; i < len(a); i++ {
		for j := i + 1; j < len(a); j++ {
			if a[j] < a[i] {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}

// GET /api/config/skills
func (h *Handlers) ListSkills(w http.ResponseWriter, r *http.Request) {
	dir, err := skillsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	type skillInfo struct {
		Name        string `json:"name"`
		HasSkillMD  bool   `json:"hasSkillMD"`
		Description string `json:"description"`
	}
	var skills []skillInfo
	for _, e := range entries {
		if e.IsDir() {
			skillPath := filepath.Join(dir, e.Name(), "SKILL.md")
			hasSkill := false
			desc := ""
			if data, err := os.ReadFile(skillPath); err == nil {
				hasSkill = true
				// Extract description from frontmatter
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "description:") {
						desc = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
						break
					}
				}
			}
			skills = append(skills, skillInfo{
				Name:        e.Name(),
				HasSkillMD:  hasSkill,
				Description: desc,
			})
		}
	}
	writeJSON(w, http.StatusOK, skills)
}

// GET /api/config/skills/{name}
func (h *Handlers) GetSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill name"})
		return
	}

	skillsDirPath, err := skillsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	path := filepath.Join(skillsDirPath, name, "SKILL.md")

	// Prevent path traversal
	absPath, err := filepath.Abs(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	baseDir, err := filepath.Abs(skillsDirPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !strings.HasPrefix(absPath, baseDir) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "path outside allowed directory"})
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "skill not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"name": name, "content": string(data)})
}

// DELETE /api/config/skills/{name}
func (h *Handlers) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill name"})
		return
	}

	dir, err := skillsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	path := filepath.Join(dir, name)
	cleanPath := filepath.Clean(path)
	if !strings.HasPrefix(cleanPath, filepath.Clean(dir)) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}

	if err := os.RemoveAll(cleanPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GET /api/config/mcp
func (h *Handlers) ListMCP(w http.ResponseWriter, r *http.Request) {
	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config struct {
		MCP map[string]json.RawMessage `json:"mcp"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type mcpInfo struct {
		Name    string          `json:"name"`
		Config  json.RawMessage `json:"config"`
		Enabled bool            `json:"enabled"`
	}
	var mcps []mcpInfo
	for name, cfg := range config.MCP {
		// Check if disabled or enabled flag is false
		var serverCfg map[string]interface{}
		enabled := true
		if err := json.Unmarshal(cfg, &serverCfg); err == nil {
			if disabled, ok := serverCfg["disabled"].(bool); ok && disabled {
				enabled = false
			} else if val, ok := serverCfg["enabled"].(bool); ok && !val {
				enabled = false
			}
		}
		mcps = append(mcps, mcpInfo{Name: name, Config: cfg, Enabled: enabled})
	}
	writeJSON(w, http.StatusOK, mcps)
}

// PUT /api/config/mcp/{name} - toggle enabled/disabled
func (h *Handlers) PutMCP(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid mcp name"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Read current config
	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Get MCP section
	var mcpSection map[string]json.RawMessage
	if mcpRaw, ok := config["mcp"]; ok {
		if err := json.Unmarshal(mcpRaw, &mcpSection); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	} else {
		mcpSection = make(map[string]json.RawMessage)
	}

	// Toggle: add/remove "disabled" field, toggle "enabled" field
	if serverRaw, ok := mcpSection[name]; ok {
		var serverCfg map[string]interface{}
		if err := json.Unmarshal(serverRaw, &serverCfg); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if body.Enabled {
			delete(serverCfg, "disabled")
			serverCfg["enabled"] = true
		} else {
			serverCfg["disabled"] = true
			serverCfg["enabled"] = false
		}

		updated, _ := json.Marshal(serverCfg)
		mcpSection[name] = updated
	} else {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mcp server not found"})
		return
	}

	// Write back
	mcpJSON, _ := json.Marshal(mcpSection)
	config["mcp"] = mcpJSON
	pretty, _ := json.MarshalIndent(config, "", "  ")

	// Backup
	_ = os.WriteFile(path+".bak", data, 0644)

	if err := os.WriteFile(path, pretty, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// GET /api/config/providers - list all providers
func (h *Handlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Get provider section
	if providerRaw, ok := config["provider"]; ok {
		var providers map[string]json.RawMessage
		if err := json.Unmarshal(providerRaw, &providers); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, providers)
	} else {
		writeJSON(w, http.StatusOK, map[string]json.RawMessage{})
	}
}

// PUT /api/config/providers/{name} - create or update a provider
func (h *Handlers) PutProvider(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid provider name"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit

	var provider json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&provider); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Get provider section
	var providerSection map[string]json.RawMessage
	if providerRaw, ok := config["provider"]; ok {
		if err := json.Unmarshal(providerRaw, &providerSection); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	} else {
		providerSection = make(map[string]json.RawMessage)
	}

	providerSection[name] = provider

	// Write back
	providerJSON, _ := json.Marshal(providerSection)
	config["provider"] = providerJSON
	pretty, _ := json.MarshalIndent(config, "", "  ")

	// Backup
	_ = os.WriteFile(path+".bak", data, 0644)
	if err := os.WriteFile(path, pretty, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// DELETE /api/config/providers/{name} - delete a provider
func (h *Handlers) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "provider name required"})
		return
	}

	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Get provider section
	var providerSection map[string]json.RawMessage
	if providerRaw, ok := config["provider"]; ok {
		if err := json.Unmarshal(providerRaw, &providerSection); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	if _, ok := providerSection[name]; ok {
		delete(providerSection, name)
	} else {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
		return
	}

	// Write back
	providerJSON, _ := json.Marshal(providerSection)
	config["provider"] = providerJSON
	pretty, _ := json.MarshalIndent(config, "", "  ")

	// Backup
	_ = os.WriteFile(path+".bak", data, 0644)
	if err := os.WriteFile(path, pretty, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
