package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// AgentProfile represents a parsed agent from ywai/agents/.
type AgentProfile struct {
	Name        string
	Description string
	Prompt      string
	Permission  map[string]string
	Skills      []string
	Mode        string
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

	return profiles, nil
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

// InstallOpenCode injects agent profiles into opencode.json (or opencode.jsonc).
// If the file does not exist, it is created with an empty agent section.
func InstallOpenCode(configPath string, profiles map[string]AgentProfile) error {
	root := map[string]any{}

	if _, err := os.Stat(configPath); err == nil {
		var readErr error
		root, readErr = config.ReadJSONC(configPath)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", configPath, readErr)
		}
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Get or create agent section
	agentsRaw, ok := root["agent"]
	if !ok {
		agentsRaw = map[string]any{}
		root["agent"] = agentsRaw
	}
	agents, ok := agentsRaw.(map[string]any)
	if !ok {
		agents = map[string]any{}
		root["agent"] = agents
	}

	installed := 0
	for name, profile := range profiles {
		if existing, exists := agents[name]; exists {
			// Migrate agents that were injected with frontmatter in the prompt (old bug).
			existingMap, ok := existing.(map[string]any)
			if !ok {
				continue
			}
			existingPrompt, ok := existingMap["prompt"].(string)
			if !ok || !strings.HasPrefix(existingPrompt, "---") {
				continue
			}
			existingMap["mode"] = profile.Mode
			existingMap["description"] = profile.Description
			existingMap["prompt"] = profile.Prompt
			existingMap["permission"] = profile.Permission
			delete(existingMap, "tools") // remove deprecated tools field
			installed++
			continue
		}

		agents[name] = map[string]any{
			"mode":        profile.Mode,
			"description": profile.Description,
			"prompt":      profile.Prompt,
			"permission":  profile.Permission,
		}
		installed++
	}

	if installed == 0 {
		return nil
	}

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}

	fmt.Printf("  Installed %d agent profiles\n", installed)
	return nil
}

// InstallClaude writes agent .md files to ~/.claude/agents/.
func InstallClaude(agentsDir string, profiles map[string]AgentProfile) error {
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", agentsDir, err)
	}

	installed := 0
	for name, profile := range profiles {
		targetPath := filepath.Join(agentsDir, name+".md")

		// Ensure parent directory exists for nested agent names (e.g. qa-automation/qa-orchestrator)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			fmt.Printf("  Warning: failed to create dir for %s: %v\n", targetPath, err)
			continue
		}

		// Skip if already exists
		if _, err := os.Stat(targetPath); err == nil {
			continue
		}

		// Build Claude-style agent file, deriving tools from the parsed profile.
		toolsStr := claudeToolsString(profile.Permission)

		content := fmt.Sprintf("---\nname: %s\ndescription: >\n  %s\ntools: %s\n---\n\n%s",
			name, profile.Description, toolsStr, profile.Prompt)

		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			fmt.Printf("  Warning: failed to write %s: %v\n", targetPath, err)
			continue
		}
		installed++
	}

	if installed > 0 {
		fmt.Printf("  Installed %d agent profiles to %s\n", installed, agentsDir)
	}
	return nil
}

// piToolsString renders the enabled tools from a parsed profile as a
// lowercase comma-separated list of PI.dev-style tool names, in stable order.
func piToolsString(perms map[string]string) string {
	order := []struct{ oc, pi string }{
		{"read", "read"},
		{"edit", "edit"},
		{"write", "write"},
		{"bash", "bash"},
		{"glob", "glob"},
		{"grep", "grep"},
		{"webfetch", "webfetch"},
		{"websearch", "websearch"},
	}

	var names []string
	for _, t := range order {
		if v, ok := perms[t.oc]; ok && (v == "allow" || v == "ask") {
			names = append(names, t.pi)
		}
	}
	if len(names) == 0 {
		return "read, glob, grep"
	}
	return strings.Join(names, ", ")
}

// InstallPi writes agent .md files to ~/.pi/agent/agents/.
// Frontmatter uses PI.dev format: lowercase name/description/tools, no mode/permission.
// Respects overwrite: when false, skips existing files (same as InstallOpenCodeMarkdown).
func InstallPi(agentsDir string, profiles map[string]AgentProfile, overwrite bool) error {
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", agentsDir, err)
	}

	installed := 0
	for name, profile := range profiles {
		targetPath := filepath.Join(agentsDir, name+".md")

		// Ensure parent directory exists for nested agent names
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			fmt.Printf("  Warning: failed to create dir for %s: %v\n", targetPath, err)
			continue
		}

		if !overwrite {
			if _, err := os.Stat(targetPath); err == nil {
				continue
			}
		}

		toolsStr := piToolsString(profile.Permission)
		prompt := stripFrontmatter(profile.Prompt)

		// Use a folded block scalar for the description (same as InstallClaude):
		// descriptions can contain ": " and literal quotes (e.g. `Trigger: "build X"`),
		// which break an inline YAML scalar. The folded block keeps them safe.
		content := fmt.Sprintf("---\nname: %s\ndescription: >\n  %s\ntools: %s\n---\n\n%s",
			name, profile.Description, toolsStr, prompt)

		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			fmt.Printf("  Warning: failed to write %s: %v\n", targetPath, err)
			continue
		}
		installed++
	}

	if installed > 0 {
		fmt.Printf("  Installed %d agent profiles to %s\n", installed, agentsDir)
	}
	return nil
}

// InstallCursor writes agent .md files to ~/.cursor/agents/.
func InstallCursor(agentsDir string, profiles map[string]AgentProfile) error {
	return InstallClaude(agentsDir, profiles) // same format
}

// InstallVSCode writes agent profiles as .instructions.md files to VS Code Copilot prompts dir.
// VS Code Copilot reads *.instructions.md files from the User/prompts/ directory.
// Users activate them from Copilot Chat with @workspace or participant selection.
func InstallVSCode(promptsDir string, profiles map[string]AgentProfile) error {
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", promptsDir, err)
	}

	installed := 0
	for name, profile := range profiles {
		targetPath := filepath.Join(promptsDir, name+".instructions.md")

		// Ensure parent directory exists for nested agent names
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			fmt.Printf("  Warning: failed to create dir for %s: %v\n", targetPath, err)
			continue
		}

		// Skip if already exists
		if _, err := os.Stat(targetPath); err == nil {
			continue
		}

		// Build VS Code Copilot instructions file
		// Strip YAML frontmatter from prompt — VS Code doesn't use it
		prompt := stripFrontmatter(profile.Prompt)

		content := fmt.Sprintf("---\nname: %s\ndescription: %s\napplyTo: '**'\n---\n\n%s",
			name, profile.Description, prompt)

		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			fmt.Printf("  Warning: failed to write %s: %v\n", targetPath, err)
			continue
		}
		installed++
	}

	if installed > 0 {
		fmt.Printf("  Installed %d agent profiles to %s\n", installed, promptsDir)
	}
	return nil
}

// claudeToolsString renders the enabled tools from a parsed profile as a
// comma-separated list of Claude-style tool names, in a stable order.
func claudeToolsString(perms map[string]string) string {
	// Ordered opencode tool name -> Claude display name.
	order := []struct{ oc, claude string }{
		{"read", "Read"},
		{"edit", "Edit"},
		{"write", "Write"},
		{"bash", "Bash"},
		{"glob", "Glob"},
		{"grep", "Grep"},
		{"lsp", "LSP"},
		{"ast_grep", "ASTGrep"},
		{"websearch", "WebSearch"},
		{"code_search", "CodeSearch"},
	}

	var names []string
	for _, t := range order {
		if v, ok := perms[t.oc]; ok && (v == "allow" || v == "ask") {
			names = append(names, t.claude)
		}
	}
	if len(names) == 0 {
		return "Read, Glob, Grep"
	}
	return strings.Join(names, ", ")
}

// stripFrontmatter removes YAML frontmatter from a markdown string.
func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	end := strings.Index(content[3:], "---")
	if end == -1 {
		return content
	}
	return strings.TrimSpace(content[end+6:])
}

// VSCodePromptsDir returns the VS Code User prompts directory for the current platform.
func VSCodePromptsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return vsCodeUserDir(home)
}

func vsCodeUserDir(home string) string {
	switch {
	case isDarwin():
		return filepath.Join(home, "Library", "Application Support", "Code", "User", "prompts")
	case isWindows():
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Code", "User", "prompts")
	default: // linux
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg == "" {
			xdg = filepath.Join(home, ".config")
		}
		return filepath.Join(xdg, "Code", "User", "prompts")
	}
}

func isDarwin() bool  { return runtime.GOOS == "darwin" }
func isWindows() bool { return runtime.GOOS == "windows" }

// AgentsSourceDir returns the path to ywai/agents/ directory.
func AgentsSourceDir() string {
	return config.AgentsSourceDir()
}

// InstallOpenCodeMarkdown writes agent profiles as .md files to ~/.config/opencode/agents/.
// This is the native OpenCode format and takes precedence over JSON configuration.
// When overwrite is true, existing files are overwritten.
func InstallOpenCodeMarkdown(agentsDir string, profiles map[string]AgentProfile, overwrite bool) error {
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", agentsDir, err)
	}

	installed := 0
	for name, profile := range profiles {
		targetPath := filepath.Join(agentsDir, name+".md")

		// Ensure parent directory exists for nested agent names
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			fmt.Printf("  Warning: failed to create dir for %s: %v\n", targetPath, err)
			continue
		}

		if !overwrite {
			if _, err := os.Stat(targetPath); err == nil {
				continue
			}
		}

		// Build OpenCode-style markdown with YAML frontmatter
		content := buildOpenCodeMarkdown(name, profile)

		if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
			fmt.Printf("  Warning: failed to write %s: %v\n", targetPath, err)
			continue
		}
		installed++
	}

	if installed > 0 {
		fmt.Printf("  Installed %d agent profiles to %s\n", installed, agentsDir)
	}
	return nil
}

// buildOpenCodeMarkdown converts an AgentProfile to OpenCode markdown format.
func buildOpenCodeMarkdown(name string, profile AgentProfile) string {
	var b strings.Builder

	// YAML frontmatter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("description: %s\n", profile.Description))
	b.WriteString(fmt.Sprintf("mode: %s\n", profile.Mode))
	b.WriteString("temperature: 0.1\n")

	// Permission as nested YAML
	b.WriteString("permission:\n")
	permOrder := []string{"read", "edit", "write", "bash", "glob", "grep", "lsp", "ast_grep", "websearch", "code_search", "webfetch", "task", "delegate", "question", "skill", "memory", "intercom", "ado", "mcp"}
	written := map[string]bool{}
	for _, key := range permOrder {
		if val, ok := profile.Permission[key]; ok {
			b.WriteString(fmt.Sprintf("  %s: %s\n", key, val))
			written[key] = true
		}
	}
	// Append any remaining permission keys not in permOrder (e.g. custom/MCP tools)
	var remaining []string
	for k := range profile.Permission {
		if !written[k] {
			remaining = append(remaining, k)
		}
	}
	// Simple sort
	for i := 0; i < len(remaining); i++ {
		for j := i + 1; j < len(remaining); j++ {
			if remaining[j] < remaining[i] {
				remaining[i], remaining[j] = remaining[j], remaining[i]
			}
		}
	}
	for _, key := range remaining {
		b.WriteString(fmt.Sprintf("  %s: %s\n", key, profile.Permission[key]))
	}
	b.WriteString("---\n\n")

	// Prompt body (strip frontmatter if present)
	prompt := stripFrontmatter(profile.Prompt)
	b.WriteString(prompt)

	return b.String()
}

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

	// Get agent section
	agentsRaw, ok := root["agent"]
	if !ok {
		return nil // No agents to migrate
	}

	agents, ok := agentsRaw.(map[string]any)
	if !ok {
		return nil // Invalid format, skip
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

		content := buildOpenCodeMarkdown(name, profile)
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
			// Remove agent section entirely if empty
			delete(root, "agent")
		} else {
			root["agent"] = agents
		}

		if err := config.WriteJSONC(configPath, root); err != nil {
			return fmt.Errorf("write updated config: %w", err)
		}
	}

	return nil
}

// mapToAgentProfile converts a map from JSON to AgentProfile.
func mapToAgentProfile(name string, m map[string]any) AgentProfile {
	prompt := ""
	if p, ok := m["prompt"].(string); ok {
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

	// Prefer permission over deprecated tools
	permission := map[string]string{"read": "allow", "edit": "allow", "write": "allow", "bash": "allow"}
	if perm, ok := m["permission"].(map[string]any); ok {
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

	// Build set of allowed agent names
	allowed := make(map[string]bool)

	// Core is always included
	if core, ok := manifest.Groups["core"]; ok {
		for _, name := range core.Agents {
			allowed[name] = true
		}
	}

	// Add requested groups
	for _, groupName := range filter.Groups {
		if def, ok := manifest.Groups[groupName]; ok {
			for _, name := range def.Agents {
				allowed[name] = true
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
		if allowed[name] {
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
