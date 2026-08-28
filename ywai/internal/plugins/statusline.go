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

// RemoveSubAgentStatusline removes the retired, OpenCode v2-incompatible TUI
// plugin from both current and legacy global TUI configuration files.
func RemoveSubAgentStatusline() error {
	for _, path := range []string{tuiConfigPath(), leftoverTuiConfigPath()} {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("reading %s: %w", filepath.Base(path), err)
		}
		var root map[string]any
		if err := json.Unmarshal(data, &root); err != nil {
			return fmt.Errorf("parsing %s: %w", filepath.Base(path), err)
		}
		plugins := v2Plugins(root)
		filtered := make([]any, 0, len(plugins))
		for _, plugin := range plugins {
			if name, ok := plugin.(string); ok && name == subAgentStatuslinePlugin {
				continue
			}
			filtered = append(filtered, plugin)
		}
		writePlugins(root, filtered)
		updated, err := json.MarshalIndent(root, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling %s: %w", filepath.Base(path), err)
		}
		if err := os.WriteFile(path, append(updated, '\n'), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}
