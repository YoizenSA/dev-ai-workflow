package control

import (
	"net/http"
	"sort"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/skills"
)

// sddAgentStatus is the per-agent SDD asset count for the status endpoint.
type sddAgentStatus struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// sddStatusResponse is the shape returned by GET /api/settings/sdd-status.
type sddStatusResponse struct {
	Agents []sddAgentStatus `json:"agents"`
	Total  int              `json:"total"`
}

// sddRemoveResponse is the shape returned by POST /api/settings/remove-sdd.
type sddRemoveResponse struct {
	Removed []string `json:"removed"`
	Errors  []string `json:"errors,omitempty"`
	Agents  int      `json:"agents"`
	Total   int      `json:"total"`
}

// registerSettingsRoutes registers the Settings maintenance API.
func (s *Server) registerSettingsRoutes() {
	s.mux.HandleFunc("GET /api/settings/sdd-status", s.handleSddStatus)
	s.mux.HandleFunc("POST /api/settings/remove-sdd", s.handleRemoveSdd)
}

// handleSddStatus reports how many SDD-managed assets are present in each
// resolved agent's config directory. It is read-only.
func (s *Server) handleSddStatus(w http.ResponseWriter, r *http.Request) {
	resp := sddStatusResponse{Agents: []sddAgentStatus{}}
	for _, a := range agent.Resolve() {
		count := skills.CountSddAssets(a.SkillsDir)
		if count == 0 {
			continue
		}
		resp.Agents = append(resp.Agents, sddAgentStatus{Name: a.Name, Count: count})
		resp.Total += count
	}
	sort.Slice(resp.Agents, func(i, j int) bool {
		return resp.Agents[i].Name < resp.Agents[j].Name
	})
	writeJSON(w, http.StatusOK, resp)
}

// handleRemoveSdd deletes every SDD-managed asset from all resolved agents'
// config directories. This is a one-shot cleanup of the assets that
// `gentle-ai sync` previously wrote; since ywai no longer runs that sync,
// the assets do not come back after removal.
func (s *Server) handleRemoveSdd(w http.ResponseWriter, r *http.Request) {
	resp := sddRemoveResponse{Removed: []string{}, Errors: []string{}}
	for _, a := range agent.Resolve() {
		removed, err := skills.RemoveSddAssets(a.SkillsDir)
		if err != nil {
			resp.Errors = append(resp.Errors, a.Name+": "+err.Error())
		}
		if len(removed) > 0 {
			resp.Agents++
		}
		for _, rel := range removed {
			resp.Removed = append(resp.Removed, a.Name+"/"+rel)
			resp.Total++
		}
	}
	sort.Strings(resp.Removed)
	writeJSON(w, http.StatusOK, resp)
}
