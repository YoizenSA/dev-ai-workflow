package plugins

import (
	"fmt"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// mcpConfigKey returns the top-level key for MCP servers based on agent format.
func mcpConfigKey(agentName string) string {
	switch agentName {
	case "claude-code":
		return "mcpServers"
	default:
		return "mcp"
	}
}

// InstallKanbanMCP adds the ywai-kanban MCP server to the agent's config file.
// This enables the Kanban board UI and tracking for orchestrator agents.
func InstallKanbanMCP(configPath, agentName string) error {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	key := mcpConfigKey(agentName)

	if key == "mcpServers" {
		// Claude Code format
		mcp, _ := root[key].(map[string]any)
		if mcp == nil {
			mcp = map[string]any{}
			root[key] = mcp
		}
		if _, exists := mcp["ywai-kanban"]; !exists {
			mcp["ywai-kanban"] = map[string]any{
				"command": "ywai",
				"args":    []any{"serve", "--mcp-only"},
			}
			root[key] = mcp
		}
	} else {
		// opencode format
		mcp, _ := root[key].(map[string]any)
		if mcp == nil {
			mcp = map[string]any{}
			root[key] = mcp
		}
		if _, exists := mcp["ywai-kanban"]; !exists {
			mcp["ywai-kanban"] = map[string]any{
				"type":    "local",
				"command": []any{"ywai", "serve", "--mcp-only"},
				"enabled": true,
			}
			root[key] = mcp
		}
	}

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", configPath, err)
	}

	return nil
}

// InstallMicrosoftLearnMCP adds the Microsoft Learn MCP server to the agent's config file.
func InstallMicrosoftLearnMCP(configPath, agentName string) error {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	key := mcpConfigKey(agentName)

	if key == "mcpServers" {
		// Claude Code format
		mcp, _ := root[key].(map[string]any)
		if mcp == nil {
			mcp = map[string]any{}
			root[key] = mcp
		}
		if _, exists := mcp["microsoft-learn"]; !exists {
			mcp["microsoft-learn"] = map[string]any{
				"command": "npx",
				"args":    []any{"@anthropic/mcp-server-microsoft-learn"},
			}
			root[key] = mcp
		}
	} else {
		// opencode format
		mcp, _ := root[key].(map[string]any)
		if mcp == nil {
			mcp = map[string]any{}
			root[key] = mcp
		}
		if _, exists := mcp["microsoft-learn"]; !exists {
			mcp["microsoft-learn"] = map[string]any{
				"type":    "remote",
				"url":     "https://learn.microsoft.com/api/mcp",
				"enabled": true,
			}
			root[key] = mcp
		}
	}

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", configPath, err)
	}

	return nil
}
