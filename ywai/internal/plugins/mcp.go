package plugins

import (
	"encoding/json"
	"fmt"
	"os"
)

// InstallMicrosoftLearnMCP adds the Microsoft Learn MCP server to opencode.json
func InstallMicrosoftLearnMCP(configPath string) error {
	// Read opencode.json
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read opencode.json: %w", err)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("failed to parse opencode.json: %w", err)
	}

	// Get or create mcp object
	mcpRaw, ok := root["mcp"]
	if !ok {
		mcpRaw = map[string]any{}
		root["mcp"] = mcpRaw
	}

	mcp, ok := mcpRaw.(map[string]any)
	if !ok {
		mcp = map[string]any{}
		root["mcp"] = mcp
	}

	// Add microsoft-learn MCP server if not already present
	if _, exists := mcp["microsoft-learn"]; !exists {
		mcp["microsoft-learn"] = map[string]any{
			"type":    "remote",
			"url":     "https://learn.microsoft.com/api/mcp",
			"enabled": true,
		}
		root["mcp"] = mcp
	}

	// Write updated opencode.json
	updated, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal opencode.json: %w", err)
	}
	updated = append(updated, '\n')

	if err := os.WriteFile(configPath, updated, 0o644); err != nil {
		return fmt.Errorf("failed to write opencode.json: %w", err)
	}

	return nil
}
