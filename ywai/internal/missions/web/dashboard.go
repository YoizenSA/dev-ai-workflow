package web

import (
	"net/http"
)

// uiHandler returns an http.Handler that serves the Missions API routes only.
// The UI is now served by the control server.
func uiHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
}
