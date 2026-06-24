package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// tuiPluginsSubdir is opencode's auto-discovered directory for TUI plugins,
// resolved next to the opencode config. Unlike regular opencode plugins, files
// here are loaded automatically — so installing the logo is a pure file copy
// with no config patching.
const tuiPluginsSubdir = "tui-plugins"

// InstallTuiLogo vendors the ywai TUI logo plugin into the tui-plugins dir
// alongside the given opencode config. configPath is the path to
// opencode.json(c).
func InstallTuiLogo(configPath string) error {
	bundle, err := config.TuiLogoBundlePath()
	if err != nil {
		return err
	}
	return installTuiLogoWithBundle(configPath, bundle)
}

// installTuiLogoWithBundle copies the logo source at bundleSrc into the
// tui-plugins dir alongside configPath. Split out from InstallTuiLogo so the
// copy glue is unit testable without resolving the real embedded/source bundle.
func installTuiLogoWithBundle(configPath, bundleSrc string) error {
	destDir := filepath.Join(filepath.Dir(configPath), tuiPluginsSubdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create tui-plugins dir %s: %w", destDir, err)
	}

	destTSX := filepath.Join(destDir, config.TuiLogoBundleName)
	if err := copyFile(bundleSrc, destTSX); err != nil {
		return fmt.Errorf("copy tui logo: %w", err)
	}
	return nil
}
