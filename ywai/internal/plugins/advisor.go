package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// InstallAdvisor vendors the advisor opencode plugin and registers it in the
// config's "plugin" array.
//
// The bundle is installed unconditionally; the plugin itself is inert until
// both advisor_enabled and advisor_model are set in ~/.ywai/config.json. That
// keeps enabling it a config change rather than a reinstall, and means a
// disabled advisor registers no hooks at all.
func InstallAdvisor(configPath string) error {
	bundle, err := config.AdvisorBundlePath()
	if err != nil {
		return err
	}
	return installAdvisorWithBundle(configPath, bundle)
}

func installAdvisorWithBundle(configPath, bundleSrc string) error {
	destDir := filepath.Join(filepath.Dir(configPath), ywaiPluginsSubdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create plugins dir %s: %w", destDir, err)
	}

	destJS := filepath.Join(destDir, config.AdvisorBundleName)
	if err := copyFile(bundleSrc, destJS); err != nil {
		return fmt.Errorf("copy advisor bundle: %w", err)
	}

	return patchOpenCodePluginPath(configPath, destJS)
}
