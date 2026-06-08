package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

// Handlers holds references to the store and hub for HTTP handlers.
type Handlers struct {
	store     *missions.MissionsStore
	hub       *Hub
	startTime time.Time
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
			Status:         m.Status,
			CreatedAt:      m.CreatedAt,
			UpdatedAt:      m.UpdatedAt,
			CompletedAt:    m.CompletedAt,
			FeatureCount:   len(m.Features),
			MilestoneCount: len(m.Milestones),
		})
	}

	writeJSON(w, http.StatusOK, summaries)
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
