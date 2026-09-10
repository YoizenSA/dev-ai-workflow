package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// containsString reports whether want is present in the []any slice.
func containsString(slice []any, want string) bool {
	for _, v := range slice {
		if s, ok := v.(string); ok && s == want {
			return true
		}
	}
	return false
}

// pluginArray returns the root v2 "plugins" array as []any (fatal if absent/wrong type).
func pluginArray(t *testing.T, path string) []any {
	t.Helper()
	root := readConfigRoot(t, path)
	if _, ok := root["plugins"]; ok {
		t.Fatalf("config still has legacy \"plugins\" key: %v", root["plugins"])
	}
	arr, ok := root["plugin"].([]any)
	if !ok {
		t.Fatalf("config has no []any \"plugin\" array; got %T", root["plugin"])
	}
	return arr
}

// TestInstallBackgroundAgents_Integration exercises the full public installer
// against the real resolved bundle (source checkout dist/). It is skipped when
// no bundle has been built (e.g. CI without bun), so it never fails spuriously.
func TestInstallBackgroundAgents_Integration(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
	if _, err := config.BackgroundAgentsBundlePath(); err != nil {
		t.Skipf("no background-agents bundle built: %v", err)
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	writeJSON(t, configPath, map[string]any{})

	if err := InstallBackgroundAgents(configPath); err != nil {
		t.Fatalf("InstallBackgroundAgents() error = %v", err)
	}

	// v2: the bundle is vendored where OpenCode scans by itself; the config
	// stays out of it entirely.
	destJS := filepath.Join(dir, autoDiscoveredPluginsSubdir, config.BackgroundAgentsBundleName)
	info, err := os.Stat(destJS)
	if err != nil {
		t.Fatalf("expected bundle at %s: %v", destJS, err)
	}
	// The real bundle is ~1.5 MB; guard against an empty/truncated copy.
	if info.Size() < 1024 {
		t.Errorf("copied bundle size = %d bytes, want a real bundle (>1KB)", info.Size())
	}
	if _, err := os.Stat(filepath.Join(dir, autoDiscoveredPluginsSubdir, FlavorMarkerName)); !os.IsNotExist(err) {
		t.Errorf("flavor marker must not be written anymore; the v2-only plugin never reads it (err = %v)", err)
	}
	root := readConfigRoot(t, configPath)
	if _, ok := root["plugin"]; ok {
		t.Errorf("v2 install must not write an explicit plugin array, got %v", root["plugin"])
	}
}
