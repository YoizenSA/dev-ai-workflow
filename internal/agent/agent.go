package agent

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/Yoizen/ywai/internal/config"
)

// Agent represents an AI code assistant tool that can be detected and managed.
type Agent struct {
	// Name is the human-readable name of the agent
	Name string

	// SkillsDir is the directory where agent-specific skills are stored
	SkillsDir string

	// BinaryName is the full path to the agent's executable
	BinaryName string
}

// KnownAgents is a list of supported AI code assistants that the system can detect.
// Each agent includes its binary name and a function to determine its skills directory.
var KnownAgents = []struct {
	// Name is the human-readable name of the agent
	Name string

	// Binary is the command name used to find the agent in PATH
	Binary string

	// SkillsPath returns the directory where skills for this agent should be stored
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

// homeDir returns the user's home directory path.
// It tries multiple methods to determine the home directory and logs warnings if all fail.
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}

	// Fallback to HOME environment variable
	home := os.Getenv("HOME")
	if home == "" {
		log.Printf("Warning: Could not determine home directory, using current directory")
		home = "."
	}
	return home
}

// createSkillsDir creates the skills directory with appropriate permissions.
// On Windows, it uses 700 (owner rwx only). On Unix-like systems, it uses 750 (owner rwx, group r-x).
func createSkillsDir(path string) error {
	if config.IsWindows() {
		// Windows: More restrictive permissions for security
		return os.MkdirAll(path, 0o700)
	}
	// Unix-like systems: 750 (owner rwx, group r-x, others ---)
	return os.MkdirAll(path, 0o750)
}

// findBinary searches for a binary in the system PATH.
// It first tries the exact name, then on Windows it tries common extensions (.exe, .cmd, .bat).
// It logs a warning if the binary is not found but returns an empty string instead of erroring.
func findBinary(name string) string {
	// First try exact name
	if path, err := exec.LookPath(name); err == nil {
		return path
	}

	// On Windows, try common extensions
	if runtime.GOOS == "windows" {
		for _, ext := range []string{".exe", ".cmd", ".bat"} {
			if path, err := exec.LookPath(name + ext); err == nil {
				return path
			}
		}
	}

	// Log warning if binary not found (but don't error out)
	log.Printf("Warning: Binary '%s' not found in PATH", name)
	return ""
}

// Detect scans the system PATH for known AI code assistants and returns a list of found agents.
// It automatically creates skills directories for agents that are found but don't have them yet.
func Detect() []Agent {
	var found []Agent
	for _, ka := range KnownAgents {
		path := findBinary(ka.Binary)
		if path == "" {
			log.Printf("Skipping agent '%s': binary not found", ka.Name)
			continue
		}

		skillsDir := ka.SkillsPath()

		if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
			log.Printf("Creating skills directory for agent '%s' at '%s'", ka.Name, skillsDir)
			if err := createSkillsDir(skillsDir); err != nil {
				log.Printf("Warning: Could not create skills directory for '%s': %v", ka.Name, err)
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

// FindByName searches for a specific agent by name among the detected agents.
// It performs a full detection scan and returns the first matching agent.
func FindByName(name string) (*Agent, error) {
	for _, a := range Detect() {
		if a.Name == name {
			return &a, nil
		}
	}
	return nil, fmt.Errorf("agent %q not found or not installed", name)
}

// AvailableNames returns a list of all known agent names that this system can detect.
// This is useful for showing users which agents are available to install or use.
func AvailableNames() []string {
	return []string{"opencode", "claude-code", "cursor", "windsurf", "gemini-cli", "vscode-copilot", "codex"}
}
