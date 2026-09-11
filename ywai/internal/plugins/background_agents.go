package plugins

import (
	"errors"
	"fmt"
	"os"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// ywaiPluginsSubdir is a ywai-owned directory under the opencode config dir
// where vendored plugin bundles live. It deliberately avoids opencode's own
// auto-discovered "plugin"/"plugins" directory so the bundle is loaded exactly
// once — via the explicit absolute path we add to the "plugin" array — instead
// of being double-loaded by directory discovery.
const ywaiPluginsSubdir = "ywai-plugins"

// InstallBackgroundAgents vendors the background-agents plugin bundle into the
// location OpenCode 2 scans by itself. The plugin is v2-only: it is the
// supervision layer on top of v2's built-in `subagent` tool (notifications,
// steer/stop, watchdog, crash recovery, artifacts) and no longer carries the
// v1 host surface. FlavorMarkerName remains only so installs can sweep the
// marker files the old v1+v2 dual plugin wrote beside its bundles; the
// v2-only plugin never reads them.
const FlavorMarkerName = "ywai-opencode-flavor.json"

func InstallBackgroundAgents(configPath string) error {
	bundle, err := config.BackgroundAgentsBundlePath()
	if err != nil {
		return err
	}
	return installBackgroundAgentsWithBundle(configPath, bundle)
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
	if !agent.OpenCodeIsV2() {
		// The plugin is v2-only: it supervises v2's built-in `subagent` tool
		// and no longer ships the v1 host surface it once also needed.
		return errors.New("background-agents requires OpenCode 2 (opencode2); skip it under OpenCode v1")
	}
	return installBackgroundAgentsV2(configPath, bundleSrc)
}

// installBackgroundAgentsV2 vendors the bundle into the directory OpenCode
// scans by itself and leaves the config array alone.
//
// v2 accepts only directories as explicit plugin paths, so the v1 arrangement —
// a .js under ywai-plugins/ referenced by absolute path — is dropped with a
// warning and the plugin never loads. Auto-discovery takes plain .js files, so
// the bundle simply lives where OpenCode already looks. Any stale explicit
// entry is removed, otherwise the warning keeps firing on every start.
func installBackgroundAgentsV2(configPath, bundleSrc string) error {
	// The v1-era dual plugin wrote a flavor marker beside its bundles; the
	// v2-only plugin never reads it, so sweep stale copies on every install.
	if err := sweepFlavorMarkers(configPath); err != nil {
		return err
	}

	// Drop the v1-shaped entry and the copy it pointed at, so the bundle is
	// discovered once rather than also being pointed at and rejected.
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

// openCodePlugins returns the OpenCode v1 "plugin" array. If only a v2
// "plugins" key exists, those entries are migrated. The v2 key is always
// deleted so the file never carries both.
func openCodePlugins(root map[string]any) []any {
	var out []any
	if raw, ok := root["plugin"]; ok {
		out = pluginsToSlice(raw)
	} else if raw, ok := root["plugins"]; ok {
		out = pluginsToSlice(raw)
	}
	// Both spellings are cleared here; writePlugins puts back the one the
	// active flavor reads. Leaving the other behind would strand a second,
	// stale plugin list in the file.
	delete(root, "plugin")
	delete(root, "plugins")
	if out == nil {
		return []any{}
	}
	return out
}

func pluginsToSlice(raw any) []any {
	switch v := raw.(type) {
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

// writePlugins stores the plugin list under the key the active OpenCode reads:
// v1 uses "plugin", v2 renamed it to "plugins". Only one is written, so the
// other never lingers as a stale second list. Every opencode.json and cli.json
// plugin edit funnels through here — see openCodePlugins for the read side,
// which accepts either spelling so a flavor switch keeps existing entries.
func writePlugins(root map[string]any, plugins []any) {
	key, stale := "plugin", "plugins"
	if agent.OpenCodeIsV2() {
		key, stale = "plugins", "plugin"
	}
	delete(root, stale)
	root[key] = plugins
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
