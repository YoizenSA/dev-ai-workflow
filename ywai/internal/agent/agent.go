package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		Name:   "opencode",
		Binary: "opencode",
		SkillsPath: func() string {
			return filepath.Join(homeDir(), ".config", "opencode", "skills")
		},
	},
	{
		Name:   "claude-code",
		Binary: "claude",
		SkillsPath: func() string {
			return filepath.Join(homeDir(), ".claude", "skills")
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
		Name:   "windsurf",
		Binary: "windsurf",
		SkillsPath: func() string {
			return filepath.Join(homeDir(), ".windsurf", "skills")
		},
	},
	{
		Name:   "gemini-cli",
		Binary: "gemini",
		SkillsPath: func() string {
			return filepath.Join(homeDir(), ".gemini", "skills")
		},
	},
	{
		Name:   "vscode-copilot",
		Binary: "code",
		SkillsPath: func() string {
			if runtime.GOOS == "windows" {
				return filepath.Join(os.Getenv("APPDATA"), "Code", "User", "skills")
			}
			if runtime.GOOS == "darwin" {
				return filepath.Join(homeDir(), "Library", "Application Support", "Code", "User", "skills")
			}
			return filepath.Join(homeDir(), ".config", "Code", "skills")
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
		Name:   "kilocode",
		Binary: "kilo",
		SkillsPath: func() string {
			return filepath.Join(homeDir(), ".config", "kilo", "skills")
		},
	},
	{
		Name:   "kimi",
		Binary: "kimi",
		SkillsPath: func() string {
			return filepath.Join(homeDir(), ".config", "agents", "skills")
		},
	},
	{
		Name:   "qwen-code",
		Binary: "qwen",
		SkillsPath: func() string {
			return filepath.Join(homeDir(), ".qwen", "skills")
		},
	},
	{
		Name:   "antigravity",
		Binary: "",
		SkillsPath: func() string {
			return filepath.Join(homeDir(), ".gemini", "antigravity", "skills")
		},
	},
	{
		Name:   "kiro-ide",
		Binary: "kiro",
		SkillsPath: func() string {
			return filepath.Join(homeDir(), ".kiro", "skills")
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

func findBinary(name string) string {
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
		if ka.Binary == "" {
			if detectByConfigDir(ka.Name, ka.SkillsPath()) {
				found = append(found, Agent{
					Name:      ka.Name,
					SkillsDir: ka.SkillsPath(),
				})
			}
			continue
		}

		path := findBinary(ka.Binary)
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
	agentMarker := filepath.Join(parentDir, "AGENTS.md")
	if _, err := os.Stat(agentMarker); err != nil {
		return false
	}

	createSkillsDir(skillsDir)
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
// Used by plugins and other install steps.
func SettingsPaths() map[string]string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	return map[string]string{
		"opencode":   filepath.Join(home, ".config", "opencode", "opencode.json"),
		"kilocode":   filepath.Join(home, ".config", "kilo", "opencode.json"),
		"windsurf":   pathIfExists(filepath.Join(home, ".windsurf", "settings.json")),
		"gemini-cli": pathIfExists(filepath.Join(home, ".gemini", "settings.json")),
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
		"opencode", "claude-code", "cursor", "windsurf",
		"gemini-cli", "vscode-copilot", "codex",
		"kilocode", "kimi", "qwen-code", "antigravity", "kiro-ide",
	}
}
