package kanban

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUIHandler_Returns404(t *testing.T) {
	// The kanban standalone server no longer serves UI.
	// UI is served by the control server at /ui.
	handler := uiHandler()

	paths := []string{"/", "/index.html", "/static/app.css", "/static/app.js"}
	for _, path := range paths {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("For path %q, expected status 404, got %d", path, w.Code)
		}
	}
}
