package tokenbank

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchModels_ReadsV1Catalog(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer pk-test" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"id":         "deepseek-v4-flash",
					"name":       "DeepSeek V4 Flash",
					"tool_call":  true,
					"attachment": false,
					"modalities": map[string]interface{}{
						"input":  []string{"text"},
						"output": []string{"text"},
					},
					"limit": map[string]interface{}{"context": 1000000, "output": 384000},
				},
				{
					"id":         "kimi-k3",
					"name":       "Kimi K3",
					"attachment": true,
					"modalities": map[string]interface{}{
						"input":  []string{"text", "image"},
						"output": []string{"text"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	got, err := FetchModels(srv.URL, "pk-test")
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if gotPath != "/v1/models" {
		t.Fatalf("GET path = %q, want /v1/models (the key's live catalog)", gotPath)
	}
	if len(got.Models) != 2 {
		t.Fatalf("models = %d, want 2, got %#v", len(got.Models), got.Models)
	}
	flash := got.Models[0]
	if flash.ID != "deepseek-v4-flash" || flash.MaxInputTokens != 1000000 || flash.MaxOutputToken != 384000 {
		t.Fatalf("flash = %+v, want v1 id + limit.context/output", flash)
	}
	if flash.Vision {
		t.Fatalf("text-only flash must not be flagged vision")
	}
	if !IsVisionModel(got.Models[1]) {
		t.Fatalf("kimi-k3 attachment/image must count as vision")
	}
	if got.DefaultModel != "deepseek-v4-flash" {
		t.Fatalf("DefaultModel = %q, want deepseek-v4-flash", got.DefaultModel)
	}
}
