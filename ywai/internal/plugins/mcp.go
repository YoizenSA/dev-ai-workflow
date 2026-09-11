package plugins

import (
	"fmt"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	mcppkg "github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
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
		servers := mcppkg.CollectOpenCodeServers(mcp)
		for _, id := range config.RetiredMCPServers {
			if _, exists := servers[id]; exists {
				delete(servers, id)
				removed = append(removed, id)
			}
		}
		if len(removed) == 0 {
			return nil, nil
		}
		root[key] = mcppkg.WriteOpenCodeMCP(mcp, servers)
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

// installMCPEntry merges one server definition into the agent's config,
// writing the shape that format reads. buildEntry receives the config key
// ("mcpServers" for claude-code/pi, "mcp" for opencode) so one server can
// carry a per-format transport. An existing entry always wins: a user's own
// settings are never overwritten by a re-install.
func installMCPEntry(configPath, agentName, id string, buildEntry func(configKey string) map[string]any) error {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	key := mcpConfigKey(agentName)
	entry := buildEntry(key)
	mcp, _ := root[key].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
	}

	if key == "mcpServers" {
		if _, exists := mcp[id]; !exists {
			mcp[id] = entry
		}
		root[key] = mcp
	} else {
		servers := mcppkg.CollectOpenCodeServers(mcp)
		if _, exists := servers[id]; !exists {
			servers[id] = entry
		}
		root[key] = mcppkg.WriteOpenCodeMCP(mcp, servers)
	}

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("failed to write %s: %w", configPath, err)
	}
	return nil
}

// remoteEntry builds a remote-server entry in the spelling each config format
// reads: claude-code/pi say "http", opencode says "remote". Disabled writes
// the format's own off flag so an endpoint-less entry is never probed.
func remoteEntry(configKey, url string, disabled bool) map[string]any {
	if configKey == "mcpServers" {
		entry := map[string]any{"type": "http", "url": url}
		if disabled {
			entry["disabled"] = true
		}
		return entry
	}
	entry := map[string]any{"type": "remote", "url": url}
	if disabled {
		entry["enabled"] = false
	}
	return entry
}

// catalogEntry fetches a catalog entry by id. An unknown id is an installer
// bug, not a user problem, so it errors instead of silently writing nothing.
func catalogEntry(id string) (mcppkg.CatalogEntry, error) {
	entry, ok := mcppkg.CatalogByID(id)
	if !ok {
		return mcppkg.CatalogEntry{}, fmt.Errorf("unknown MCP catalog id %q", id)
	}
	return entry, nil
}

// argvAny copies a string argv into []any, the type JSON configs round-trip.
func argvAny(argv []string) []any {
	out := make([]any, len(argv))
	for i, s := range argv {
		out[i] = s
	}
	return out
}

// InstallMetaDevToolsMCP adds Meta Developer Tools (manage Meta apps, webhooks,
// compliance, app status, developer docs) to the agent's config. The endpoint
// is catalog data; authentication is the client's OAuth sign-in, not something
// ywai can do — the endpoint answers 401 until the user signs in from their
// agent, which is expected.
func InstallMetaDevToolsMCP(configPath, agentName string) error {
	entry, err := catalogEntry("meta-devtools")
	if err != nil {
		return err
	}
	return installMCPEntry(configPath, agentName, entry.ID, func(configKey string) map[string]any {
		return remoteEntry(configKey, entry.URL, false)
	})
}

// InstallMicrosoftLearnMCP adds the Microsoft Learn MCP server to the agent's
// config file. The endpoint is catalog data; claude-code/pi run the npm stdio
// server instead of the remote one.
func InstallMicrosoftLearnMCP(configPath, agentName string) error {
	entry, err := catalogEntry("microsoft-learn")
	if err != nil {
		return err
	}
	return installMCPEntry(configPath, agentName, entry.ID, func(configKey string) map[string]any {
		if configKey == "mcpServers" {
			return map[string]any{
				"command": "npx",
				"args":    []any{"@anthropic/mcp-server-microsoft-learn"},
			}
		}
		return remoteEntry(configKey, entry.URL, false)
	})
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
		servers := mcppkg.CollectOpenCodeServers(mcp)
		if _, exists := servers["mcp-vision"]; !exists {
			return nil
		}
		delete(servers, "mcp-vision")
		root[key] = mcppkg.WriteOpenCodeMCP(mcp, servers)
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

// InstallChromeDevToolsMCP registers the Chrome DevTools MCP server in the
// agent's config, leaving an existing entry untouched.
//
// It ships by default rather than behind a flag because the scenario-runner
// agent needs a browser to run a UI scenario at all: without it the agent
// installs fine and then cannot do the one thing it exists for.
func InstallChromeDevToolsMCP(configPath, agentName string) error {
	entry, err := catalogEntry("chrome-devtools")
	if err != nil {
		return err
	}
	return installMCPEntry(configPath, agentName, entry.ID, func(configKey string) map[string]any {
		if configKey == "mcpServers" {
			// Claude Code / pi split the argv into command + args.
			return map[string]any{
				"command": entry.Command[0],
				"args":    argvAny(entry.Command[1:]),
			}
		}
		// OpenCode: mcp.servers.<id> with type "local" and a full argv.
		return map[string]any{"type": "local", "command": argvAny(entry.Command)}
	})
}

// InstallGrafanaMCP registers the Grafana MCP server, disabled and with no
// endpoint.
//
// A Grafana MCP lives on the user's own network, so ywai has no URL to write
// and shipping one in a public repo is out of the question. Installing it blank
// but disabled is what makes it visible in Settings, where the URL is filled in
// and the server switched on — a catalog entry nobody installs is a server
// nobody discovers. Disabled matters: an enabled entry with no URL is a server
// the agent tries and fails to reach on every start.
func InstallGrafanaMCP(configPath, agentName string) error {
	entry, err := catalogEntry("grafana")
	if err != nil {
		return err
	}
	return installMCPEntry(configPath, agentName, entry.ID, func(configKey string) map[string]any {
		return remoteEntry(configKey, entry.URL, true)
	})
}
