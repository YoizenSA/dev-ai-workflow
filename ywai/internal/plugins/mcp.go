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

// RemoveRetiredMCPs deletes every retired ywai MCP server entry from the
// agent's config, preserving sibling MCP entries and unrelated top-level keys.
// It reports which ids it removed so callers can tell the user what changed.
func RemoveRetiredMCPs(configPath, agentName string) ([]string, error) {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	key := mcpConfigKey(agentName)
	mcp, _ := root[key].(map[string]any)
	if mcp == nil {
		return nil, nil
	}

	var removed []string
	if key == "mcp" {
		servers := collectOpenCodeServers(mcp)
		for _, id := range config.RetiredMCPServers {
			if _, exists := servers[id]; exists {
				delete(servers, id)
				removed = append(removed, id)
			}
		}
		if len(removed) == 0 {
			return nil, nil
		}
		root[key] = nestOpenCodeMCP(mcp, servers)
	} else {
		for _, id := range config.RetiredMCPServers {
			if _, exists := mcp[id]; exists {
				delete(mcp, id)
				removed = append(removed, id)
			}
		}
		if len(removed) == 0 {
			return nil, nil
		}
		root[key] = mcp
	}

	if err := config.WriteJSONC(configPath, root); err != nil {
		return nil, fmt.Errorf("failed to write %s: %w", configPath, err)
	}
	return removed, nil
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
		// OpenCode v2: mcp.servers.<id> (type/url; no "enabled").
		mcp, _ := root[key].(map[string]any)
		if mcp == nil {
			mcp = map[string]any{}
		}
		servers := collectOpenCodeServers(mcp)
		if _, exists := servers["microsoft-learn"]; !exists {
			servers["microsoft-learn"] = map[string]any{
				"type": "remote",
				"url":  "https://learn.microsoft.com/api/mcp",
			}
		}
		root[key] = nestOpenCodeMCP(mcp, servers)
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
	if key == "mcp" {
		servers := collectOpenCodeServers(mcp)
		if _, exists := servers["mcp-vision"]; !exists {
			return nil
		}
		delete(servers, "mcp-vision")
		root[key] = nestOpenCodeMCP(mcp, servers)
	} else {
		if _, exists := mcp["mcp-vision"]; !exists {
			return nil
		}
		delete(mcp, "mcp-vision")
		root[key] = mcp
	}

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", configPath, err)
	}
	return nil
}

func openCodeReservedMCPKey(k string) bool {
	return k == "servers" || k == "timeout"
}

func collectOpenCodeServers(mcp map[string]any) map[string]any {
	out := map[string]any{}
	if mcp == nil {
		return out
	}
	if nested, ok := mcp["servers"].(map[string]any); ok {
		for k, v := range nested {
			out[k] = v
		}
	}
	for k, v := range mcp {
		if openCodeReservedMCPKey(k) {
			continue
		}
		if _, ok := v.(map[string]any); ok {
			if _, exists := out[k]; !exists {
				out[k] = v
			}
		}
	}
	return out
}

func nestOpenCodeMCP(mcp map[string]any, servers map[string]any) map[string]any {
	clean := map[string]any{}
	for id, raw := range servers {
		entry, ok := raw.(map[string]any)
		if !ok {
			clean[id] = raw
			continue
		}
		next := make(map[string]any, len(entry))
		for k, v := range entry {
			next[k] = v
		}
		if enabled, ok := next["enabled"].(bool); ok {
			delete(next, "enabled")
			if !enabled {
				next["disabled"] = true
			}
		}
		clean[id] = next
	}
	out := map[string]any{"servers": clean}
	if mcp == nil {
		return out
	}
	for k, v := range mcp {
		if k == "servers" {
			continue
		}
		if _, isObj := v.(map[string]any); isObj && !openCodeReservedMCPKey(k) {
			continue
		}
		out[k] = v
	}
	return out
}
