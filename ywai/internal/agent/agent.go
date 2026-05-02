package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/Yoizen/ywai/internal/config"
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
	return ""
}

func Detect() []Agent {
	var found []Agent
	for _, ka := range KnownAgents {
		path := findBinary(ka.Binary)
		if path == "" {
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

func FindByName(name string) (*Agent, error) {
	for _, a := range Detect() {
		if a.Name == name {
			return &a, nil
		}
	}
	return nil, fmt.Errorf("agent %q not found or not installed", name)
}

func AvailableNames() []string {
	return []string{"opencode", "claude-code", "cursor", "windsurf", "gemini-cli", "vscode-copilot", "codex"}
}
