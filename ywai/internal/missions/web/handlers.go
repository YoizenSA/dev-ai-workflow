package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
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
	store          *missions.MissionsStore
	projectStore   *missions.ProjectStore
	hub            *Hub
	startTime      time.Time
	opencodeClient opencode.Client
	engramClient    engram.Client
	consolidations  *ConsolidationManager
	eventSink      func(evtType string, payload interface{})

	// planner, when non-nil, overrides the default opencode-based plan
	// generation used by the /api/missions/auto endpoint. Used for testing.
	planner func(goal, project, model, agent string) *missions.PlanMission
	// engineRunner, when non-nil, overrides the default background engine
	// execution for /api/missions/auto. Used for testing so the handler can be
	// exercised without spawning a real opencode-driven Engine.
	engineRunner func(missionID string) error

	// runningMu guards runningMissions which tracks which mission IDs currently
	// have an Engine executing them. Prevents accidental double-execution when
	// the wizard auto-runs after approval AND the user clicks Run separately.
	runningMu       sync.Mutex
	runningMissions map[string]struct{}
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

// ─── Refine Goal ──────────────────────────────────────────────────────────

// RefineGoal uses the opencode CLI (not the HTTP server, which has known issues
// processing prompts via REST) to refine a user goal into a structured mission
// description with scope, out-of-scope, and acceptance criteria.
func (h *Handlers) RefineGoal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Goal    string `json:"goal"`
		Context string `json:"context,omitempty"`
		Model   string `json:"model,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Goal) == "" {
		writeError(w, http.StatusBadRequest, "goal is required")
		return
	}

	refined := missions.RefineGoalWithOpencode(req.Goal, req.Context, req.Model)
	if refined == "" {
		writeError(w, http.StatusInternalServerError, "no response from AI")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"refined": refined,
	})
}

// ─── OpenCode Config ───────────────────────────────────────────────────────

// ListModels returns available opencode models from config.
func (h *Handlers) ListModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	models, err := h.opencodeClient.ListModels(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"models": []string{}})
		return
	}

	// Get connected providers from opencode status
	status, err := h.opencodeClient.Status(ctx)
	connectedProviders := make(map[string]bool)
	if err == nil && status.ConnectedProviders != nil {
		for _, p := range status.ConnectedProviders {
			connectedProviders[p] = true
		}
	}

	// Group models by provider. When the opencode server is reachable, filter by
	// connected providers so the list only shows usable models. When the server
	// is down (LocalClient), connectedProviders is empty — in that case show all
	// models rather than none, since the CLI can still use them.
	modelsByProvider := make(map[string][]map[string]interface{})
	for _, m := range models {
		if len(connectedProviders) > 0 && m.Provider != "" && !connectedProviders[m.Provider] {
			continue
		}
		provider := m.Provider
		if provider == "" {
			provider = "default"
		}
		modelsByProvider[provider] = append(modelsByProvider[provider], map[string]interface{}{
			"id":       m.ID,
			"name":     m.Name,
			"provider": m.Provider,
		})
	}

	var defaultModel string
	if len(models) > 0 {
		defaultModel = models[0].ID
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"modelsByProvider": modelsByProvider,
		"default":          defaultModel,
	})
}

// ListAgents returns available opencode agent profiles.
func (h *Handlers) ListAgents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agents, err := h.opencodeClient.ListAgents(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"agents": []string{}})
		return
	}

	ids := make([]string, len(agents))
	for i, a := range agents {
		ids[i] = a.ID
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agents": ids,
	})
}

// OpenCodeStatus returns the opencode server connection status.
func (h *Handlers) OpenCodeStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status, err := h.opencodeClient.Status(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"connected": false,
			"error":     err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// StartOpencode starts the opencode server if not already running.
func (h *Handlers) StartOpencode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// "Already running" means the opencode SERVER is reachable and serving
	// sessions. A LocalClient reports Connected=true just because opencode.json
	// exists, but it does not support sessions — so we must require Source=="server".
	status, err := h.opencodeClient.Status(ctx)
	if err == nil && status.Connected && status.Source == "server" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "already_running",
			"message": "opencode server is already connected",
		})
		return
	}

	// Start opencode serve in background on the port DefaultClient probes.
	cmd := exec.Command("opencode", "serve", "--port", "4096")
	cmd.Env = append(os.Environ(), "OPENCODE_SERVER_PASSWORD=")
	if err := cmd.Start(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start opencode: %v", err))
		return
	}

	// Wait a moment for server to start
	time.Sleep(2 * time.Second)

	// Probe the server directly; if it's up, switch to a ServerClient so that
	// subsequent session operations work (the original client may be a LocalClient
	// captured at startup that does not support sessions).
	if h.trySwitchToServerClient(ctx) {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "started",
			"message": "opencode server started successfully",
			"pid":     cmd.Process.Pid,
		})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":  "starting",
		"message": "opencode server is starting, please wait a moment and try again",
		"pid":     cmd.Process.Pid,
	})
}

// trySwitchToServerClient probes the opencode server at the configured URL and,
// if reachable, replaces h.opencodeClient with a ServerClient. Returns true if
// the switch happened. This lets the UI recover after starting opencode serve,
// without restarting the ywai server.
func (h *Handlers) trySwitchToServerClient(ctx context.Context) bool {
	url := os.Getenv("OPENCODE_URL")
	if url == "" {
		url = "http://127.0.0.1:4096"
	}
	sc := opencode.NewServerClient(url)
	st, err := sc.Status(ctx)
	if err != nil || !st.Connected || st.Source != "server" {
		return false
	}
	h.opencodeClient = sc
	return true
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
		ID             string                 `json:"id"`
		Name           string                 `json:"name"`
		Project        string                 `json:"project,omitempty"`
		Status         missions.MissionStatus `json:"status"`
		CreatedAt      time.Time              `json:"createdAt"`
		UpdatedAt      time.Time              `json:"updatedAt"`
		CompletedAt    *time.Time             `json:"completedAt,omitempty"`
		FeatureCount   int                    `json:"featureCount"`
		MilestoneCount int                    `json:"milestoneCount"`
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

// DeleteMission cancels a running mission (if active) and permanently deletes it.
func (h *Handlers) DeleteMission(w http.ResponseWriter, r *http.Request) {
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

	// Cancel first if active/paused
	if mission.Status == missions.MissionActive || mission.Status == missions.MissionPaused {
		if err := missions.CancelMission(h.store, mission); err != nil {
			log.Printf("warning: failed to cancel mission before delete: %v", err)
		}
	}

	// Broadcast deletion event before removing
	h.hub.BroadcastEvent("mission_deleted", map[string]interface{}{
		"id": mission.ID,
	})

	// Delete from store
	if err := h.store.DeleteMission(id); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete mission: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "deleted",
		"id":     id,
	})
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

	// Use RetryFeatureFromSurface which handles status check, requeue, and broadcast
	broadcastFn := func(evtType string, payload interface{}) {
		h.hub.BroadcastEvent(evtType, payload)
	}
	if err := missions.RetryFeatureFromSurface(h.store, id, featureID, broadcastFn); err != nil {
		if strings.Contains(err.Error(), "cannot retry feature in state") {
			writeJSON(w, http.StatusConflict, map[string]interface{}{
				"error":  err.Error(),
				"status": feat.Status,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to retry feature: %v", err))
		return
	}

	// Reload mission to get updated state
	mission, err = h.store.LoadMission(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to reload mission: %v", err))
		return
	}

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

// MkdirFS creates a single new directory inside parentPath. It is the write-side
// counterpart to BrowseFS. The name is sanitized to a single path segment: nested
// paths and traversal attempts are rejected.
func (h *Handlers) MkdirFS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ParentPath string `json:"parentPath"`
		Name       string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// A folder name must be a single segment: reject anything containing a path
	// separator or traversal attempt before any normalization.
	rawName := strings.TrimSpace(req.Name)
	if rawName == "" || rawName == "." || rawName == ".." {
		writeError(w, http.StatusBadRequest, "folder name is required")
		return
	}
	if strings.ContainsAny(rawName, `/\`) || strings.Contains(rawName, "..") {
		writeError(w, http.StatusBadRequest, "folder name must be a single path segment")
		return
	}
	// Reject control characters / null bytes.
	if strings.ContainsAny(rawName, "\x00\r\n") {
		writeError(w, http.StatusBadRequest, "folder name contains invalid characters")
		return
	}

	name := rawName

	parent := req.ParentPath
	if strings.TrimSpace(parent) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			writeError(w, http.StatusBadRequest, "parentPath is required and home dir unavailable")
			return
		}
		parent = home
	}
	parent = filepath.Clean(parent)

	full := filepath.Join(parent, name)
	// Second line of defense: the joined result must sit directly under parent.
	if filepath.Dir(full) != parent {
		writeError(w, http.StatusBadRequest, "invalid folder name")
		return
	}

	if err := os.Mkdir(full, 0755); err != nil {
		if os.IsExist(err) {
			writeError(w, http.StatusConflict, "folder already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("cannot create folder: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"path": full,
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

	// Idempotency guard: refuse to spawn a second Engine for a mission already
	// being executed. Without this guard, three engines could end up writing to
	// the same log file (and broadcasting the same WS event) — once because the
	// wizard auto-runs on approval, once because the user clicked Run, once for
	// any direct API caller.
	h.runningMu.Lock()
	if _, already := h.runningMissions[id]; already {
		h.runningMu.Unlock()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":    "already-running",
			"missionId": id,
		})
		return
	}
	h.runningMissions[id] = struct{}{}
	h.runningMu.Unlock()

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

		// Also forward to the event sink (e.g., kanban projector)
		if h.eventSink != nil {
			h.eventSink(evtType, payload)
		}
	}

	engine := missions.NewEngine(h.store, missions.DefaultEngineConfig(), broadcast)

	// Run in background; release the running-mission lock when the engine returns.
	go func() {
		defer func() {
			h.runningMu.Lock()
			delete(h.runningMissions, id)
			h.runningMu.Unlock()
		}()
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

// AutoMission is the one-shot autonomous flow: Goal → Plan → Approve → Run.
//
// Unlike the two-step CreateMission/ApprovePlan dance, this endpoint takes a
// goal, generates a plan, creates the mission (auto-approving it to active),
// and kicks off execution in the background — mirroring the Factory.ai
// "describe your goal and let Droid manage the work" experience.
//
// Request body:
//
//	{ "goal": "...", "project": "demo", "model": "...", "agent": "...", "autoApprove": true }
//
// Response: 202 Accepted with { "missionId": "...", "status": "started" }.
// Validation errors (empty goal) → 400. Plan generation failure → 502.
func (h *Handlers) AutoMission(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Goal        string `json:"goal"`
		Project     string `json:"project,omitempty"`
		Model       string `json:"model,omitempty"`
		Agent       string `json:"agent,omitempty"`
		AutoApprove bool   `json:"autoApprove,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Goal) == "" {
		writeError(w, http.StatusBadRequest, "goal is required")
		return
	}

	// Generate the plan. Use the injected planner when present (testing),
	// otherwise fall back to the real opencode-driven generator.
	var plan *missions.PlanMission
	if h.planner != nil {
		plan = h.planner(req.Goal, req.Project, req.Model, req.Agent)
	} else {
		// Resolve the project repo path so the planner grounds the plan in the
		// real codebase (Droid-aligned one-shot investigation for auto mode).
		repoPath := ""
		if h.projectStore != nil && req.Project != "" {
			if proj, pErr := h.projectStore.Get(req.Project); pErr == nil {
				repoPath = proj.Path
			}
		}
		plan = missions.GeneratePlanWithRepo(req.Goal, nil, req.Project, req.Model, req.Agent, repoPath)
	}
	if plan == nil {
		writeError(w, http.StatusBadGateway, "failed to generate plan")
		return
	}

	// Create + approve the mission (no interactive prompt).
	mission, err := missions.CreateMissionFromPlan(h.store, plan)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create mission: %v", err))
		return
	}

	if req.AutoApprove || mission.Status == missions.MissionPlanning {
		if err := missions.ApprovePlan(h.store, mission); err != nil {
			_ = h.store.DeleteMission(mission.ID)
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("approve plan: %v", err))
			return
		}
	}

	// Track running state to prevent double-execution (same guard as RunMission).
	h.runningMu.Lock()
	if _, already := h.runningMissions[mission.ID]; already {
		h.runningMu.Unlock()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":    "already-running",
			"missionId": mission.ID,
		})
		return
	}
	h.runningMissions[mission.ID] = struct{}{}
	h.runningMu.Unlock()

	// Broadcast the planning → active transition.
	h.hub.BroadcastEvent("mission_status_changed", map[string]interface{}{
		"id":     mission.ID,
		"status": mission.Status,
		"action": "start",
	})

	// Launch execution. Use the injected runner when present (testing); otherwise
	// build a real Engine with broadcast + project resolution.
	go func(missionID string) {
		defer func() {
			h.runningMu.Lock()
			delete(h.runningMissions, missionID)
			h.runningMu.Unlock()
		}()

		if h.engineRunner != nil {
			if err := h.engineRunner(missionID); err != nil {
				log.Printf("auto mission %s engine error: %v", missionID, err)
			}
			return
		}

		// Broadcast wrapper used by the real engine.
		broadcast := func(evtType string, payload interface{}) {
			msg := map[string]interface{}{"type": evtType}
			if p, ok := payload.(map[string]interface{}); ok {
				for k, v := range p {
					msg[k] = v
				}
			}
			if data, mErr := json.Marshal(msg); mErr == nil {
				h.hub.Broadcast(data)
			}
			if h.eventSink != nil {
				h.eventSink(evtType, payload)
			}
		}

		cfg := missions.DefaultEngineConfig()
		if h.projectStore != nil {
			cfg.RepoResolver = missions.NewProjectRepoResolver(h.projectStore)
		}
		engine := missions.NewEngine(h.store, cfg, broadcast)
		if err := engine.RunMission(missionID); err != nil {
			log.Printf("auto mission %s failed: %v", missionID, err)
			h.hub.BroadcastEvent("mission_error", map[string]interface{}{
				"missionId": missionID,
				"error":     err.Error(),
			})
		}
	}(mission.ID)

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":    "started",
		"missionId": mission.ID,
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

// GetProjectGitInfo returns the git state of a project's repo path: whether it
// is a git repo, the current branch, and the local branches. On a non-git dir
// it returns IsGitRepo=false (not an error) so the UI can offer git init.
func (h *Handlers) GetProjectGitInfo(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "project name is required")
		return
	}
	proj, err := h.projectStore.Get(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	wm := missions.NewWorkspaceManager(proj.Path)
	info, err := wm.GitInfo()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("git info: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// InitProjectGit initializes a git repo for a project that doesn't have one yet.
// Idempotent: returns 200 when git is already present. Useful when a user
// registers a plain directory and then decides to turn it into a repo.
func (h *Handlers) InitProjectGit(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "project name is required")
		return
	}
	proj, err := h.projectStore.Get(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	wm := missions.NewWorkspaceManager(proj.Path)
	if err := wm.InitGitRepo(); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("init git: %v", err))
		return
	}
	// Return the fresh git info so the UI can update without a second call.
	info, _ := wm.GitInfo()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "initialized",
		"git":    info,
	})
}

// ─── Mission Artifacts ───────────────────────────────────────────────────────

// GetMissionArtifact returns a mission artifact file content.
func (h *Handlers) GetMissionArtifact(w http.ResponseWriter, r *http.Request) {
	missionID := r.PathValue("id")
	artifactType := r.PathValue("type")

	if missionID == "" || artifactType == "" {
		writeError(w, http.StatusBadRequest, "mission ID and artifact type are required")
		return
	}

	missionDir := h.store.MissionDir(missionID)
	var filePath string

	switch artifactType {
	case "architecture":
		filePath = missionDir + "/architecture.md"
	case "validation-contract":
		filePath = missionDir + "/validation-contract.md"
	case "validation-state":
		filePath = missionDir + "/validation-state.json"
	case "services":
		filePath = missionDir + "/services.yaml"
	case "agents":
		filePath = missionDir + "/AGENTS.md"
	case "mission":
		filePath = missionDir + "/mission.md"
	case "report":
		filePath = filepath.Join(missionDir, "report", "REPORT.md")
	default:
		writeError(w, http.StatusBadRequest, "unknown artifact type")
		return
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact not found")
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(content)
}

// ValidateContract checks validation contract coverage for a mission.
func (h *Handlers) ValidateContract(w http.ResponseWriter, r *http.Request) {
	missionID := r.PathValue("id")

	if missionID == "" {
		writeError(w, http.StatusBadRequest, "mission ID is required")
		return
	}

	mission, err := h.store.LoadMission(missionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "mission not found")
		return
	}

	if err := missions.CheckValidationContractCoverage(h.store, mission); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid": true,
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
