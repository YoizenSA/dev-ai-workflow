package configapi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
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
	// Trim the delimiter newlines. Without this the slice carries a leading and
	// trailing "" through Split/Join, and every writer that rebuilds the block
	// (setScalarFrontmatterField) adds one blank line per call — frontmatter
	// that grows a pair of blank lines on every model/field edit.
	fm := strings.Trim(content[3:end+3], "\n")
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
	// Then the sidecar ywai writes beside the flat agent files. Group used to
	// live in the frontmatter, but opencode v2 treats an unknown agent key as a
	// legacy v1 file and drops the v2 permission rules, so it moved out.
	if dir, err := agentsDir(); err == nil {
		if group := agents.ReadGroupSidecar(dir, agentName); group != "" {
			return group
		}
	}
	// Finally the legacy frontmatter field, for agents installed before the
	// sidecar existed and not yet reinstalled.
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
// Supports the v2 rule array and, as a legacy fallback for files installed by
// older ywai releases, the v1 nested maps:
//   - v2:  permissions:\n  - action: read\n    resource: "*"\n    effect: allow
//   - v1:  permission:\n  read: allow\n  edit: deny
//   - v0:  tools:\n  read: true\n  edit: false
func extractPermissionsFromFrontmatter(fm string) map[string]string {
	if rules, ok := agents.ParsePermissionRulesYAML(fm); ok {
		return agents.InternalMapFromRules(rules)
	}
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
		// Strip surrounding quotes: expanded pattern keys (ado_*, ywai-kanban_*)
		// are emitted quoted, so the on-disk key looks like "ado_*".
		key := strings.Trim(strings.TrimSpace(parts[0]), `"'`)
		val := strings.TrimSpace(parts[1])

		if headerKey == "tools" {
			// Old format: tools: read: true → convert to permission string
			if val == "true" {
				perms[key] = "allow"
			} else if val == "false" {
				perms[key] = "deny"
			}
		} else {
			// v1 format: permission: read: allow (bash renders as a nested
			// block whose children cannot be split back out — take the first
			// line's shape: a bare "bash:" means at least conditional shell).
			if key == "bash" && val == "" {
				perms[key] = "verify"
				continue
			}
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

// updatePermissionsInFrontmatter replaces the permissions block (v2 rule
// array, or legacy permission:/tools: maps) with a fresh v2 array rendered
// from the given internal permission map. The subagent (delegation) rules are
// preserved: the map carried by this endpoint is tool permissions only. If no
// block exists, one is added. Returns the updated full markdown content.
func updatePermissionsInFrontmatter(content string, perms map[string]string) string {
	fm, _ := parseFrontmatter(content)

	// Preserve the delegation graph: swap tool rules, keep subagent rules.
	var subagentRules []agents.PermissionRule
	if fm != "" {
		if existing, ok := agents.ParsePermissionRulesYAML(fm); ok {
			for _, r := range existing {
				if r.Action == "subagent" {
					subagentRules = append(subagentRules, r)
				}
			}
		}
	}
	rules := append(agents.RulesFromPermissionMap("", perms), subagentRules...)
	return replacePermissionsBlock(content, rules)
}

// replacePermissionsBlock swaps the frontmatter permissions block (v2 rule
// array, dropping any legacy permission:/tools: maps) for a freshly rendered
// one, leaving every other frontmatter key untouched. Content without
// frontmatter gets a minimal frontmatter with the block added.
func replacePermissionsBlock(content string, rules []agents.PermissionRule) string {
	fm, body := parseFrontmatter(content)

	var b strings.Builder
	b.WriteString("---\n")
	if fm != "" {
		// Rebuild the frontmatter keeping every key except the permission-
		// bearing blocks (v2 permissions: array + v1 permission:/tools: maps).
		// Block children are indented lines that follow their header until the
		// next top-level key; other indented lines (e.g. description block
		// scalars) belong to kept keys and must survive.
		inDropped := false
		for _, line := range strings.Split(strings.TrimPrefix(fm, "\n"), "\n") {
			indented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
			trimmed := strings.TrimSpace(line)
			if !indented && trimmed != "" {
				// `group:` is not a valid opencode v2 agent key; keeping it
				// sends the file down the legacy v1 decode path, which drops
				// the v2 permissions array. Group lives in the sidecar.
				if strings.HasPrefix(trimmed, "group:") {
					inDropped = false
					continue
				}
				if trimmed == "permissions:" || trimmed == "permission:" ||
					trimmed == "tools:" || strings.HasPrefix(trimmed, "permission:") ||
					strings.HasPrefix(trimmed, "tools:") {
					inDropped = true
					continue
				}
				inDropped = false
				b.WriteString(line + "\n")
				continue
			}
			if inDropped {
				continue
			}
			b.WriteString(line + "\n")
		}
	} else {
		b.WriteString("description: agent\n")
	}
	for _, line := range agents.RenderPermissionRulesYAML(rules) {
		b.WriteString(line + "\n")
	}
	b.WriteString("---\n\n")
	b.WriteString(body)
	return b.String()
}

// getScalarFrontmatterField returns the value of a top-level scalar key in the
// frontmatter (e.g. "model"), or "" if absent. Quotes around the value are stripped.
func getScalarFrontmatterField(content, key string) string {
	fm, _ := parseFrontmatter(content)
	if fm == "" {
		return ""
	}
	prefix := key + ":"
	for _, line := range strings.Split(fm, "\n") {
		trimmed := strings.TrimSpace(line)
		// Skip indented lines so a nested "model:" inside another block is ignored.
		if line != trimmed {
			continue
		}
		if strings.HasPrefix(trimmed, prefix) {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
			return strings.Trim(val, `"'`)
		}
	}
	return ""
}

// setScalarFrontmatterField sets a top-level scalar key to value in the
// frontmatter, inserting it if absent. An empty value removes the key. Returns
// the updated full markdown content.
func setScalarFrontmatterField(content, key, value string) string {
	fm, body := parseFrontmatter(content)
	prefix := key + ":"

	if fm == "" {
		if value == "" {
			return content
		}
		return fmt.Sprintf("---\n%s %s\n---\n\n%s", prefix, value, body)
	}

	lines := strings.Split(fm, "\n")
	var result []string
	replaced := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if line == trimmed && strings.HasPrefix(trimmed, prefix) {
			replaced = true
			if value != "" {
				result = append(result, fmt.Sprintf("%s %s", prefix, value))
			}
			continue // drop the old line (and skip entirely when clearing)
		}
		result = append(result, line)
	}
	if !replaced && value != "" {
		result = append(result, fmt.Sprintf("%s %s", prefix, value))
	}

	newFM := strings.Join(result, "\n")
	return "---\n" + newFM + "\n---\n\n" + body
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

// --- Markdown body section helpers (operate on the prompt body, NOT frontmatter) ---
//
// These let the delegation-rules endpoint read/write a specific "### Header"
// section of an agent's prompt body without touching frontmatter or the rest
// of the prose. A "section" is the heading line itself plus every line until
// the next heading of equal-or-higher level (e.g. a "#### Sub" is owned by the
// preceding "### Parent" until another "### " appears).

// headingLevel returns the number of leading '#' of a markdown ATX heading
// (1..6), or 0 if the line is not a heading.
func headingLevel(line string) int {
	trimmed := strings.TrimSpace(line)
	n := 0
	for n < len(trimmed) && trimmed[n] == '#' && n < 6 {
		n++
	}
	if n == 0 || n >= len(trimmed) || trimmed[n] != ' ' {
		return 0
	}
	return n
}

// headingText returns the title of an ATX heading line (without the '#') and a
// trailing "#### Mandatory..." -> "Mandatory...". Returns "" for non-headings.
func headingText(line string) string {
	level := headingLevel(line)
	if level == 0 {
		return ""
	}
	return strings.TrimSpace(strings.TrimSpace(line)[level:])
}

// extractMarkdownSection returns the body text under the first heading whose
// title equals headerText (compared case-insensitively, ignoring the leading
// "### " markers). The returned content is the section body WITHOUT the heading
// line and WITHOUT nested sub-sections: it stops at the next heading of equal
// or higher level. The boolean reports whether the heading was found.
//
// Example: for body
//
//	### Delegation Rules
//	core principle ...
//	| Action | Inline | Delegate |
//	#### Mandatory Triggers
//	...
//	### Cost
//
// extractMarkdownSection(body, "Delegation Rules", false) returns the lines
// between "### Delegation Rules" and "#### Mandatory Triggers" (the table),
// because a "####" is a higher numeric level (shallower depth) ... wait:
// includeSubsections=false stops at the NEXT heading of equal-or-HIGHER level
// (same or fewer '#'). "####" has more '#', so it is a sub-section and is NOT
// a stop boundary when includeSubsections is false only in the "equal-or-fewer"
// sense — see the implementation: stop when nextLevel>0 && nextLevel<=level.
//
// Concretely:
//   - includeSubsections=false returns only the direct content (stops at any
//     heading with level <= the section's level, i.e. same-or-shallower).
//   - includeSubsections=true returns the direct content PLUS nested headings
//     (stops only at headings of the same-or-shallower level as the section).
func extractMarkdownSection(body, headerText string, includeSubsections bool) (string, bool) {
	target := strings.ToLower(strings.TrimSpace(headerText))
	level := 0
	startLine := -1
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if headingLevel(line) == 0 {
			continue
		}
		if strings.ToLower(headingText(line)) == target {
			level = headingLevel(line)
			startLine = i + 1
			break
		}
	}
	if startLine < 0 {
		return "", false
	}

	var out []string
	for _, line := range lines[startLine:] {
		lvl := headingLevel(line)
		if lvl > 0 && lvl <= level {
			// A heading at the same level or shallower ends the section.
			break
		}
		if !includeSubsections && lvl > level {
			// Caller asked for direct content only — a nested heading is a stop
			// boundary for the *direct* slice.
			break
		}
		out = append(out, line)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n"), true
}

// replaceMarkdownSection replaces the body content under the heading
// headerText with newContent. If the heading does not exist, the section
// (heading + "\n\n" + newContent) is appended at the end of the body.
//
// The heading line itself is preserved; only the lines between it and the next
// same-or-shallower heading are replaced. Nested sub-sections (deeper headings)
// are preserved as-is when includeSubsections is false.
func replaceMarkdownSection(body, headerText, headingPrefix, newContent string, includeSubsections bool) string {
	target := strings.ToLower(strings.TrimSpace(headerText))
	lines := strings.Split(body, "\n")
	level := 0
	headingLineIdx := -1
	for i, line := range lines {
		if headingLevel(line) == 0 {
			continue
		}
		if strings.ToLower(headingText(line)) == target {
			level = headingLevel(line)
			headingLineIdx = i
			break
		}
	}

	// Section absent → append at end.
	if headingLineIdx < 0 {
		prefix := strings.TrimSpace(headingPrefix)
		if prefix == "" {
			prefix = "###"
		}
		sep := "\n\n"
		if body == "" || strings.HasSuffix(body, "\n") {
			sep = "\n"
		}
		return body + sep + prefix + " " + headerText + "\n\n" + newContent + "\n"
	}

	// Find the end of the existing section (next same-or-shallower heading).
	endIdx := len(lines)
	for j := headingLineIdx + 1; j < len(lines); j++ {
		lvl := headingLevel(lines[j])
		if lvl > 0 && lvl <= level {
			endIdx = j
			break
		}
	}

	var rebuilt []string
	rebuilt = append(rebuilt, lines[:headingLineIdx+1]...)
	rebuilt = append(rebuilt, "") // blank line after heading
	for _, l := range strings.Split(newContent, "\n") {
		rebuilt = append(rebuilt, l)
	}
	rebuilt = append(rebuilt, lines[endIdx:]...)
	return strings.Join(rebuilt, "\n")
}
