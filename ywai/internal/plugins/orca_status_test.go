package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const orcaFixture = `export const OrcaOpenCodeStatusPlugin = async (_ctx) => {
  return { event: async ({ event }) => {} };
};

export default {
  id: "orca-opencode-status",
  server: OrcaOpenCodeStatusPlugin,
};`

func writeOrcaPlugin(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	cfg := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(cfg, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(dir, autoDiscoveredPluginsSubdir)
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginsDir, orcaStatusBundleName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func readOrcaPlugin(t *testing.T, cfg string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(filepath.Dir(cfg), autoDiscoveredPluginsSubdir, orcaStatusBundleName))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// v2 rejects a default export carrying only server(). Without setup the plugin
// never loads and the status bar stays dead.
func TestRepairOrcaStatusPluginV2_AddsSetup(t *testing.T) {
	cfg := writeOrcaPlugin(t, orcaFixture)

	patched, err := RepairOrcaStatusPluginV2(cfg)
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if !patched {
		t.Fatal("reported no change on a v1-shaped export")
	}

	body := readOrcaPlugin(t, cfg)
	for _, want := range []string{"setup: ywaiSetupV2", "server: OrcaOpenCodeStatusPlugin", orcaStatusShimMarker} {
		if !strings.Contains(body, want) {
			t.Errorf("patched file missing %q", want)
		}
	}
	// v2 dropped message.updated; without synthesis the plugin loads and
	// silently reports nothing.
	if !strings.Contains(body, `"message.updated"`) {
		t.Error("no message.updated synthesis â€” the plugin would load but stay blind")
	}
	if strings.Count(body, "export default") != 1 {
		t.Errorf("produced %d default exports, want 1", strings.Count(body, "export default"))
	}
	// The original must be recoverable: there is no source for this file.
	if _, err := os.Stat(filepath.Join(filepath.Dir(cfg), autoDiscoveredPluginsSubdir, orcaStatusBundleName+".ywai-bak")); err != nil {
		t.Errorf("no backup of a file with no upstream source: %v", err)
	}
}

// Orca redeploys the plugin, so the repair runs on every install and must not
// stack shims.
func TestRepairOrcaStatusPluginV2_Idempotent(t *testing.T) {
	cfg := writeOrcaPlugin(t, orcaFixture)

	if _, err := RepairOrcaStatusPluginV2(cfg); err != nil {
		t.Fatalf("first repair: %v", err)
	}
	first := readOrcaPlugin(t, cfg)

	patched, err := RepairOrcaStatusPluginV2(cfg)
	if err != nil {
		t.Fatalf("second repair: %v", err)
	}
	if patched {
		t.Error("patched an already-patched file")
	}
	if readOrcaPlugin(t, cfg) != first {
		t.Error("second run changed the file")
	}
}

// An export we do not recognise must be left alone: there is no source to
// rebuild this plugin from if the patch corrupts it.
func TestRepairOrcaStatusPluginV2_RefusesUnknownExport(t *testing.T) {
	body := "export default { id: \"orca-opencode-status\", somethingElse: true };"
	cfg := writeOrcaPlugin(t, body)

	if _, err := RepairOrcaStatusPluginV2(cfg); err == nil {
		t.Error("silently accepted an export shape it does not know")
	}
	if readOrcaPlugin(t, cfg) != body {
		t.Error("modified a file it did not recognise")
	}
}

// No Orca on this machine is the common case and must be silent.
func TestRepairOrcaStatusPluginV2_NoPluginIsSilent(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(cfg, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	patched, err := RepairOrcaStatusPluginV2(cfg)
	if err != nil || patched {
		t.Errorf("patched=%v err=%v, want false/nil when Orca is not installed", patched, err)
	}
}
