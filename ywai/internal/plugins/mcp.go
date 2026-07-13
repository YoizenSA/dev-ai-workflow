package plugins

import (
	"fmt"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// mcpConfigKey returns the top-level key for MCP servers based on agent format.
func mcpConfigKey(agentName string) string {
	switch agentName {
	case "claude-code", "pi":
		return "mcpServers"
	default:
		return "mcp"
	}
}

// hasLegacyDaemonToken reports whether argv (a JSON-decoded []any) contains
// the literal "daemon" used by pre-rename ywai versions. It is the detection
// signal for entries that need migration to the "serve --mcp-only" command.
func hasLegacyDaemonToken(argv []any) bool {
	for _, tok := range argv {
		if s, _ := tok.(string); s == "daemon" {
			return true
		}
	}
	return false
}

// migrateKanbanEntry rewrites entry in place if it holds the legacy
// "ywai daemon [--mcp]" argv. It returns true when a rewrite happened.
//
// Two shapes are supported:
//   - opencode format: command is a full []any argv.
//   - claude-code / pi format: command is the string "ywai" and args is the
//     []any argv. Only the argv is rewritten; the string command is left
//     untouched so we don't churn unrelated fields.
//
// Sibling keys inside the entry (type, enabled, url, ...) are not touched:
// only the argv-bearing field is replaced.
func migrateKanbanEntry(agentName string, entry map[string]any) bool {
	if agentName == "claude-code" || agentName == "pi" {
		args, ok := entry["args"].([]any)
		if !ok || !hasLegacyDaemonToken(args) {
			return false
		}
		entry["args"] = []any{"serve", "--mcp-only"}
		return true
	}
	cmd, ok := entry["command"].([]any)
	if !ok || !hasLegacyDaemonToken(cmd) {
		return false
	}
	entry["command"] = []any{"ywai", "serve", "--mcp-only"}
	return true
}

// InstallKanbanMCP adds the ywai-kanban MCP server to the agent's config file.
// This enables the Kanban board UI and tracking for orchestrator agents.
//
// If a previous install left a legacy "ywai daemon [--mcp]" entry, the existing
// argv is rewritten in place to the current "ywai serve --mcp-only" form so
// the MCP server starts cleanly after upgrade. Sibling MCP entries and
// unrelated top-level keys are preserved.
func InstallKanbanMCP(configPath, agentName string) error {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	key := mcpConfigKey(agentName)

	mcp, _ := root[key].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
	}

	existing, exists := mcp["ywai-kanban"]
	if exists {
		if entry, ok := existing.(map[string]any); ok {
			migrateKanbanEntry(agentName, entry)
		}
	} else if key == "mcpServers" {
		// Claude Code / pi format
		mcp["ywai-kanban"] = map[string]any{
			"command": "ywai",
			"args":    []any{"serve", "--mcp-only"},
		}
	} else {
		// opencode format
		mcp["ywai-kanban"] = map[string]any{
			"type":    "local",
			"command": []any{"ywai", "serve", "--mcp-only"},
			"enabled": true,
		}
	}
	root[key] = mcp

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", configPath, err)
	}

	return nil
}

// InstallFastfsMCP adds the ywai-fastfs MCP server (in-process file search/read
// with mtime cache). Prefer this over shelling out to rg/cat for exploration.
// Idempotent: does not overwrite an existing entry.
func InstallFastfsMCP(configPath, agentName string) error {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	key := mcpConfigKey(agentName)
	mcp, _ := root[key].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
	}
	if _, exists := mcp["ywai-fastfs"]; exists {
		root[key] = mcp
		return config.WriteJSONC(configPath, root)
	}

	if key == "mcpServers" {
		mcp["ywai-fastfs"] = map[string]any{
			"command": "ywai",
			"args":    []any{"mcp", "fastfs"},
		}
	} else {
		mcp["ywai-fastfs"] = map[string]any{
			"type":    "local",
			"command": []any{"ywai", "mcp", "fastfs"},
			"enabled": true,
		}
	}
	root[key] = mcp
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
		// Claude Code / pi format
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

// RemoveVisionMCP removes the legacy mcp-vision MCP server entry from the
// agent's config. Vision for text-only models is handled by the vision-bridge
// OpenCode plugin, not an MCP server.
func RemoveVisionMCP(configPath, agentName string) error {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	key := mcpConfigKey(agentName)
	mcp, _ := root[key].(map[string]any)
	if mcp == nil {
		return nil
	}
	if _, exists := mcp["mcp-vision"]; !exists {
		return nil
	}
	delete(mcp, "mcp-vision")
	root[key] = mcp

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", configPath, err)
	}
	return nil
}
