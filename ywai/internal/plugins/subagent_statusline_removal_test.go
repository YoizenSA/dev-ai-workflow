package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// The vendored statusline is retired in favour of opencode2's native subagent
// display, so an install must take it back off a machine that already has it —
// leaving the bundles on disk would keep a second statusline claiming the same
// slots as the host's own.
func TestRemoveSubagentStatusline_SweepsBothHalvesAndItsEntry(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
	configPath := writeAgentConfig(t, "opencode.json", map[string]any{})
	dir := filepath.Dir(configPath)

	server := filepath.Join(dir, autoDiscoveredPluginsSubdir, config.SubagentStatuslineServerBundleName)
	tuiDir := filepath.Join(dir, autoDiscoveredPluginsSubdir, SubagentStatuslineTuiPluginDir)
	mustWrite(t, server, "server bundle")
	mustWrite(t, filepath.Join(tuiDir, tuiEntryName), "tui bundle")

	tuiConfig := filepath.Join(dir, tuiConfigName)
	writeJSON(t, tuiConfig, map[string]any{
		"plugins": []any{"keep-me.js", tuiDir},
		"mouse":   true,
	})

	removed, err := RemoveSubagentStatusline(configPath)
	if err != nil {
		t.Fatalf("RemoveSubagentStatusline() error = %v", err)
	}
	if !removed {
		t.Error("removed = false, want true when both halves were present")
	}
	if _, err := os.Stat(server); !os.IsNotExist(err) {
		t.Errorf("server bundle survived (err = %v)", err)
	}
	if _, err := os.Stat(tuiDir); !os.IsNotExist(err) {
		t.Errorf("tui plugin dir survived (err = %v)", err)
	}

	root := readConfigRoot(t, tuiConfig)
	entries, _ := root["plugins"].([]any)
	if containsString(entries, tuiDir) {
		t.Errorf("tui entry survived: %v", entries)
	}
	if !containsString(entries, "keep-me.js") {
		t.Errorf("unrelated entry dropped: %v", entries)
	}
	if root["mouse"] != true {
		t.Errorf("unrelated key dropped: %v", root)
	}
}

// Reinstalling on a machine that never had it, or running twice, must not
// report a removal it did not make.
func TestRemoveSubagentStatusline_IsQuietWhenAbsent(t *testing.T) {
	configPath := writeAgentConfig(t, "opencode.json", map[string]any{})

	removed, err := RemoveSubagentStatusline(configPath)
	if err != nil {
		t.Fatalf("RemoveSubagentStatusline() error = %v", err)
	}
	if removed {
		t.Error("removed = true with nothing installed, want false")
	}
}

// An install predating the directory layout registered the loose bundle file.
func TestRemoveSubagentStatusline_SweepsTheLegacyLayout(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
	configPath := writeAgentConfig(t, "opencode.json", map[string]any{})
	dir := filepath.Dir(configPath)

	legacy := filepath.Join(dir, tuiPluginsSubdir, config.SubagentStatuslineTuiBundleName)
	mustWrite(t, legacy, "tui bundle")
	writeJSON(t, filepath.Join(dir, tuiConfigName), map[string]any{"plugins": []any{legacy}})

	removed, err := RemoveSubagentStatusline(configPath)
	if err != nil {
		t.Fatalf("RemoveSubagentStatusline() error = %v", err)
	}
	if !removed {
		t.Error("removed = false, want true for a legacy install")
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy bundle survived (err = %v)", err)
	}
	entries, _ := readConfigRoot(t, filepath.Join(dir, tuiConfigName))["plugins"].([]any)
	if containsString(entries, legacy) {
		t.Errorf("legacy entry survived: %v", entries)
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
