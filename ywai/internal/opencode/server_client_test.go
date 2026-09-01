package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── Agent endpoint test ───────────────────────────────────────────────────

func TestServerClient_ListAgents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/agent" {
			http.NotFound(w, r)
			return
		}
		resp := []map[string]interface{}{
			{
				"id":          "dev",
				"name":        "Developer Agent",
				"description": "Handles implementation",
				"provider":    map[string]interface{}{"id": "openai", "name": "OpenAI"},
				"model":       map[string]interface{}{"modelID": "gpt-4", "providerID": "openai"},
			},
			{
				"id":          "qa",
				"name":        "QA Agent",
				"description": "Handles testing",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewServerClient(srv.URL)
	ctx := context.Background()
	agents, err := c.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents() failed: %v", err)
	}

	if len(agents) != 2 {
		t.Fatalf("Expected 2 agents, got %d", len(agents))
	}

	if agents[0].ID != "dev" || agents[0].Provider != "openai" || agents[0].Model != "gpt-4" {
		t.Fatalf("Unexpected agent[0]: %+v", agents[0])
	}
	if agents[1].ID != "qa" {
		t.Fatalf("Unexpected agent[1]: %+v", agents[1])
	}
}

func TestServerClient_ListAgents_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewServerClient(srv.URL)
	_, err := c.ListAgents(context.Background())
	if err == nil {
		t.Fatal("Expected error for server error")
	}
}

// ─── Provider endpoint (v1) test ───────────────────────────────────────────

func TestServerClient_ListModels_V1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/provider" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		resp := map[string]interface{}{
			"all": []map[string]interface{}{
				{
					"name": "OpenAI",
					"id":   "openai",
					"type": "openai",
					"models": map[string]interface{}{
						"gpt-4": map[string]interface{}{
							"id":   "gpt-4",
							"name": "GPT-4",
						},
					},
				},
				{
					"name": "Anthropic",
					"id":   "anthropic",
					"type": "anthropic",
					"models": map[string]interface{}{
						"claude-3": map[string]interface{}{
							"id":   "claude-3",
							"name": "Claude 3",
						},
					},
				},
			},
			"default":   map[string]string{},
			"connected": []string{},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewServerClient(srv.URL)
	c.useCLI = false
	ctx := context.Background()
	models, err := c.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels() failed: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(models))
	}
}

// ─── Provider V2 endpoint test ─────────────────────────────────────────────

func TestServerClient_ListModels_V2(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/model" {
			http.NotFound(w, r)
			return
		}
		resp := map[string]interface{}{
			"location": map[string]string{},
			"data": []map[string]interface{}{
				{"id": "gpt-4", "modelID": "gpt-4", "providerID": "openai", "name": "GPT-4"},
				{"id": "claude-3", "modelID": "claude-3", "providerID": "anthropic", "name": "Claude 3"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewServerClient(srv.URL)
	c.useCLI = false
	ctx := context.Background()
	models, err := c.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels() failed: %v", err)
	}

	if len(models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(models))
	}
	want := map[string]string{
		"openai/gpt-4":       "openai",
		"anthropic/claude-3": "anthropic",
	}
	for _, m := range models {
		if want[m.ID] != m.Provider {
			t.Errorf("model %+v is not a provider/model id", m)
		}
	}
}

func TestServerClient_ListModels_DoesNotTreatProvidersAsModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/provider":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "opencode-admin", "name": "Token Bank Proxy"},
					{"id": "openai", "name": "OpenAI"},
				},
			})
		case "/provider":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"all": []map[string]interface{}{
					{
						"id":   "opencode-admin",
						"name": "Token Bank Proxy",
						"models": map[string]interface{}{
							"deepseek-v4-flash": map[string]interface{}{"name": "DeepSeek V4 Flash"},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewServerClient(srv.URL)
	c.useCLI = false
	models, err := c.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() failed: %v", err)
	}
	for _, m := range models {
		if m.ID == "opencode-admin" || m.Name == "Token Bank Proxy" && m.ID == "opencode-admin" {
			t.Fatalf("listed a provider as a model: %+v", m)
		}
	}
	if len(models) != 1 || models[0].ID != "opencode-admin/deepseek-v4-flash" {
		t.Fatalf("want the v1 nested model, got %+v", models)
	}
}

// ─── Status test ───────────────────────────────────────────────────────────

func TestServerClient_Status_Connected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := NewServerClient(srv.URL)
	ctx := context.Background()
	status, err := c.Status(ctx)
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}
	if !status.Connected {
		t.Fatal("Expected Connected=true")
	}
	if status.Source != "server" {
		t.Fatalf("Expected Source=server, got %q", status.Source)
	}
}

func TestServerClient_Status_HealthFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewServerClient(srv.URL)
	ctx := context.Background()
	status, err := c.Status(ctx)
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}
	if !status.Connected {
		t.Fatal("Expected Connected=true via health fallback")
	}
}

func TestServerClient_Status_Disconnected(t *testing.T) {
	// Use a server that will be closed, so connection fails
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	srv.Close()

	c := NewServerClient(srv.URL)
	ctx := context.Background()
	status, err := c.Status(ctx)
	if err != nil {
		t.Fatalf("Status() should not error: %v", err)
	}
	if status.Connected {
		t.Fatal("Expected Connected=false")
	}
}
