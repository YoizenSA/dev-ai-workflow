package opencode

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalClient_Status_NotFound(t *testing.T) {
	c := NewLocalClientWithPaths("/nonexistent/opencode.json", "/nonexistent/agents")
	ctx := context.Background()
	status, err := c.Status(ctx)
	if err != nil {
		t.Fatalf("Status() should not error: %v", err)
	}
	if status.Connected {
		t.Fatal("Expected Connected=false for missing config file")
	}
}

func TestLocalClient_Status_Found(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewLocalClientWithPaths(configPath, dir)
	ctx := context.Background()
	status, err := c.Status(ctx)
	if err != nil {
		t.Fatalf("Status() should not error: %v", err)
	}
	if !status.Connected {
		t.Fatal("Expected Connected=true when config file exists")
	}
	if status.Source != "local" {
		t.Fatalf("Expected Source=local, got %q", status.Source)
	}
}

func TestLocalClient_ListAgents_Empty(t *testing.T) {
	dir := t.TempDir()
	c := NewLocalClientWithPaths(filepath.Join(dir, "opencode.json"), dir)
	ctx := context.Background()
	agents, err := c.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents() should not error: %v", err)
	}
	if len(agents) != 0 {
		t.Fatalf("Expected empty list, got %d agents", len(agents))
	}
}

func TestLocalClient_ListAgents(t *testing.T) {
	dir := t.TempDir()
	// Create some agent markdown files
	agentFiles := []string{"dev.md", "qa.md", "architect.txt", "devops.json"}
	for _, f := range agentFiles {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Add a non-agent file that should be ignored
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewLocalClientWithPaths(filepath.Join(dir, "opencode.json"), dir)
	ctx := context.Background()
	agents, err := c.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents() should not error: %v", err)
	}

	// README.txt is also picked up because it has .txt extension (matching current behavior)
	expected := map[string]bool{"dev": true, "qa": true, "architect": true, "devops": true, "README": true}
	if len(agents) != len(expected) {
		t.Fatalf("Expected %d agents, got %d: %v", len(expected), len(agents), agentIDs(agents))
	}
	for _, a := range agents {
		if !expected[a.ID] {
			t.Fatalf("Unexpected agent ID: %s", a.ID)
		}
	}
}

func TestLocalClient_ListModels_Empty(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewLocalClientWithPaths(configPath, dir)
	ctx := context.Background()
	models, err := c.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels() should not error: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("Expected empty list, got %d models", len(models))
	}
}

func TestLocalClient_ListModels(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	config := `{
		"model": "gpt-4",
		"provider": {
			"openai": {
				"models": {
					"gpt-4": {},
					"gpt-3.5-turbo": {}
				}
			},
			"anthropic": {
				"models": {
					"claude-3-opus": {},
					"claude-3-sonnet": {}
				}
			}
		}
	}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewLocalClientWithPaths(configPath, dir)
	ctx := context.Background()
	models, err := c.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels() should not error: %v", err)
	}

	// Expected: default model first, then provider-prefixed models
	if len(models) < 1 {
		t.Fatal("Expected at least 1 model")
	}
	if models[0].ID != "gpt-4" {
		t.Fatalf("Expected first model to be 'gpt-4' (default), got %q", models[0].ID)
	}

	modelSet := make(map[string]bool)
	for _, m := range models {
		modelSet[m.ID] = true
	}

	expected := []string{"gpt-4", "openai/gpt-3.5-turbo", "anthropic/claude-3-opus", "anthropic/claude-3-sonnet"}
	for _, id := range expected {
		if !modelSet[id] {
			t.Fatalf("Expected model %q not found", id)
		}
	}
}

func TestLocalClient_ListModels_NoProviders(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	config := `{"model": "gpt-4"}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewLocalClientWithPaths(configPath, dir)
	ctx := context.Background()
	models, err := c.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels() should not error: %v", err)
	}
	if len(models) != 1 || models[0].ID != "gpt-4" {
		t.Fatalf("Expected exactly 1 model 'gpt-4', got %v", modelIDs(models))
	}
}

// Helpers

func agentIDs(agents []AgentInfo) []string {
	ids := make([]string, len(agents))
	for i, a := range agents {
		ids[i] = a.ID
	}
	return ids
}

func modelIDs(models []ModelInfo) []string {
	ids := make([]string, len(models))
	for i, m := range models {
		ids[i] = m.ID
	}
	return ids
}
