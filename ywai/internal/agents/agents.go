package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AgentProfile represents a parsed agent from ywai/agents/.
type AgentProfile struct {
	Name        string
	Description string
	Prompt      string
	Permission  map[string]string
	Skills      []string
	Mode        string
	Group       string // group name from groups.json (e.g. "core", "qa-automation")
}

// GroupManifest represents the groups.json file.
type GroupManifest struct {
	Groups map[string]GroupDefinition `json:"groups"`
}

// GroupDefinition defines a single agent group.
type GroupDefinition struct {
	Description string   `json:"description"`
	Agents      []string `json:"agents"`
}

// GroupFilter controls which groups to install.
type GroupFilter struct {
	Groups    []string // group names to include (in addition to core)
	AllGroups bool     // install ALL groups (backward compat)
}

// LoadProfiles reads all agent directories from the given source dir.
// It walks subdirectories recursively (e.g. core/, social-refactor/) and
// loads any directory containing AGENT.md.
func LoadProfiles(sourceDir string) (map[string]AgentProfile, error) {
	profiles := map[string]AgentProfile{}

	err := filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		// Skip the root dir itself
		if path == sourceDir {
			return nil
		}
		// Skip if this dir doesn't have AGENT.md (it's a group folder, not an agent)
		if _, err := os.Stat(filepath.Join(path, "AGENT.md")); err != nil {
			return nil
		}

		profile, err := loadProfile(path, sourceDir)
		if err != nil {
			fmt.Printf("  Warning: skip agent %s: %v\n", filepath.Base(path), err)
			return nil
		}

		profiles[profile.Name] = *profile
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk agents dir %s: %w", sourceDir, err)
	}

	// Assign groups from groups.json
	assignGroups(sourceDir, profiles)

	return profiles, nil
}

// assignGroups reads groups.json and sets the Group field on matching profiles.
func assignGroups(sourceDir string, profiles map[string]AgentProfile) {
	manifest, err := LoadGroupManifest(sourceDir)
	if err != nil {
		return // groups.json missing — leave Group empty
	}

	// Build lookup: try both full name ("core/orchestrator") and base name ("orchestrator")
	profileByName := make(map[string]string) // name -> profile key in profiles map
	for key := range profiles {
		profileByName[key] = key
		profileByName[filepath.Base(key)] = key
	}

	for groupName, def := range manifest.Groups {
		for _, agentName := range def.Agents {
			if key, ok := profileByName[agentName]; ok {
				p := profiles[key]
				p.Group = groupName
				profiles[key] = p
			}
		}
	}
}

func loadProfile(dir string, sourceDir string) (*AgentProfile, error) {
	// Use the relative path from sourceDir as the agent name.
	// This ensures agents in subdirectories (e.g. qa-automation/qa-orchestrator)
	// match the names referenced in groups.json.
	rel, err := filepath.Rel(sourceDir, dir)
	if err != nil {
		rel = filepath.Base(dir)
	}
	name := filepath.ToSlash(rel)

	// Read AGENT.md
	agentFile := filepath.Join(dir, "AGENT.md")
	promptBytes, err := os.ReadFile(agentFile)
	if err != nil {
		return nil, fmt.Errorf("read AGENT.md: %w", err)
	}

	prompt := string(promptBytes)
	description := extractDescription(prompt)
	mode := extractMode(prompt)
	if mode == "" {
		mode = "primary"
	}
	_ = extractRole(prompt) // extracted for future use
	sections := extractSections(prompt)
	prompt = stripFrontmatter(prompt)

	// Read permissions.json
	permsFile := filepath.Join(dir, "permissions.json")
	perms, err := parsePermissions(permsFile)
	if err != nil {
		fmt.Printf("  Warning: agent %s permissions.json error: %v, using defaults\n", name, err)
		perms = map[string]string{"read": "allow", "edit": "allow", "write": "allow", "bash": "allow"}
	}

	// Read skills.txt (optional)
	var skills []string
	skillsFile := filepath.Join(dir, "skills.txt")
	if data, err := os.ReadFile(skillsFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				skills = append(skills, line)
			}
		}
	}

	return &AgentProfile{
		Name:        name,
		Description: description,
		Prompt:      appendSections(promptWithSkills(prompt, skills), sections, sourceDir),
		Permission:  perms,
		Skills:      skills,
		Mode:        mode,
	}, nil
}

// promptWithSkills appends a section listing the ywai skills this agent should
// prefer, so the model knows which domain skills to invoke. No-op when empty.
func promptWithSkills(prompt string, skills []string) string {
	if len(skills) == 0 {
		return prompt
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(prompt, "\n"))
	b.WriteString("\n\n## Preferred ywai Skills\n\n")
	b.WriteString("When relevant, invoke these ywai skills for domain-specific guidance:\n\n")
	for _, s := range skills {
		b.WriteString("- `" + s + "`\n")
	}
	return b.String()
}

// extractSections parses the `sections:` field from YAML frontmatter.
// Returns a slice of section names (e.g. ["handoff", "kanban"]).
func extractSections(prompt string) []string {
	if !strings.HasPrefix(prompt, "---") {
		return nil
	}
	end := strings.Index(prompt[3:], "---")
	if end == -1 {
		return nil
	}
	frontmatter := prompt[3 : end+3]
	for _, line := range strings.Split(frontmatter, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "sections:") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "sections:"))
			// Parse YAML array: [handoff, kanban]
			value = strings.Trim(value, "[]")
			if value == "" {
				return nil
			}
			parts := strings.Split(value, ",")
			var result []string
			for _, p := range parts {
				s := strings.TrimSpace(p)
				if s != "" {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

// appendSections resolves and appends shared section files from the
// agents/sections/ directory. Each section is a .md file named after
// the section (e.g. "handoff" -> sections/handoff.md).
func appendSections(prompt string, sections []string, sourceDir string) string {
	if len(sections) == 0 {
		return prompt
	}
	sectionsDir := filepath.Join(sourceDir, "sections")
	var b strings.Builder
	b.WriteString(strings.TrimRight(prompt, "\n"))
	for _, s := range sections {
		path := filepath.Join(sectionsDir, s+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue // section not found, skip silently
		}
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(string(data)))
	}
	return b.String()
}

// extractDescription returns the agent description. It prefers the YAML
// frontmatter `description:` field (including folded `>`/literal `|` blocks),
// and falls back to the first body line, then the first heading.
func extractDescription(prompt string) string {
	if desc := frontmatterDescription(prompt); desc != "" {
		return desc
	}

	// Strip YAML frontmatter
	content := prompt
	if strings.HasPrefix(content, "---") {
		end := strings.Index(content[3:], "---")
		if end != -1 {
			content = content[end+6:]
		}
	}

	// Find the first non-empty line
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			if len(line) > 120 {
				return line[:120] + "..."
			}
			return line
		}
	}

	// Fallback: use first heading
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}

	return "agent"
}

// frontmatterDescription parses the `description:` field from YAML frontmatter.
// It supports inline values (`description: text`) and folded/literal block
// scalars (`description: >` followed by indented lines).
func frontmatterDescription(prompt string) string {
	if !strings.HasPrefix(prompt, "---") {
		return ""
	}
	end := strings.Index(prompt[3:], "---")
	if end == -1 {
		return ""
	}
	frontmatter := prompt[3 : end+3]
	lines := strings.Split(frontmatter, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "description:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))

		// Inline value (not a block scalar indicator).
		if value != "" && value != ">" && value != "|" && value != ">-" && value != "|-" {
			return value
		}

		// Block scalar: collect subsequent more-indented lines until the next key.
		var parts []string
		for _, next := range lines[i+1:] {
			if strings.TrimSpace(next) == "" {
				continue
			}
			// A new top-level key (e.g. `role:`, `tools:`) ends the block.
			if !strings.HasPrefix(next, " ") && !strings.HasPrefix(next, "\t") {
				break
			}
			parts = append(parts, strings.TrimSpace(next))
		}
		return strings.Join(parts, " ")
	}
	return ""
}

// extractMode parses the `mode:` field from YAML frontmatter.
func extractMode(prompt string) string {
	if !strings.HasPrefix(prompt, "---") {
		return ""
	}
	end := strings.Index(prompt[3:], "---")
	if end == -1 {
		return ""
	}
	frontmatter := prompt[3 : end+3]
	for _, line := range strings.Split(frontmatter, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "mode:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "mode:"))
		}
	}
	return ""
}

// extractRole parses the `role:` field from YAML frontmatter.
func extractRole(prompt string) string {
	if !strings.HasPrefix(prompt, "---") {
		return ""
	}
	end := strings.Index(prompt[3:], "---")
	if end == -1 {
		return ""
	}
	frontmatter := prompt[3 : end+3]
	for _, line := range strings.Split(frontmatter, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "role:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "role:"))
		}
	}
	return ""
}

// parsePermissions reads permissions.json and returns a permission map.
func parsePermissions(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var perms map[string]string
	if err := json.Unmarshal(data, &perms); err != nil {
		return nil, err
	}

	return perms, nil
}

// LoadGroupManifest loads and parses groups.json, merging with groups.local.json if present.
func LoadGroupManifest(sourceDir string) (*GroupManifest, error) {
	manifest := &GroupManifest{Groups: make(map[string]GroupDefinition)}

	path := filepath.Join(sourceDir, "groups.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read groups.json: %w", err)
	}

	var base GroupManifest
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("invalid groups.json: %w", err)
	}
	for k, v := range base.Groups {
		manifest.Groups[k] = v
	}

	// Try local override (groups.local.json)
	localPath := filepath.Join(sourceDir, "groups.local.json")
	if localData, err := os.ReadFile(localPath); err == nil {
		var local GroupManifest
		if err := json.Unmarshal(localData, &local); err == nil {
			for k, v := range local.Groups {
				manifest.Groups[k] = v
			}
		}
	}

	return manifest, nil
}

// LoadProfilesByGroup loads agent profiles filtered by group.
func LoadProfilesByGroup(sourceDir string, filter GroupFilter) (map[string]AgentProfile, error) {
	if filter.AllGroups {
		return LoadProfiles(sourceDir)
	}

	manifest, err := LoadGroupManifest(sourceDir)
	if err != nil {
		// Fallback: if groups.json is broken/missing, load only core-like agents
		// (agents in the root of sourceDir, not in subfolders)
		fmt.Printf("  Warning: groups.json unavailable (%v), loading root agents only\n", err)
		return LoadProfiles(sourceDir)
	}

	// Build set of allowed agent names + group assignment
	allowed := make(map[string]bool)
	groupOf := make(map[string]string) // agent name -> group name

	// Core is always included
	if core, ok := manifest.Groups["core"]; ok {
		for _, name := range core.Agents {
			allowed[name] = true
			groupOf[name] = "core"
		}
	}

	// Add requested groups
	for _, groupName := range filter.Groups {
		if def, ok := manifest.Groups[groupName]; ok {
			for _, name := range def.Agents {
				allowed[name] = true
				groupOf[name] = groupName
			}
		}
	}

	// Load all profiles, then filter
	allProfiles, err := LoadProfiles(sourceDir)
	if err != nil {
		return nil, err
	}

	result := make(map[string]AgentProfile)
	for name, profile := range allProfiles {
		// Match by full name ("core/orchestrator") or base name ("orchestrator")
		if allowed[name] {
			profile.Group = groupOf[name]
			result[name] = profile
		} else if base := filepath.Base(name); allowed[base] {
			profile.Group = groupOf[base]
			result[name] = profile
		}
	}

	return result, nil
}

// ListGroups returns available group names sorted alphabetically.
func ListGroups(sourceDir string) ([]string, error) {
	manifest, err := LoadGroupManifest(sourceDir)
	if err != nil {
		return nil, err
	}

	var names []string
	for name := range manifest.Groups {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
