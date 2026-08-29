package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const subAgentStatuslinePlugin = "opencode-subagent-statusline"

// tuiConfigPath is the client config the TUI reads. opencode renamed it from
// tui.json to cli.json, so the name comes from the shared constant rather than
// being hardcoded here.
func tuiConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "opencode", tuiConfigName)
}

// InstallSubAgentStatusline registers the sub-agent statusline TUI plugin,
// which surfaces delegation activity in the sidebar and footer. It works on
// opencode v1; it was only dropped for v2, and the install used to strip it on
// every run, quietly undoing the entry Engram's own installer had written.
func InstallSubAgentStatusline() error {
	path := tuiConfigPath()

	// Ensure the config directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating opencode config dir: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading %s: %w", tuiConfigName, err)
		}
		// tui.json does not exist yet — create it with the plugin.
		root := map[string]any{
			"plugin": []any{subAgentStatuslinePlugin},
		}
		updated, mErr := json.MarshalIndent(root, "", "  ")
		if mErr != nil {
			return fmt.Errorf("marshaling %s: %w", tuiConfigName, mErr)
		}
		if wErr := os.WriteFile(path, append(updated, '\n'), 0o644); wErr != nil {
			return fmt.Errorf("writing %s: %w", tuiConfigName, wErr)
		}
		fmt.Printf("  Created %s with %s plugin\n", tuiConfigName, subAgentStatuslinePlugin)
		return nil
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parsing %s: %w", tuiConfigName, err)
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
		return fmt.Errorf("marshaling %s: %w", tuiConfigName, err)
	}

	if err := os.WriteFile(path, append(updated, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", tuiConfigName, err)
	}

	fmt.Printf("  Added %s plugin to tui.json\n", subAgentStatuslinePlugin)
	return nil
}
