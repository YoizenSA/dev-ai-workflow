package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// InstallVisionBridge vendors the vision-bridge opencode plugin and registers it
// in the config's "plugin" array. The plugin auto-analyzes attached images via
// TokenBank vision models when the active chat model does not support image input.
func InstallVisionBridge(configPath string) error {
	bundle, err := config.VisionBridgeBundlePath()
	if err != nil {
		return err
	}
	return installVisionBridgeWithBundle(configPath, bundle)
}

func installVisionBridgeWithBundle(configPath, bundleSrc string) error {
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

// patchOpenCodePluginPath appends pluginJSPath to the config "plugin" array
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

	plugins, _ := root["plugin"].([]any)
	if !containsPluginPath(plugins, pluginJSPath) {
		plugins = append(plugins, pluginJSPath)
	}
	root["plugin"] = plugins

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
}
