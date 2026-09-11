package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLiveModels_MapsAndSorts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/model" {
			t.Errorf("path = %s, want /api/model", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"b","modelID":"b","providerID":"zeta","name":"B Model"},
			{"id":"a","modelID":"","providerID":"alpha","name":""},
			{"id":"a2","modelID":"a2","providerID":"alpha","name":"A Two"}
		]}`))
	}))
	defer srv.Close()

	models, err := fetchLiveModels(srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("fetchLiveModels: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("models = %d, want 3", len(models))
	}
	// Sorted by provider, then id; empty modelID falls back to id, empty
	// name falls back to the id.
	if models[0].Provider != "alpha" || models[0].ID != "a" || models[0].Name != "a" {
		t.Fatalf("models[0] = %+v", models[0])
	}
	if models[2].Provider != "zeta" || models[2].Name != "B Model" {
		t.Fatalf("models[2] = %+v", models[2])
	}
}

func TestHandleEvalModelsLive_BadGateway(t *testing.T) {
	// Unroutable base URL: no server there, the handler must 502 (never 500:
	// the UI treats any non-2xx as "fall back to the static file").
	srv := &Server{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/evals/models-live", nil)
	// Point the resolver at a dead port by env override for this test only.
	t.Setenv("OPENCODE_URL", "http://127.0.0.1:1")
	srv.handleEvalModelsLive(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("code = %d, want 502", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] == "" {
		t.Fatalf("body = %q, want JSON with error", rec.Body.String())
	}
}
