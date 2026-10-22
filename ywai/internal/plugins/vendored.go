package plugins

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// installVendorPluginV2 vendors a plugin bundle into ~/.config/opencode/plugins/
// where OpenCode auto-discovers plain .js files. Any stale explicit entry is
// purged from the config.
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
	for _, raw := range pluginArray(root) {
		if s, ok := raw.(string); ok && strings.Contains(s, bundleName) {
			continue
		}
		kept = append(kept, raw)
	}
	writePluginArray(root, kept)
	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	if err := os.Remove(filepath.Join(filepath.Dir(configPath), ywaiPluginsSubdir, bundleName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
