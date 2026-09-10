package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// backgroundAgentsPermissions are the opencode permission keys the
// background-agents plugin needs. "delegate" launches an async sub-agent;
// "delegation_*" globs the supervisor/retrieval tools (read, list, status,
// peek, steer, stop). Set at the top-level config so the primary agent can use
// them; per-agent frontmatter still governs sub-agents (see installers.go).
var backgroundAgentsPermissions = map[string]string{
	"delegate":     "allow",
	"delegation_*": "allow",
}

// ywaiPluginsSubdir is a ywai-owned directory under the opencode config dir
// where vendored plugin bundles live. It deliberately avoids opencode's own
// auto-discovered "plugin"/"plugins" directory so the bundle is loaded exactly
// once — via the explicit absolute path we add to the "plugin" array — instead
// of being double-loaded by directory discovery.
const ywaiPluginsSubdir = "ywai-plugins"

// RemoveBackgroundAgents unwires the delegation plugin from a config. OpenCode
// v1 has no native delegation tool, so this is only used to clean up, never as
// part of a normal install: InstallBackgroundAgents is what wires it in.
func RemoveBackgroundAgents(configPath string) error {
	var root map[string]any
	if _, err := os.Stat(configPath); err == nil {
		var readErr error
		root, readErr = config.ReadJSONC(configPath)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", configPath, readErr)
		}
	} else if os.IsNotExist(err) {
		return nil
	} else {
		return fmt.Errorf("stat %s: %w", configPath, err)
	}

	plugins := openCodePlugins(root)
	kept := make([]any, 0, len(plugins))
	for _, plugin := range plugins {
		path := ""
		if value, ok := plugin.(string); ok {
			path = value
		} else if value, ok := plugin.(map[string]any); ok {
			path, _ = value["package"].(string)
		}
		if filepath.Base(path) != config.BackgroundAgentsBundleName {
			kept = append(kept, plugin)
		}
	}
	writePlugins(root, kept)

	if perms, ok := root["permission"].(map[string]any); ok {
		for action := range backgroundAgentsPermissions {
			delete(perms, action)
		}
		if len(perms) == 0 {
			delete(root, "permission")
		} else {
			root["permission"] = perms
		}
	}

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	if err := os.Remove(filepath.Join(filepath.Dir(configPath), ywaiPluginsSubdir, config.BackgroundAgentsBundleName)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy background-agents bundle: %w", err)
	}
	return nil
}

// InstallBackgroundAgents vendors the background-agents plugin bundle next to
// the given opencode config and wires it into the config (plugin array +
// delegation permissions). configPath is the path to opencode.json(c).
// FlavorMarkerName is the file ywai writes beside the vendored bundle so the
// plugin knows which OpenCode it is running under. The plugin cannot work this
// out on its own: v1 and v2 expose no capability that cleanly separates them,
// and guessing would either add a dead tool on v1 or skip the override on v2.
const FlavorMarkerName = "ywai-opencode-flavor.json"

// writeFlavorMarker records the active flavor next to the vendored bundles.
// Absent or unreadable means v1, which is the conservative default: the
// subagent override is skipped rather than registering a tool that shadows
// nothing.
func writeFlavorMarker(destDir string) error {
	flavor := "v1"
	if agent.OpenCodeIsV2() {
		flavor = "v2"
	}
	body := []byte(`{"opencodeVersion":"` + flavor + `"}` + "\n")
	return os.WriteFile(filepath.Join(destDir, FlavorMarkerName), body, 0o644)
}

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
	if agent.OpenCodeIsV2() {
		return installBackgroundAgentsV2(configPath, bundleSrc)
	}

	destDir := filepath.Join(filepath.Dir(configPath), ywaiPluginsSubdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create plugins dir %s: %w", destDir, err)
	}

	destJS := filepath.Join(destDir, config.BackgroundAgentsBundleName)
	if err := copyFile(bundleSrc, destJS); err != nil {
		return fmt.Errorf("copy plugin bundle: %w", err)
	}

	if err := writeFlavorMarker(destDir); err != nil {
		return fmt.Errorf("write flavor marker: %w", err)
	}

	return patchOpenCodeBackgroundAgents(configPath, destJS)
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
	destDir := filepath.Join(filepath.Dir(configPath), autoDiscoveredPluginsSubdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create plugins dir %s: %w", destDir, err)
	}

	destJS := filepath.Join(destDir, config.BackgroundAgentsBundleName)
	if err := copyFile(bundleSrc, destJS); err != nil {
		return fmt.Errorf("copy plugin bundle: %w", err)
	}

	// The plugin reads the marker from its own directory.
	if err := writeFlavorMarker(destDir); err != nil {
		return fmt.Errorf("write flavor marker: %w", err)
	}

	// Drop the v1-shaped entry and the copy it pointed at, so the bundle is
	// discovered once rather than also being pointed at and rejected.
	root, err := config.ReadJSONC(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", configPath, err)
	}
	kept := make([]any, 0)
	for _, raw := range openCodePlugins(root) {
		// String entries only; map-form entries are intentionally left as-is
		// here (see subagentStatuslineServerEntry for the map-aware variant).
		if s, ok := raw.(string); ok && strings.Contains(s, config.BackgroundAgentsBundleName) {
			continue
		}
		kept = append(kept, raw)
	}
	writePlugins(root, kept)
	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	if err := os.Remove(filepath.Join(filepath.Dir(configPath), ywaiPluginsSubdir, config.BackgroundAgentsBundleName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// patchOpenCodeBackgroundAgents adds pluginJSPath to the config's v2 "plugins"
// array (idempotently) and merges the delegation allow rules into the
// top-level v2 "permissions" array, preserving existing rules. A leftover v1
// "permission" map is deleted entirely. It is safe to call repeatedly.
func patchOpenCodeBackgroundAgents(configPath, pluginJSPath string) error {
	var root map[string]any
	if _, err := os.Stat(configPath); err == nil {
		var readErr error
		root, readErr = config.ReadJSONC(configPath)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", configPath, readErr)
		}
	}

	// Ensure parent dir exists (config may not have been created yet).
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	plugins := openCodePlugins(root)
	if !containsPluginPath(plugins, pluginJSPath) {
		plugins = append(plugins, pluginJSPath)
	}
	writePlugins(root, plugins)

	// Top-level v1 permission map: add one entry per delegation action, without
	// clobbering a value the user already set.
	perms, _ := root["permission"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
	}
	for action, effect := range backgroundAgentsPermissions {
		if _, covered := perms[action]; covered {
			continue
		}
		perms[action] = effect
	}
	root["permission"] = perms

	delete(root, "permissions")

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
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
