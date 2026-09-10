package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func TestInstallVisionBridge_V2UsesAutoDiscovery(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")

	// Pre-seed config with a v1-shaped explicit entry
	oldEntry := filepath.Join(dir, ywaiPluginsSubdir, config.VisionBridgeBundleName)
	if err := config.WriteJSONC(configPath, map[string]any{
		"plugins": []any{oldEntry, "keep.js"},
	}); err != nil {
		t.Fatal(err)
	}

	bundleSrc := filepath.Join(t.TempDir(), config.VisionBridgeBundleName)
	if err := os.WriteFile(bundleSrc, []byte("export default {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installVisionBridgeWithBundle(configPath, bundleSrc); err != nil {
		t.Fatalf("installVisionBridgeWithBundle: %v", err)
	}

	// Bundle must land in autoDiscoveredPluginsSubdir ("plugins/")
	discovered := filepath.Join(dir, autoDiscoveredPluginsSubdir, config.VisionBridgeBundleName)
	if _, err := os.Stat(discovered); err != nil {
		t.Errorf("bundle not vendored into plugins/ dir: %v", err)
	}

	// Stale explicit entry must be purged from config
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range openCodePlugins(root) {
		if s, ok := raw.(string); ok && strings.Contains(s, config.VisionBridgeBundleName) {
			t.Errorf("config still references %s explicitly: %v", config.VisionBridgeBundleName, raw)
		}
	}
}

func TestInstallAdvisor_V2UsesAutoDiscovery(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")

	oldEntry := filepath.Join(dir, ywaiPluginsSubdir, config.AdvisorBundleName)
	if err := config.WriteJSONC(configPath, map[string]any{
		"plugins": []any{oldEntry, "keep.js"},
	}); err != nil {
		t.Fatal(err)
	}

	bundleSrc := filepath.Join(t.TempDir(), config.AdvisorBundleName)
	if err := os.WriteFile(bundleSrc, []byte("export default {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installAdvisorWithBundle(configPath, bundleSrc); err != nil {
		t.Fatalf("installAdvisorWithBundle: %v", err)
	}

	discovered := filepath.Join(dir, autoDiscoveredPluginsSubdir, config.AdvisorBundleName)
	if _, err := os.Stat(discovered); err != nil {
		t.Errorf("bundle not vendored into plugins/ dir: %v", err)
	}

	root, err := config.ReadJSONC(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range openCodePlugins(root) {
		if s, ok := raw.(string); ok && strings.Contains(s, config.AdvisorBundleName) {
			t.Errorf("config still references %s explicitly: %v", config.AdvisorBundleName, raw)
		}
	}
}
