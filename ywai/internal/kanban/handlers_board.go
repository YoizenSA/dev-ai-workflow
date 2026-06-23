package kanban

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
)

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
	case "active":
		err = h.store.OpenSession(id)
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

func (h *Handlers) DeleteSessions(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project is required"})
		return
	}
	if err := h.store.DeleteSessionsByProject(project); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.broadcastUpdate("sessions.deleted", map[string]string{"project": project})
	writeJSON(w, http.StatusOK, map[string]string{"message": "sessions deleted"})
}

func (h *Handlers) UpdateSessions(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project is required"})
		return
	}
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
		err = h.store.CloseSessionsByProject(project)
	case "active":
		err = h.store.OpenSessionsByProject(project)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid status"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	h.broadcastUpdate("sessions.status_changed", map[string]string{"project": project, "status": req.Status})
	writeJSON(w, http.StatusOK, map[string]string{"message": "sessions updated"})
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

	// Validate the agent against the real roster (opencode.json + agents dir).
	// If the roster can't be resolved (empty), fall back to accepting any
	// non-empty agent rather than blocking delegation entirely.
	if roster := collectAgentNames(); len(roster) > 0 && !roster[req.Agent] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid agent: %q is not in the configured roster", req.Agent)})
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
