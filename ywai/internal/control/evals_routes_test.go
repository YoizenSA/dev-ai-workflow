package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleSessionAnalytics_ServesFixtureDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opencode.db")
	seedMiniOpenCodeDB(t, dbPath)
	t.Setenv("OPENCODE_DB", dbPath)

	s := &Server{mux: http.NewServeMux()}
	s.registerEvalsRoutes()

	req := httptest.NewRequest(http.MethodGet, "/api/evals/session-analytics?days=0", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var body SessionAnalytics
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Summary.Sessions != 3 {
		t.Fatalf("sessions=%d want 3", body.Summary.Sessions)
	}
	if len(body.Skills) < 1 {
		t.Fatal("expected skills")
	}
}

func TestHandleEvalRuns_EmptyList(t *testing.T) {
	// The bench store is a package global that persists to $HOME/.ywai; real
	// runs from this machine would leak into the test and make it order-
	// dependent. Swap in a temp-dir store for the duration of the test.
	benchRuns = newBenchStoreAt(t.TempDir())
	s := &Server{mux: http.NewServeMux()}
	s.registerEvalsRoutes()
	req := httptest.NewRequest(http.MethodGet, "/api/evals/runs", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	runs, ok := body["runs"].([]any)
	if !ok {
		t.Fatalf("runs type: %T", body["runs"])
	}
	if len(runs) != 0 {
		t.Fatalf("want empty runs, got %v", runs)
	}
}

func TestDefaultOpenCodeDBPath_EnvOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom.db")
	t.Setenv("OPENCODE_DB", want)
	// Ensure XDG does not win over OPENCODE_DB
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "xdg"))
	if got := defaultOpenCodeDBPath(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	_ = os.Unsetenv("OPENCODE_DB")
}
