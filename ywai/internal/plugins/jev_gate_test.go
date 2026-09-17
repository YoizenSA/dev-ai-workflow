package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// TestJevGate_ManifestInstall exercises the real install path: the embedded
// manifest entry, gated behind the "jev-gate" flag, dispatched by RunManifest.
// There is no exported installer — the vendor-js dispatcher is the only caller.
// Skipped when no bundle has been built (e.g. CI without bun), so it never
// fails spuriously.
func TestJevGate_ManifestInstall(t *testing.T) {
	if _, err := config.JevGateBundlePath(); err != nil {
		t.Skipf("no jev-gate bundle built: %v", err)
	}

	mf, warnings := LoadManifest()
	if len(warnings) > 0 {
		t.Fatalf("LoadManifest() warnings = %v", warnings)
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	writeJSON(t, configPath, map[string]any{})

	// Without the flag the entry must stay out entirely: it is experimental.
	RunManifest(mf, "opencode", configPath, nil)
	destJS := filepath.Join(dir, autoDiscoveredPluginsSubdir, config.JevGateBundleName)
	if _, err := os.Stat(destJS); err == nil {
		t.Fatalf("jev-gate installed without its flag at %s", destJS)
	}

	for _, r := range RunManifest(mf, "opencode", configPath, map[string]bool{"jev-gate": true}) {
		if r.ID == "jev-gate" && r.Err != nil {
			t.Fatalf("jev-gate install error = %v", r.Err)
		}
	}

	// v2: the bundle is vendored where OpenCode scans by itself; the config
	// stays out of it entirely.
	info, err := os.Stat(destJS)
	if err != nil {
		t.Fatalf("expected bundle at %s: %v", destJS, err)
	}
	// The spike bundle is small but real; guard against an empty copy.
	if info.Size() < 256 {
		t.Errorf("copied bundle size = %d bytes, want a real bundle (>256B)", info.Size())
	}
	root := readConfigRoot(t, configPath)
	if _, ok := root["plugin"]; ok {
		t.Errorf("v2 install must not write an explicit plugin array, got %v", root["plugin"])
	}
}
