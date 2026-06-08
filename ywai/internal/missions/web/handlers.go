package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// Handlers holds references to the store, project store, and hub for HTTP handlers.
type Handlers struct {
	store        *missions.MissionsStore
	projectStore *missions.ProjectStore
	hub          *Hub
	startTime    time.Time
}

// ─── Health Check ──────────────────────────────────────────────────────────

// HealthCheck returns server health status.
func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check if store is accessible
	_, err := h.store.ListMissions()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status":  "degraded",
			"error":   fmt.Sprintf("store unavailable: %v", err),
			"version": "dev",
			"uptime":  time.Since(h.startTime).String(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"version": "dev",
		"uptime":  time.Since(h.startTime).String(),
	})
}

// ─── OpenCode Config ───────────────────────────────────────────────────────

// ListModels returns available opencode models from config.
func (h *Handlers) ListModels(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"models": []string{}})
		return
	}

	var config struct {
		Model string `json:"model"`
	}
	json.Unmarshal(data, &config)

	models := []string{}
	if config.Model != "" {
		models = append(models, config.Model)
	}
	// Add common alternatives
	for _, m := range []string{
		"openai/gpt-4o", "openai/gpt-4o-mini", "openai/o3-mini",
		"anthropic/claude-sonnet-4", "anthropic/claude-haiku-4-5",
		"google/gemini-2.5-flash", "google/gemini-2.5-pro",
		"deepseek/deepseek-v4-flash", "deepseek/deepseek-v4",
		"xiaomi-token-plan-sgp/deepseek-v4-flash",
		"xiaomi-token-plan-sgp/deepseek-v4",
		"xiaomi-token-plan-sgp/gpt-4o",
	} {
		if m != config.Model {
			models = append(models, m)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"models":  models,
		"default": config.Model,
	})
}

// ListAgents returns available opencode agent profiles.
func (h *Handlers) ListAgents(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	agentsDir := filepath.Join(home, ".config", "opencode", "agents")

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"agents": []string{}})
		return
	}

	var agents []string
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".md") || strings.HasSuffix(e.Name(), ".txt") || strings.HasSuffix(e.Name(), ".json")) {
			name := strings.TrimSuffix(e.Name(), ".md")
			name = strings.TrimSuffix(name, ".txt")
			name = strings.TrimSuffix(name, ".json")
			agents = append(agents, name)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agents": agents,
	})
}

// ─── Missions ──────────────────────────────────────────────────────────────

// ListMissions returns all missions, sorted by createdAt descending.
func (h *Handlers) ListMissions(w http.ResponseWriter, r *http.Request) {
	missionsList, err := h.store.ListMissions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list missions: %v", err))
		return
	}

	// Sort by createdAt descending (newest first)
	sort.Slice(missionsList, func(i, j int) bool {
		return missionsList[i].CreatedAt.After(missionsList[j].CreatedAt)
	})

	// Filter by status query param if provided
	statusFilter := r.URL.Query().Get("status")
	if statusFilter != "" {
		var filtered []*missions.Mission
		for _, m := range missionsList {
			if string(m.Status) == statusFilter {
				filtered = append(filtered, m)
			}
		}
		missionsList = filtered
	}

	// Return summary without full features list for list view
	type missionSummary struct {
		ID             string                `json:"id"`
		Name           string                `json:"name"`
		Project        string                `json:"project,omitempty"`
		Status         missions.MissionStatus `json:"status"`
		CreatedAt      time.Time             `json:"createdAt"`
		UpdatedAt      time.Time             `json:"updatedAt"`
		CompletedAt    *time.Time            `json:"completedAt,omitempty"`
		FeatureCount   int                   `json:"featureCount"`
		MilestoneCount int                   `json:"milestoneCount"`
	}

	summaries := make([]missionSummary, 0, len(missionsList))
	for _, m := range missionsList {
		summaries = append(summaries, missionSummary{
			ID:             m.ID,
			Name:           m.Name,
			Project:        m.Project,
			Status:         m.Status,
			CreatedAt:      m.CreatedAt,
			UpdatedAt:      m.UpdatedAt,
			CompletedAt:    m.CompletedAt,
			FeatureCount:   len(m.Features),
			MilestoneCount: len(m.Milestones),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"missions": summaries,
	})
}

// handleEmptyMissionID returns 400 when the mission ID is empty (e.g., /api/missions/).
func (h *Handlers) handleEmptyMissionID(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusBadRequest, "mission id is required")
}

// GetMission returns a single mission with full detail.
func (h *Handlers) GetMission(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "mission id is required")
		return
	}

	mission, err := h.store.LoadMission(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, fmt.Sprintf("mission %q not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load mission: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, mission)
}

// ─── Features ──────────────────────────────────────────────────────────────

// ListFeatures returns the features for a mission, optionally filtered by status.
func (h *Handlers) ListFeatures(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "mission id is required")
		return
	}

	mission, err := h.store.LoadMission(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, fmt.Sprintf("mission %q not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load mission: %v", err))
		return
	}

	features := mission.Features
	if features == nil {
		features = []missions.Feature{}
	}

	// Filter by status
	statusFilter := r.URL.Query().Get("status")
	if statusFilter != "" {
		var filtered []missions.Feature
		for _, f := range features {
			if string(f.Status) == statusFilter {
				filtered = append(filtered, f)
			}
		}
		features = filtered
	}

	// Build response with milestone grouping info
	type featureWithStatus struct {
		ID          string                 `json:"id"`
		Description string                 `json:"description"`
		Status      missions.FeatureStatus `json:"status"`
		Milestone   string                 `json:"milestone"`
		SkillName   string                 `json:"skillName"`
		CreatedAt   time.Time              `json:"createdAt"`
	}

	result := make([]featureWithStatus, 0, len(features))
	for _, f := range features {
		result = append(result, featureWithStatus{
			ID:          f.ID,
			Description: f.Description,
			Status:      f.Status,
			Milestone:   f.Milestone,
			SkillName:   f.SkillName,
			CreatedAt:   f.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// ─── Pause / Resume ────────────────────────────────────────────────────────

// PauseMission pauses an active mission.
func (h *Handlers) PauseMission(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "mission id is required")
		return
	}

	mission, err := h.store.LoadMission(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, fmt.Sprintf("mission %q not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load mission: %v", err))
		return
	}

	// State validation: only active missions can be paused
	if mission.Status != missions.MissionActive {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error":  fmt.Sprintf("cannot pause mission in state %q", mission.Status),
			"status": mission.Status,
		})
		return
	}

	newStatus, err := missions.TransitionMissionStatus(mission.Status, missions.MissionPaused)
	if err != nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("cannot pause mission: %v", err))
		return
	}

	mission.Status = newStatus
	mission.UpdatedAt = time.Now()

	if err := h.store.SaveMission(mission); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save mission: %v", err))
		return
	}

	// Broadcast state change via WebSocket
	h.hub.BroadcastEvent("mission_status_changed", map[string]interface{}{
		"id":     mission.ID,
		"status": mission.Status,
		"action": "pause",
	})

	writeJSON(w, http.StatusOK, mission)
}

// ResumeMission resumes a paused mission.
func (h *Handlers) ResumeMission(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "mission id is required")
		return
	}

	mission, err := h.store.LoadMission(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, fmt.Sprintf("mission %q not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load mission: %v", err))
		return
	}

	// State validation: only paused missions can be resumed
	if mission.Status != missions.MissionPaused {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error":  fmt.Sprintf("cannot resume mission in state %q", mission.Status),
			"status": mission.Status,
		})
		return
	}

	newStatus, err := missions.TransitionMissionStatus(mission.Status, missions.MissionActive)
	if err != nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("cannot resume mission: %v", err))
		return
	}

	mission.Status = newStatus
	mission.UpdatedAt = time.Now()

	if err := h.store.SaveMission(mission); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save mission: %v", err))
		return
	}

	// Broadcast state change via WebSocket
	h.hub.BroadcastEvent("mission_status_changed", map[string]interface{}{
		"id":     mission.ID,
		"status": mission.Status,
		"action": "resume",
	})

	writeJSON(w, http.StatusOK, mission)
}

// ─── Cancel / Retry ──────────────────────────────────────────────────────────

// CancelMission cancels an active or paused mission.
func (h *Handlers) CancelMission(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "mission id is required")
		return
	}

	mission, err := h.store.LoadMission(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, fmt.Sprintf("mission %q not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load mission: %v", err))
		return
	}

	// State validation: only active or paused missions can be cancelled
	if mission.Status != missions.MissionActive && mission.Status != missions.MissionPaused {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error":  fmt.Sprintf("cannot cancel mission in state %q", mission.Status),
			"status": mission.Status,
		})
		return
	}

	if err := missions.CancelMission(h.store, mission); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to cancel mission: %v", err))
		return
	}

	// Reload mission to get updated state
	mission, err = h.store.LoadMission(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load mission after cancel: %v", err))
		return
	}

	// Broadcast state change via WebSocket
	h.hub.BroadcastEvent("mission_status_changed", map[string]interface{}{
		"id":     mission.ID,
		"status": mission.Status,
		"action": "cancel",
	})

	writeJSON(w, http.StatusOK, mission)
}

// RetryFeature re-queues a failed feature for retry.
func (h *Handlers) RetryFeature(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	featureID := r.PathValue("featureId")
	if id == "" || featureID == "" {
		writeError(w, http.StatusBadRequest, "mission id and feature id are required")
		return
	}

	mission, err := h.store.LoadMission(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, fmt.Sprintf("mission %q not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load mission: %v", err))
		return
	}

	feat, err := missions.GetFeatureByID(mission, featureID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("feature %q not found: %v", featureID, err))
		return
	}

	// Only failed features can be retried
	if feat.Status != missions.FeatureFailed {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error":  fmt.Sprintf("cannot retry feature in state %q", feat.Status),
			"status": feat.Status,
		})
		return
	}

	if _, err := missions.RequeueFeature(h.store, mission, featureID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to retry feature: %v", err))
		return
	}

	// Reload mission to get updated state
	mission, err = h.store.LoadMission(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to reload mission: %v", err))
		return
	}

	// Broadcast state change via WebSocket
	h.hub.BroadcastEvent("feature_status_changed", map[string]interface{}{
		"missionId":  mission.ID,
		"featureId":  featureID,
		"status":     missions.FeaturePending,
		"action":     "retry",
	})

	writeJSON(w, http.StatusOK, mission)
}

// ─── Validation ──────────────────────────────────────────────────────────────

// GetValidation returns the validation state for a mission.
func (h *Handlers) GetValidation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "mission id is required")
		return
	}

	vs, err := h.store.LoadValidationState(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"missionId":  id,
				"status":     "not_started",
				"assertions": []interface{}{},
				"reports":    []interface{}{},
			})
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load validation state: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, vs)
}

// ─── Logs ────────────────────────────────────────────────────────────────────

// GetFeatureLogs returns the log content for a feature.
func (h *Handlers) GetFeatureLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	featureID := r.PathValue("featureId")
	if id == "" || featureID == "" {
		writeError(w, http.StatusBadRequest, "mission id and feature id are required")
		return
	}

	logContent, err := h.store.ReadWorkerLog(id, featureID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"missionId": id,
			"featureId": featureID,
			"content":   "",
			"error":     err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"missionId": id,
		"featureId": featureID,
		"content":   logContent,
	})
}

// ─── Filesystem Browser ────────────────────────────────────────────────────

// BrowseFS lists directories and files at the given path.
func (h *Handlers) BrowseFS(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			writeError(w, http.StatusBadRequest, "path is required")
			return
		}
		path = home
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("cannot read path: %v", err))
		return
	}

	type Entry struct {
		Name    string `json:"name"`
		IsDir   bool   `json:"isDir"`
		Size    int64  `json:"size,omitempty"`
		ModTime string `json:"modTime"`
	}

	var result []Entry
	for _, e := range entries {
		info, err := e.Info()
		size := int64(0)
		modTime := ""
		if err == nil {
			size = info.Size()
			modTime = info.ModTime().Format(time.RFC3339)
		}
		result = append(result, Entry{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    size,
			ModTime: modTime,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":    path,
		"entries": result,
	})
}

// ─── Mission Creation via Opencode ─────────────────────────────────────────

// CreateMission generates a plan from a goal via opencode and returns the proposed plan.
func (h *Handlers) CreateMission(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Goal    string `json:"goal"`
		Project string `json:"project,omitempty"`
		Model   string `json:"model,omitempty"`
		Agent   string `json:"agent,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Goal) == "" {
		writeError(w, http.StatusBadRequest, "goal is required")
		return
	}

	// Generate plan using opencode
	plan := missions.GeneratePlanWithOpencode(req.Goal, nil, req.Project, req.Model, req.Agent)
	if plan == nil {
		writeError(w, http.StatusInternalServerError, "failed to generate plan")
		return
	}

	// Don't save the mission yet - return the proposed plan for approval
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"plan": plan,
	})
}

// ApprovePlan creates a mission from the approved plan.
func (h *Handlers) ApprovePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Plan *missions.PlanMission `json:"plan"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Plan == nil {
		writeError(w, http.StatusBadRequest, "plan is required")
		return
	}

	// Validate the plan
	if err := missions.ValidatePlan(req.Plan); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid plan: %v", err))
		return
	}

	// Create the mission from the plan
	mission, err := missions.CreateMissionFromPlan(h.store, req.Plan)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create mission: %v", err))
		return
	}

	// Transition from planning to active
	if mission.Status == missions.MissionPlanning {
		newStatus, err := missions.TransitionMissionStatus(mission.Status, missions.MissionActive)
		if err == nil {
			mission.Status = newStatus
			mission.UpdatedAt = time.Now()
			_ = h.store.SaveMission(mission)
		}
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"mission": mission,
	})
}

// RunMission executes a mission's features.
func (h *Handlers) RunMission(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "mission id is required")
		return
	}

	mission, err := h.store.LoadMission(id)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("mission %q not found", id))
		return
	}
	_ = mission // mission is loaded; engine will reload it

	// Create broadcast function that sends events via websocket hub.
	// We flatten payload into the top-level message so the JS handler
	// can destructure fields like missionId, featureId directly.
	broadcast := func(evtType string, payload interface{}) {
		msg := map[string]interface{}{
			"type": evtType,
		}
		if p, ok := payload.(map[string]interface{}); ok {
			for k, v := range p {
				msg[k] = v
			}
		}
		data, err := json.Marshal(msg)
		if err != nil {
			log.Printf("broadcast marshal error: %v", err)
			return
		}
		h.hub.Broadcast(data)
	}

	engine := missions.NewEngine(h.store, missions.DefaultEngineConfig(), broadcast)

	// Run in background
	go func() {
		if err := engine.RunMission(id); err != nil {
			h.hub.BroadcastEvent("mission_error", map[string]interface{}{
				"missionId": id,
				"error":     err.Error(),
			})
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":    "started",
		"missionId": id,
	})
}

// ─── Project Management ────────────────────────────────────────────────────

// ListProjects returns all registered projects.
func (h *Handlers) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects := h.projectStore.List()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"projects": projects,
	})
}

// CreateProject registers a new project.
func (h *Handlers) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		Description string `json:"description,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "project name is required")
		return
	}

	project, err := h.projectStore.Create(req.Name, req.Path, req.Description)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"project": project,
	})
}

// DeleteProject removes a project by name.
func (h *Handlers) DeleteProject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "project name is required")
		return
	}

	if err := h.projectStore.Delete(name); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "deleted"})
}

// ─── WebSocket ─────────────────────────────────────────────────────────────

// HandleWebSocket upgrades HTTP connection to WebSocket.
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

	// Send initial state to the new client
	missionsList, err := h.store.ListMissions()
	if err == nil {
		sort.Slice(missionsList, func(i, j int) bool {
			return missionsList[i].CreatedAt.After(missionsList[j].CreatedAt)
		})
		initialMsg, _ := json.Marshal(map[string]interface{}{
			"type":    "initial_state",
			"payload": missionsList,
		})
		client.write(initialMsg)
	}

	go client.writePump()
	go client.readPump()
}
