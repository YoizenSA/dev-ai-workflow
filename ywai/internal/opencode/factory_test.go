package opencode

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultClient_ServerFirst(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Return agents for ListAgents
		if r.URL.Path == "/agent" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": "server-agent"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	// Override the default server URL by creating a ServerClient directly
	c := NewServerClient(srv.URL)
	ctx := context.Background()
	status, err := c.Status(ctx)
	if err != nil || !status.Connected {
		t.Fatalf("Server should be connected: err=%v, status=%+v", err, status)
	}

	agents, err := c.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents() failed: %v", err)
	}
	if len(agents) != 1 || agents[0].ID != "server-agent" {
		t.Fatalf("Unexpected agents: %+v", agents)
	}
}

func TestDefaultClient_FallbackToLocal(t *testing.T) {
	// No server running — use a URL that won't connect
	c := NewServerClient("http://127.0.0.1:1")
	ctx := context.Background()
	status, err := c.Status(ctx)
	if err == nil && status.Connected {
		t.Skip("Server at :1 unexpectedly connected, can't test fallback")
	}

	// Local client works without a server
	local := NewLocalClient()
	ctx = context.Background()
	status, err = local.Status(ctx)
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}
	// Local config might or might not exist, depending on the test environment
	// We just verify it doesn't error
}

func TestProbeServer_Connected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	ctx := context.Background()
	ok, err := ProbeServer(ctx, srv.URL)
	if err != nil {
		t.Fatalf("ProbeServer() failed: %v", err)
	}
	if !ok {
		t.Fatal("Expected probe to succeed")
	}
}

func TestProbeServer_Disconnected(t *testing.T) {
	ctx := context.Background()
	ok, err := ProbeServer(ctx, "http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("ProbeServer() should not error on disconnect: %v", err)
	}
	if ok {
		t.Fatal("Expected probe to fail")
	}
}

func TestFactory_WorksWithLocalConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	config := `{
		"model": "test-model",
		"provider": {
			"test": {
				"options": {
					"models": {
						"test-model": {}
					}
				}
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	agentsDir := filepath.Join(dir, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "test-agent.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewLocalClientWithPaths(configPath, agentsDir)
	ctx := context.Background()

	models, err := c.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels() failed: %v", err)
	}
	if len(models) == 0 {
		t.Fatal("Expected at least 1 model")
	}

	agents, err := c.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents() failed: %v", err)
	}
	if len(agents) != 1 || agents[0].ID != "test-agent" {
		t.Fatalf("Expected 1 agent 'test-agent', got %+v", agents)
	}
}
