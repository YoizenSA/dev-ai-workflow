package control

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/skills"
)

// registerSkillsRoutes adds skill CRUD endpoints.
func (s *Server) registerSkillsRoutes() {
	s.mux.HandleFunc("GET /api/config/skills", s.handleListSkills)
	s.mux.HandleFunc("GET /api/config/skills/{name}", s.handleGetSkill)
	s.mux.HandleFunc("POST /api/config/skills", s.handleCreateSkill)
	s.mux.HandleFunc("PUT /api/config/skills/{name}", s.handleUpdateSkill)
	s.mux.HandleFunc("DELETE /api/config/skills/{name}", s.handleDeleteSkill)
}

// handleListSkills returns all skills (bundled + custom).
func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	store := skills.NewStore()
	list, err := store.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Return the simplified list format the UI expects.
	type skillItem struct {
		Name        string `json:"name"`
		HasSkillMD  bool   `json:"hasSkillMD"`
		Description string `json:"description"`
		Scope       string `json:"scope"`
	}
	items := make([]skillItem, 0, len(list))
	for _, sk := range list {
		items = append(items, skillItem{
			Name:        sk.Name,
			HasSkillMD:  true,
			Description: sk.Description,
			Scope:       sk.Scope,
		})
	}
	writeJSON(w, http.StatusOK, items)
}

// handleGetSkill returns a single skill with its full body.
func (s *Server) handleGetSkill(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "skill name is required"})
		return
	}

	store := skills.NewStore()
	sk, err := store.Get(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":        sk.Name,
		"description": sk.Description,
		"content":     sk.Body,
		"scope":       sk.Scope,
	})
}

// createSkillRequest is the JSON body for creating a skill.
type createSkillRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// handleCreateSkill creates a new custom skill.
func (s *Server) handleCreateSkill(w http.ResponseWriter, r *http.Request) {
	var req createSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid JSON: %v", err)})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "skill name is required"})
		return
	}

	store := skills.NewStore()
	err := store.Create(skills.Skill{
		Name:        req.Name,
		Description: req.Description,
		Body:        req.Content,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

// updateSkillRequest is the JSON body for updating a skill.
type updateSkillRequest struct {
	Content     string `json:"content"`
	Description string `json:"description"`
}

// handleUpdateSkill updates an existing custom skill.
func (s *Server) handleUpdateSkill(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "skill name is required"})
		return
	}

	var req updateSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid JSON: %v", err)})
		return
	}

	store := skills.NewStore()
	existing, err := store.Get(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if existing.Scope == "bundled" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot update bundled skills"})
		return
	}

	err = store.Update(name, skills.Skill{
		Description: req.Description,
		Body:        req.Content,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// handleDeleteSkill deletes a custom skill.
func (s *Server) handleDeleteSkill(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "skill name is required"})
		return
	}

	store := skills.NewStore()
	existing, err := store.Get(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if existing.Scope == "bundled" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot delete bundled skills"})
		return
	}

	if err := store.Delete(name); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
