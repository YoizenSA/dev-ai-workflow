package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// RemoveQuota removes the opencode-quota plugin from opencode.json and deletes the quota directory
func RemoveQuota(configPath string) error {
	// Remove plugin from opencode.json
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	_, hadPlugins := root["plugins"]
	_, hadLegacy := root["plugin"]
	if !hadPlugins && !hadLegacy {
		return nil
	}

	plugins := v2Plugins(root)
	quotaPlugin := "@slkiser/opencode-quota"
	filteredPlugins := []any{}
	for _, p := range plugins {
		if pStr, ok := p.(string); ok && pStr == quotaPlugin {
			continue
		}
		if m, ok := p.(map[string]any); ok {
			if pkg, _ := m["package"].(string); pkg == quotaPlugin {
				continue
			}
		}
		filteredPlugins = append(filteredPlugins, p)
	}

	if len(filteredPlugins) != len(plugins) || hadLegacy {
		writePlugins(root, filteredPlugins)
		if err := config.WriteJSONC(configPath, root); err != nil {
			return fmt.Errorf("failed to write %s: %w", configPath, err)
		}
	}

	// Delete the quota directory
	configDir := filepath.Dir(configPath)
	quotaDir := filepath.Join(configDir, "opencode-quota")
	if _, err := os.Stat(quotaDir); err == nil {
		if err := os.RemoveAll(quotaDir); err != nil {
			return fmt.Errorf("failed to remove opencode-quota directory: %w", err)
		}
	}

	return nil
}
