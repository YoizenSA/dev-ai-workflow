package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// AgentProfile represents a parsed agent from ywai/agents/.
type AgentProfile struct {
	Name        string
	Description string
	Prompt      string
	Tools       map[string]bool
	Skills      []string
	Mode        string
}

// toolMapping maps our tool names to opencode-style tool names.
var opencodeToolMap = map[string]string{
	"Read":       "read",
	"Edit":       "edit",
	"Write":      "write",
	"Bash":       "bash",
	"Glob":       "glob",
	"Grep":       "grep",
	"LSP":        "lsp",
	"ASTGrep":    "ast_grep",
	"WebSearch":  "web_search",
	"CodeSearch": "code_search",
}

// LoadProfiles reads all agent directories from the given source dir.
// Each subdirectory should have AGENT.md and tools.json.
func LoadProfiles(sourceDir string) (map[string]AgentProfile, error) {
	profiles := map[string]AgentProfile{}

	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("read agents dir %s: %w", sourceDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		agentDir := filepath.Join(sourceDir, entry.Name())
		profile, err := loadProfile(agentDir)
		if err != nil {
			fmt.Printf("  Warning: skip agent %s: %v\n", entry.Name(), err)
			continue
		}

		profiles[profile.Name] = *profile
	}

	return profiles, nil
}

func loadProfile(dir string) (*AgentProfile, error) {
	name := filepath.Base(dir)

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
	prompt = stripFrontmatter(prompt)

	// Read tools.json
	toolsFile := filepath.Join(dir, "tools.json")
	tools, err := parseOpenCodeTools(toolsFile)
	if err != nil {
		fmt.Printf("  Warning: agent %s tools.json error: %v, using defaults\n", name, err)
		tools = map[string]bool{"read": true, "edit": true, "write": true, "bash": true}
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
		Prompt:      promptWithSkills(prompt, skills),
		Tools:       tools,
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

// parseOpenCodeTools reads tools.json and converts to opencode tool map.
func parseOpenCodeTools(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw struct {
		Allowed []string `json:"allowed"`
		Denied  []string `json:"denied"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	// Build opencode tool map
	tools := map[string]bool{}

	// Start with all false for common tools
	for _, oc := range []string{"read", "edit", "write", "bash"} {
		tools[oc] = false
	}

	// Enable allowed tools
	for _, t := range raw.Allowed {
		if oc, ok := opencodeToolMap[t]; ok {
			tools[oc] = true
		}
	}

	// Explicitly disable denied tools
	for _, t := range raw.Denied {
		if oc, ok := opencodeToolMap[t]; ok {
			tools[oc] = false
		}
	}

	return tools, nil
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
			existingMap["tools"] = profile.Tools
			installed++
			continue
		}

		agents[name] = map[string]any{
			"mode":        profile.Mode,
			"description": profile.Description,
			"prompt":      profile.Prompt,
			"tools":       profile.Tools,
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

		// Skip if already exists
		if _, err := os.Stat(targetPath); err == nil {
			continue
		}

		// Build Claude-style agent file, deriving tools from the parsed profile.
		toolsStr := claudeToolsString(profile.Tools)

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
func claudeToolsString(tools map[string]bool) string {
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
		{"web_search", "WebSearch"},
		{"code_search", "CodeSearch"},
	}

	var names []string
	for _, t := range order {
		if tools[t.oc] {
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
