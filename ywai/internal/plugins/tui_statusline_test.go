package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func seedStatuslineBundle(t *testing.T) (configPath, bundle string) {
	t.Helper()
	dir := t.TempDir()
	configPath = filepath.Join(dir, "opencode.json")
	if err := config.WriteJSONC(configPath, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	bundle = filepath.Join(t.TempDir(), config.TuiStatuslineBundleName)
	if err := os.WriteFile(bundle, []byte("export default {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return configPath, bundle
}

// On v2 the statusline is vendored and registered in the TUI client config.
func TestInstallTuiStatusline_V2VendorsAndRegisters(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
	configPath, bundle := seedStatuslineBundle(t)

	if err := installTuiStatuslineWithBundle(configPath, bundle); err != nil {
		t.Fatalf("install: %v", err)
	}

	dest := filepath.Join(filepath.Dir(configPath), tuiPluginsSubdir, config.TuiStatuslineBundleName)
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("bundle not vendored: %v", err)
	}

	root, err := config.ReadJSONC(filepath.Join(filepath.Dir(configPath), tuiConfigName))
	if err != nil {
		t.Fatalf("read tui config: %v", err)
	}
	entries, ok := root["plugins"].([]any)
	if !ok {
		t.Fatalf("v2 must register under the plugins key, got %v", root)
	}
	if len(entries) != 1 || entries[0] != dest {
		t.Errorf("plugins = %v, want [%s]", entries, dest)
	}
	// Mouse capture belongs to the logo's click easter eggs, not here.
	if _, mouse := root["mouse"]; mouse {
		t.Errorf("statusline must not enable mouse capture: %v", root)
	}
}

// On v1 the published opencode-subagent-statusline still works and is what gets
// installed, so this one must stay out of the slot entirely.
func TestInstallTuiStatusline_V1IsNoOp(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v1")
	configPath, _ := seedStatuslineBundle(t)

	if err := InstallTuiStatusline(configPath); err != nil {
		t.Fatalf("install: %v", err)
	}

	dest := filepath.Join(filepath.Dir(configPath), tuiPluginsSubdir, config.TuiStatuslineBundleName)
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("statusline vendored on v1, want no-op (err = %v)", err)
	}
}

// Re-running an install must not add the same entry twice.
func TestInstallTuiStatusline_Idempotent(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
	configPath, bundle := seedStatuslineBundle(t)

	for i := 0; i < 2; i++ {
		if err := installTuiStatuslineWithBundle(configPath, bundle); err != nil {
			t.Fatalf("install %d: %v", i, err)
		}
	}

	root, _ := config.ReadJSONC(filepath.Join(filepath.Dir(configPath), tuiConfigName))
	entries, _ := root["plugins"].([]any)
	if len(entries) != 1 {
		t.Errorf("plugins = %v, want a single entry after two installs", entries)
	}
}
