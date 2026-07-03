package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type projectsHandler struct {
	store *ProjectStore
}

// NewProjectsHandler creates an HTTP handler for project operations.
func NewProjectsHandler(store *ProjectStore) http.Handler {
	return &projectsHandler{store: store}
}

func (h *projectsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract optional ID from path: /api/hub/projects or /api/hub/projects/{id}
	id, _ := strings.CutPrefix(r.URL.Path, "/api/hub/projects/")
	hasID := id != "" && id != r.URL.Path // true when there's an ID segment

	switch {
	case !hasID && r.Method == http.MethodGet:
		h.listProjects(w, r)
	case !hasID && r.Method == http.MethodPost:
		h.createProject(w, r)
	case hasID && r.Method == http.MethodGet:
		h.getProject(w, r, id)
	case hasID && r.Method == http.MethodPut:
		h.updateProject(w, r, id)
	case hasID && r.Method == http.MethodDelete:
		h.deleteProject(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *projectsHandler) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.store.ListProjects(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(projects); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *projectsHandler) createProject(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if input.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	now := time.Now()
	project := Project{
		ID:        fmt.Sprintf("%d", now.UnixNano()),
		Name:      input.Name,
		Path:      input.Path,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.store.AddProject(context.Background(), project); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(project); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *projectsHandler) getProject(w http.ResponseWriter, r *http.Request, id string) {
	project, err := h.store.GetProject(context.Background(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(project); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *projectsHandler) updateProject(w http.ResponseWriter, r *http.Request, id string) {
	var input struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if input.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	if input.Path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	now := time.Now()
	project := Project{
		Name:      input.Name,
		Path:      input.Path,
		UpdatedAt: now,
	}

	updated, err := h.store.UpdateProject(context.Background(), id, project)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(updated); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *projectsHandler) deleteProject(w http.ResponseWriter, r *http.Request, id string) {
	if _, err := h.store.GetProject(context.Background(), id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.store.RemoveProject(context.Background(), id)
	w.WriteHeader(http.StatusNoContent)
}
