package overrides

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ApplyOpenSpecToSDDOverride changes openspec/ → .sdd/ in convention files
// This makes all projects use .sdd/ instead of openspec/ for SDD artifacts
func ApplyOpenSpecToSDDOverride(agentSkillsDirs map[string]string) error {
	for agentName, skillsDir := range agentSkillsDirs {
		conventionFile := filepath.Join(skillsDir, "_shared", "openspec-convention.md")
		if _, err := os.Stat(conventionFile); err != nil {
			continue
		}

		if err := replaceInFile(conventionFile, "openspec/", ".sdd/"); err != nil {
			fmt.Printf("  Warning: failed to update %s: %v\n", agentName, err)
			continue
		}
		fmt.Printf("  [%s] Updated openspec-convention.md: openspec/ → .sdd/\n", agentName)
	}

	return nil
}

func replaceInFile(path string, old, new string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	updated := strings.ReplaceAll(string(content), old, new)

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return err
	}

	return nil
}

// AgentSkillsDirs returns the skills directory for each detected agent.
func AgentSkillsDirs() map[string]string {
	home, _ := os.UserHomeDir()
	return map[string]string{
		"opencode":    filepath.Join(home, ".config", "opencode", "skills"),
		"claude-code": filepath.Join(home, ".claude", "skills"),
		"cursor":      filepath.Join(home, ".cursor", "skills"),
		"windsurf":    filepath.Join(home, ".codeium", "windsurf", "skills"),
		"gemini-cli":  filepath.Join(home, ".gemini", "skills"),
		"codex":       filepath.Join(home, ".codex", "skills"),
		"kilocode":    filepath.Join(home, ".config", "kilo", "skills"),
		"kimi":        filepath.Join(home, ".kimi", "skills"),
		"qwen-code":   filepath.Join(home, ".qwen", "skills"),
		"kiro-ide":    filepath.Join(home, ".kiro", "skills"),
		"openclaw":    filepath.Join(home, ".openclaw", "skills"),
		"trae-ide":    filepath.Join(home, ".trae", "skills"),
		"antigravity": filepath.Join(home, ".gemini", "antigravity", "skills"),
	}
}
