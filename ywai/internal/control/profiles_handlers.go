package control

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/configapi"
)

// profileStore is the singleton instance used by the handlers.
var profileStoreInstance *ProfileStore

// getProfileStore returns the singleton ProfileStore, initializing it on first call.
func getProfileStore() (*ProfileStore, error) {
	if profileStoreInstance == nil {
		var err error
		profileStoreInstance, err = NewProfileStore()
		if err != nil {
			return nil, err
		}
	}
	return profileStoreInstance, nil
}

func (s *Server) registerProfileRoutes() {
	s.mux.HandleFunc("GET /api/profiles", s.handleListProfiles)
	s.mux.HandleFunc("GET /api/profiles/active", s.handleGetActiveProfile)
	s.mux.HandleFunc("POST /api/profiles", s.handleSaveProfile)
	s.mux.HandleFunc("PUT /api/profiles/{name}", s.handleUpdateProfile)
	s.mux.HandleFunc("DELETE /api/profiles/{name}", s.handleDeleteProfile)
	s.mux.HandleFunc("POST /api/profiles/activate/{name}", s.handleActivateProfile)
}

// handleListProfiles returns all profiles and the active profile name.
// GET /api/profiles
func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	store, err := getProfileStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	profiles, err := store.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	active, _ := store.GetActive()

	// Build the response as a map keyed by profile name (matching frontend expectations).
	profileMap := make(map[string]Profile, len(profiles))
	for _, p := range profiles {
		profileMap[p.Name] = p
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"profiles": profileMap,
		"active":   active,
	})
}

// handleGetActiveProfile returns the name of the active profile.
// GET /api/profiles/active
func (s *Server) handleGetActiveProfile(w http.ResponseWriter, r *http.Request) {
	store, err := getProfileStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	active, err := store.GetActive()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"active": active})
}

// saveProfileRequest is the body for creating or updating a profile.
type saveProfileRequest struct {
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Agents      map[string]AgentConfig `json:"agents,omitempty"`
	Config      *ProfileConfig         `json:"config,omitempty"`
}

// AgentConfig mirrors the frontend's OrchestratorModelMapping.
type AgentConfig struct {
	Model string `json:"model"`
}

// handleSaveProfile creates or overwrites a profile.
// POST /api/profiles
func (s *Server) handleSaveProfile(w http.ResponseWriter, r *http.Request) {
	var req saveProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}

	store, err := getProfileStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Build config from either `config` or `agents` field.
	cfg := ProfileConfig{}
	if req.Config != nil {
		cfg = *req.Config
	} else if req.Agents != nil {
		cfg.Agents = make(map[string]ProfileAgentConfig, len(req.Agents))
		for role, ac := range req.Agents {
			cfg.Agents[role] = ProfileAgentConfig{Model: ac.Model}
		}
	}

	now := time.Now()
	p := Profile{
		Name:      name,
		Config:    cfg,
		IsDefault: name == "default",
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Preserve CreatedAt if profile already exists.
	if existing, err := store.Get(name); err == nil {
		p.CreatedAt = existing.CreatedAt
	}

	if err := store.Save(p); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, p)
}

// handleUpdateProfile updates an existing profile.
// PUT /api/profiles/{name}
func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}

	var req saveProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	store, err := getProfileStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	existing, err := store.Get(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	// Merge updates.
	if req.DisplayName != "" {
		// DisplayName is not stored in our simple Profile struct but
		// we handle it through AgentConfig labels.
	}
	_ = req.Description

	cfg := existing.Config
	if req.Config != nil {
		cfg = *req.Config
	} else if req.Agents != nil {
		if cfg.Agents == nil {
			cfg.Agents = make(map[string]ProfileAgentConfig, len(req.Agents))
		}
		for role, ac := range req.Agents {
			if ac.Model == "" {
				delete(cfg.Agents, role)
			} else {
				cfg.Agents[role] = ProfileAgentConfig{Model: ac.Model}
			}
		}
	}

	existing.Config = cfg
	existing.UpdatedAt = time.Now()

	if err := store.Save(existing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// If this is the active profile, push models to all host agent files.
	applied := 0
	if active, _ := store.GetActive(); active == name {
		for role, ac := range cfg.Agents {
			if ac.Model == "" {
				continue
			}
			if configapi.ApplyAgentModel(role, ac.Model) {
				applied++
			}
		}
		// Also refresh the orchestration policy from userconfig when this is a
		// shipped profile (models + edit/write flip on the orchestrator).
		if ucfg, err := config.LoadConfig(); err == nil {
			if _, ok := ucfg.OrchestratorProfiles[name]; ok {
				if n, err := configapi.ApplyActiveOrchestratorProfile(); err == nil && n > applied {
					applied = n
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"profile":        existing,
		"agents_applied": applied,
	})
}

// handleDeleteProfile deletes a profile by name.
// DELETE /api/profiles/{name}
func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}

	store, err := getProfileStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := store.Delete(name); err != nil {
		// Check for specific error types.
		if strings.Contains(err.Error(), "cannot delete") {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
			return
		}
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleActivateProfile sets a profile as the active one and writes each
// role's model into installed agent markdown on all hosts (opencode/pi/omp/…).
// POST /api/profiles/activate/{name}
func (s *Server) handleActivateProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "profile name is required"})
		return
	}

	store, err := getProfileStore()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := store.SetActive(name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Apply models to disk agents + keep userconfig active in sync when the
	// name matches a shipped orchestrator profile (balanced/fast/deep).
	applied := 0
	if p, err := store.Get(name); err == nil {
		for role, ac := range p.Config.Agents {
			if ac.Model == "" {
				continue
			}
			if configapi.ApplyAgentModel(role, ac.Model) {
				applied++
			}
		}
	}
	if cfg, err := config.LoadConfig(); err == nil {
		if _, ok := cfg.OrchestratorProfiles[name]; ok {
			cfg.ActiveOrchestratorProfile = name
			_ = config.SaveConfig(cfg)
			// Shipped profile: re-apply from userconfig (models + orchestration
			// policy) so the orchestrator's generated policy and edit/write
			// permissions follow the activated profile.
			if n, err := configapi.ApplyActiveOrchestratorProfile(); err == nil && n > applied {
				applied = n
			}
		} else {
			// Custom UI profile: not in userconfig. Point userconfig back at the
			// shipped default so a later `ywai install` applies a known baseline
			// instead of the previously-active profile's models/policy silently
			// reverting this activation.
			if cfg.ActiveOrchestratorProfile != config.DefaultOrchestratorModelProfileName {
				cfg.ActiveOrchestratorProfile = config.DefaultOrchestratorModelProfileName
				_ = config.SaveConfig(cfg)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active":         name,
		"agents_applied": applied,
	})
}
