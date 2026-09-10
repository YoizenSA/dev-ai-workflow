package plugins

import (
	"fmt"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// InstallTuiStatusline vendors the ywai TUI statusline plugin next to the given
// opencode config and registers it in the TUI client config.
//
// It is the v2 replacement for the published opencode-subagent-statusline,
// whose peer range is "@opencode-ai/plugin >=1.14.50 <2" and therefore cannot
// load on OpenCode 2. On v1 the published package still works and is what
// InstallPublishedSubAgentStatusline registers, so this one installs only on
// v2 rather than putting two statuslines in the same slot.
//
// Superseded on v2 by InstallSubagentStatusline, which installs the full
// monitor and removes this minimal stand-in. Kept intentionally without
// callers pending the v1-only cleanup decision.
func InstallTuiStatusline(configPath string) error {
	if !agent.OpenCodeIsV2() {
		return nil
	}
	bundle, err := config.TuiStatuslineBundlePath()
	if err != nil {
		return err
	}
	return installTuiStatuslineWithBundle(configPath, bundle)
}

// installTuiStatuslineWithBundle vendors the statusline source at bundleSrc as
// a plugin directory alongside configPath and registers it. Split out so the
// copy + patch glue is unit testable without resolving the real bundle.
func installTuiStatuslineWithBundle(configPath, bundleSrc string) error {
	destDir, err := installTuiPluginDir(configPath, bundleSrc, TuiStatuslinePluginDir, config.TuiStatuslineBundleName)
	if err != nil {
		return fmt.Errorf("install tui statusline: %w", err)
	}

	tuiConfig := filepath.Join(filepath.Dir(configPath), tuiConfigName)
	return patchTuiPlugin(tuiConfig, destDir, false)
}
