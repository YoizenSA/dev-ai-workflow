package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// TestInstallTuiPluginDir_LayoutContract pins the layout the v2 loader
// requires. The loader resolves a plugin's TUI half as <directory>/tui and
// swallows the resulting ENOTDIR when the entry names a loose file, so a
// registration that points at a .tsx is dropped with no log line and no error.
// The only observable difference is a plugin that never renders — exactly the
// failure this layout replaced. Assert the directory shape, not the copy.
func TestInstallTuiPluginDir_LayoutContract(t *testing.T) {
	configPath := writeAgentConfig(t, "opencode.json", map[string]any{})
	bundle := writeBundle(t, "// source")

	dir, err := installTuiPluginDir(configPath, bundle, "ywai-probe", "ywai-probe.tsx")
	if err != nil {
		t.Fatalf("installTuiPluginDir() error = %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("registered path %s must be a directory (err = %v)", dir, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tui.tsx")); err != nil {
		t.Errorf("plugin dir has no tui entry the loader can resolve: %v", err)
	}
}

// TestInstallTuiPluginDir_SweepsLegacyLooseFile guards the upgrade path: an
// install made under the old tui-plugins/ layout must not leave a dead second
// copy behind once the directory layout takes over.
func TestInstallTuiPluginDir_SweepsLegacyLooseFile(t *testing.T) {
	configPath := writeAgentConfig(t, "opencode.json", map[string]any{})
	bundle := writeBundle(t, "// source")

	legacy := filepath.Join(filepath.Dir(configPath), tuiPluginsSubdir, "ywai-probe.tsx")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("// stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := installTuiPluginDir(configPath, bundle, "ywai-probe", "ywai-probe.tsx"); err != nil {
		t.Fatalf("installTuiPluginDir() error = %v", err)
	}

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy loose file survived the upgrade (err = %v)", err)
	}
}

// TestDropLegacyTuiPluginEntries_ClearsStaleRegistrations checks the config
// side of the same upgrade: an entry pointing into tui-plugins/ names a file
// the install just deleted, so it must go, while unrelated entries stay put in
// order.
func TestDropLegacyTuiPluginEntries_ClearsStaleRegistrations(t *testing.T) {
	legacy := filepath.Join("/home/u/.config/opencode", tuiPluginsSubdir, config.TuiLogoBundleName)
	in := []any{
		"keep-me.js",
		legacy,
		map[string]any{"package": legacy},
		map[string]any{"path": legacy},
		"/home/u/.config/opencode/plugins/ywai-logo",
	}

	got := dropLegacyTuiPluginEntries(in)

	want := []any{"keep-me.js", "/home/u/.config/opencode/plugins/ywai-logo"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %v, want %v", i, got[i], want[i])
		}
	}
}
