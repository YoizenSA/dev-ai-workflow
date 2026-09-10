package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
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
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
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

// v1 is refused outright: the plugin is the supervision layer on v2's native
// subagent tool and carries no v1 host surface anymore. Nothing may be
// written on a v1 host.
func TestInstallBackgroundAgents_V1Refused(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v1")
	configPath, bundle := seedBundle(t)
	dir := filepath.Dir(configPath)

	err := installBackgroundAgentsWithBundle(configPath, bundle)
	if err == nil {
		t.Fatal("install must fail on a v1 host")
	}

	if _, err := os.Stat(filepath.Join(dir, autoDiscoveredPluginsSubdir, config.BackgroundAgentsBundleName)); !os.IsNotExist(err) {
		t.Errorf("refused install must not vendor any bundle (err = %v)", err)
	}
	// The config the earlier (v1-shaped) install left behind stays untouched.
	root, readErr := config.ReadJSONC(configPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	entries, _ := root["plugin"].([]any)
	if len(entries) != 2 {
		t.Errorf("plugin array = %v, want the original 2 entries", entries)
	}
}
