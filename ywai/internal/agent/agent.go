package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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
	// Check known install locations
	home := homeDir()
	knownPaths := map[string][]string{
		"opencode": {
			filepath.Join(home, ".opencode", "bin", "opencode"),
		},
	}
	if paths, ok := knownPaths[name]; ok {
		for _, p := range paths {
			if runtime.GOOS == "windows" {
				p += ".exe"
			}
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
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

func AvailableNames() []string {
	return []string{
		"opencode", "claude-code", "cursor", "windsurf",
		"gemini-cli", "vscode-copilot", "codex",
		"kilocode", "kimi", "qwen-code", "antigravity", "kiro-ide",
	}
}
