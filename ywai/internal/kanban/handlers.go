package kanban

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

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
		HandoffPreview *string `json:"handoff_preview"`
		Blocker        *string `json:"blocker"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Map kanban status to FSM and validate transition
	if req.Status != nil && *req.Status != "" {
		fsmStatus := MapKanbanStatusToFSM(*req.Status)
		s := string(fsmStatus)
		req.Status = &s // store will receive FSM value

		// Load current delegation to get current FSM status
		if cur, ok := h.store.GetDelegation(id); ok {
			if err := missions.IsValidTransition(
				missions.MissionStatus(cur.Status),
				fsmStatus,
			); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
		}
	}

	d, err := h.store.UpdateDelegation(id, req.Status, req.Column, req.HandoffPreview, req.Blocker)
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

// --- Markdown Frontmatter Helpers ---

// readAgentMarkdownPath returns the path to an agent's .md file, or "" if it doesn't exist.
func readAgentMarkdownPath(name string) string {
	dir, err := agentsDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(dir, name+".md")
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// parseFrontmatter extracts YAML frontmatter from a markdown file.
// Returns frontmatter body (between --- delimiters) and the rest of the content.
func parseFrontmatter(content string) (frontmatter string, body string) {
	if !strings.HasPrefix(content, "---") {
		return "", content
	}
	end := strings.Index(content[3:], "---")
	if end == -1 {
		return "", content
	}
	fm := content[3 : end+3]
	body = strings.TrimSpace(content[end+6:])
	return fm, body
}

// extractPermissionsFromFrontmatter parses permissions from YAML frontmatter.
// Supports both formats:
//   - New: permission:\n  read: allow\n  edit: deny\n ...
//   - Old: tools:\n  read: true\n  edit: false\n ...
func extractPermissionsFromFrontmatter(fm string) map[string]string {
	perms := map[string]string{}
	lines := strings.Split(fm, "\n")

	// Find the permission: or tools: key
	headerIdx := -1
	headerKey := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "permission:" || strings.HasPrefix(trimmed, "permission:") {
			headerIdx = i
			headerKey = "permission"
			break
		}
		if trimmed == "tools:" || strings.HasPrefix(trimmed, "tools:") {
			headerIdx = i
			headerKey = "tools"
			break
		}
	}
	if headerIdx == -1 {
		return perms
	}

	// Collect indented lines under the key
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Must be indented (child of the key)
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break // next top-level key
		}
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if headerKey == "tools" {
			// Old format: tools: read: true → convert to permission string
			if val == "true" {
				perms[key] = "allow"
			} else if val == "false" {
				perms[key] = "deny"
			}
		} else {
			// New format: permission: read: allow
			perms[key] = val
		}
	}
	return perms
}

// extractModeFromFrontmatter parses the mode: field from YAML frontmatter.
func extractModeFromFrontmatter(fm string) string {
	for _, line := range strings.Split(fm, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "mode:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "mode:"))
		}
	}
	return ""
}

// updatePermissionsInFrontmatter replaces the permission: block in frontmatter
// with the given permissions map. If no permission: block exists, it adds one.
// Returns the updated full markdown content.
func updatePermissionsInFrontmatter(content string, perms map[string]string) string {
	fm, body := parseFrontmatter(content)
	if fm == "" {
		// No frontmatter at all — wrap content with new frontmatter
		var b strings.Builder
		b.WriteString("---\npermission:\n")
		for _, k := range sortedKeys(perms) {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, perms[k]))
		}
		b.WriteString("---\n\n")
		b.WriteString(body)
		return b.String()
	}

	lines := strings.Split(fm, "\n")
	var result []string
	skipping := false
	inserted := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect start of permission: or tools: block
		if !skipping && (trimmed == "permission:" || trimmed == "tools:" ||
			strings.HasPrefix(trimmed, "permission:") || strings.HasPrefix(trimmed, "tools:")) {
			skipping = true
			// Insert new permission: block here
			result = append(result, "permission:")
			for _, k := range sortedKeys(perms) {
				result = append(result, fmt.Sprintf("  %s: %s", k, perms[k]))
			}
			inserted = true
			continue
		}

		if skipping {
			// Skip child lines (indented)
			if strings.TrimSpace(line) == "" {
				continue
			}
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				continue
			}
			// Not indented anymore — stop skipping
			skipping = false
		}

		result = append(result, line)

		// If we reached the end of frontmatter without finding a block, insert before end
		if !inserted && i == len(lines)-1 {
			result = append(result, "permission:")
			for _, k := range sortedKeys(perms) {
				result = append(result, fmt.Sprintf("  %s: %s", k, perms[k]))
			}
			inserted = true
		}
	}

	newFM := strings.Join(result, "\n")
	return "---\n" + newFM + "\n---\n\n" + body
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// sortedPermissionKeys returns ValidPermissionKeys as a sorted slice for error messages.
func sortedPermissionKeys() []string {
	keys := make([]string, 0, len(ValidPermissionKeys))
	for k := range ValidPermissionKeys {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
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
	if h.opencodeClient == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	ctx := r.Context()
	agents, err := h.opencodeClient.ListAgents(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	type agentInfo struct {
		Name string            `json:"name"`
		Size int64             `json:"size"`
		Mode string            `json:"mode,omitempty"`
		Perm map[string]string `json:"permission,omitempty"`
	}
	result := make([]agentInfo, len(agents))
	for i, a := range agents {
		result[i] = agentInfo{Name: a.ID, Size: 0}
	}
	writeJSON(w, http.StatusOK, result)
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

	// Validate permission keys and values
	var invalidKeys []string
	var invalidValues []string
	for k, v := range body {
		if !ValidPermissionKeys[k] {
			invalidKeys = append(invalidKeys, k)
		}
		if !ValidPermissionValues[v] {
			invalidValues = append(invalidValues, fmt.Sprintf("%s=%q", k, v))
		}
	}
	if len(invalidKeys) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid permission key(s)",
			"invalid": invalidKeys,
			"valid":   sortedPermissionKeys(),
		})
		return
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

			// Discover tools based on server type
			var tools []string

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

			if len(tools) > 0 {
				mcpTools[name] = tools
				for _, t := range tools {
					toolSet[t] = true
				}
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

// discoverStdioMCPTools starts a stdio MCP server process, sends initialize
// + tools/list JSON-RPC requests, and returns the discovered tool names.
// The process is killed after discovery.
func discoverStdioMCPTools(command []string, env map[string]string) ([]string, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}
	defer func() {
		stdin.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)

	// Send initialize request
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "ywai-kanban",
				"version": "1.0.0",
			},
		},
	}
	if err := sendJSONRPC(stdin, initReq); err != nil {
		return nil, fmt.Errorf("send initialize: %w", err)
	}

	// Read initialize response (skip any notifications)
	if _, err := readJSONRPCResponse(reader); err != nil {
		return nil, fmt.Errorf("read initialize: %w", err)
	}

	// Send initialized notification
	initialized := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	_ = sendJSONRPC(stdin, initialized)

	// Send tools/list request
	toolsReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}
	if err := sendJSONRPC(stdin, toolsReq); err != nil {
		return nil, fmt.Errorf("send tools/list: %w", err)
	}

	// Read tools/list response
	resp, err := readJSONRPCResponse(reader)
	if err != nil {
		return nil, fmt.Errorf("read tools/list: %w", err)
	}

	// Parse tool names from result
	var names []string
	if result, ok := resp["result"].(map[string]interface{}); ok {
		if tools, ok := result["tools"].([]interface{}); ok {
			for _, t := range tools {
				if tool, ok := t.(map[string]interface{}); ok {
					if name, ok := tool["name"].(string); ok && name != "" {
						names = append(names, name)
					}
				}
			}
		}
	}
	return names, nil
}

func sendJSONRPC(w io.Writer, msg map[string]interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", data)
	return err
}

func readJSONRPCResponse(reader *bufio.Reader) (map[string]interface{}, error) {
	for i := 0; i < 50; i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var resp map[string]interface{}
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}
		// Skip notifications (no id field)
		if _, ok := resp["id"]; !ok {
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("no response after 50 lines")
}

// discoverPluginTools parses plugin source code to find tool registrations.
// Looks for patterns like: toolName: tool({ ... }) or "toolName": tool({ ... })
func discoverPluginTools(pluginDir string) []string {
	var tools []string

	// Find the main dist/index.js file
	indexPath := filepath.Join(pluginDir, "dist", "index.js")
	if _, err := os.Stat(indexPath); err != nil {
		// Try root index.js
		indexPath = filepath.Join(pluginDir, "index.js")
		if _, err := os.Stat(indexPath); err != nil {
			return tools
		}
	}

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return tools
	}

	content := string(data)

	// Pattern 1: toolName: tool({ ... }) — look for tool keys before `: tool(`
	// This matches lines like: ado_prs: tool({ or "ado_prs": tool({
	toolPattern := regexp.MustCompile(`(?:"|')?([a-z][a-z0-9_]*)(?:"|')?:\s*tool\s*\(`)
	for _, match := range toolPattern.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 && match[1] != "" {
			tools = append(tools, match[1])
		}
	}

	// Pattern 2: look for tool name in description patterns
	// Some plugins register tools differently
	if len(tools) == 0 {
		// Fallback: look for patterns like name: "toolname" in tool definitions
		namePattern := regexp.MustCompile(`name\s*:\s*["']([a-z][a-z0-9_]*)["']`)
		for _, match := range namePattern.FindAllStringSubmatch(content, -1) {
			if len(match) > 1 && match[1] != "" {
				tools = append(tools, match[1])
			}
		}
	}

	return tools
}

// discoverAllPluginTools scans the opencode packages directory for plugin tools.
func discoverAllPluginTools() map[string][]string {
	result := map[string][]string{}

	home, err := os.UserHomeDir()
	if err != nil {
		return result
	}

	// Check .cache/opencode/packages/ for npm plugins
	packagesDir := filepath.Join(home, ".cache", "opencode", "packages")
	if _, err := os.Stat(packagesDir); err != nil {
		return result
	}

	entries, err := os.ReadDir(packagesDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Check for @scope/package pattern
		if strings.HasPrefix(entry.Name(), "@") {
			scopeEntries, err := os.ReadDir(filepath.Join(packagesDir, entry.Name()))
			if err != nil {
				continue
			}
			for _, scopeEntry := range scopeEntries {
				if !scopeEntry.IsDir() {
					continue
				}
				pluginPath := filepath.Join(packagesDir, entry.Name(), scopeEntry.Name())
				// Check for node_modules/@scope/package/dist or node_modules/@scope/package
				nmPath := filepath.Join(pluginPath, "node_modules", entry.Name(), scopeEntry.Name())
				tools := discoverPluginTools(nmPath)
				if len(tools) == 0 {
					tools = discoverPluginTools(pluginPath)
				}
				if len(tools) > 0 {
					pluginName := entry.Name() + "/" + scopeEntry.Name()
					result[pluginName] = tools
				}
			}
		} else {
			pluginPath := filepath.Join(packagesDir, entry.Name())
			tools := discoverPluginTools(pluginPath)
			if len(tools) > 0 {
				result[entry.Name()] = tools
			}
		}
	}

	return result
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
