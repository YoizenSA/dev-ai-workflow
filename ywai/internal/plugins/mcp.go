package plugins

import (
	"fmt"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// InstallKanbanMCP adds the ywai-kanban MCP server to opencode.json.
// This enables the Kanban board UI and tracking for orchestrator agents.
func InstallKanbanMCP(configPath string) error {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
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

	// Add ywai-kanban MCP server if not already present
	if _, exists := mcp["ywai-kanban"]; !exists {
		mcp["ywai-kanban"] = map[string]any{
			"type":    "local",
			"command": []string{"ywai", "daemon", "--mcp"},
			"enabled": true,
		}
		root["mcp"] = mcp
	}

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", configPath, err)
	}

	return nil
}

// InstallMicrosoftLearnMCP adds the Microsoft Learn MCP server to opencode.json
func InstallMicrosoftLearnMCP(configPath string) error {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
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

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", configPath, err)
	}

	return nil
}
