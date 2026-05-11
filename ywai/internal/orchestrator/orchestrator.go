package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	oldName = "gentle-orchestrator"
	newName = "sdd-orchestrator"
)

type RenameResult struct {
	Agent string
	File  string
}

func RenameAll(agentConfigs map[string]string) []RenameResult {
	var results []RenameResult

	for agentName, settingsPath := range agentConfigs {
		if settingsPath == "" {
			continue
		}

		renamed, err1 := renameInJSON(settingsPath)
		injected, err2 := injectAskAgent(settingsPath)
		if err1 != nil || err2 != nil {
			continue
		}
		if renamed || injected {
			results = append(results, RenameResult{Agent: agentName, File: settingsPath})
		}
	}

	subAgentDirs := findSubAgentDirs()
	for _, dir := range subAgentDirs {
		renamed, _ := renameSubAgentFile(dir)
		if renamed != "" {
			results = append(results, RenameResult{Agent: filepath.Base(filepath.Dir(filepath.Dir(renamed))), File: renamed})
		}
		injectAskSubAgentFile(dir)
	}

	return results
}

func renameInJSON(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return false, err
	}

	agentsRaw, ok := root["agent"]
	if !ok {
		return false, nil
	}
	agentsMap, ok := agentsRaw.(map[string]any)
	if !ok {
		return false, nil
	}

	entry, exists := agentsMap[oldName]
	if !exists {
		return false, nil
	}

	agentsMap[newName] = entry
	delete(agentsMap, oldName)

	updated, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, err
	}
	updated = append(updated, '\n')

	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return false, err
	}

	return true, nil
}

func findSubAgentDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	dirs := []string{
		filepath.Join(home, ".cursor", "agents"),
		filepath.Join(home, ".kimi", "agents"),
		filepath.Join(home, ".kiro", "agents"),
		filepath.Join(home, ".config", "opencode", "commands"),
	}

	var existing []string
	for _, d := range dirs {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			existing = append(existing, d)
		}
	}
	return existing
}

func renameSubAgentFile(agentsDir string) (string, error) {
	for _, ext := range []string{".md", ".yaml"} {
		oldPath := filepath.Join(agentsDir, oldName+ext)
		if _, err := os.Stat(oldPath); err != nil {
			continue
		}

		newPath := filepath.Join(agentsDir, newName+ext)

		content, err := os.ReadFile(oldPath)
		if err != nil {
			return "", err
		}

		updated := strings.ReplaceAll(string(content), oldName, newName)

		if err := os.WriteFile(newPath, []byte(updated), 0o644); err != nil {
			return "", err
		}

		os.Remove(oldPath)

		return newPath, nil
	}
	return "", nil
}

func AgentSettingsPaths() map[string]string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	return map[string]string{
		"opencode":  filepath.Join(home, ".config", "opencode", "opencode.json"),
		"kilocode":  filepath.Join(home, ".config", "kilo", "opencode.json"),
		"windsurf":  settingsPathIfExists(filepath.Join(home, ".windsurf", "settings.json")),
		"gemini-cli": settingsPathIfExists(filepath.Join(home, ".gemini", "settings.json")),
	}
}

func settingsPathIfExists(path string) string {
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

func injectAskAgent(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return false, err
	}

	agentsRaw, ok := root["agent"]
	if !ok {
		agentsRaw = map[string]any{}
		root["agent"] = agentsRaw
	}
	agentsMap, ok := agentsRaw.(map[string]any)
	if !ok {
		return false, nil
	}

	if _, exists := agentsMap["ask"]; exists {
		return false, nil
	}

	agentsMap["ask"] = map[string]any{
		"mode":        "primary",
		"description": "Read-only Q&A — answers questions without modifying code or making plans",
		"prompt":      "You are a read-only assistant. You can ONLY read files and answer questions. You must NOT edit files, write files, create plans, or suggest code changes. When asked to implement something, refuse and explain the user should switch to the main agent. Answer concisely and directly. Use evidence from the codebase.",
		"tools": map[string]any{
			"read":  true,
			"bash":  false,
			"edit":  false,
			"write": false,
		},
	}

	updated, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, err
	}
	updated = append(updated, '\n')

	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return false, err
	}

	return true, nil
}

func injectAskSubAgentFile(agentsDir string) {
	askPath := filepath.Join(agentsDir, "ask.md")
	if _, err := os.Stat(askPath); err == nil {
		return
	}

	content := `---
inclusion: always
---

You are a read-only assistant. You can ONLY read files and answer questions.

Rules:
- You must NOT edit files, write files, create plans, or suggest code changes.
- When asked to implement something, refuse and explain the user should switch to the main agent.
- Answer concisely and directly.
- Use evidence from the codebase.
- No tools that modify the filesystem are available to you.
`

	os.WriteFile(askPath, []byte(content), 0o644)
}

func PrintResults(results []RenameResult) {
	if len(results) == 0 {
		return
	}
	fmt.Printf("  Renamed %s → %s in:\n", oldName, newName)
	for _, r := range results {
		fmt.Printf("    [%s] %s\n", r.Agent, shortPath(r.File))
	}
}

func shortPath(p string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}
