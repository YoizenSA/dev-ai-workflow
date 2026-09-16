package plugins

import (
	"fmt"
	"os"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// ywaiPluginsSubdir is a ywai-owned directory under the opencode config dir
// where older installs left vendored plugin copies. New bundles go straight
// into the auto-discovered plugins directory; the old directory is only
// cleaned up, never written.
const ywaiPluginsSubdir = "ywai-plugins"

// InstallBackgroundAgents vendors the background-agents plugin bundle into
// the location OpenCode scans by itself: the supervision layer on top of the
// built-in `subagent` tool (notifications, steer/stop, watchdog, crash
// recovery, artifacts).
func InstallBackgroundAgents(configPath string) error {
	// Agent-agnostic: the manifest drives the opencode-only gating, and this
	// entry names no slash command, so no agent name is needed.
	return installVendorJS("", configPath, ManifestEntry{Bundle: "background-agents"})
}

// installBackgroundAgentsWithBundle copies the bundle at bundleSrc into the
// ywai-plugins dir alongside configPath and patches the config to reference it.
// Split out from InstallBackgroundAgents so the copy + patch glue is unit
// testable without resolving the real embedded/source bundle.
// AutoDiscoveredPluginsSubdir is the directory opencode scans for plugins on
// its own. v2 rejects an absolute path to a .js file in the config array —
// "configured plugin path must be a directory" — so on v2 the bundle has to be
// discovered from here instead of being pointed at. It is exported so
// cmd/ywai's uninstall removes from the same directory this package installs
// into.
const AutoDiscoveredPluginsSubdir = "plugins"

// autoDiscoveredPluginsSubdir is the in-package spelling used by the sibling
// installers; keep both names pointed at one value.
const autoDiscoveredPluginsSubdir = AutoDiscoveredPluginsSubdir

func installBackgroundAgentsWithBundle(configPath, bundleSrc string) error {
	// Drop the stale explicit entry and the copy it pointed at, so the
	// bundle is discovered once rather than also being pointed at.
	return installVendorPluginV2(configPath, bundleSrc, config.BackgroundAgentsBundleName)
}

// containsPluginPath reports whether the plugin array already references path.
func containsPluginPath(plugins []any, path string) bool {
	for _, p := range plugins {
		if s, ok := p.(string); ok && s == path {
			return true
		}
		if m, ok := p.(map[string]any); ok {
			if pkg, _ := m["package"].(string); pkg == path {
				return true
			}
		}
	}
	return false
}

// pluginArray returns the "plugins" array. A lone string is tolerated for
// hand-edited files; anything else reads as empty. Every opencode.json and
// cli.json plugin edit funnels through here and writePluginArray.
func pluginArray(root map[string]any) []any {
	switch v := root["plugins"].(type) {
	case []any:
		return append([]any{}, v...)
	case string:
		if v == "" {
			return []any{}
		}
		return []any{v}
	default:
		return []any{}
	}
}

// writePluginArray stores the plugin list under "plugins", dropping the stale
// v1 "plugin" key when an older install left it behind.
func writePluginArray(root map[string]any, plugins []any) {
	delete(root, "plugin")
	root["plugins"] = plugins
}

// copyFile copies src to dst, truncating dst if it exists.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}
