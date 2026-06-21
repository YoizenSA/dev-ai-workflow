package kanban

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// --- Markdown Frontmatter Helpers ---

// resolveAgentFile finds the .md file for an agent name within dir.
//
// Agents may live either directly under dir (e.g. dir/gentle-orchestrator.md)
// or inside a group subdirectory (e.g. dir/core/architect.md,
// dir/social-refactor/migration-orchestrator.md). ListAgents already scans
// both layouts when listing; this helper mirrors that so the single-agent
// handlers (GetAgent, PutAgent, DeleteAgent, permissions) resolve the same
// file the list presented — otherwise selecting a nested agent returns 404.
//
// Returns the absolute path or "" when nothing matches.
func resolveAgentFile(dir, name string) string {
	target := name + ".md"

	// 1. Flat layout: dir/{name}.md
	flat := filepath.Join(dir, target)
	if info, err := os.Stat(flat); err == nil && !info.IsDir() {
		return flat
	}

	// 2. Grouped layout: dir/{group}/{name}.md — scan one level of subdirs.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		nested := filepath.Join(dir, e.Name(), target)
		if info, err := os.Stat(nested); err == nil && !info.IsDir() {
			return nested
		}
	}
	return ""
}

// readAgentMarkdownPath returns the path to an agent's .md file, or "" if it doesn't exist.
func readAgentMarkdownPath(name string) string {
	dir, err := agentsDir()
	if err != nil {
		return ""
	}
	return resolveAgentFile(dir, name)
}

// parseFrontmatter extracts YAML frontmatter from a markdown file.
// Returns frontmatter body (between --- delimiters) and the rest of the content.
func parseFrontmatter(content string) (frontmatter string, body string) {
	if !strings.HasPrefix(content, "---") {
		return "", content
	}
	end := strings.Index(content[3:], "---")
	if end == -1 {
		return "", content
	}
	fm := content[3 : end+3]
	body = strings.TrimSpace(content[end+6:])
	return fm, body
}

// extractFrontmatterField extracts a simple scalar field from YAML frontmatter.
// Handles both inline (key: value) and block (key:\n  value) formats.
func extractFrontmatterField(data []byte, fieldName string) string {
	content := string(data)
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	end := strings.Index(content[3:], "---")
	if end == -1 {
		return ""
	}
	fm := content[3 : end+3]

	// Try simple key: value first
	pattern := fieldName + ":"
	for _, line := range strings.Split(fm, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, pattern) {
			val := strings.TrimSpace(trimmed[len(pattern):])
			if val != "" && !strings.HasPrefix(val, "\n") {
				return val
			}
		}
	}
	return ""
}

// detectAgentTeam scans ywai/agents/{team}/ directories to find which team
// an agent belongs to. Falls back to the "group" frontmatter field.
func detectAgentTeam(agentName string, agentData []byte) string {
	// Try ywai/agents/{team}/ directory structure first
	ywaiAgentsDir := findYwaiAgentsDir()
	if ywaiAgentsDir != "" {
		entries, err := os.ReadDir(ywaiAgentsDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				teamDir := filepath.Join(ywaiAgentsDir, entry.Name())
				// Check if agent exists in this team directory
				// Agents can be in team/{agent-name}/AGENT.md or team/{agent-name}.md
				if _, err := os.Stat(filepath.Join(teamDir, agentName, "AGENT.md")); err == nil {
					return entry.Name()
				}
				if _, err := os.Stat(filepath.Join(teamDir, agentName+".md")); err == nil {
					return entry.Name()
				}
			}
		}
	}
	// Fall back to frontmatter "group" field
	return extractFrontmatterField(agentData, "group")
}

// findYwaiAgentsDir searches for the ywai/agents/ directory.
func findYwaiAgentsDir() string {
	// Try relative to current working directory
	cwd, err := os.Getwd()
	if err == nil {
		dir := cwd
		for i := 0; i < 8; i++ {
			candidate := filepath.Join(dir, "ywai", "agents")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
			if filepath.Base(dir) == "agents" {
				if _, err := os.Stat(filepath.Join(dir, "core")); err == nil {
					return dir
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	// Try GOPATH/src/github.com/Yoizen/dev-ai-workflow/ywai/agents
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		candidate := filepath.Join(gopath, "src", "github.com", "Yoizen", "dev-ai-workflow", "ywai", "agents")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// Try ~/Documents/GitHub/dev-ai-workflow/ywai/agents
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, "Documents", "GitHub", "dev-ai-workflow", "ywai", "agents")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// extractPermissionsFromFrontmatter parses permissions from YAML frontmatter.
// Supports both formats:
//   - New: permission:\n  read: allow\n  edit: deny\n ...
//   - Old: tools:\n  read: true\n  edit: false\n ...
func extractPermissionsFromFrontmatter(fm string) map[string]string {
	perms := map[string]string{}
	lines := strings.Split(fm, "\n")

	// Find the permission: or tools: key
	headerIdx := -1
	headerKey := ""
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "permission:" || strings.HasPrefix(trimmed, "permission:") {
			headerIdx = i
			headerKey = "permission"
			break
		}
		if trimmed == "tools:" || strings.HasPrefix(trimmed, "tools:") {
			headerIdx = i
			headerKey = "tools"
			break
		}
	}
	if headerIdx == -1 {
		return perms
	}

	// Collect indented lines under the key
	for _, line := range lines[headerIdx+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Must be indented (child of the key)
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break // next top-level key
		}
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if headerKey == "tools" {
			// Old format: tools: read: true → convert to permission string
			if val == "true" {
				perms[key] = "allow"
			} else if val == "false" {
				perms[key] = "deny"
			}
		} else {
			// New format: permission: read: allow
			perms[key] = val
		}
	}
	return perms
}

// extractModeFromFrontmatter parses the mode: field from YAML frontmatter.
func extractModeFromFrontmatter(fm string) string {
	for _, line := range strings.Split(fm, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "mode:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "mode:"))
		}
	}
	return ""
}

// updatePermissionsInFrontmatter replaces the permission: block in frontmatter
// with the given permissions map. If no permission: block exists, it adds one.
// Returns the updated full markdown content.
func updatePermissionsInFrontmatter(content string, perms map[string]string) string {
	fm, body := parseFrontmatter(content)
	if fm == "" {
		// No frontmatter at all — wrap content with new frontmatter
		var b strings.Builder
		b.WriteString("---\npermission:\n")
		for _, k := range sortedKeys(perms) {
			b.WriteString(fmt.Sprintf("  %s: %s\n", k, perms[k]))
		}
		b.WriteString("---\n\n")
		b.WriteString(body)
		return b.String()
	}

	lines := strings.Split(fm, "\n")
	var result []string
	skipping := false
	inserted := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect start of permission: or tools: block
		if !skipping && (trimmed == "permission:" || trimmed == "tools:" ||
			strings.HasPrefix(trimmed, "permission:") || strings.HasPrefix(trimmed, "tools:")) {
			skipping = true
			// Insert new permission: block here
			result = append(result, "permission:")
			for _, k := range sortedKeys(perms) {
				result = append(result, fmt.Sprintf("  %s: %s", k, perms[k]))
			}
			inserted = true
			continue
		}

		if skipping {
			// Skip child lines (indented)
			if strings.TrimSpace(line) == "" {
				continue
			}
			if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				continue
			}
			// Not indented anymore — stop skipping
			skipping = false
		}

		result = append(result, line)

		// If we reached the end of frontmatter without finding a block, insert before end
		if !inserted && i == len(lines)-1 {
			result = append(result, "permission:")
			for _, k := range sortedKeys(perms) {
				result = append(result, fmt.Sprintf("  %s: %s", k, perms[k]))
			}
			inserted = true
		}
	}

	newFM := strings.Join(result, "\n")
	return "---\n" + newFM + "\n---\n\n" + body
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// sortedPermissionKeys returns ValidPermissionKeys as a sorted slice for error messages.
func sortedPermissionKeys() []string {
	keys := make([]string, 0, len(ValidPermissionKeys))
	for k := range ValidPermissionKeys {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
