package plugins

import (
	"fmt"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// BackgroundAgentsNotifyPluginDir is the directory the notification sidecar is
// installed under: the v2 loader resolves the TUI entry (<dir>/tui) against a
// directory, so this is what lands in cli.json's plugins array.
const BackgroundAgentsNotifyPluginDir = "background-agents-notify"

// InstallBackgroundAgentsNotify vendors the notification sidecar next to the
// opencode config and registers it in cli.json. It renders nothing (event
// subscriber only), so unlike the logo it needs no mouse capture.
func InstallBackgroundAgentsNotify(configPath string) error {
	bundle, err := config.BackgroundAgentsNotifyBundlePath()
	if err != nil {
		return err
	}
	return installBackgroundAgentsNotifyWithBundle(configPath, bundle)
}

// installBackgroundAgentsNotifyWithBundle is InstallBackgroundAgentsNotify
// with the bundle path injected, so the copy+patch glue is unit testable.
func installBackgroundAgentsNotifyWithBundle(configPath, bundleSrc string) error {
	destDir, err := installTuiPluginDir(configPath, bundleSrc, BackgroundAgentsNotifyPluginDir, config.BackgroundAgentsNotifyBundleName)
	if err != nil {
		return fmt.Errorf("install background-agents notify sidecar: %w", err)
	}

	tuiConfig := filepath.Join(filepath.Dir(configPath), tuiConfigName)
	return patchTuiPlugin(tuiConfig, destDir, false)
}
