package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A retired workflow must not survive a seed: the whole point of the rename
// map is that seeding copies by file name and never deletes.
func TestMigrateRenamedWorkflows(t *testing.T) {
	t.Run("renames the retired file and patches id/name", func(t *testing.T) {
		dir := t.TempDir()
		writeJSON(t, filepath.Join(dir, "goal.json"), map[string]any{
			"id": "goal", "name": "goal",
			"nodes": []any{map[string]any{"id": "mine", "type": "prompt"}},
		})

		if err := migrateRenamedWorkflows(dir); err != nil {
			t.Fatalf("migrate: %v", err)
		}

		if _, err := os.Stat(filepath.Join(dir, "goal.json")); !os.IsNotExist(err) {
			t.Error("retired goal.json still present")
		}
		var got map[string]any
		data, err := os.ReadFile(filepath.Join(dir, "ship.json"))
		if err != nil {
			t.Fatalf("read ship.json: %v", err)
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("parse ship.json: %v", err)
		}
		if got["id"] != "ship" || got["name"] != "ship" {
			t.Errorf("id/name = %v/%v, want ship/ship", got["id"], got["name"])
		}
		// The user's own nodes ride along; only the identity is rewritten.
		if nodes, _ := got["nodes"].([]any); len(nodes) != 1 {
			t.Errorf("custom nodes lost: %v", got["nodes"])
		}
	})

	t.Run("drops the retired file when the replacement already exists", func(t *testing.T) {
		dir := t.TempDir()
		writeJSON(t, filepath.Join(dir, "goal.json"), map[string]any{"id": "goal", "name": "goal"})
		writeJSON(t, filepath.Join(dir, "ship.json"), map[string]any{"id": "ship", "name": "ship", "keep": true})

		if err := migrateRenamedWorkflows(dir); err != nil {
			t.Fatalf("migrate: %v", err)
		}

		if _, err := os.Stat(filepath.Join(dir, "goal.json")); !os.IsNotExist(err) {
			t.Error("retired goal.json survived alongside its replacement")
		}
		data, err := os.ReadFile(filepath.Join(dir, "ship.json"))
		if err != nil {
			t.Fatalf("read ship.json: %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("parse ship.json: %v", err)
		}
		if got["keep"] != true {
			t.Error("existing replacement was overwritten by the retired file")
		}
	})

	t.Run("is a no-op when nothing is retired", func(t *testing.T) {
		dir := t.TempDir()
		if err := migrateRenamedWorkflows(dir); err != nil {
			t.Fatalf("migrate on empty dir: %v", err)
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 0 {
			t.Errorf("migrate created files out of nothing: %v", entries)
		}
	})
}
