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

// InstallVisionBridge vendors the vision-bridge opencode plugin and registers it
// in the config's "plugins" array. The plugin auto-analyzes attached images via
// TokenBank vision models when the active chat model does not support image input.
func InstallVisionBridge(configPath string) error {
	bundle, err := config.VisionBridgeBundlePath()
	if err != nil {
		return err
	}
	return installVisionBridgeWithBundle(configPath, bundle)
}

func installVisionBridgeWithBundle(configPath, bundleSrc string) error {
	if agent.OpenCodeIsV2() {
		return installVendorPluginV2(configPath, bundleSrc, config.VisionBridgeBundleName)
	}

	destDir := filepath.Join(filepath.Dir(configPath), ywaiPluginsSubdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create plugins dir %s: %w", destDir, err)
	}

	destJS := filepath.Join(destDir, config.VisionBridgeBundleName)
	if err := copyFile(bundleSrc, destJS); err != nil {
		return fmt.Errorf("copy vision-bridge bundle: %w", err)
	}

	return patchOpenCodePluginPath(configPath, destJS)
}

// installVendorPluginV2 vendors a plugin bundle into ~/.config/opencode/plugins/
// where OpenCode v2 auto-discovers plain .js files without needing an entry in
// the plugins array (which only accepts directories in v2). Any stale explicit
// path entry from a v1 install is purged from the config.
func installVendorPluginV2(configPath, bundleSrc, bundleName string) error {
	destDir := filepath.Join(filepath.Dir(configPath), autoDiscoveredPluginsSubdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create plugins dir %s: %w", destDir, err)
	}

	destJS := filepath.Join(destDir, bundleName)
	if err := copyFile(bundleSrc, destJS); err != nil {
		return fmt.Errorf("copy plugin bundle: %w", err)
	}

	root, err := config.ReadJSONC(configPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", configPath, err)
	}
	kept := make([]any, 0)
	for _, raw := range openCodePlugins(root) {
		if s, ok := raw.(string); ok && strings.Contains(s, bundleName) {
			continue
		}
		kept = append(kept, raw)
	}
	writePlugins(root, kept)
	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	if err := os.Remove(filepath.Join(filepath.Dir(configPath), ywaiPluginsSubdir, bundleName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// patchOpenCodePluginPath appends pluginJSPath to the config "plugins" array
// idempotently (shared by vision-bridge and reusable for other local plugins).
func patchOpenCodePluginPath(configPath, pluginJSPath string) error {
	root := map[string]any{}
	if _, err := os.Stat(configPath); err == nil {
		var readErr error
		root, readErr = config.ReadJSONC(configPath)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", configPath, readErr)
		}
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	plugins := openCodePlugins(root)
	if !containsPluginPath(plugins, pluginJSPath) {
		plugins = append(plugins, pluginJSPath)
	}
	writePlugins(root, plugins)

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
}
