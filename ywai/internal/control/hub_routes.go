package control

import (
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/hub"
)

func (s *Server) registerHubRoutes() {
	store := hub.NewProjectStore()
	h := hub.NewProjectsHandler(store)
	// The hub handler does its own method+path routing internally,
	// so register without method prefixes for both list and detail.
	s.mux.Handle("/api/hub/projects", h)
	s.mux.Handle("/api/hub/projects/{id}", h)
}
