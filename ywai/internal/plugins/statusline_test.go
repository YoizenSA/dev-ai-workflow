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
	got := tuiConfigPath()
	want := filepath.Join(home, ".config", "opencode", "cli.json")
	if got != want {
		t.Fatalf("tuiConfigPath() = %q, want %q", got, want)
	}
}

func TestInstallSubAgentStatuslineWritesCliJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := InstallSubAgentStatusline(); err != nil {
		t.Fatalf("InstallSubAgentStatusline() error = %v", err)
	}

	path := filepath.Join(home, ".config", "opencode", "cli.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cli.json: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse cli.json: %v", err)
	}
	if _, ok := root["plugin"]; ok {
		t.Fatalf("cli.json still has legacy \"plugin\" key")
	}
	arr, ok := root["plugins"].([]any)
	if !ok {
		t.Fatalf("cli.json plugins type = %T, want []any", root["plugins"])
	}
	if !containsString(arr, subAgentStatuslinePlugin) {
		t.Fatalf("cli.json plugins %v missing %q", arr, subAgentStatuslinePlugin)
	}
}

func TestInstallSubAgentStatuslineMigratesPluginsFromTuiJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := []byte(`{"plugins":["legacy-plugin"]}` + "\n")
	if err := os.WriteFile(filepath.Join(dir, "tui.json"), legacy, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := InstallSubAgentStatusline(); err != nil {
		t.Fatalf("InstallSubAgentStatusline() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "cli.json"))
	if err != nil {
		t.Fatalf("read cli.json: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse cli.json: %v", err)
	}
	arr, _ := root["plugins"].([]any)
	if !containsString(arr, "legacy-plugin") {
		t.Errorf("migrated plugins %v missing leftover tui.json entry", arr)
	}
	if !containsString(arr, subAgentStatuslinePlugin) {
		t.Errorf("migrated plugins %v missing statusline", arr)
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
	arr, _ := root["plugins"].([]any)
	if !containsString(arr, "from-tui") {
		t.Errorf("cli.json plugins %v missing leftover tui.json entry", arr)
	}
	if root["mouse"] != false {
		t.Errorf("cli.json mouse = %v, want false from leftover tui.json", root["mouse"])
	}
}
