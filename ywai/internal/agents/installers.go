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
		if v, ok := perms[t.oc]; ok && toolStringEnabled(t.oc, v) {
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
		if v, ok := perms[t.oc]; ok && toolStringEnabled(t.oc, v) {
			names = append(names, t.claude)
		}
	}
	if len(names) == 0 {
		return "Read, Glob, Grep"
	}
	return strings.Join(names, ", ")
}

// toolStringEnabled reports whether a flat host tool string (Claude/PI) should
// include this opencode permission. bash: verify is OpenCode-only nested
// enforcement — other hosts get no shell rather than a full shell.
func toolStringEnabled(_, val string) bool {
	return val == "allow" || val == "ask"
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
		content := BuildOpenCodeMarkdown(name, profile)

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

// RemoveAgentsWithoutDescription deletes every flat .md in agentsDir whose
// frontmatter has no description (or an empty one). opencode rejects such files
// with "Expected string | undefined, got null description", so leaving them in
// place poisons the whole config. This sweeps up orphans left by past installs
// (e.g. stubs from the delegation bug, hand-edited files, half-written exports).
//
// Only top-level .md files are scanned — subdirectories are removed wholesale by
// removeLegacyGroupDirs. Returns the number of files removed.
func RemoveAgentsWithoutDescription(agentsDir string) int {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(agentsDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		// frontmatterDescription returns "" when there is no description field, no
		// frontmatter at all, or the value (inline or block scalar) is empty. All
		// three mean opencode will see null/undefined and refuse to load the agent.
		if strings.TrimSpace(frontmatterDescription(string(data))) != "" {
			continue
		}
		if err := os.Remove(path); err != nil {
			fmt.Printf("  Warning: failed to remove agent without description %s: %v\n", path, err)
			continue
		}
		removed++
		fmt.Printf("  Removed agent without description: %s\n", e.Name())
	}
	return removed
}

// ywaiBucketPatterns maps ywai's coarse permission buckets to the opencode-native
// wildcard patterns that actually gate the underlying MCP tools. opencode matches
// permission keys as globs against real tool names, so the bare bucket names
// (e.g. "memory") never match the prefixed tool ids (engram_mem_save) and are
// silently ignored. Expanding them here makes the
// generated permission block enforceable in opencode itself, not just inside
// a separate runtime gate.
//
// The blanket "mcp" bucket covers every MCP server not claimed by a dedicated
// bucket above. The static entries are the servers ywai installs itself;
// mcpBucketPatterns extends them with whatever else the user has configured, so
// granting "mcp" grants the MCP surface that actually exists on the machine
// rather than a list that has to be edited in Go for every new server.
var ywaiBucketPatterns = map[string][]string{
	"memory":   {"engram_*"},
	"intercom": {"intercom_*"},
	"mcp":      {"codegraph_*", "context7_*"},
	// "delegate" launches an async sub-agent (background-agents plugin); the
	// "delegation_*" glob covers the supervisor/retrieval tools (read, list,
	// status, peek, steer, stop). Without the glob, an agent whitelisted for
	// "delegate" under the "*: deny" pattern could start a delegation but never
	// read or control it.
	"delegate": {"delegate", "delegation_*"},
}

// falseGreenBashPatterns are commands that make a check pass without making the
// code correct. They are denied for every agent that can run bash.
//
// The failure they prevent is the expensive one: a suite that reports green
// while the behaviour is broken. Regenerating a snapshot turns a real assertion
// into a recording of current output, and an agent under pressure to finish
// reaches for it precisely when the test was about to be useful. No amount of
// prompt instruction reliably stops that — a denied command does.
//
// Formatters and linters that rewrite code are deliberately NOT here: fixing
// style is not faking a result.
var falseGreenBashPatterns = []string{
	// Snapshot rewriting, across the runners in use here.
	"* -u",
	"* -u *",
	"*--update-snapshot*",
	"*--updateSnapshot*",
	"*vitest*--update*",
	"*jest*--ci=false*",
	"go test *-update*",
	// Skipping the check rather than passing it.
	"*--passWithNoTests*",
	"go test *-run TestNothing*",
	// Silencing the type checker instead of satisfying it.
	"*tsc*--noEmitOnError*",
	"*tsc*--skipLibCheck*--noEmit*",
}

// verifyBashAllowPatterns is the small shell surface for bash: verify agents
// (orchestrators). Deny-by-default with these allows: read-only git inspect plus
// a multi-lang test/lint set so the coordinator can spot-check handoffs without
// a write shell. Covers stacks ywai agents hit regularly (Go, JS/TS, .NET,
// Python); keep the list verification-shaped only (no install, no format-write).
var verifyBashAllowPatterns = []string{
	"git diff*",
	"git status*",
	"git log*",
	"git show*",
	// Go
	"go test*",
	// JS / TS
	"npm test*",
	"npm run lint*",
	"npm run build*",
	"npm run typecheck*",
	"npx tsc --noEmit*",
	"pnpm test*",
	"pnpm run lint*",
	"pnpm run build*",
	"yarn test*",
	"yarn lint*",
	// .NET
	"dotnet test*",
	"dotnet build*",
	// Python
	"pytest*",
	"python -m pytest*",
	"python3 -m pytest*",
	"python -m unittest*",
	"python3 -m unittest*",
	"ruff check*",
	"mypy *",
	"mypy*",
}

// noCommitBashDenyPatterns block commit/push for code executors. Review-then-
// commit: edits land via the executor; release actions stay with the
// coordinator/user after review. OpenCode is the enforcement authority; Claude
// and PI do not get nested bash rules.
var noCommitBashDenyPatterns = []string{
	"git commit*",
	"git push*",
	"git * commit*",
	"git * push*",
	"env git commit*",
	"env git push*",
	"git.exe commit*",
	"git.exe push*",
	"git.exe * commit*",
	"git.exe * push*",
}

// noCommitAgents may edit code but must not commit or push. devops is excluded
// on purpose: deploy flows legitimately push.
var noCommitAgents = map[string]bool{
	"dev":    true,
	"qa-dev": true,
}

// openCodeNativePermissionKeys are tools that remain deny-by-default when an
// agent omits them from permissions.json. MCP server tools intentionally do not
// appear here: OpenCode leaves unlisted tools enabled, so newly configured MCPs
// are available immediately and can be disabled by name when needed.
var openCodeNativePermissionKeys = []string{
	"read", "edit", "write", "bash", "glob", "grep", "list", "lsp",
	"ast_grep", "websearch", "code_search", "webfetch", "task", "todowrite",
	"delegate", "question", "skill", "external_directory", "doom_loop",
}

// ExpandPermissionBuckets returns a copy of perms with ywai's coarse permission
// buckets (ado, memory, intercom, mcp) expanded to the opencode-native wildcard
// patterns that actually gate the underlying tools. Keys without a bucket mapping
// pass through unchanged. This mirrors the expansion BuildOpenCodeMarkdown applies
// at install time so permissions written by any other path (e.g. the config API
// permissions API patching frontmatter in place) stay enforceable in opencode
// instead of leaving bare bucket names that opencode silently ignores.
func ExpandPermissionBuckets(perms map[string]string) map[string]string {
	out := make(map[string]string, len(perms))
	for key, val := range perms {
		patterns := bucketPatterns(key)
		if patterns == nil {
			out[key] = val
			continue
		}
		for _, p := range patterns {
			// An explicit per-tool rule the profile already set (e.g.
			// engram_mem_save: deny) must survive the broader bucket.
			if _, exists := perms[p]; exists && p != key {
				continue
			}
			out[p] = val
		}
	}
	// Re-apply explicit keys last so a narrow rule always beats the bucket it
	// overlaps, regardless of map iteration order.
	for key, val := range perms {
		if bucketPatterns(key) == nil {
			out[key] = val
		}
	}
	return out
}

// bucketPatterns returns the tool globs a coarse bucket expands to, or nil when
// the key is not a bucket. The "mcp" bucket is resolved dynamically so it
// covers the servers configured on this machine, not just the ones ywai ships.
func bucketPatterns(key string) []string {
	if key == "mcp" {
		return mcpBucketPatterns()
	}
	patterns, ok := ywaiBucketPatterns[key]
	if !ok {
		return nil
	}
	return patterns
}

// mcpServersFunc resolves the configured MCP servers. It is a variable so the
// permission expansion stays deterministic under test: reading the developer's
// real opencode.json would make results depend on whichever MCP servers happen
// to be installed on the machine running the suite.
var mcpServersFunc = ConfiguredMCPServers

// SetMCPServerResolver overrides how the blanket "mcp" bucket discovers
// servers, and returns a function restoring the previous resolver. Intended for
// tests and for callers exporting against a config other than the user's own.
func SetMCPServerResolver(fn func() []string) func() {
	prev := mcpServersFunc
	if fn == nil {
		fn = ConfiguredMCPServers
	}
	mcpServersFunc = fn
	return func() { mcpServersFunc = prev }
}

// ConfiguredMCPServers lists the MCP server ids present in the user's
// opencode.json, sorted. Missing or unreadable config yields nil — callers fall
// back to the static patterns rather than failing an install over it.
func ConfiguredMCPServers() []string {
	path := config.FindJSONCPath(config.OpenCodeConfigDir(), "opencode")
	root, err := config.ReadJSONC(path)
	if err != nil {
		return nil
	}
	raw, ok := root["mcp"].(map[string]any)
	if !ok {
		return nil
	}
	servers := make([]string, 0, len(raw))
	for name := range raw {
		if strings.TrimSpace(name) != "" {
			servers = append(servers, name)
		}
	}
	sort.Strings(servers)
	return servers
}

// dedicatedBucketPrefixes are MCP servers that already have their own bucket,
// so the blanket "mcp" bucket must not also claim them — otherwise granting
// "mcp" would silently widen an agent that was deliberately denied "memory".
var dedicatedBucketPrefixes = map[string]bool{
	"engram":   true, // memory
	"intercom": true,
}

// mcpBucketPatterns is the blanket MCP grant: the servers ywai ships plus every
// other MCP server configured in opencode.json.
//
// MCP tools are meant to be available to every kind of agent — a reviewer that
// cannot reach the docs server, or a designer that cannot drive the browser, is
// crippled for no security gain, since the tools are the user's own. Deriving
// the list from the live config means adding an MCP server makes it usable
// everywhere without a code change here.
func mcpBucketPatterns() []string {
	patterns := append([]string(nil), ywaiBucketPatterns["mcp"]...)
	seen := map[string]bool{}
	for _, p := range patterns {
		seen[p] = true
	}
	for _, server := range mcpServersFunc() {
		if dedicatedBucketPrefixes[server] {
			continue
		}
		// A retired server may still sit in the config when profiles are
		// written (cleanup runs later in the install), so granting from the
		// raw config would reinstate the dead permission.
		if config.IsRetiredMCPServer(server) {
			continue
		}
		p := server + "_*"
		if !seen[p] {
			patterns = append(patterns, p)
			seen[p] = true
		}
	}
	sort.Strings(patterns)
	return patterns
}

// BuildOpenCodeMarkdown converts an AgentProfile to OpenCode markdown format.
// Exported so the workflows exporter can reuse the single source of truth for
// permission rendering and bucket expansion (the workflow's sub-agent nodes
// become opencode agents, and must follow the exact same frontmatter rules).
func BuildOpenCodeMarkdown(name string, profile AgentProfile) string {
	var b strings.Builder

	// YAML frontmatter. A bare "description:" (empty value) parses as YAML null,
	// which opencode rejects with "Expected string | undefined, got null
	// description". Fall back to the agent name so the field is always a non-empty
	// string regardless of how the profile was built (loader, migration,
	// workflows exporter, config API).
	description := strings.TrimSpace(profile.Description)
	if description == "" {
		description = name
	}

	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("description: %s\n", description))
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
		// bash renders as a nested allow/deny map when the agent may run
		// commands at all, so specific commands can be denied inside a general
		// allow. permissions.json is a flat string map and cannot express that.
		//
		// Modes:
		//   deny   → single-line deny (no shell)
		//   verify → "*": deny + verify allows + false-green denials
		//   allow  → "*": allow + false-green denials [+ no-commit denials]
		if key == "bash" && val != "deny" {
			b.WriteString("  bash:\n")
			if val == "verify" {
				b.WriteString(fmt.Sprintf("    %q: deny\n", "*"))
				for _, pattern := range verifyBashAllowPatterns {
					b.WriteString(fmt.Sprintf("    %q: allow\n", pattern))
				}
			} else {
				b.WriteString(fmt.Sprintf("    %q: %s\n", "*", val))
			}
			for _, pattern := range falseGreenBashPatterns {
				b.WriteString(fmt.Sprintf("    %q: deny\n", pattern))
			}
			// Profile keys may be grouped ("core/dev", "qa-automation/qa-dev");
			// match the flat agent id OpenCode registers.
			if noCommitAgents[filepath.Base(name)] && val != "verify" {
				for _, pattern := range noCommitBashDenyPatterns {
					b.WriteString(fmt.Sprintf("    %q: deny\n", pattern))
				}
			}
			return
		}
		if patterns, ok := ywaiBucketPatterns[key]; ok {
			for _, p := range patterns {
				// Quote the key: it contains glob/hyphen chars (e.g. codegraph_*).
				b.WriteString(fmt.Sprintf("  %q: %s\n", p, val))
			}
			return
		}
		// Quote keys with YAML-special characters (*, :, #, etc.).
		if key == "*" || strings.ContainsAny(key, "*:#&!|>',[]{}%`@") {
			b.WriteString(fmt.Sprintf("  %q: %s\n", key, val))
		} else {
			b.WriteString(fmt.Sprintf("  %s: %s\n", key, val))
		}
	}

	// Emit explicit restrictions, but do not add a catch-all "*: deny" rule.
	// OpenCode enables unlisted tools by default, which keeps MCP servers added
	// outside ywai available without requiring a release for each new server.
	// Native tool restrictions remain explicit in each profile's permissions.json.
	written := map[string]bool{}
	for _, key := range openCodeNativePermissionKeys {
		val := profile.Permission[key]
		if val == "" {
			val = "deny"
		}
		emitPermission(key, val)
		written[key] = true
	}
	// Append custom tool permissions deterministically. They include explicit
	// MCP overrides, which still take precedence over the permissive default.
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
	b.WriteString("---\n\n")

	// Prompt body (strip frontmatter if present)
	prompt := stripFrontmatter(profile.Prompt)
	b.WriteString(prompt)

	return b.String()
}

// TeammateProfile represents a PI.dev teammate profile JSON structure.
type TeammateProfile struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Model        string   `json:"model"`
	Thinking     bool     `json:"thinking"`
	SystemPrompt string   `json:"system_prompt"`
	Prompt       string   `json:"prompt"`
	Tools        []string `json:"tools"`
	Skills       []string `json:"skills"`
}

// opencodeToPiTool maps opencode tool names to PI.dev tool names.
var opencodeToPiTool = map[string]string{
	"read":      "Read",
	"edit":      "Edit",
	"write":     "Write",
	"bash":      "Bash",
	"glob":      "Glob",
	"grep":      "Grep",
	"websearch": "WebSearch",
}

// agentDefaults returns the static defaults (description, tools, model) for a given agent name.
// An empty model string means "inherit from lead agent".
func agentDefaults(name string) (description string, tools []string, model string) {
	switch name {
	case "orchestrator":
		return "Technical lead that decomposes work and delegates to subagents",
			[]string{"member_prompt", "member_steer", "member_wait", "task_*", "message_*"}, ""
	case "dev":
		return "Implements features, fixes bugs, and refactors code",
			[]string{"Read", "Write", "Edit", "Bash", "Grep", "Glob"}, ""
	case "qa":
		return "Writes and runs tests, ensures quality",
			[]string{"Read", "Write", "Edit", "Bash", "Grep"}, ""
	case "architect":
		return "Designs architecture and makes technical decisions",
			[]string{"Read", "Write", "Edit", "Grep", "Glob", "Bash"}, ""
	case "designer":
		// Read-only like architect: specs and design findings, never the diff.
		return "Designs and audits UI/UX against the design system and accessibility standards",
			[]string{"Read", "Grep", "Glob"}, ""
	case "reviewer":
		return "Reviews code for correctness and quality",
			[]string{"Read", "Grep", "Glob", "Bash"}, ""
	case "devops":
		return "Manages CI/CD, deployments, and infrastructure",
			[]string{"Read", "Write", "Edit", "Bash"}, ""
	case "finder":
		return "Explores codebase and searches for information",
			[]string{"Read", "Grep", "Glob", "Bash"}, "anthropic/claude-haiku-4-5"
	case "ask":
		return "Answers questions and does research",
			[]string{"Read", "WebSearch"}, "anthropic/claude-haiku-4-5"
	default:
		return "", nil, ""
	}
}

// convertPermissionsToPiTools converts an opencode Permission map to a sorted list of PI.dev tool names.
func convertPermissionsToPiTools(perm map[string]string) []string {
	var tools []string
	for ocTool, val := range perm {
		if val == "allow" {
			if piTool, ok := opencodeToPiTool[ocTool]; ok {
				tools = append(tools, piTool)
			}
		}
	}
	sort.Strings(tools)
	return tools
}

// InstallPiTeamProfiles generates PI.dev teammate profile JSON files for all agent profiles.
func InstallPiTeamProfiles(agentsDir string, profiles map[string]AgentProfile, overwrite bool) error {
	teamDir := filepath.Join(agentsDir, "team-profiles")
	if err := os.MkdirAll(teamDir, 0o755); err != nil {
		return fmt.Errorf("create team-profiles dir: %w", err)
	}

	for _, profile := range profiles {
		targetPath := filepath.Join(teamDir, profile.Name+".json")
		if _, err := os.Stat(targetPath); err == nil && !overwrite {
			continue // skip existing
		}

		desc, tools, model := agentDefaults(profile.Name)
		if desc == "" {
			continue // unknown agent, skip
		}

		permTools := convertPermissionsToPiTools(profile.Permission)
		tools = append(tools, permTools...)
		sort.Strings(tools)
		tools = uniqueSortedStrings(tools)

		tp := TeammateProfile{
			Name:         profile.Name,
			Description:  desc,
			Model:        model,
			Thinking:     false,
			SystemPrompt: profile.Prompt,
			Prompt:       profile.Prompt,
			Tools:        tools,
			Skills:       profile.Skills,
		}

		data, err := json.MarshalIndent(tp, "", "  ")
		if err != nil {
			fmt.Printf("  Warning: failed to marshal %s: %v\n", profile.Name, err)
			continue
		}

		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			fmt.Printf("  Warning: failed to write %s: %v\n", targetPath, err)
			continue
		}
	}

	return nil
}

// uniqueSortedStrings deduplicates a sorted string slice in place.
func uniqueSortedStrings(s []string) []string {
	if len(s) < 2 {
		return s
	}
	result := make([]string, 0, len(s))
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			result = append(result, v)
		}
	}
	return result
}
