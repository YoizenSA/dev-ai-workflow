package plugins

import (
	"fmt"
	"os"
	"path/filepath"

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

// InstallBackgroundAgents vendors the background-agents plugin bundle next to
// the given opencode config and wires it into the config (plugin array +
// delegation permissions). configPath is the path to opencode.json(c).
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
func installBackgroundAgentsWithBundle(configPath, bundleSrc string) error {
	destDir := filepath.Join(filepath.Dir(configPath), ywaiPluginsSubdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create plugins dir %s: %w", destDir, err)
	}

	destJS := filepath.Join(destDir, config.BackgroundAgentsBundleName)
	if err := copyFile(bundleSrc, destJS); err != nil {
		return fmt.Errorf("copy plugin bundle: %w", err)
	}

	return patchOpenCodeBackgroundAgents(configPath, destJS)
}

// patchOpenCodeBackgroundAgents adds pluginJSPath to the config's "plugin"
// array (idempotently) and merges the delegation permissions into the
// "permission" block, preserving any existing entries. It is safe to call
// repeatedly.
func patchOpenCodeBackgroundAgents(configPath, pluginJSPath string) error {
	root := map[string]any{}
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

	// plugin array — append the bundle path if not already present.
	plugins, _ := root["plugin"].([]any)
	if !containsPluginPath(plugins, pluginJSPath) {
		plugins = append(plugins, pluginJSPath)
	}
	root["plugin"] = plugins

	// permission block — merge our keys without clobbering existing ones.
	perms, ok := root["permission"].(map[string]any)
	if !ok {
		perms = map[string]any{}
	}
	for key, val := range backgroundAgentsPermissions {
		perms[key] = val
	}
	root["permission"] = perms

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
	}
	return false
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
