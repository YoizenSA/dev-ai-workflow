package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// InstallQuota adds the opencode-quota plugin to opencode.json and creates quota-toast.json
// with percentDisplayMode set to "used"
func InstallQuota(configPath string) error {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	// Add plugin to plugin array if not already present
	pluginsRaw, ok := root["plugin"]
	if !ok {
		pluginsRaw = []any{}
		root["plugin"] = pluginsRaw
	}

	plugins, ok := pluginsRaw.([]any)
	if !ok {
		plugins = []any{}
		root["plugin"] = plugins
	}

	quotaPlugin := "@slkiser/opencode-quota"
	alreadyInstalled := false
	for _, p := range plugins {
		if pStr, ok := p.(string); ok && pStr == quotaPlugin {
			alreadyInstalled = true
			break
		}
	}

	if !alreadyInstalled {
		plugins = append(plugins, quotaPlugin)
		root["plugin"] = plugins
	}

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", configPath, err)
	}

	// Create opencode-quota directory and quota-toast.json
	configDir := filepath.Dir(configPath)
	quotaDir := filepath.Join(configDir, "opencode-quota")
	if err := os.MkdirAll(quotaDir, 0o755); err != nil {
		return fmt.Errorf("failed to create opencode-quota directory: %w", err)
	}

	quotaConfigPath := filepath.Join(quotaDir, "quota-toast.json")
	quotaConfig := map[string]any{
		"percentDisplayMode": "used",
	}

	quotaData, err := json.MarshalIndent(quotaConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal quota-toast.json: %w", err)
	}
	quotaData = append(quotaData, '\n')

	if err := os.WriteFile(quotaConfigPath, quotaData, 0o644); err != nil {
		return fmt.Errorf("failed to write quota-toast.json: %w", err)
	}

	return nil
}
