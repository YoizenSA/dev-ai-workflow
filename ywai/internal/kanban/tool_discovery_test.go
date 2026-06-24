package kanban

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestDiscoverConfigPluginTools_KnownBundle verifies that a seeded ywai bundle
// referenced from the opencode "plugin" array resolves to its declared tool
// set (not a regex scan of the minified file).
func TestDiscoverConfigPluginTools_KnownBundle(t *testing.T) {
	entries := []string{
		"/home/user/.config/opencode/background-agents.js",
		"file:/home/user/.config/opencode/background-agents.js",
	}

	got := discoverConfigPluginTools(entries)

	tools, ok := got["background-agents"]
	if !ok {
		t.Fatalf("expected background-agents plugin in result, got keys: %v", keysOf(got))
	}
	for _, want := range []string{"delegate", "delegation_read", "delegation_stop"} {
		if !slices.Contains(tools, want) {
			t.Errorf("background-agents tools missing %q; got %v", want, tools)
		}
	}
}

// TestDiscoverConfigPluginTools_LocalFileScan verifies the best-effort source
// scan for a non-ywai local plugin file.
func TestDiscoverConfigPluginTools_LocalFileScan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-plugin.js")
	src := `export default {
		my_tool: tool({ description: "x" }),
		other: tool({ description: "y" }),
	}`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write plugin file: %v", err)
	}

	got := discoverConfigPluginTools([]string{path})
	tools := got["my-plugin"]
	for _, want := range []string{"my_tool", "other"} {
		if !slices.Contains(tools, want) {
			t.Errorf("expected scanned tool %q; got %v", want, tools)
		}
	}
}

func keysOf(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
