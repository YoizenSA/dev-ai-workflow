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

// tuiPluginsSubdir is the legacy directory next to the opencode config where
// ywai used to vendor TUI plugin sources as loose .tsx files. The v2 loader
// resolves a plugin's TUI entry as <directory>/tui, so a loose file resolves
// to "<file>.tsx/tui", fails with ENOTDIR, and is swallowed: the plugin is
// dropped with no log line and no error. Kept only so installs made under the
// old layout can be cleaned up.
const tuiPluginsSubdir = "tui-plugins"

// tuiEntryName is the filename the v2 loader resolves inside a plugin
// directory to find its TUI half. Must stay in sync with the loader's
// candidate list ("tui"); the extension is resolved by the host.
const tuiEntryName = "tui.tsx"

// tuiConfigName is OpenCode 2's client config (was tui.json in v1).
// TUI plugins must be listed by absolute path in the "plugins" array.
const tuiConfigName = "cli.json"
const legacyTuiConfigName = "tui.json"

// InstallTuiLogo vendors the ywai TUI logo plugin next to the given opencode
// config and registers it in cli.json (plugins array + mouse capture, required
// by the click easter eggs). configPath is the path to opencode.json(c).
func InstallTuiLogo(configPath string) error {
	bundle, err := config.TuiLogoBundlePath()
	if err != nil {
		return err
	}
	return installTuiLogoWithBundle(configPath, bundle)
}

// installTuiLogoWithBundle vendors the logo source at bundleSrc as a plugin
// directory alongside configPath and patches cli.json to reference it. Split
// out from InstallTuiLogo so the copy + patch glue is unit testable without
// resolving the real embedded/source bundle.
func installTuiLogoWithBundle(configPath, bundleSrc string) error {
	destDir, err := installTuiPluginDir(configPath, bundleSrc, TuiLogoPluginDir, config.TuiLogoBundleName)
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
// reference. The v2 loader resolves the TUI entry against a directory, so the
// directory — not the file — is the unit a plugin entry names.
//
// It also removes the loose legacy file the old tui-plugins/ layout left
// behind, so an upgrade does not leave a second dead copy on disk.
func installTuiPluginDir(configPath, bundleSrc, pluginDir, legacyName string) (string, error) {
	cfgDir := filepath.Dir(configPath)

	destDir := filepath.Join(cfgDir, autoDiscoveredPluginsSubdir, pluginDir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create plugin dir %s: %w", destDir, err)
	}
	if err := copyFile(bundleSrc, filepath.Join(destDir, tuiEntryName)); err != nil {
		return "", fmt.Errorf("copy tui entry: %w", err)
	}

	legacy := filepath.Join(cfgDir, tuiPluginsSubdir, legacyName)
	if err := os.Remove(legacy); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("remove legacy %s: %w", legacy, err)
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
	src := tuiConfigPath
	if _, err := os.Stat(tuiConfigPath); err != nil {
		legacy := filepath.Join(filepath.Dir(tuiConfigPath), legacyTuiConfigName)
		if _, lerr := os.Stat(legacy); lerr == nil {
			src = legacy
		} else {
			src = ""
		}
	}
	if src != "" {
		var readErr error
		root, readErr = config.ReadJSONC(src)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", src, readErr)
		}
	}

	if err := os.MkdirAll(filepath.Dir(tuiConfigPath), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	plugins := openCodePlugins(root)
	if agent.OpenCodeIsV2() {
		// The write below moves this array to the "plugins" key v2 reads, so a
		// v1-only entry that was inert until now would start being loaded.
		kept := plugins[:0]
		for _, raw := range plugins {
			if s, ok := raw.(string); ok && s == subAgentStatuslinePlugin {
				continue
			}
			kept = append(kept, raw)
		}
		plugins = kept
	}
	// Entries under the legacy tui-plugins/ dir name a loose .tsx the v2
	// loader silently discards. Drop them, or the config keeps pointing at a
	// file that was just deleted by installTuiPluginDir.
	plugins = dropLegacyTuiPluginEntries(plugins)
	if !containsPluginPath(plugins, pluginPath) {
		plugins = append(plugins, pluginPath)
	}
	writePlugins(root, plugins)

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

// dropLegacyTuiPluginEntries removes plugin entries that point inside the
// legacy tui-plugins/ dir. Both the plain-string and the {"package": …} shapes
// are dropped; unrelated entries are preserved in order.
func dropLegacyTuiPluginEntries(plugins []any) []any {
	legacyDir := string(os.PathSeparator) + tuiPluginsSubdir + string(os.PathSeparator)
	isLegacy := func(s string) bool {
		return strings.Contains(filepath.ToSlash(s), "/"+tuiPluginsSubdir+"/") ||
			strings.Contains(s, legacyDir)
	}

	kept := make([]any, 0, len(plugins))
	for _, raw := range plugins {
		switch v := raw.(type) {
		case string:
			if isLegacy(v) {
				continue
			}
		case map[string]any:
			if s, ok := v["package"].(string); ok && isLegacy(s) {
				continue
			}
			if s, ok := v["path"].(string); ok && isLegacy(s) {
				continue
			}
		}
		kept = append(kept, raw)
	}
	return kept
}
