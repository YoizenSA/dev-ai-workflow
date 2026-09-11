package control

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/envprofile"
)

// User presets: copies of a builtin or of an env's content, editable from
// the web Envs page. Builtins stay embedded and read-only.
func (s *Server) registerEnvPresetRoutes() {
	s.mux.HandleFunc("POST /api/env-presets", s.handlePresetCopy)
	s.mux.HandleFunc("PATCH /api/env-presets/{name}", s.handlePresetPatch)
	s.mux.HandleFunc("DELETE /api/env-presets/{name}", s.handlePresetDelete)
}

// customPresetNames lists the user (non-builtin) presets.
func customPresetNames() []string {
	out := []string{}
	for _, name := range envprofile.Presets() {
		if !envprofile.IsBuiltinPreset(name) {
			out = append(out, name)
		}
	}
	return out
}

// writePresetErr maps envprofile preset errors to HTTP statuses.
func writePresetErr(w http.ResponseWriter, err error) {
	msg := err.Error()
	status := http.StatusInternalServerError
	switch {
	case strings.Contains(msg, "unknown preset"):
		status = http.StatusNotFound
	case strings.Contains(msg, "invalid preset name"):
		status = http.StatusBadRequest
	case strings.Contains(msg, "already exists"), strings.Contains(msg, "built in"), strings.Contains(msg, "in use"):
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": msg})
}

// presetCopyRequest is the POST /api/env-presets body. With env set, the
// copy starts from that env's preset plus its content overrides ("save env
// as preset"); otherwise from preset `from` as is.
type presetCopyRequest struct {
	From        string `json:"from"`
	Env         string `json:"env"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (s *Server) handlePresetCopy(w http.ResponseWriter, r *http.Request) {
	var req presetCopyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body: " + err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.From = strings.TrimSpace(req.From)
	var ov *envprofile.ProfileOverrides
	if env := strings.TrimSpace(req.Env); env != "" {
		p, err := envprofile.Get(env)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("unknown env %q", env)})
			return
		}
		ov = p.Overrides
		if req.From == "" {
			req.From = p.Preset
		}
	}
	if req.From == "" || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and from (or env) are required"})
		return
	}
	if err := envprofile.SavePresetCopy(req.From, req.Name, req.Description, ov); err != nil {
		writePresetErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

// presetPatchRequest renames and/or re-describes a user preset.
type presetPatchRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func (s *Server) handlePresetPatch(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	var req presetPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body: " + err.Error()})
		return
	}
	if req.Description != nil {
		if err := envprofile.UpdatePresetDescription(name, *req.Description); err != nil {
			writePresetErr(w, err)
			return
		}
	}
	if req.Name != nil {
		if next := strings.TrimSpace(*req.Name); next != name {
			if err := envprofile.RenamePreset(name, next); err != nil {
				writePresetErr(w, err)
				return
			}
			name = next
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name})
}

func (s *Server) handlePresetDelete(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	if err := envprofile.DeletePreset(name); err != nil {
		writePresetErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "name": name})
}
