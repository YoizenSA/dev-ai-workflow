package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOrcaStatus_InstallsTuiBridgeAndRetiresLegacyServerPlugin(t *testing.T) {
	cfgDir := t.TempDir()
	configPath := filepath.Join(cfgDir, "opencode.json")
	seedConfig(t, configPath, `{}`)
	pluginsDir := filepath.Join(cfgDir, autoDiscoveredPluginsSubdir)
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(pluginsDir, "orca-opencode-status.js")
	seedConfig(t, legacy, "// legacy server plugin")

	mf := Manifest{Install: []ManifestEntry{{ID: "orca-status"}}}
	for i := 0; i < 2; i++ { // idempotent: Orca redeploys, ywai re-runs
		if got := RunManifest(mf, "opencode", configPath, nil); len(got) != 1 || got[0].Err != nil {
			t.Fatalf("run %d results = %+v, want one clean install", i, got)
		}
	}

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy server plugin still loadable: stat err = %v", err)
	}
	if _, err := os.Stat(legacy + ".disabled"); err != nil {
		t.Fatalf("legacy plugin not kept as .disabled: %v", err)
	}
	pluginDir := filepath.Join(pluginsDir, "orca-status")
	if _, err := os.Stat(filepath.Join(pluginDir, tuiEntryName)); err != nil {
		t.Fatalf("tui bridge not installed: %v", err)
	}
	cli := readJSONFile(t, filepath.Join(cfgDir, tuiConfigName))
	if !containsPluginPath(pluginArray(cli), pluginDir) {
		t.Fatalf("cli.json plugins = %v, want %s", cli["plugins"], pluginDir)
	}
}
