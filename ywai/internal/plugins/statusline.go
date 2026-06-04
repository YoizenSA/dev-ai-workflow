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
	return filepath.Join(home, ".config", "opencode", "tui.json")
}

func InstallSubAgentStatusline() error {
	path := tuiConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading tui.json: %w", err)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parsing tui.json: %w", err)
	}

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

	for _, p := range plugins {
		if s, ok := p.(string); ok && s == subAgentStatuslinePlugin {
			fmt.Printf("  %s plugin already installed in tui.json\n", subAgentStatuslinePlugin)
			return nil
		}
	}

	root["plugin"] = append(plugins, subAgentStatuslinePlugin)

	updated, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling tui.json: %w", err)
	}

	if err := os.WriteFile(path, append(updated, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing tui.json: %w", err)
	}

	fmt.Printf("  Added %s plugin to tui.json\n", subAgentStatuslinePlugin)
	return nil
}
