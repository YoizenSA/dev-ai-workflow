package control

import (
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/health"
)

func (s *Server) registerHealthRoutes() {
	svc := health.NewService(":memory:", "")
	h := health.NewHandler(svc)
	s.mux.Handle("GET /api/health", h)
}
