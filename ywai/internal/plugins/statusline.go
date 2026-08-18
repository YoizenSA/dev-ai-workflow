package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const subAgentStatuslinePlugin = "opencode-subagent-statusline"

func tuiConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", tuiConfigName)
}

func leftoverTuiConfigPath() string {
	return filepath.Join(filepath.Dir(tuiConfigPath()), legacyTuiConfigName)
}

func InstallSubAgentStatusline() error {
	path := tuiConfigPath()

	// Ensure the config directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating opencode config dir: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading cli.json: %w", err)
		}
		legacy, lerr := os.ReadFile(leftoverTuiConfigPath())
		if lerr == nil {
			data = legacy
		} else if os.IsNotExist(lerr) {
			root := map[string]any{
				"plugins": []any{subAgentStatuslinePlugin},
			}
			updated, mErr := json.MarshalIndent(root, "", "  ")
			if mErr != nil {
				return fmt.Errorf("marshaling cli.json: %w", mErr)
			}
			if wErr := os.WriteFile(path, append(updated, '\n'), 0o644); wErr != nil {
				return fmt.Errorf("writing cli.json: %w", wErr)
			}
			fmt.Printf("  Created cli.json with %s plugin\n", subAgentStatuslinePlugin)
			return nil
		} else {
			return fmt.Errorf("reading leftover tui.json: %w", lerr)
		}
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parsing cli.json: %w", err)
	}

	_, hadLegacy := root["plugin"]
	plugins := v2Plugins(root)
	already := containsPluginPath(plugins, subAgentStatuslinePlugin)
	if !already {
		plugins = append(plugins, subAgentStatuslinePlugin)
	}
	if already && !hadLegacy {
		fmt.Printf("  %s plugin already installed in cli.json\n", subAgentStatuslinePlugin)
		return nil
	}
	writePlugins(root, plugins)

	updated, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cli.json: %w", err)
	}

	if err := os.WriteFile(path, append(updated, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing cli.json: %w", err)
	}

	fmt.Printf("  Added %s plugin to cli.json\n", subAgentStatuslinePlugin)
	return nil
}
