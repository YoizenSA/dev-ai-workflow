package control

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// TeamAPI handles team mode orchestration endpoints.
type TeamAPI struct {
	store *TeamStore
}

// NewTeamAPI creates a new TeamAPI.
func NewTeamAPI() *TeamAPI {
	return &TeamAPI{
		store: NewTeamStore(),
	}
}

// TeamMember represents a running teammate.
type TeamMember struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"` // "running", "idle", "done", "error"
	StartedAt string `json:"started_at"`
	TaskID    string `json:"task_id,omitempty"`
}

// TeamTask represents a task in the queue.
type TeamTask struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"` // "pending", "claimed", "running", "done", "failed"
	Assignee string `json:"assignee,omitempty"`
	Result   string `json:"result,omitempty"`
	Priority string `json:"priority"` // "high", "medium", "low"
}

func (api *TeamAPI) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"members": api.store.Members(),
		"tasks":   api.store.Tasks(),
	})
}

func (api *TeamAPI) handleSpawn(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Profile string `json:"profile"`
		Prompt  string `json:"prompt"`
		Task    string `json:"task"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	memberID := generateID("member")
	taskID := generateID("task")

	// Default values
	if req.Profile == "" {
		req.Profile = "dev"
	}
	if req.Task == "" {
		req.Task = req.Prompt
	}

	// Check if pi binary is available
	piBin, _ := exec.LookPath("pi")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if piBin != "" {
			// Try pi team member_prompt mode first
			cmd := exec.CommandContext(ctx, piBin,
				"team", "member_prompt",
				"--profile", req.Profile,
				"--prompt", req.Prompt,
			)
			output, err := cmd.CombinedOutput()

			if err != nil {
				// Fallback: direct pi prompt with profile env
				fallbackCmd := exec.CommandContext(ctx, piBin, "--prompt", req.Prompt)
				fallbackCmd.Env = append(os.Environ(), "PI_PROFILE="+req.Profile)
				output, _ = fallbackCmd.CombinedOutput()
			}

			result := strings.TrimSpace(string(output))
			api.store.UpdateTask(taskID, "done", result)
			api.store.UpdateMember(memberID, "done")
		} else {
			api.store.UpdateMember(memberID, "done")
		}
	}()

	api.store.AddMember(TeamMember{
		ID:        memberID,
		Name:      req.Profile,
		Status:    "running",
		StartedAt: time.Now().Format(time.RFC3339),
		TaskID:    taskID,
	})

	api.store.AddTask(TeamTask{
		ID:       taskID,
		Title:    req.Task,
		Status:   "running",
		Assignee: memberID,
		Priority: "high",
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"member_id": memberID,
		"task_id":   taskID,
	})
}

func (api *TeamAPI) handleSteer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MemberID string `json:"member_id"`
		Message  string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// In a real implementation, this would send steer to the RPC process
	// For now, record the steer in the store
	api.store.RecordSteer(req.MemberID, req.Message)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (api *TeamAPI) handleShutdown(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MemberID string `json:"member_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	api.store.UpdateMember(req.MemberID, "done")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func (api *TeamAPI) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title    string `json:"title"`
		Priority string `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Priority == "" {
		req.Priority = "medium"
	}

	task := TeamTask{
		ID:       generateID("task"),
		Title:    req.Title,
		Status:   "pending",
		Priority: req.Priority,
	}

	api.store.AddTask(task)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// generateID generates a simple unique ID.
var idMu sync.Mutex
var idCounter int

func generateID(prefix string) string {
	idMu.Lock()
	defer idMu.Unlock()
	idCounter++
	return fmt.Sprintf("%s-%d", prefix, idCounter)
}

// RegisterTeamRoutes registers team mode API routes.
func (s *Server) RegisterTeamRoutes(api *TeamAPI) {
	s.mux.HandleFunc("GET /api/team/status", api.handleStatus)
	s.mux.HandleFunc("POST /api/team/spawn", api.handleSpawn)
	s.mux.HandleFunc("POST /api/team/steer", api.handleSteer)
	s.mux.HandleFunc("POST /api/team/shutdown", api.handleShutdown)
	s.mux.HandleFunc("POST /api/team/task", api.handleCreateTask)
}
