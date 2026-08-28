package agents

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// MigrateOpenCodeAgents migrates agents from opencode.json to markdown format.
// After successful migration, agents are removed from the JSON file.
func MigrateOpenCodeAgents(configPath, agentsDir string) error {
	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // Nothing to migrate
	}

	// Read existing config
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", configPath, err)
	}

	// Prefer the v2 `agents` key; also drain leftover v1 `agent`.
	agents := map[string]any{}
	if raw, ok := root["agents"].(map[string]any); ok {
		for name, entry := range raw {
			agents[name] = entry
		}
	}
	if raw, ok := root["agent"].(map[string]any); ok {
		for name, entry := range raw {
			if _, exists := agents[name]; exists {
				continue
			}
			agents[name] = entry
		}
	}

	if len(agents) == 0 {
		return nil
	}

	// Ensure agents directory exists
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return fmt.Errorf("create agents dir %s: %w", agentsDir, err)
	}

	migrated := 0
	for name, agentRaw := range agents {
		agentMap, ok := agentRaw.(map[string]any)
		if !ok {
			continue
		}

		// Convert to AgentProfile
		profile := mapToAgentProfile(name, agentMap)

		// Write markdown file
		targetPath := filepath.Join(agentsDir, name+".md")
		if _, err := os.Stat(targetPath); err == nil {
			// Already exists, skip migration but remove from JSON
			delete(agents, name)
			migrated++
			continue
		}

		content := BuildOpenCodeMarkdown(name, profile)
		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			fmt.Printf("  Warning: failed to migrate agent %s: %v\n", name, err)
			continue
		}

		// Remove from JSON
		delete(agents, name)
		migrated++
	}

	if migrated > 0 {
		fmt.Printf("  Migrated %d agents from %s to markdown\n", migrated, filepath.Base(configPath))

		// Update config file
		if len(agents) == 0 {
			delete(root, "agent")
			delete(root, "agents")
		} else {
			root["agents"] = agents
			delete(root, "agent")
		}

		if err := config.WriteJSONC(configPath, root); err != nil {
			return fmt.Errorf("write updated config: %w", err)
		}
	}

	return nil
}

// mapToAgentProfile converts a map from JSON to AgentProfile. Handles both
// the v1 entry shape (prompt + permission/tools maps) and the v2 shape
// (system + permissions rule array).
func mapToAgentProfile(name string, m map[string]any) AgentProfile {
	prompt := ""
	if p, ok := m["prompt"].(string); ok {
		prompt = p
	}
	// v2 renamed prompt → system.
	if p, ok := m["system"].(string); ok && prompt == "" {
		prompt = p
	}

	description := ""
	if d, ok := m["description"].(string); ok {
		description = d
	}

	mode := "primary"
	if md, ok := m["mode"].(string); ok {
		mode = md
	}

	permission := map[string]string{"read": "allow", "edit": "allow", "write": "allow", "bash": "allow"}
	if rules, ok := m["permissions"].([]any); ok {
		// v2 rule array → internal map (subagent rules are the delegation
		// graph, re-injected separately, so skip them here).
		for _, raw := range rules {
			r, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			action, _ := r["action"].(string)
			resource, _ := r["resource"].(string)
			effect, _ := r["effect"].(string)
			if action == "subagent" || resource != "*" || effect == "" {
				continue
			}
			key := NormalizePermissionKey(action)
			if key == "edit" {
				permission["edit"] = effect
				permission["write"] = effect
			} else {
				permission[key] = effect
			}
		}
	} else if perm, ok := m["permission"].(map[string]any); ok {
		for k, v := range perm {
			if val, ok := v.(string); ok {
				permission[k] = val
			}
		}
	} else if t, ok := m["tools"].(map[string]any); ok {
		// Legacy: convert tools bool map to permission strings
		for k, v := range t {
			if enabled, ok := v.(bool); ok {
				if enabled {
					permission[k] = "allow"
				} else {
					permission[k] = "deny"
				}
			}
		}
	}

	return AgentProfile{
		Name:        name,
		Description: description,
		Prompt:      prompt,
		Permission:  permission,
		Mode:        mode,
	}
}
