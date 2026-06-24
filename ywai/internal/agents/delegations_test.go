package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDelegations_FileAbsent(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadDelegations(dir)
	if err != nil {
		t.Fatalf("expected no error when file absent: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
}

func TestLoadDelegations_Parses(t *testing.T) {
	dir := t.TempDir()
	doc := `{"delegations":{"orchestrator":{"*":"deny","dev":"allow"},"dev":{"*":"deny"}}}`
	os.WriteFile(filepath.Join(dir, DelegationsFile), []byte(doc), 0o644)

	got, err := LoadDelegations(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got["orchestrator"]["dev"] != "allow" {
		t.Errorf("orchestrator->dev = %q, want allow", got["orchestrator"]["dev"])
	}
	if got["dev"]["*"] != "deny" {
		t.Errorf("dev catch-all = %q, want deny", got["dev"]["*"])
	}
}

func TestApplyDelegations_CreatesAgentAndTaskMap(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	os.MkdirAll(dir, 0o755)

	delegations := map[string]map[string]string{
		"orchestrator": {"*": "deny", "dev": "allow"},
	}
	if err := ApplyDelegations(configPath, delegations); err != nil {
		t.Fatal(err)
	}

	root, _ := loadJSON(t, configPath)
	agents := root["agent"].(map[string]any)
	orch := agents["orchestrator"].(map[string]any)
	perm := orch["permission"].(map[string]any)
	task := perm["task"].(map[string]any)
	if task["dev"] != "allow" || task["*"] != "deny" {
		t.Errorf("task map = %+v, want dev=allow *=deny", task)
	}
}

func TestApplyDelegations_PreservesExistingScalars(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	// Pre-existing agent with a scalar permission block + a top-level model.
	seed := map[string]any{
		"agent": map[string]any{
			"orchestrator": map[string]any{
				"mode": "primary",
				"model": "opencode-go/glm-5.1",
				"permission": map[string]any{
					"read": "allow",
					"edit": "deny",
				},
			},
		},
	}
	data, _ := json.MarshalIndent(seed, "", "  ")
	os.WriteFile(configPath, data, 0o644)

	delegations := map[string]map[string]string{
		"orchestrator": {"*": "deny", "dev": "allow"},
	}
	if err := ApplyDelegations(configPath, delegations); err != nil {
		t.Fatal(err)
	}

	root, _ := loadJSON(t, configPath)
	orch := root["agent"].(map[string]any)["orchestrator"].(map[string]any)
	perm := orch["permission"].(map[string]any)

	// Scalar permissions preserved.
	if perm["read"] != "allow" || perm["edit"] != "deny" {
		t.Errorf("scalar perms clobbered: %+v", perm)
	}
	// Model preserved.
	if orch["model"] != "opencode-go/glm-5.1" {
		t.Errorf("model lost: %+v", orch["model"])
	}
	// Task map added.
	task := perm["task"].(map[string]any)
	if task["dev"] != "allow" {
		t.Errorf("task map not written: %+v", task)
	}
}

func TestApplyDelegations_Idempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	os.MkdirAll(dir, 0o755)
	delegations := map[string]map[string]string{
		"orch": {"*": "deny", "dev": "allow"},
	}
	ApplyDelegations(configPath, delegations)
	first, _ := os.ReadFile(configPath)
	ApplyDelegations(configPath, delegations)
	second, _ := os.ReadFile(configPath)
	if string(first) != string(second) {
		t.Errorf("ApplyDelegations is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestApplyDelegations_EmptyMapRemovesTask(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	seed := map[string]any{
		"agent": map[string]any{
			"finder": map[string]any{
				"permission": map[string]any{
					"task": map[string]any{"*": "allow"},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(seed, "", "  ")
	os.WriteFile(configPath, data, 0o644)

	delegations := map[string]map[string]string{
		"finder": {}, // empty -> remove task
	}
	if err := ApplyDelegations(configPath, delegations); err != nil {
		t.Fatal(err)
	}
	root, _ := loadJSON(t, configPath)
	perm := root["agent"].(map[string]any)["finder"].(map[string]any)["permission"].(map[string]any)
	if _, exists := perm["task"]; exists {
		t.Errorf("expected task removed for empty map, got %+v", perm["task"])
	}
}

func TestApplyDelegations_EmptyInputNoop(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	// File does not exist and no delegations -> must NOT create a file.
	if err := ApplyDelegations(configPath, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(configPath); err == nil {
		t.Error("empty delegations should not create a config file")
	}
}

func loadJSON(t *testing.T, path string) (map[string]any, error) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
