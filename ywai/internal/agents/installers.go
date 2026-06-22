package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

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

// InstallOpenCodeMarkdown writes agent profiles as flat .md files to
// ~/.config/opencode/agents/. opencode derives the agent id from the file's
// path under the agents dir, so agents are written FLAT at the root using their
// base name (e.g. "orchestrator.md", not "core/orchestrator.md") to match the
// flat ids the role-defaults reference. Group membership is preserved via the
// `group:` frontmatter field, not the directory layout. When overwrite is true,
// existing files are overwritten.
func InstallOpenCodeMarkdown(agentsDir string, profiles map[string]AgentProfile, overwrite bool) error {
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", agentsDir, err)
	}

	installed := 0
	for name, profile := range profiles {
		// Always install flat at the root using the base name so opencode
		// registers the agent under its flat id (e.g. "orchestrator").
		targetPath := filepath.Join(agentsDir, filepath.Base(name)+".md")

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

	// Drop any grouped/legacy subdirectories: opencode derives the agent id from
	// the file path, so a nested copy (e.g. core/orchestrator.md) registers as
	// "core/orchestrator" and shadows the canonical flat "orchestrator" id.
	removeLegacyGroupDirs(agentsDir)

	return nil
}

// removeLegacyGroupDirs deletes every subdirectory under the agents dir. The
// flat layout is the only valid one (opencode registers nested files under a
// path-derived id that shadows the flat id), so any subdirectory — whether it
// came from a current group profile or an orphaned legacy install whose profile
// no longer exists — is removed wholesale. Top-level flat .md files are left
// untouched.
func removeLegacyGroupDirs(agentsDir string) {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(agentsDir, e.Name())
		if err := os.RemoveAll(dir); err != nil {
			fmt.Printf("  Warning: failed to remove legacy agent dir %s: %v\n", dir, err)
			continue
		}
		fmt.Printf("  Removed legacy grouped agent dir %s\n", e.Name())
	}
}

// ywaiBucketPatterns maps ywai's coarse permission buckets to the opencode-native
// wildcard patterns that actually gate the underlying MCP tools. opencode matches
// permission keys as globs against real tool names, so the bare bucket names
// (e.g. "ado", "memory") never match the prefixed tool ids (ado_pr_create,
// engram_mem_save) and are silently ignored. Expanding them here makes the
// generated permission block enforceable in opencode itself, not just inside
// ywai's own MCPEnforce layer.
//
// The blanket "mcp" bucket enumerates every MCP server not covered by a
// dedicated bucket above. Kept explicit (one pattern per server) so a new MCP
// server is a visible, deliberate addition rather than a silent catch-all.
var ywaiBucketPatterns = map[string][]string{
	"ado":      {"ado_*"},
	"memory":   {"engram_*"},
	"intercom": {"intercom_*"},
	"mcp":      {"codegraph_*", "context7_*", "ywai-kanban_*"},
}

// ExpandPermissionBuckets returns a copy of perms with ywai's coarse permission
// buckets (ado, memory, intercom, mcp) expanded to the opencode-native wildcard
// patterns that actually gate the underlying tools. Keys without a bucket mapping
// pass through unchanged. This mirrors the expansion buildOpenCodeMarkdown applies
// at install time so permissions written by any other path (e.g. the kanban
// permissions API patching frontmatter in place) stay enforceable in opencode
// instead of leaving bare bucket names that opencode silently ignores.
func ExpandPermissionBuckets(perms map[string]string) map[string]string {
	out := make(map[string]string, len(perms))
	for key, val := range perms {
		if patterns, ok := ywaiBucketPatterns[key]; ok {
			for _, p := range patterns {
				out[p] = val
			}
			continue
		}
		out[key] = val
	}
	return out
}

// buildOpenCodeMarkdown converts an AgentProfile to OpenCode markdown format.
func buildOpenCodeMarkdown(name string, profile AgentProfile) string {
	var b strings.Builder

	// YAML frontmatter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("description: %s\n", profile.Description))
	b.WriteString(fmt.Sprintf("mode: %s\n", profile.Mode))
	b.WriteString("temperature: 0.1\n")
	if profile.Group != "" {
		b.WriteString(fmt.Sprintf("group: %s\n", profile.Group))
	}

	// Permission as nested YAML using the "*: deny" + whitelist pattern
	// (same as opencode's built-in agents like explore). This ensures only
	// the explicitly allowed tools are exposed to the LLM — denied tools
	// are filtered out by opencode's resolveTools() before the request.
	//
	// ywai's coarse buckets (ado, memory, intercom, mcp) are expanded to
	// opencode-native wildcard patterns so the deny/allow is actually
	// enforced by opencode, not silently dropped.
	b.WriteString("permission:\n")
	emitPermission := func(key, val string) {
		if patterns, ok := ywaiBucketPatterns[key]; ok {
			for _, p := range patterns {
				// Quote the key: it contains glob/hyphen chars (e.g. ywai-kanban_*).
				b.WriteString(fmt.Sprintf("  %q: %s\n", p, val))
			}
			return
		}
		b.WriteString(fmt.Sprintf("  %s: %s\n", key, val))
	}

	// Check if any tool is denied — if so, use "*: deny" + whitelist pattern.
	hasDeny := false
	for _, v := range profile.Permission {
		if v == "deny" {
			hasDeny = true
			break
		}
	}

	if hasDeny {
		// Emit "*: deny" first (blocks everything by default).
		emitPermission("*", "deny")
		// Then emit only the "allow" rules (whitelist).
		allowOrder := []string{"read", "edit", "write", "bash", "glob", "grep", "lsp", "ast_grep", "websearch", "code_search", "webfetch", "task", "delegate", "question", "skill", "memory", "intercom", "ado", "mcp"}
		written := map[string]bool{}
		for _, key := range allowOrder {
			if val, ok := profile.Permission[key]; ok && val == "allow" {
				emitPermission(key, val)
				written[key] = true
			}
		}
		// Append any remaining allow keys not in allowOrder (e.g. custom/MCP tools).
		var remaining []string
		for k, v := range profile.Permission {
			if v == "allow" && !written[k] {
				remaining = append(remaining, k)
			}
		}
		sort.Strings(remaining)
		for _, key := range remaining {
			emitPermission(key, profile.Permission[key])
		}
	} else {
		// No deny rules — emit all permissions as-is (full access agent).
		permOrder := []string{"read", "edit", "write", "bash", "glob", "grep", "lsp", "ast_grep", "websearch", "code_search", "webfetch", "task", "delegate", "question", "skill", "memory", "intercom", "ado", "mcp"}
		written := map[string]bool{}
		for _, key := range permOrder {
			if val, ok := profile.Permission[key]; ok {
				emitPermission(key, val)
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
		sort.Strings(remaining)
		for _, key := range remaining {
			emitPermission(key, profile.Permission[key])
		}
	}
	b.WriteString("---\n\n")

	// Prompt body (strip frontmatter if present)
	prompt := stripFrontmatter(profile.Prompt)
	b.WriteString(prompt)

	return b.String()
}
