package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	AppName          = "ywai"
	GentleAIBin      = "gentle-ai"
	SkillsDirName    = "skills"
	AgentsDirName    = "agents"
	PluginsDirName   = "plugins"
	WorkflowsDirName = "workflows"
)

var repoRootOverride string

func SetRepoRoot(path string) {
	repoRootOverride = path
}

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

func DataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".ywai")
}

func DataSkillsDir() string {
	return filepath.Join(DataDir(), SkillsDirName)
}

func DataAgentsDir() string {
	return filepath.Join(DataDir(), AgentsDirName)
}

func DataPluginsDir() string {
	return filepath.Join(DataDir(), PluginsDirName)
}

// DataWorkflowsDir is the source-of-truth directory for visual workflows
// authored in the Workflow Studio editor. Each workflow is a single JSON file
// named <name>.json. The exported opencode artifacts (commands/, agents/) are
// generated from these files, mirroring the agents/<group>/<name>/AGENT.md →
// ~/.config/opencode/agents/<name>.md source/output split.
func DataWorkflowsDir() string {
	return filepath.Join(DataDir(), WorkflowsDirName)
}

// OpenCodeConfigDir returns the opencode config directory (~/.config/opencode).
// opencode stores its agents, commands, skills, and config here. Several
// packages hardcode this path; these helpers centralize it.
func OpenCodeConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".config", "opencode")
}

// OpenCodeAgentsDir is where opencode loads agent personas from (one flat .md
// per agent; the filename minus .md is the agent id).
func OpenCodeAgentsDir() string {
	return filepath.Join(OpenCodeConfigDir(), "agents")
}

// OpenCodeCommandsDir is where opencode loads slash commands from (one flat
// .md per command; the filename minus .md is the /<name> command).
func OpenCodeCommandsDir() string {
	return filepath.Join(OpenCodeConfigDir(), "commands")
}

// OpenCodeSkillsDir is where opencode loads Agent Skills from (one directory
// per skill, each holding a SKILL.md).
func OpenCodeSkillsDir() string {
	return filepath.Join(OpenCodeConfigDir(), "skills")
}

// ClaudeConfigDir returns the Claude Code config directory (~/.claude), where
// Claude Code loads agents (~/.claude/agents) and slash commands
// (~/.claude/commands), one flat .md per item.
func ClaudeConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".claude")
}

// ClaudeAgentsDir is where Claude Code loads sub-agents from.
func ClaudeAgentsDir() string {
	return filepath.Join(ClaudeConfigDir(), "agents")
}

// ClaudeCommandsDir is where Claude Code loads slash commands from.
func ClaudeCommandsDir() string {
	return filepath.Join(ClaudeConfigDir(), "commands")
}

func RepoRoot() string {
	if repoRootOverride != "" {
		return repoRootOverride
	}

	if cwd, err := os.Getwd(); err == nil {
		if root := findUp(cwd, "go.mod"); root != "" {
			if isOurRepo(root) {
				return root
			}
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		if isOurRepo(cwd) {
			return cwd
		}
	}

	if hasData, _ := dataDirPopulated(); hasData {
		return DataDir()
	}

	ex, err := os.Executable()
	if err != nil {
		return "."
	}
	dir := filepath.Dir(ex)
	if isOurRepo(dir) {
		return dir
	}

	return "."
}

func SkillsSourceDir() string {
	return findSourceDir(SkillsDirName)
}

func AgentsSourceDir() string {
	return findSourceDir(AgentsDirName)
}

// WorkflowsSourceDir locates the bundled seed workflows (ywai/workflows/).
func WorkflowsSourceDir() string {
	return findSourceDir(WorkflowsDirName)
}

func PluginsSourceDir() string {
	return findSourceDir(PluginsDirName)
}

// findSourceDir locates a source directory (skills/).
// It checks: repoRoot/ywai/{name}, repoRoot/{name}, DataDir()/{name}.
func findSourceDir(name string) string {
	root := RepoRoot()

	// When running from source repo: ywai/{name}
	candidate := filepath.Join(root, "ywai", name)
	if IsDirPopulated(candidate) {
		return candidate
	}

	// Direct child of repo root
	candidate = filepath.Join(root, name)
	if IsDirPopulated(candidate) {
		return candidate
	}

	// Seeded data dir
	candidate = filepath.Join(DataDir(), name)
	if IsDirPopulated(candidate) {
		return candidate
	}

	return filepath.Join(root, name)
}

func findUp(start, target string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, target)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// IsOurRepoByPath reports whether dir is a checkout of this repository.
//
// Identification is by Go module path, because that is the only marker nothing
// else can accidentally satisfy. It previously keyed on a skills subdirectory
// name (`skills/_shared`, `skills/angular`), which any machine with a personal
// `~/skills` folder matches — and one did: `$HOME` was identified as the repo,
// so a server started from home served the user's unrelated `~/skills` and
// reported no skills at all.
//
// A false positive here is not cosmetic: it redirects where skills, agents and
// workflows are read from.
func IsOurRepoByPath(dir string) bool {
	if dir == "" {
		return false
	}
	// The module lives at the repo root or one level down in ywai/.
	for _, candidate := range []string{
		filepath.Join(dir, "go.mod"),
		filepath.Join(dir, "ywai", "go.mod"),
	} {
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), modulePath) {
			return true
		}
	}
	return false
}

// modulePath identifies this repository Go module.
const modulePath = "github.com/Yoizen/dev-ai-workflow/ywai"

func isOurRepo(dir string) bool {
	return IsOurRepoByPath(dir)
}

func dataDirPopulated() (bool, error) {
	entries, err := os.ReadDir(DataSkillsDir())
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}

func IsDirPopulated(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	return len(entries) > 0
}

// RetiredMCPServers are MCP server ids ywai used to install and has since
// removed. Two independent consumers need this list, which is why it lives
// here rather than beside either one:
//
//   - install deletes these entries from the user's config (internal/plugins)
//   - permission expansion must never grant them again (internal/agents)
//
// The second matters because profiles are written before the config cleanup
// runs: without this filter, an update regenerates permissions from a config
// that still lists the retired server, so the dead grant is reinstated on the
// very run meant to remove it.
//
// Entries stay here permanently — this is what upgrades installs that predate
// each removal.
var RetiredMCPServers = []string{
	"ywai-kanban", // kanban board removed from ywai
	"ywai-fastfs", // fastfs MCP removed as redundant with codegraph
}

// IsRetiredMCPServer reports whether an MCP server id is one ywai retired.
func IsRetiredMCPServer(id string) bool {
	for _, r := range RetiredMCPServers {
		if r == id {
			return true
		}
	}
	return false
}
