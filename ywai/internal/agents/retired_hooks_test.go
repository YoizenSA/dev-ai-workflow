package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Sweeping the .atl directories is not enough while the hook that regenerates
// them still runs on every prompt. Foreign hooks must survive untouched.
func TestRemoveRetiredHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	settings := `{
  "model": "opus",
  "hooks": {
    "UserPromptSubmit": [
      {"matcher": "", "hooks": [{"type": "command", "command": "gentle-ai skill-registry refresh --quiet --no-gitignore || true"}]},
      {"matcher": "", "hooks": [{"type": "command", "command": "orca-hook"}]}
    ],
    "Stop": [
      {"matcher": "", "hooks": [{"type": "command", "command": "gentle-ai skill-registry refresh"}]}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := RemoveRetiredHooks(path); got != 2 {
		t.Fatalf("removed = %d, want 2", got)
	}

	var root map[string]any
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("settings.json is no longer valid JSON: %v", err)
	}
	if root["model"] != "opus" {
		t.Error("unrelated settings must be preserved")
	}
	hooks := root["hooks"].(map[string]any)
	if _, ok := hooks["Stop"]; ok {
		t.Error("an event left with no groups must be dropped")
	}
	if n := len(hooks["UserPromptSubmit"].([]any)); n != 1 {
		t.Fatalf("UserPromptSubmit groups = %d, want 1 (the foreign hook)", n)
	}

	// Idempotent: a clean file is never rewritten.
	if got := RemoveRetiredHooks(path); got != 0 {
		t.Errorf("second sweep removed = %d, want 0", got)
	}
}

func TestRemoveRetiredHooksMissingFile(t *testing.T) {
	if got := RemoveRetiredHooks(filepath.Join(t.TempDir(), "absent.json")); got != 0 {
		t.Errorf("removed = %d, want 0", got)
	}
}
