package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

type Agent struct {
	Name       string
	SkillsDir  string
	BinaryName string
}

var KnownAgents = []struct {
	Name       string
	Binary     string
	SkillsPath func() string
}{
	{
		Name: "opencode",
		// Placeholder: Detect resolves this entry through FindOpenCode so the
		// OpenCode 2 binary wins.
		Binary: "opencode2",
		SkillsPath: func() string {
			// Single canonical copy: OpenCode reads ~/.agents/skills natively,
			// so no per-host copy. Sandbox-aware (see AgentsSkillsDir): under
			// YWAI_PROFILE this resolves inside the profile instead of
			// leaking to the global dir.
			return config.AgentsSkillsDir()
		},
	},
	{
		Name:   "claude-code",
		Binary: "claude",
		SkillsPath: func() string {
			// Same canonical dir; Claude Code reaches it through per-skill
			// compat links (skills.EnsureClaudeCompatLinks).
			return config.AgentsSkillsDir()
		},
	},
	{
		Name:   "cursor",
		Binary: "cursor",
		SkillsPath: func() string {
			return filepath.Join(homeDir(), ".cursor", "skills")
		},
	},
	{
		Name:   "codex",
		Binary: "codex",
		SkillsPath: func() string {
			return filepath.Join(homeDir(), ".codex", "skills")
		},
	},
	{
		Name:   "pi",
		Binary: "pi",
		SkillsPath: func() string {
			return filepath.Join(homeDir(), ".pi", "agent", "npm", "node_modules", "gentle-pi", "skills")
		},
	},
	{
		// oh-my-pi (omp) — Pi fork with its own config tree under ~/.omp/
		Name:   "omp",
		Binary: "omp",
		SkillsPath: func() string {
			return filepath.Join(homeDir(), ".omp", "agent", "skills")
		},
	},
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = "."
	}
	return home
}

func createSkillsDir(path string) error {
	perm := os.FileMode(0o750)
	if config.IsWindows() {
		perm = 0o700
	}
	return os.MkdirAll(path, perm)
}

// FindBinary resolves the absolute path to an agent binary by trying, in order:
// exec.LookPath (system PATH), Windows extensions, well-known install dirs
// (~/.<name>/bin, ~/.local/bin), and finally a login-shell `which`/`where`
// fallback so binaries installed via nvm/asdf/etc. (not in the raw process
// PATH) are still found. Returns "" if not found.
func FindBinary(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	if runtime.GOOS == "windows" {
		for _, ext := range []string{".exe", ".cmd", ".bat"} {
			if path, err := exec.LookPath(name + ext); err == nil {
				return path
			}
		}
	}
	// Check well-known install directories
	home := homeDir()
	wellKnownDirs := []string{
		filepath.Join(home, "."+name, "bin"),
		// The opencode2 install lands in ~/.opencode/bin (no "2" in the dir
		// name), so probing "opencode2" must also look there.
		filepath.Join(home, ".opencode", "bin"),
		filepath.Join(home, ".local", "bin"),
	}
	for _, dir := range wellKnownDirs {
		candidate := filepath.Join(dir, name)
		if runtime.GOOS == "windows" {
			candidate += ".exe"
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// Fallback: run which/where via shell to pick up user's profile PATH
	if p := whichViaShell(name); p != "" {
		return p
	}
	return ""
}

// FindOpenCode resolves the OpenCode 2 CLI binary (opencode2). OpenCode v1 is
// withdrawn (docs/adr/0001-drop-opencode-v1-support.md) and is never resolved;
// a machine that only carries the v1 binary gets the withdrawal notice from
// GateOpenCodeV2 at install/apply time. Returns the resolved path and the
// binary name, or "" when opencode2 is not found.
func FindOpenCode() (string, string) {
	if p := FindBinary("opencode2"); p != "" {
		return p, "opencode2"
	}
	// OpenCode 2 now also ships as plain `opencode`; accept it when it
	// reports a 2.x (or later) version.
	if p := FindBinary("opencode"); p != "" && isOpenCodeV2(p) {
		return p, "opencode"
	}
	return "", ""
}

// isOpenCodeV2 reports whether the binary's --version is 2.x or later.
func isOpenCodeV2(path string) bool {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return false
	}
	v := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(string(out)), "opencode "), "v")
	major, _, _ := strings.Cut(v, ".")
	n, err := strconv.Atoi(major)
	return err == nil && n >= 2
}

// OpenCodeBinaryName is the binary name of the active OpenCode host. It falls
// back to opencode2 when the binary is not installed, so callers that only
// need a name (config writers, install summaries) never get "".
func OpenCodeBinaryName() string {
	if _, name := FindOpenCode(); name != "" {
		return name
	}
	return "opencode2"
}

// GateOpenCodeV2 is the OpenCode 2 minimum-version gate for install/apply.
// It returns an error when the only OpenCode binary installed is the retired
// v1 `opencode` CLI: ywai writes v2-shaped config, which v1 rejects, so the
// run must stop with the withdrawal notice instead of silently breaking the
// user's config. A machine with neither binary passes — callers report their
// own not-found error.
func GateOpenCodeV2() error {
	if p, _ := FindOpenCode(); p != "" {
		return nil
	}
	if FindBinary("opencode") != "" {
		return fmt.Errorf("ywai requires OpenCode 2 (the opencode2 binary): the installed 'opencode' is the " +
			"retired v1 CLI, which ywai no longer supports (docs/adr/0001-drop-opencode-v1-support.md). " +
			"Install OpenCode 2 and run this command again")
	}
	return nil
}

func whichViaShell(name string) string {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "where", name)
	} else {
		cmd = exec.Command("sh", "-lc", "which "+name)
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return ""
	}
	// where on Windows may return multiple lines, take the first
	if i := strings.IndexByte(path, '\n'); i > 0 {
		path = path[:i]
	}
	path = strings.TrimSpace(path)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func Detect() []Agent {
	var found []Agent
	for _, ka := range KnownAgents {
		path := FindBinary(ka.Binary)
		if ka.Name == "opencode" {
			path, _ = FindOpenCode()
		}
		if path == "" {
			// Fallback: detect by config dir even if binary not in PATH
			if detectByConfigDir(ka.Name, ka.SkillsPath()) {
				found = append(found, Agent{
					Name:      ka.Name,
					SkillsDir: ka.SkillsPath(),
				})
			}
			continue
		}

		skillsDir := ka.SkillsPath()

		if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
			if err := createSkillsDir(skillsDir); err != nil {
				continue
			}
		}

		found = append(found, Agent{
			Name:       ka.Name,
			SkillsDir:  skillsDir,
			BinaryName: path,
		})
	}
	return found
}

func detectByConfigDir(name, skillsDir string) bool {
	if _, err := os.Stat(skillsDir); err == nil {
		return true
	}

	parentDir := filepath.Dir(skillsDir)
	// Shared markers: AGENTS.md (most hosts). OMP also has models.yml / config.yml
	// under ~/.omp/agent even before skills are created.
	markers := []string{"AGENTS.md"}
	if name == "omp" {
		markers = append(markers, "models.yml", "models.yaml", "models.json", "config.yml")
	}
	found := false
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(parentDir, m)); err == nil {
			found = true
			break
		}
	}
	if !found {
		return false
	}

	_ = createSkillsDir(skillsDir)
	return true
}

func FindByName(name string) (*Agent, error) {
	for _, a := range Detect() {
		if a.Name == name {
			return &a, nil
		}
	}
	return nil, fmt.Errorf("agent %q not found or not installed", name)
}

// SettingsPaths returns the config file paths for agents that have JSON settings.
// OpenCode prefers .jsonc when it exists, falling back to .json.
// Used by plugins and other install steps.
func SettingsPaths() map[string]string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	return map[string]string{
		"opencode":    config.FindJSONCPath(config.OpenCodeConfigDir(), "opencode"),
		"claude-code": pathIfExists(filepath.Join(home, ".claude", "settings.json")),
		"pi":          pathIfExists(filepath.Join(home, ".pi", "agent", "mcp.json")),
		// OMP models live in models.yml; expose the agent dir for callers that
		// only need "is configured" via a path (tokenbank / install hooks).
		"omp": pathIfExists(filepath.Join(home, ".omp", "agent", "models.yml")),
	}
}

func pathIfExists(path string) string {
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

func AvailableNames() []string {
	return []string{
		"opencode", "claude-code", "cursor", "codex", "pi", "omp",
	}
}

// ProfileInstallHosts are agents for which ywai actually installs agent
// profiles (see install switch in cmd/ywai/root.go). Detection may find more
// binaries on PATH; install UI and default install target only these.
//
// cursor was dropped: nobody here runs it, and carrying an
// install path costs a branch in every host switch. Uninstall still knows how
// to clean it so an older install can be removed.
var ProfileInstallHosts = []string{
	"opencode",
	"claude-code",
	"pi",
	"omp",
}

// SupportsProfileInstall reports whether ywai has a real profile-install path
// for this agent (not just "binary found on PATH").
func SupportsProfileInstall(name string) bool {
	for _, h := range ProfileInstallHosts {
		if h == name {
			return true
		}
	}
	return false
}

// FilterProfileInstallAgents keeps only agents with a real ywai install path.
func FilterProfileInstallAgents(agents []Agent) []Agent {
	out := make([]Agent, 0, len(agents))
	for _, a := range agents {
		if SupportsProfileInstall(a.Name) {
			out = append(out, a)
		}
	}
	return out
}

// Resolve returns agents based on user config or auto-detection.
// If the user has configured specific agents in ~/.ywai/config.yaml,
// only those agents are returned. Otherwise, falls back to Detect().
func Resolve() []Agent {
	cfg, err := config.LoadConfig()
	if err == nil && len(cfg.Agents) > 0 {
		return filterByConfig(cfg.Agents)
	}
	return Detect()
}

// filterByConfig filters the detected agents to only include those
// explicitly configured by the user.
func filterByConfig(names []string) []Agent {
	all := Detect()
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[strings.TrimSpace(n)] = true
	}

	var filtered []Agent
	for _, a := range all {
		if nameSet[a.Name] {
			filtered = append(filtered, a)
		}
	}
	return filtered
}
