package engram

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultClient_EnvOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("ENGRAM_URL", srv.URL)
	c := DefaultClient()
	if c.baseURL != srv.URL {
		t.Fatalf("Expected baseURL=%s, got %s", srv.URL, c.baseURL)
	}
}
