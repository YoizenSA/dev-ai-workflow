package plugins

import (
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func seedBundle(t *testing.T) (configPath, bundle string) {
	t.Helper()
	dir := t.TempDir()
	configPath = filepath.Join(dir, "opencode.json")
	if err := config.WriteJSONC(configPath, map[string]any{
		// A v1-shaped entry, as an earlier install would have left it.
		"plugin": []any{filepath.Join(dir, ywaiPluginsSubdir, config.BackgroundAgentsBundleName), "other.js"},
	}); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(dir, ywaiPluginsSubdir)
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, config.BackgroundAgentsBundleName), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle = filepath.Join(t.TempDir(), config.BackgroundAgentsBundleName)
	if err := os.WriteFile(bundle, []byte("export default {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return configPath, bundle
}

// v2 rejects an absolute path to a .js file with "configured plugin path must
// be a directory", so the bundle has to land where OpenCode scans by itself.
func TestInstallBackgroundAgents_V2UsesAutoDiscovery(t *testing.T) {
	configPath, bundle := seedBundle(t)
	dir := filepath.Dir(configPath)

	if err := installBackgroundAgentsWithBundle(configPath, bundle); err != nil {
		t.Fatalf("install: %v", err)
	}

	discovered := filepath.Join(dir, autoDiscoveredPluginsSubdir, config.BackgroundAgentsBundleName)
	if _, err := os.Stat(discovered); err != nil {
		t.Errorf("bundle not vendored into the scanned dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, autoDiscoveredPluginsSubdir, FlavorMarkerName)); !os.IsNotExist(err) {
		t.Errorf("flavor marker must not be written anymore; the v2-only plugin never reads it (err = %v)", err)
	}

	root, err := config.ReadJSONC(configPath)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := root["plugins"].([]any)
	for _, e := range entries {
		if s, ok := e.(string); ok && filepath.Base(s) == config.BackgroundAgentsBundleName {
			t.Errorf("explicit entry left behind; v2 warns and skips it: %v", entries)
		}
	}
	// Unrelated entries must survive the rewrite.
	if len(entries) != 1 || entries[0] != "other.js" {
		t.Errorf("plugins = %v, want [other.js]", entries)
	}
	// The copy the rejected entry pointed at is gone, so it cannot load twice.
	if _, err := os.Stat(filepath.Join(dir, ywaiPluginsSubdir, config.BackgroundAgentsBundleName)); !os.IsNotExist(err) {
		t.Errorf("stale v1 copy still present (err = %v)", err)
	}
}
