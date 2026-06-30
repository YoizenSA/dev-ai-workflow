package plugins

import (
	"encoding/json"
	"os"
	"testing"
)

// ─── helpers ──────────────────────────────────────────────────────────────

// writeJSON writes data as pretty JSON to path. It is a shared test helper
// used by the plugin install/remove tests (mcp, background-agents, ADO).
func writeJSON(t *testing.T, path string, data any) {
	t.Helper()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// readJSON reads and unmarshals path into target. Shared test helper.
func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
}
