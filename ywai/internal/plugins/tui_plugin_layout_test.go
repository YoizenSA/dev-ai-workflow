package plugins

import (
	"os"
	"path/filepath"
	"testing"
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

	dir, err := installTuiPluginDir(configPath, bundle, "ywai-probe")
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
