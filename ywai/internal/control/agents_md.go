package control

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// agentsMdPath returns the path to the AGENTS.md file in opencode config dir.
func agentsMdPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	path := filepath.Join(home, ".config", "opencode", "AGENTS.md")
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// If it doesn't exist, return the expected path anyway (so we can create it)
	return path
}

// registerAgentsMdRoutes registers API routes for AGENTS.md editing.
func (s *Server) registerAgentsMdRoutes() {
	s.mux.HandleFunc("GET /api/agents-md", s.handleAgentsMdGet)
	s.mux.HandleFunc("PUT /api/agents-md", s.handleAgentsMdSave)
}

// handleAgentsMdGet returns the content of AGENTS.md.
func (s *Server) handleAgentsMdGet(w http.ResponseWriter, r *http.Request) {
	path := agentsMdPath()
	if path == "" {
		http.Error(w, `{"error": "AGENTS.md not found"}`, http.StatusNotFound)
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, `{"error": "Failed to read AGENTS.md"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"path":    path,
		"content": string(content),
	})
}

// handleAgentsMdSave saves the content of AGENTS.md.
func (s *Server) handleAgentsMdSave(w http.ResponseWriter, r *http.Request) {
	path := agentsMdPath()
	if path == "" {
		http.Error(w, `{"error": "AGENTS.md not found"}`, http.StatusNotFound)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error": "Failed to read request body"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if err := os.WriteFile(path, []byte(req.Content), 0o644); err != nil {
		http.Error(w, `{"error": "Failed to write AGENTS.md"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "saved",
		"path":   path,
	})
}
