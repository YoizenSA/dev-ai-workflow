package control

import (
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/scheduler"
)

func (s *Server) registerSchedulerRoutes() {
	store := scheduler.NewMemoryStore()
	sch := scheduler.NewScheduler(store)
	h := scheduler.NewHandler(sch)
	// Forward both exact and sub-paths to the scheduler's own mux.
	s.mux.Handle("/api/schedules", h)
	s.mux.Handle("/api/schedules/", h)
}
