package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// installVendoredV1 stages a plugin bundle under the ywai-owned ywai-plugins
// dir and points the config at it by absolute path. It is the shared OpenCode
// v1 half of every vendored plugin install; callers only pass their bundle.
func installVendoredV1(configPath, bundleSrc, bundleName string) error {
	destDir := filepath.Join(filepath.Dir(configPath), ywaiPluginsSubdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create plugins dir %s: %w", destDir, err)
	}

	destJS := filepath.Join(destDir, bundleName)
	if err := copyFile(bundleSrc, destJS); err != nil {
		return fmt.Errorf("copy %s bundle: %w", bundleName, err)
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

// sweepFlavorMarkers deletes the flavor marker files the old v1+v2 dual
// plugin wrote beside its bundles. The v2-only plugins never read them.
func sweepFlavorMarkers(configPath string) error {
	for _, markerDir := range []string{
		filepath.Join(filepath.Dir(configPath), autoDiscoveredPluginsSubdir),
		filepath.Join(filepath.Dir(configPath), ywaiPluginsSubdir),
	} {
		if err := os.Remove(filepath.Join(markerDir, FlavorMarkerName)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove stale flavor marker: %w", err)
		}
	}
	return nil
}

// patchOpenCodePluginPath appends pluginJSPath to the config "plugins" array
// idempotently (shared by every v1 vendored plugin install).
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
