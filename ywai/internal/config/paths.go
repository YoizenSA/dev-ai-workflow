package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	AppName         = "ywai"
	GentleAIBin     = "gentle-ai"
	SkillsDirName   = "skills"
	ProjectTypesDir = "project-types"
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

func DataProjectTypesDir() string {
	return filepath.Join(DataDir(), ProjectTypesDir)
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

func ProjectTypesSourceDir() string {
	return findSourceDir(ProjectTypesDir)
}

// findSourceDir locates a source directory (skills/ or project-types/).
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

func IsOurRepoByPath(dir string) bool {
	for _, marker := range []string{"_shared", "react-19", "angular"} {
		if _, err := os.Stat(filepath.Join(dir, SkillsDirName, marker)); err == nil {
			return true
		}
	}
	return false
}

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
