package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// tuiEntryName is the filename the loader resolves inside a plugin
// directory to find its TUI half. Must stay in sync with the loader's
// candidate list ("tui"); the extension is resolved by the host.
const tuiEntryName = "tui.tsx"

// tuiConfigName is OpenCode's client config. TUI plugins are listed by
// absolute path in the "plugins" array.
const tuiConfigName = "cli.json"

// InstallTuiLogo vendors the ywai TUI logo plugin next to the given opencode
// config and registers it in cli.json (plugins array + mouse capture, required
// by the click easter eggs). configPath is the path to opencode.json(c).
func InstallTuiLogo(configPath string) error {
	return installVendorTUI(configPath, ManifestEntry{
		ID: "tui-logo", Bundle: "tui-logo", Dir: TuiLogoPluginDir, Mouse: true,
	})
}

// installTuiLogoWithBundle vendors the logo source at bundleSrc as a plugin
// directory alongside configPath and patches cli.json to reference it. Split
// out from InstallTuiLogo so the copy + patch glue is unit testable without
// resolving the real embedded/source bundle.
func installTuiLogoWithBundle(configPath, bundleSrc string) error {
	destDir, err := installTuiPluginDir(configPath, bundleSrc, TuiLogoPluginDir)
	if err != nil {
		return fmt.Errorf("install tui logo: %w", err)
	}

	tuiConfig := filepath.Join(filepath.Dir(configPath), tuiConfigName)
	return patchTuiLogo(tuiConfig, destDir)
}

// TuiLogoPluginDir is the plugin directory name the logo is installed under.
const TuiLogoPluginDir = "ywai-logo"

// SubagentStatuslineTuiPluginDir is the plugin directory name the vendored
// sub-agent statusline TUI half is installed under.
const SubagentStatuslineTuiPluginDir = "subagent-statusline"

// installTuiPluginDir vendors a TUI plugin source as <config>/plugins/<dir>/tui.tsx
// and returns the plugin directory, which is what the TUI client config must
// reference. The loader resolves the TUI entry against a directory, so the
// directory — not the file — is the unit a plugin entry names.
func installTuiPluginDir(configPath, bundleSrc, pluginDir string) (string, error) {
	cfgDir := filepath.Dir(configPath)

	destDir := filepath.Join(cfgDir, autoDiscoveredPluginsSubdir, pluginDir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create plugin dir %s: %w", destDir, err)
	}
	if err := copyFile(bundleSrc, filepath.Join(destDir, tuiEntryName)); err != nil {
		return "", fmt.Errorf("copy tui entry: %w", err)
	}

	return destDir, nil
}

// patchTuiLogo registers pluginPath in cli.json's "plugins" array (idempotently)
// and enables mouse capture so the logo click easter eggs work. It preserves
// any existing plugin entries — including the array-form entries opencode uses
// for parameterized plugins. Safe to call repeatedly.
func patchTuiLogo(tuiConfigPath, pluginPath string) error {
	// Mouse capture is what makes the logo's click easter eggs reachable; other
	// TUI plugins register through patchTuiPlugin without it.
	return patchTuiPlugin(tuiConfigPath, pluginPath, true)
}

// patchTuiPlugin registers pluginPath in the TUI client config's plugin array
// (idempotently), preserving existing entries including the array-form ones
// opencode uses for parameterized plugins. Safe to call repeatedly.
func patchTuiPlugin(tuiConfigPath, pluginPath string, enableMouse bool) error {
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

	plugins := pluginArray(root)
	kept := plugins[:0]
	for _, raw := range plugins {
		if s, ok := raw.(string); ok && s == subAgentStatuslinePlugin {
			continue
		}
		kept = append(kept, raw)
	}
	plugins = kept
	if !containsPluginPath(plugins, pluginPath) {
		plugins = append(plugins, pluginPath)
	}
	writePluginArray(root, plugins)

	// Enable mouse capture only for plugins that need it, and only when the
	// user has not explicitly opted out (no existing key).
	if enableMouse {
		if _, ok := root["mouse"]; !ok {
			root["mouse"] = true
		}
	}

	if err := config.WriteJSONC(tuiConfigPath, root); err != nil {
		return fmt.Errorf("write %s: %w", tuiConfigPath, err)
	}
	return nil
}
