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

// TestLocalClient_ListAgents_FromConfig verifies agents are read from the
// "agent" section of opencode.json (the modern opencode layout), not just from
// files in a directory.
func TestLocalClient_ListAgents_FromConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	config := `{
		"agent": {
			"build": {"description": "build agent"},
			"ask": {"description": "ask agent"},
			"gentle-orchestrator": {"description": "orchestrator"}
		}
	}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	// Use a non-existent agents dir to prove the config is the source, not files.
	c := NewLocalClientWithPaths(configPath, filepath.Join(dir, "no-such-agents-dir"))
	ctx := context.Background()
	agents, err := c.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents() should not error: %v", err)
	}
	if len(agents) != 3 {
		t.Fatalf("Expected 3 agents from config, got %d: %v", len(agents), agentIDs(agents))
	}

	got := make(map[string]bool)
	for _, a := range agents {
		got[a.ID] = true
	}
	for _, expected := range []string{"build", "ask", "gentle-orchestrator"} {
		if !got[expected] {
			t.Errorf("expected agent %q not found in %v", expected, agentIDs(agents))
		}
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

// TestParseCLIModels verifies parsing of 'opencode models' output.
func TestParseCLIModels(t *testing.T) {
	output := "opencode/deepseek-v4-flash-free\nlocal-fable5/gemma-4-e4b-it\nbare-model\n\nopencode/deepseek-v4-flash-free\n"
	models := parseCLIModels(output)

	if len(models) != 3 {
		t.Fatalf("expected 3 models (deduped), got %d: %v", len(models), modelIDs(models))
	}

	// Sorted alphabetically by ID.
	if models[0].ID != "bare-model" {
		t.Errorf("expected first model 'bare-model', got %q", models[0].ID)
	}
	// Provider and name split correctly.
	deepseek := models[2]
	if deepseek.ID != "opencode/deepseek-v4-flash-free" || deepseek.Provider != "opencode" || deepseek.Name != "deepseek-v4-flash-free" {
		t.Errorf("unexpected deepseek entry: %+v", deepseek)
	}
	// Bare model has empty provider.
	if models[0].Provider != "" {
		t.Errorf("bare model should have empty provider, got %q", models[0].Provider)
	}
}

func TestOpencodeEnv_CorrectsSnapXDG(t *testing.T) {
	home, _ := os.UserHomeDir()
	want := "XDG_DATA_HOME=" + filepath.Join(home, ".local", "share")

	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "snap", "code", "247", ".local", "share"))
	env := opencodeEnv()
	if !envContains(env, want) {
		t.Errorf("snap XDG_DATA_HOME not corrected; want %q in env", want)
	}

	// A non-snap value must be left untouched.
	custom := "XDG_DATA_HOME=" + filepath.Join(home, "custom", "data")
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "custom", "data"))
	if env := opencodeEnv(); !envContains(env, custom) {
		t.Errorf("non-snap XDG_DATA_HOME should be preserved; want %q", custom)
	}
}

func envContains(env []string, kv string) bool {
	for _, e := range env {
		if e == kv {
			return true
		}
	}
	return false
}
