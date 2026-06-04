package kanban

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUIHandler(t *testing.T) {
	handler := uiHandler()

	tests := []struct {
		path          string
		expectedCodes []int
	}{
		{"/", []int{http.StatusOK}},
		{"/index.html", []int{http.StatusOK, http.StatusMovedPermanently}},
		{"/static/app.css", []int{http.StatusOK}},
		{"/static/app.js", []int{http.StatusOK}},
		{"/invalid", []int{http.StatusNotFound}},
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		found := false
		for _, code := range tt.expectedCodes {
			if w.Code == code {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("For path %q, expected status %v, got %d", tt.path, tt.expectedCodes, w.Code)
		}
	}
}
