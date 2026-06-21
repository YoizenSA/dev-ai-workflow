package kanban

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	userconfig "github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
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
	store          *Store
	hub            *Hub
	opencodeClient opencode.Client
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
	status := r.URL.Query().Get("status")
	project := r.URL.Query().Get("project")
	query := r.URL.Query().Get("q")

	sessions := h.store.ListSessions(status, project, query)

	groupParam := r.URL.Query().Get("group")
	if groupParam == "project" {
		grouped := make(map[string][]*Session)
		for _, s := range sessions {
			p := s.Project
			if p == "" {
				p = "(no project)"
			}
			grouped[p] = append(grouped[p], s)
		}
		writeJSON(w, http.StatusOK, grouped)
		return
	}

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

	// Validate dependencies exist in the same session
	for _, depID := range req.Dependencies {
		dep, ok := h.store.GetDelegation(depID)
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("dependency %s not found", depID)})
			return
		}
		if dep.SessionID != req.SessionID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("dependency %s belongs to a different session", depID)})
			return
		}
	}

	d, err := h.store.CreateDelegation(req.SessionID, req.Agent, req.TaskSummary, req.Dependencies)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	// Check for cycles after creation and roll back if detected
	for _, depID := range req.Dependencies {
		if h.store.HasCycle(d.ID, depID) {
			// Roll back: delete the delegation we just created
			_ = h.store.DeleteDelegation(d.ID)
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("adding dependency on %s would create a cycle", depID)})
			return
		}
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
		Handoff        *string `json:"handoff"`
		HandoffPreview *string `json:"handoff_preview"`
		Blocker        *string `json:"blocker"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Map kanban status to the FSM value stored on the delegation. The board is
	// decoupled from the mission FSM (see KanbanProjector: kanban moves never
	// affect the engine), so we do NOT enforce FSM transitions here — that only
	// caused legitimate card moves to fail silently.
	if req.Status != nil && *req.Status != "" {
		s := string(MapKanbanStatusToFSM(*req.Status))
		req.Status = &s // store will receive FSM value
	}

	d, err := h.store.UpdateDelegation(id, req.Status, req.Column, req.Handoff, req.HandoffPreview, req.Blocker)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	h.broadcastUpdate("delegation.status_changed", d)

	// Auto-unblock dependent delegations when this one moves to completed (FSM)
	if req.Status != nil && *req.Status == string(missions.MissionCompleted) {
		unblockedIDs := h.store.AutoUnblock(id)
		for _, uid := range unblockedIDs {
			if ud, ok := h.store.GetDelegation(uid); ok {
				h.broadcastUpdate("delegation.status_changed", ud)
				h.broadcastUpdate("delegation.auto_unblocked", map[string]string{
					"id":           uid,
					"unblocked_by": id,
					"task_summary": ud.TaskSummary,
				})
			}
		}
	}

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

	activity, err := h.store.ResolveActivity(id, actID, req.Resolution)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	h.broadcastUpdate("activity.resolved", activity)
	writeJSON(w, http.StatusOK, activity)
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

// GetGraph returns the dependency graph for a session.
func (h *Handlers) GetGraph(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	graph, err := h.store.GraphView(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, graph)
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
	_, _ = w.Write(data)
}

// PUT /api/config/opencode
//
// Body is treated as a sparse JSON patch: every top-level key present in the
// body replaces the matching key in opencode.json, while any key not in the
// body is preserved. This protects the file from clients that render only a
// subset of fields (e.g. the Settings UI which exposes 5 keys but the file
// also holds provider configs, mcp, etc.).
func (h *Handlers) PutOpenCodeConfig(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit

	var patch map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected JSON object: " + err.Error()})
		return
	}

	path, err := opencodeConfigPath()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Load existing config (if any) into a top-level map and merge patch over it.
	existing, _ := os.ReadFile(path)
	merged := map[string]json.RawMessage{}
	if len(existing) > 0 {
		// Preserve the existing file on disk as a .bak before mutating it.
		_ = os.WriteFile(path+".bak", existing, 0644)
		_ = json.Unmarshal(existing, &merged)
	}
	for k, v := range patch {
		merged[k] = v
	}

	pretty, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := os.WriteFile(path, pretty, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// GET /api/config/agents
func (h *Handlers) ListAgents(w http.ResponseWriter, r *http.Request) {
	type agentInfo struct {
		Name  string `json:"name"`
		Size  int64  `json:"size"`
		Mode  string `json:"mode,omitempty"`
		Group string `json:"group,omitempty"`
	}

	seen := make(map[string]bool)
	var agents []agentInfo
	agentsDirPath, _ := agentsDir()

	// 1. Read agents from opencode.json config
	configPath, err := opencodeConfigPath()
	if err == nil {
		data, readErr := os.ReadFile(configPath)
		if readErr == nil {
			var cfg struct {
				Agent map[string]json.RawMessage `json:"agent"`
			}
			if json.Unmarshal(data, &cfg) == nil && cfg.Agent != nil {
				for name, raw := range cfg.Agent {
					var a struct {
						Mode string `json:"mode"`
					}
					_ = json.Unmarshal(raw, &a)
					info := agentInfo{Name: name, Mode: a.Mode}
					info.Group = resolveTeam(name, agentsDirPath)
					agents = append(agents, info)
					seen[name] = true
				}
			}
		}
	}

	// 2. Also scan agents directory for agents not in config
	if agentsDirPath != "" {
		entries, _ := os.ReadDir(agentsDirPath)
		for _, e := range entries {
			if e.IsDir() {
				// Scan subdirectory for .md files (e.g., core/architect.md, qa-automation/qa-analyst.md)
				subEntries, _ := os.ReadDir(filepath.Join(agentsDirPath, e.Name()))
				for _, se := range subEntries {
					if !se.IsDir() && strings.HasSuffix(se.Name(), ".md") {
						name := strings.TrimSuffix(se.Name(), ".md")
						if !seen[name] {
							info := agentInfo{Name: name, Group: e.Name()}
							agents = append(agents, info)
							seen[name] = true
						}
					}
				}
			} else if strings.HasSuffix(e.Name(), ".md") {
				name := strings.TrimSuffix(e.Name(), ".md")
				if !seen[name] {
					info := agentInfo{Name: name}
					info.Group = resolveTeam(name, agentsDirPath)
					agents = append(agents, info)
					seen[name] = true
				}
			}
		}
	}

	sort.Slice(agents, func(i, j int) bool { return agents[i].Name < agents[j].Name })
	writeJSON(w, http.StatusOK, agents)
}

// resolveTeam detects the team for an agent.
func resolveTeam(agentName, agentsDirPath string) string {
	if agentsDirPath == "" {
		return ""
	}
	path := filepath.Join(agentsDirPath, agentName+".md")
	if mdData, err := os.ReadFile(path); err == nil {
		return detectAgentTeam(agentName, mdData)
	}
	return ""
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
	path := resolveAgentFile(agentsDirPath, name)
	if path == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}
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
	path := resolveAgentFile(agentsDirPath, name)

	// Prevent path traversal (resolveAgentFile only returns paths under
	// agentsDirPath, but a flat or nested layout must still be verified so a
	// crafted name can never escape the base directory).
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
	if path == "" || !strings.HasPrefix(absPath, baseDir) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}

	// The permission: block is owned by PutAgentPermissions, not this handler.
	// Re-apply the on-disk permissions onto the incoming content so a stale
	// frontmatter coming from the client cannot overwrite toggles made via the
	// permissions API in between load and save.
	finalContent := body.Content
	if existing, err := os.ReadFile(path); err == nil {
		fm, _ := parseFrontmatter(string(existing))
		currentPerms := extractPermissionsFromFrontmatter(fm)
		if len(currentPerms) > 0 {
			finalContent = updatePermissionsInFrontmatter(body.Content, currentPerms)
		}
	}

	if err := os.WriteFile(path, []byte(finalContent), 0644); err != nil {
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
	path := resolveAgentFile(agentsDirPath, name)

	// Prevent path traversal (resolveAgentFile only returns paths under
	// agentsDirPath, flat or nested — still verify so a crafted name cannot
	// escape the base directory).
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
	if path == "" || !strings.HasPrefix(absPath, baseDir) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}

	if err := os.Remove(path); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GET /api/config/agents/{name}/permissions
// Reads permissions from opencode.json first; falls back to markdown frontmatter.
func (h *Handlers) GetAgentPermissions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent name"})
		return
	}

	// Try opencode.json first
	path, err := opencodeConfigPath()
	if err == nil {
		if data, err := os.ReadFile(path); err == nil {
			var config map[string]json.RawMessage
			if err := json.Unmarshal(data, &config); err == nil {
				if agentRaw, ok := config["agent"]; ok {
					var agents map[string]json.RawMessage
					if err := json.Unmarshal(agentRaw, &agents); err == nil {
						if agentData, ok := agents[name]; ok {
							var agent map[string]json.RawMessage
							if err := json.Unmarshal(agentData, &agent); err == nil {
								if permRaw, ok := agent["permission"]; ok {
									var permission map[string]string
									if err := json.Unmarshal(permRaw, &permission); err == nil {
										writeJSON(w, http.StatusOK, permission)
										return
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Fallback: read from markdown frontmatter
	mdPath := readAgentMarkdownPath(name)
	if mdPath == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}

	mdContent, err := os.ReadFile(mdPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	fm, _ := parseFrontmatter(string(mdContent))
	perms := extractPermissionsFromFrontmatter(fm)
	writeJSON(w, http.StatusOK, perms)
}

// ValidPermissionKeys is the canonical set of allowed permission keys.
// Includes all built-in Pi tools plus extended categories (memory, intercom, ado, mcp).
var ValidPermissionKeys = map[string]bool{
	// File & code tools
	"read":        true,
	"edit":        true,
	"write":       true,
	"bash":        true,
	"glob":        true,
	"grep":        true,
	"lsp":         true,
	"ast_grep":    true,
	"websearch":   true,
	"code_search": true,
	"webfetch":    true,
	// Task & orchestration (consolidated: task=full todo, delegate=full subagent)
	"task":     true,
	"delegate": true,
	"question": true,
	"skill":    true,
	// Extended categories (plugins, MCP, integrations)
	"memory":   true,
	"intercom": true,
	"ado":      true,
	"mcp":      true,

	// Engram memory tools (from engram plugin)
	"mem_capture_passive":   true,
	"mem_compare":           true,
	"mem_context":           true,
	"mem_current_project":   true,
	"mem_delete":            true,
	"mem_doctor":            true,
	"mem_get_observation":   true,
	"mem_judge":             true,
	"mem_save":              true,
	"mem_save_prompt":       true,
	"mem_search":            true,
	"mem_session_end":       true,
	"mem_session_start":     true,
	"mem_session_summary":   true,
	"mem_stats":             true,
	"mem_suggest_topic_key": true,
	"mem_timeline":          true,
	"mem_update":            true,

	// Kanban MCP tools (from ywai-kanban MCP server)
	"add_activity":          true,
	"create_delegation":     true,
	"create_session":        true,
	"delete_session":        true,
	"get_activities":        true,
	"get_board":             true,
	"get_graph":             true,
	"get_pending_decisions": true,
	"get_ui_url":            true,
	"list_sessions":         true,
	"resolve_activity":      true,
	"update_delegation":     true,
}

// ValidPermissionValues are the only accepted permission values.
var ValidPermissionValues = map[string]bool{
	"allow": true,
	"ask":   true,
	"deny":  true,
}

// PUT /api/config/agents/{name}/permissions
// Writes permissions to opencode.json (primary) and markdown frontmatter (backward compat).
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

	// Validate permission values only (keys are dynamic — anyone can add custom tools)
	var invalidValues []string
	for k, v := range body {
		if !ValidPermissionValues[v] {
			invalidValues = append(invalidValues, fmt.Sprintf("%s=%q", k, v))
		}
	}
	if len(invalidValues) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid permission value(s), must be allow, ask, or deny",
			"invalid": invalidValues,
		})
		return
	}

	found := false

	// Write to opencode.json (primary source for GetAgentPermissions)
	if path, err := opencodeConfigPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			var config map[string]json.RawMessage
			if err := json.Unmarshal(data, &config); err == nil {
				var agents map[string]json.RawMessage
				if agentRaw, ok := config["agent"]; ok {
					if err := json.Unmarshal(agentRaw, &agents); err == nil {
						if existingRaw, exists := agents[name]; exists {
							var agentCfg map[string]json.RawMessage
							if err := json.Unmarshal(existingRaw, &agentCfg); err == nil {
								permJSON, _ := json.Marshal(body)
								agentCfg["permission"] = permJSON
								agentJSON, _ := json.Marshal(agentCfg)
								agents[name] = agentJSON
								agentsJSON, _ := json.Marshal(agents)
								config["agent"] = agentsJSON

								pretty, _ := json.MarshalIndent(config, "", "  ")
								_ = os.WriteFile(path+".bak", data, 0644)
								if err := os.WriteFile(path, pretty, 0644); err == nil {
									found = true
								}
							}
						}
					}
				}
			}
		}
	}

	// Also update markdown if it exists (backward compat)
	if mdPath := readAgentMarkdownPath(name); mdPath != "" {
		found = true
		mdContent, err := os.ReadFile(mdPath)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		updated := updatePermissionsInFrontmatter(string(mdContent), body)
		_ = os.WriteFile(mdPath+".bak", mdContent, 0644)
		if err := os.WriteFile(mdPath, []byte(updated), 0644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found in opencode.json or markdown files"})
		return
	}

	writeJSON(w, http.StatusOK, body)
}

type MCPToolGroup struct {
	Tools   []string `json:"tools"`
	Enabled bool     `json:"enabled"`
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
		"task", "delegate", "question", "skill",
		"memory", "intercom", "ado", "mcp",
	}

	// Collect all known tool names in a set
	toolSet := map[string]bool{}
	for _, t := range builtIn {
		toolSet[t] = true
	}

	// Also collect valid tools referenced in agent permissions
	// (skip deprecated keys like todoread/todowrite that aren't in ValidPermissionKeys)
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
						if ValidPermissionKeys[k] {
							toolSet[k] = true
						}
					}
				}
			}
		}
	}

	// MCP discovery — best effort for HTTP/SSE MCPs
	// Include disabled MCPs so the UI can show them as inactive.
	mcpTools := map[string]MCPToolGroup{}
	var mcpServers map[string]json.RawMessage
	if mcpRaw, ok := config["mcp"]; ok {
		_ = json.Unmarshal(mcpRaw, &mcpServers)
		for name, serverRaw := range mcpServers {
			var server map[string]interface{}
			if err := json.Unmarshal(serverRaw, &server); err != nil {
				continue
			}
			disabled := false
			if d, ok := server["disabled"].(bool); ok {
				disabled = d
			}
			// "enabled: false" is equivalent to "disabled: true"
			if e, ok := server["enabled"].(bool); ok && !e {
				disabled = true
			}

			// Discover tools based on server type
			tools := []string{}

			// Try HTTP/SSE discovery first (remote servers)
			if urlStr, ok := server["url"].(string); ok && urlStr != "" {
				tools, _ = discoverMCPTools(urlStr)
			}

			// Try stdio discovery (local servers with command)
			if len(tools) == 0 {
				var command []string
				if cmdRaw, ok := server["command"]; ok {
					switch v := cmdRaw.(type) {
					case []interface{}:
						for _, arg := range v {
							if s, ok := arg.(string); ok {
								command = append(command, s)
							}
						}
					case string:
						command = strings.Fields(v)
						if argsRaw, ok := server["args"].([]interface{}); ok {
							for _, arg := range argsRaw {
								if s, ok := arg.(string); ok {
									command = append(command, s)
								}
							}
						}
					}
				}

				if len(command) > 0 {
					env := map[string]string{}
					if envRaw, ok := server["env"].(map[string]interface{}); ok {
						for k, v := range envRaw {
							if s, ok := v.(string); ok {
								env[k] = s
							}
						}
					}
					tools, _ = discoverStdioMCPTools(command, env)
				}
			}

			mcpTools[name] = MCPToolGroup{Tools: tools, Enabled: !disabled}
			for _, t := range tools {
				toolSet[t] = true
			}
		}
	}

	// Plugin discovery — scan .cache/opencode/packages for tool registrations
	pluginTools := discoverAllPluginTools()
	for _, tools := range pluginTools {
		for _, t := range tools {
			toolSet[t] = true
		}
	}

	// Convert set to sorted slice
	var allTools []string
	for t := range toolSet {
		allTools = append(allTools, t)
	}
	sortStrings(allTools)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"built_in":     builtIn,
		"all":          allTools,
		"mcp_tools":    mcpTools,
		"plugin_tools": pluginTools,
	})
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

// PUT /api/config/skills/{name} - update a skill file
func (h *Handlers) PutSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill name"})
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	skillsDirPath, err := skillsDir()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	skillDir := filepath.Join(skillsDirPath, name)
	absPath, err := filepath.Abs(skillDir)
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

	if err := os.MkdirAll(skillDir, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(body.Content), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
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

// DELETE /api/config/mcp/{name} - delete an MCP server
func (h *Handlers) DeleteMCP(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" || !isValidName(name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid mcp name"})
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

	mcpRaw, ok := config["mcp"]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no mcp section"})
		return
	}

	var mcpSection map[string]json.RawMessage
	if err := json.Unmarshal(mcpRaw, &mcpSection); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if _, ok := mcpSection[name]; !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mcp server not found"})
		return
	}

	delete(mcpSection, name)

	mcpJSON, _ := json.Marshal(mcpSection)
	config["mcp"] = mcpJSON
	pretty, _ := json.MarshalIndent(config, "", "  ")

	if err := os.WriteFile(path, pretty, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
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

// ─── User Config (Role Defaults) ──────────────────────────────────────────

// GetUserConfig returns the full UserConfig as JSON.
// GET /api/config/user
func (h *Handlers) GetUserConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := userconfig.LoadConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Ensure RoleDefaults is materialized so the frontend sees seed values for
	// roles the user hasn't customised.
	if cfg.RoleDefaults == nil {
		cfg.RoleDefaults = userconfig.DefaultRoleDefaults()
	} else {
		seeds := userconfig.DefaultRoleDefaults()
		for _, role := range userconfig.CanonicalRoles {
			if _, ok := cfg.RoleDefaults[role]; !ok {
				cfg.RoleDefaults[role] = seeds[role]
			}
		}
	}
	writeJSON(w, http.StatusOK, cfg)
}

// PutUserConfig accepts a partial JSON body and merges it over the existing
// user config, then saves to disk.
// PUT /api/config/user
func (h *Handlers) PutUserConfig(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB
	cfg, err := userconfig.LoadConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Decode into a sparse map so absent fields don't overwrite existing config.
	var patch map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	// Generic field merge: re-marshal cfg, overlay patch, unmarshal back.
	// Keeps unknown-to-server fields intact and avoids hand-listing every key.
	base, _ := json.Marshal(cfg)
	var merged map[string]json.RawMessage
	_ = json.Unmarshal(base, &merged)
	for k, v := range patch {
		merged[k] = v
	}
	blob, _ := json.Marshal(merged)
	if err := json.Unmarshal(blob, cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "merge failed: " + err.Error()})
		return
	}

	if err := userconfig.SaveConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// GetRoleDefaults returns just the role_defaults block (a flattened view of
// what the New Mission modal needs to pre-populate its selectors).
// GET /api/config/user/role-defaults
func (h *Handlers) GetRoleDefaults(w http.ResponseWriter, r *http.Request) {
	cfg, err := userconfig.LoadConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := userconfig.RoleDefaults{}
	for _, role := range userconfig.CanonicalRoles {
		out[role] = cfg.GetRoleDefault(role)
	}
	writeJSON(w, http.StatusOK, out)
}

// BrowseDirectory opens a native OS directory picker dialog and returns the selected path.
func (h *Handlers) BrowseDirectory(w http.ResponseWriter, r *http.Request) {
	var selectedPath string

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("osascript", "-e", `tell application "System Events" to POSIX path of (choose folder)`)
		out, e := cmd.Output()
		if e != nil {
			http.Error(w, "Directory selection cancelled", http.StatusNoContent)
			return
		}
		selectedPath = strings.TrimSpace(string(out))
	case "linux":
		// Try zenity first, then kdialog
		cmd := exec.Command("zenity", "--file-selection", "--directory", "--title=Select Reference Directory")
		out, e := cmd.Output()
		if e != nil {
			cmd = exec.Command("kdialog", "--getexistingdirectory", "/")
			out, e = cmd.Output()
			if e != nil {
				http.Error(w, "Directory selection cancelled (install zenity or kdialog)", http.StatusNoContent)
				return
			}
		}
		selectedPath = strings.TrimSpace(string(out))
	case "windows":
		cmd := exec.Command("powershell", "-Command", `
			Add-Type -AssemblyName System.Windows.Forms
			$dialog = New-Object System.Windows.Forms.FolderBrowserDialog
			$dialog.Description = "Select Reference Directory"
			$dialog.ShowNewFolderButton = $true
			if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
				$dialog.SelectedPath
			}
		`)
		out, e := cmd.Output()
		if e != nil {
			http.Error(w, "Directory selection cancelled", http.StatusNoContent)
			return
		}
		selectedPath = strings.TrimSpace(string(out))
	default:
		http.Error(w, "Unsupported OS for directory picker", http.StatusBadRequest)
		return
	}

	if selectedPath == "" {
		http.Error(w, "No directory selected", http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"path": selectedPath})
}
