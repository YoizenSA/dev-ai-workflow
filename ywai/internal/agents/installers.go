package agents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

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

// toolsMapping is one ordered entry of a host tool table: the opencode
// permission key and the host's display name for it.
type toolsMapping struct {
	oc   string
	host string
}

// toolsString renders the enabled tools from a parsed profile as a
// comma-separated list of host tool names, in table order.
func toolsString(perms map[string]string, table []toolsMapping, fallback string) string {
	var names []string
	for _, t := range table {
		if v, ok := perms[t.oc]; ok && toolStringEnabled(t.oc, v) {
			names = append(names, t.host)
		}
	}
	if len(names) == 0 {
		return fallback
	}
	return strings.Join(names, ", ")
}

var (
	// piToolsTable orders PI.dev tool names (lowercase).
	piToolsTable = []toolsMapping{
		{"read", "read"},
		{"edit", "edit"},
		{"write", "write"},
		{"bash", "bash"},
		{"glob", "glob"},
		{"grep", "grep"},
		{"webfetch", "webfetch"},
		{"websearch", "websearch"},
	}

	// ompToolsTable maps opencode permission keys to OMP tool names.
	ompToolsTable = []toolsMapping{
		{"read", "read"},
		{"edit", "edit"},
		{"write", "write"},
		{"bash", "bash"},
		{"glob", "glob"},
		{"grep", "grep"},
		{"websearch", "web_search"},
		{"task", "task"},
		{"todowrite", "todo"},
		{"question", "ask"},
	}

	// claudeToolsTable maps opencode tool names to Claude display names.
	claudeToolsTable = []toolsMapping{
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
)

// piToolsString renders the enabled tools from a parsed profile as a
// lowercase comma-separated list of PI.dev-style tool names, in stable order.
func piToolsString(perms map[string]string) string {
	return toolsString(perms, piToolsTable, "read, glob, grep")
}

// ompToolsString maps ywai permissions to OMP tool names (lowercase, comma-separated).
func ompToolsString(perms map[string]string) string {
	return toolsString(perms, ompToolsTable, "read, glob, grep")
}

// claudeToolsString renders the enabled tools from a parsed profile as a
// comma-separated list of Claude-style tool names, in a stable order.
func claudeToolsString(perms map[string]string) string {
	return toolsString(perms, claudeToolsTable, "Read, Glob, Grep")
}

// InstallPi writes agent .md files to ~/.pi/agent/agents/.
// Frontmatter uses PI.dev format: lowercase name/description/tools, no mode/permission.
// Respects overwrite: when false, skips existing files (same as InstallOpenCodeMarkdown).
func InstallPi(agentsDir string, profiles map[string]AgentProfile, overwrite bool) error {
	return installPiStyleAgents(agentsDir, profiles, overwrite, piToolsString, false)
}

// InstallOmp writes core + qa-automation agent .md files to ~/.omp/agent/agents/
// for oh-my-pi. Same markdown shape as Pi (name/description/tools), flat basenames.
// Migration/social groups are skipped — use OpenCode for those catalogs.
func InstallOmp(agentsDir string, profiles map[string]AgentProfile, overwrite bool) error {
	return installPiStyleAgents(agentsDir, FilterOmpInstallProfiles(profiles), overwrite, ompToolsString, true)
}

// FilterCoreAgentProfiles keeps agents from the core group (or known core base
// names when Group is unset). Keys are flattened to the OpenCode/OMP id
// (filepath.Base).
func FilterCoreAgentProfiles(profiles map[string]AgentProfile) map[string]AgentProfile {
	return filterProfilesByGroups(profiles, map[string]bool{"core": true}, coreAgentBases())
}

// FilterOmpInstallProfiles keeps core + qa-automation groups for OMP installs.
func FilterOmpInstallProfiles(profiles map[string]AgentProfile) map[string]AgentProfile {
	return filterProfilesByGroups(profiles, map[string]bool{
		"core": true, "qa-automation": true,
	}, coreAgentBases())
}

func coreAgentBases() map[string]bool {
	return map[string]bool{
		"orchestrator": true, "ask": true, "dev": true, "qa": true,
		"architect": true, "designer": true, "advisor": true, "reviewer": true,
		"devops": true, "finder": true, "memory": true, "planning": true,
	}
}

func filterProfilesByGroups(profiles map[string]AgentProfile, groups map[string]bool, emptyGroupBases map[string]bool) map[string]AgentProfile {
	out := make(map[string]AgentProfile, len(profiles))
	for name, p := range profiles {
		base := filepath.Base(name)
		keep := groups[p.Group]
		if p.Group == "" && emptyGroupBases[base] {
			keep = true
		}
		// qa-* basenames when group is empty but name looks like qa-automation
		if !keep && p.Group == "" && strings.HasPrefix(base, "qa-") {
			keep = groups["qa-automation"]
		}
		if !keep {
			continue
		}
		p.Name = base
		out[base] = p
	}
	return out
}

// installPiStyleAgents is shared by Pi and OMP markdown agent installs.
// When flat is true, nested keys like core/dev become dev.md at agentsDir root.
func installPiStyleAgents(
	agentsDir string,
	profiles map[string]AgentProfile,
	overwrite bool,
	toolsFn func(map[string]string) string,
	flat bool,
) error {
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", agentsDir, err)
	}

	installed := 0
	for name, profile := range profiles {
		id := name
		if flat {
			id = filepath.Base(name)
		}
		targetPath := filepath.Join(agentsDir, id+".md")

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			fmt.Printf("  Warning: failed to create dir for %s: %v\n", targetPath, err)
			continue
		}

		if !overwrite {
			if _, err := os.Stat(targetPath); err == nil {
				continue
			}
		}

		toolsStr := toolsFn(profile.Permission)
		prompt := stripFrontmatter(profile.Prompt)
		desc := profile.Description
		if desc == "" {
			desc = id
		}

		content := fmt.Sprintf("---\nname: %s\ndescription: >\n  %s\ntools: %s\n---\n\n%s",
			id, desc, toolsStr, prompt)

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
			name, yamlScalar(profile.Description), prompt)

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
	PruneUnlistedAgents(agentsDir, profiles)

	if err := WriteGroupSidecar(agentsDir, profiles); err != nil {
		fmt.Printf("  Warning: failed to write group sidecar: %v\n", err)
	}

	return nil
}

// GroupSidecarFile holds agent -> group membership beside the flat agent files.
// It replaces the `group:` frontmatter field, which opencode v2 rejects as an
// unknown agent key (see BuildOpenCodeMarkdown). The leading dot keeps it out
// of opencode's *.md agent discovery.
const GroupSidecarFile = ".ywai-groups.json"

// WriteGroupSidecar records the group of every installed profile. Agents with
// no group are omitted, so an empty map removes the file instead of leaving a
// stale one behind.
func WriteGroupSidecar(agentsDir string, profiles map[string]AgentProfile) error {
	groups := make(map[string]string, len(profiles))
	for name, profile := range profiles {
		if profile.Group != "" {
			groups[filepath.Base(name)] = profile.Group
		}
	}
	path := filepath.Join(agentsDir, GroupSidecarFile)
	if len(groups) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// ReadGroupSidecar returns the recorded group for one agent, or "" when the
// sidecar is missing or does not list it.
func ReadGroupSidecar(agentsDir, agentName string) string {
	data, err := os.ReadFile(filepath.Join(agentsDir, GroupSidecarFile))
	if err != nil {
		return ""
	}
	var groups map[string]string
	if err := json.Unmarshal(data, &groups); err != nil {
		return ""
	}
	return groups[agentName]
}

// PruneUnlistedAgents deletes flat *.md files whose base name is not in
// profiles. OpenCode v2 discovers every file in the agents dir, so leftovers
// from old groups or other installers (gentle-orchestrator, goal-*) stay
// live until swept. Returns how many files were removed.
func PruneUnlistedAgents(agentsDir string, keep map[string]AgentProfile) int {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return 0
	}
	want := make(map[string]bool, len(keep))
	for k := range keep {
		want[filepath.Base(k)] = true
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".md")
		if want[base] {
			continue
		}
		if err := os.Remove(filepath.Join(agentsDir, e.Name())); err != nil {
			fmt.Printf("  Warning: failed to prune leftover agent %s: %v\n", e.Name(), err)
			continue
		}
		removed++
		fmt.Printf("  Removed leftover agent %s\n", e.Name())
	}
	return removed
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

// openCodeNativePermissionKeys are the tools opencode v1 gates by name. Every
// Hosts only read their own agents/ folders; ~/.ywai/agent-backups stays private.
var agentBackupRootFn = func() string {
	return filepath.Join(config.DataDir(), "agent-backups")
}

// WriteAgentBackup stores a pre-overwrite snapshot under ~/.ywai/agent-backups,
// never as a sibling of the live agent file.
func WriteAgentBackup(livePath string, content []byte) error {
	dest, err := backupDestPath(filepath.Base(livePath) + ".bak")
	if err != nil {
		return err
	}
	return os.WriteFile(dest, content, 0o644)
}

func backupDestPath(name string) (string, error) {
	root := agentBackupRootFn()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(root, name)
	if _, err := os.Stat(dest); err == nil {
		dest = filepath.Join(root, fmt.Sprintf("%s.%d.bak", strings.TrimSuffix(name, ".bak"), time.Now().UnixNano()))
	}
	return dest, nil
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

// openCodeNativePermissionKeys are the tools opencode v1 gates by name. Every
// one is emitted explicitly (defaulting to deny) so a profile's silence is not
// read as permission.
var openCodeNativePermissionKeys = []string{
	"read", "edit", "write", "bash", "glob", "grep", "list", "lsp",
	"ast_grep", "websearch", "code_search", "webfetch", "task", "todowrite",
	"delegate", "question", "skill", "external_directory", "doom_loop",
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
	"graft":    {"graft_*"},
	"mcp":      {"graft_*", "context7_*"},
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

// BashPermissionBlockLines is the v1 renderer kept only for the kilocode (v1
// fork) JSON install path. OpenCode v2 agents get shell rules via
// RulesFromPermissionMap instead.
func BashPermissionBlockLines(val, baseName string) []string {
	if val == "deny" {
		return []string{"  bash: deny"}
	}
	lines := []string{"  bash:"}
	if val == "verify" {
		lines = append(lines, fmt.Sprintf("    %q: deny", "*"))
		for _, pattern := range verifyBashAllowPatterns {
			lines = append(lines, fmt.Sprintf("    %q: allow", pattern))
		}
	} else {
		lines = append(lines, fmt.Sprintf("    %q: %s", "*", val))
	}
	for _, pattern := range falseGreenBashPatterns {
		lines = append(lines, fmt.Sprintf("    %q: deny", pattern))
	}
	if noCommitAgents[baseName] && val != "verify" {
		for _, pattern := range noCommitBashDenyPatterns {
			lines = append(lines, fmt.Sprintf("    %q: deny", pattern))
		}
	}
	return lines
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
	set := map[string]bool{}
	// v1 layout: servers directly under "mcp" (skip reserved v2 keys).
	if raw, ok := root["mcp"].(map[string]any); ok {
		for name := range raw {
			if name == "servers" || name == "timeout" {
				continue
			}
			if strings.TrimSpace(name) != "" {
				set[name] = true
			}
		}
	}
	// v2 layout: servers nested under mcp.servers.
	if mcp, ok := root["mcp"].(map[string]any); ok {
		if servers, ok := mcp["servers"].(map[string]any); ok {
			for name := range servers {
				if strings.TrimSpace(name) != "" {
					set[name] = true
				}
			}
		}
	}
	if len(set) == 0 {
		return nil
	}
	servers := make([]string, 0, len(set))
	for name := range set {
		servers = append(servers, name)
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

// RenderPermissionMapYAML renders an agent's permission map as the nested
// opencode v1 `permission:` frontmatter block. Exported so the config API
// rewrites permissions through the same code that installs them — two
// renderers drift, and a drifted one silently changes what an agent may do.
func RenderPermissionMapYAML(name string, profile AgentProfile) string {
	var out strings.Builder
	out.WriteString("permission:\n")
	emitPermission := func(key, val string) {
		// bash renders as a nested allow/deny map when the agent may run
		// commands at all, so specific commands can be denied inside a general
		// allow. permissions.json is a flat string map and cannot express that.
		if key == "bash" {
			for _, line := range BashPermissionBlockLines(val, filepath.Base(name)) {
				out.WriteString(line + "\n")
			}
			return
		}
		if patterns, ok := ywaiBucketPatterns[key]; ok {
			for _, p := range patterns {
				// A profile naming the exact pattern is overriding its bucket on
				// purpose. Emitting the bucket value too would write the key twice
				// and the last one wins, silently handing the tool back.
				if _, explicit := profile.Permission[p]; explicit && p != key {
					continue
				}
				out.WriteString(fmt.Sprintf("  %q: %s\n", p, val))
			}
			return
		}
		if key == "*" || strings.ContainsAny(key, "*:#&!|>',[]{}%`@") {
			out.WriteString(fmt.Sprintf("  %q: %s\n", key, val))
		} else {
			out.WriteString(fmt.Sprintf("  %s: %s\n", key, val))
		}
	}

	// Explicit restrictions only: no catch-all "*: deny". opencode enables
	// unlisted tools by default, which keeps MCP servers added outside ywai
	// available without a release for each new server.
	written := map[string]bool{}
	for _, key := range openCodeNativePermissionKeys {
		val := profile.Permission[key]
		if val == "" {
			val = "deny"
		}
		emitPermission(key, val)
		written[key] = true
	}
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
	return out.String()
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
	b.WriteString(fmt.Sprintf("description: %s\n", yamlScalar(description)))
	b.WriteString(fmt.Sprintf("mode: %s\n", profile.Mode))
	// Group membership stays out of the frontmatter and lives in
	// GroupSidecarFile. opencode v1 would accept a `group:` key, but an unknown
	// key is what makes v2 fall back to the legacy decode path, so keeping the
	// sidecar leaves the same file readable by both.

	// ywai's coarse buckets (ado, memory, intercom, mcp) expand to
	// opencode-native wildcard patterns either way, so the deny/allow is
	// enforced rather than silently dropped. The schema differs per flavor:
	// v1 reads a nested "permission:" map, v2 an ordered "permissions:" list
	// where the last matching rule wins. v2 rejects the v1 map as an unknown
	// key and falls back to the legacy decode, which drops the permissions
	// entirely and pastes the frontmatter into the system prompt.
	if agent.OpenCodeIsV2() {
		for _, line := range RenderPermissionRulesYAML(RulesFromPermissionMap(name, profile.Permission)) {
			b.WriteString(line + "\n")
		}
	} else {
		b.WriteString(RenderPermissionMapYAML(name, profile))
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
		return "Technical lead: solo/thin act or full multi-agent delivery",
			[]string{"member_prompt", "member_steer", "member_wait", "task_*", "message_*", "Read", "Write", "Edit", "Bash", "Grep", "Glob"}, ""
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

// uniqueSortedStrings sorts s in place and removes adjacent duplicates.
func uniqueSortedStrings(s []string) []string {
	slices.Sort(s)
	return slices.Compact(s)
}

// legacyAgentFrontmatterKeys are keys ywai used to emit that opencode v2 does
// not accept in agent frontmatter. Any of them makes config/plugin/agent.ts
// classify the file as legacy v1 and decode it with a schema that knows
// `permission` (a map) instead of the v2 `permissions` rule array — so the
// agent silently loses its permissions.
var legacyAgentFrontmatterKeys = []string{"group:"}

// StripLegacyAgentKeys removes those keys from every flat agent file in
// agentsDir. Several writers assemble these files by preserving existing
// frontmatter lines, so a key written once survives every later rewrite; this
// sweep runs last and is idempotent. Returns how many files it changed.
func StripLegacyAgentKeys(agentsDir string) int {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return 0
	}
	changed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(agentsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		out, dirty := stripLegacyKeysFromFrontmatter(string(data))
		if !dirty {
			continue
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err == nil {
			changed++
		}
	}
	return changed
}

// stripLegacyKeysFromFrontmatter drops the offending top-level keys from the
// YAML frontmatter only, leaving the prompt body untouched.
func stripLegacyKeysFromFrontmatter(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content, false
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return content, false
	}
	var out []string
	dirty := false
	for i, line := range lines {
		if i > 0 && i < end {
			trimmed := strings.TrimSpace(line)
			drop := false
			for _, key := range legacyAgentFrontmatterKeys {
				if strings.HasPrefix(trimmed, key) {
					drop = true
					break
				}
			}
			if drop {
				dirty = true
				continue
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n"), dirty
}
