package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// AgentProfile represents a parsed agent from ywai/agents/.
type AgentProfile struct {
	Name        string
	Description string
	Prompt      string
	Tools       map[string]bool
	Skills      []string
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
			if line != "" {
				skills = append(skills, line)
			}
		}
	}

	return &AgentProfile{
		Name:        name,
		Description: description,
		Prompt:      prompt,
		Tools:       tools,
		Skills:      skills,
	}, nil
}

// extractDescription gets the first line after the frontmatter that looks like a description.
func extractDescription(prompt string) string {
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

// InstallOpenCode injects agent profiles into opencode.json.
func InstallOpenCode(configPath string, profiles map[string]AgentProfile) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", configPath, err)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse %s: %w", configPath, err)
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
		// Skip if already present (don't overwrite existing agents)
		if _, exists := agents[name]; exists {
			continue
		}

		agents[name] = map[string]any{
			"mode":        "primary",
			"description": profile.Description,
			"prompt":      profile.Prompt,
			"tools":       profile.Tools,
		}
		installed++
	}

	if installed == 0 {
		return nil
	}

	updated, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	updated = append(updated, '\n')

	if err := os.WriteFile(configPath, updated, 0o644); err != nil {
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

		// Build Claude-style agent file
		toolsStr := "Read, Edit, Write, Glob, Grep, Bash"
		switch name {
		case "ask", "architect", "reviewer":
			toolsStr = "Read, Glob, Grep"
		case "qa":
			toolsStr = "Read, Edit, Write, Bash, Glob, Grep"
		}

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
	// Check repo root first
	repoAgents := filepath.Join(repoRoot(), "agents")
	if isDirPopulated(repoAgents) {
		return repoAgents
	}

	// Check data dir
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dataAgents := filepath.Join(home, ".ywai", "agents")
	if isDirPopulated(dataAgents) {
		return dataAgents
	}

	return repoAgents
}

func repoRoot() string {
	if cwd, err := os.Getwd(); err == nil {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd
		}
	}
	return "."
}

func isDirPopulated(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}
