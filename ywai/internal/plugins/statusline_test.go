package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestTuiConfigPathIsCliJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// os.UserHomeDir reads USERPROFILE on Windows; without pinning it the
	// path would resolve into the TestMain sandbox home there.
	t.Setenv("USERPROFILE", home)
	got := tuiConfigPath()
	want := filepath.Join(home, ".config", "opencode", "cli.json")
	if got != want {
		t.Fatalf("tuiConfigPath() = %q, want %q", got, want)
	}
}

func TestInstallTuiLogoMigratesPluginsFromTuiJSON(t *testing.T) {
	bundle := writeBundle(t, "// logo")
	configPath := writeAgentConfig(t, "opencode.json", map[string]any{})
	legacyPath := filepath.Join(filepath.Dir(configPath), "tui.json")
	if err := os.WriteFile(legacyPath, []byte(`{"plugins":["from-tui"],"mouse":false}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installTuiLogoWithBundle(configPath, bundle); err != nil {
		t.Fatalf("installTuiLogoWithBundle() error = %v", err)
	}

	cliPath := filepath.Join(filepath.Dir(configPath), "cli.json")
	root := readConfigRoot(t, cliPath)
	arr, _ := root["plugin"].([]any)
	if !containsString(arr, "from-tui") {
		t.Errorf("cli.json plugins %v missing leftover tui.json entry", arr)
	}
	if root["mouse"] != false {
		t.Errorf("cli.json mouse = %v, want false from leftover tui.json", root["mouse"])
	}
}

// sub-agent-statusline works on opencode v1 and shows delegation activity in
// the sidebar and footer. The install used to strip it on every run — a v2-era
// decision — which quietly undid the entry Engram's installer had just written,
// so the plugin was never there after an install.
func TestInstallPublishedSubAgentStatuslineAddsItToCliJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// tuiConfigPath resolves the home via os.UserHomeDir, which is
	// USERPROFILE on Windows — pin both so the install lands in this
	// test's own temp home instead of the TestMain sandbox.
	t.Setenv("USERPROFILE", home)
	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "cli.json")
	if err := os.WriteFile(path, []byte(`{"plugin":["other"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := InstallPublishedSubAgentStatusline(); err != nil {
		t.Fatalf("InstallPublishedSubAgentStatusline() error = %v", err)
	}

	arr := pluginArrayIn(t, path)
	if !containsString(arr, subAgentStatuslinePlugin) {
		t.Errorf("plugin missing after install: %v", arr)
	}
	if !containsString(arr, "other") {
		t.Errorf("pre-existing plugin dropped: %v", arr)
	}
}

// Running install twice must not list the plugin twice.
func TestInstallPublishedSubAgentStatuslineIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir reads this on Windows

	for i := 0; i < 2; i++ {
		if err := InstallPublishedSubAgentStatusline(); err != nil {
			t.Fatalf("InstallPublishedSubAgentStatusline() run %d: %v", i+1, err)
		}
	}

	arr := pluginArrayIn(t, filepath.Join(home, ".config", "opencode", "cli.json"))
	count := 0
	for _, v := range arr {
		if s, ok := v.(string); ok && s == subAgentStatuslinePlugin {
			count++
		}
	}
	if count != 1 {
		t.Errorf("plugin listed %d times, want 1: %v", count, arr)
	}
}

func pluginArrayIn(t *testing.T, path string) []any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Base(path), err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse %s: %v", filepath.Base(path), err)
	}
	arr, ok := root["plugin"].([]any)
	if !ok {
		t.Fatalf("%s has no []any \"plugin\": %v", filepath.Base(path), root)
	}
	return arr
}
