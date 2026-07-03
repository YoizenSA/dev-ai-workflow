package health

import (
	"encoding/json"
	"net/http"
)

// NewHandler returns an http.Handler for health API endpoints.
func NewHandler(svc *Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		status, err := svc.CheckHealth(r.Context())
		if err != nil {
			http.Error(w, `{"error":"health check failed"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	})
	return mux
}
