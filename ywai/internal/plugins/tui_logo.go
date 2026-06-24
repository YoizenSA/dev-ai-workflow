package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// tuiPluginsSubdir is the directory next to the opencode config where ywai
// vendors the logo plugin source.
const tuiPluginsSubdir = "tui-plugins"

// tuiConfigName is opencode's TUI config file. TUI plugins are NOT
// auto-discovered from a directory — they must be listed by absolute path in
// the "plugin" array of this file, which lives next to opencode.json.
const tuiConfigName = "tui.json"

// InstallTuiLogo vendors the ywai TUI logo plugin next to the given opencode
// config and registers it in tui.json (plugin array + mouse capture, required
// by the click easter eggs). configPath is the path to opencode.json(c).
func InstallTuiLogo(configPath string) error {
	bundle, err := config.TuiLogoBundlePath()
	if err != nil {
		return err
	}
	return installTuiLogoWithBundle(configPath, bundle)
}

// installTuiLogoWithBundle copies the logo source at bundleSrc into the
// tui-plugins dir alongside configPath and patches tui.json to reference it.
// Split out from InstallTuiLogo so the copy + patch glue is unit testable
// without resolving the real embedded/source bundle.
func installTuiLogoWithBundle(configPath, bundleSrc string) error {
	destDir := filepath.Join(filepath.Dir(configPath), tuiPluginsSubdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create tui-plugins dir %s: %w", destDir, err)
	}

	destTSX := filepath.Join(destDir, config.TuiLogoBundleName)
	if err := copyFile(bundleSrc, destTSX); err != nil {
		return fmt.Errorf("copy tui logo: %w", err)
	}

	tuiConfig := filepath.Join(filepath.Dir(configPath), tuiConfigName)
	return patchTuiLogo(tuiConfig, destTSX)
}

// patchTuiLogo registers pluginPath in tui.json's "plugin" array (idempotently)
// and enables mouse capture so the logo click easter eggs work. It preserves
// any existing plugin entries — including the array-form entries opencode uses
// for parameterized plugins. Safe to call repeatedly.
func patchTuiLogo(tuiConfigPath, pluginPath string) error {
	root := map[string]any{}
	if _, err := os.Stat(tuiConfigPath); err == nil {
		var readErr error
		root, readErr = config.ReadJSONC(tuiConfigPath)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", tuiConfigPath, readErr)
		}
	}

	if err := os.MkdirAll(filepath.Dir(tuiConfigPath), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// plugin array — append the logo path if not already present.
	plugins, _ := root["plugin"].([]any)
	if !containsPluginPath(plugins, pluginPath) {
		plugins = append(plugins, pluginPath)
	}
	root["plugin"] = plugins

	// Mouse capture is required for the click easter eggs; enable it only when
	// the user has not explicitly opted out (no existing key).
	if _, ok := root["mouse"]; !ok {
		root["mouse"] = true
	}

	if err := config.WriteJSONC(tuiConfigPath, root); err != nil {
		return fmt.Errorf("write %s: %w", tuiConfigPath, err)
	}
	return nil
}
