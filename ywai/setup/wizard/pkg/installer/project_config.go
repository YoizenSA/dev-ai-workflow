package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (i *Installer) updateGitignore() error {
	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would update .gitignore")
		return nil
	}

	gitignorePath := filepath.Join(i.targetDir, ".gitignore")

	if !i.fileExists(gitignorePath) {
		if err := os.WriteFile(gitignorePath, []byte(""), 0644); err != nil {
			return err
		}
	}

	patterns := []string{
		"# Dependencies", "node_modules/", "",
		"# Environment", ".env", ".env.local", ".env.*.local", "",
		"# AI Assistants", "CLAUDE.md", "CURSOR.md", "GEMINI.md", ".cursorrules", ".ga", ".gga", ".claude/", "",
		"# OpenCode", ".opencode/plugins/**/node_modules/", ".opencode/plugins/**/dist/", ".opencode/**/cache/", "",
		"# System", ".DS_Store", "Thumbs.db", "",
		"# Logs", "*.log", "logs/", "",
		"# IDE", ".idea/", "*.iml", ".vscode/", "",
	}

	content, _ := os.ReadFile(gitignorePath)
	existing := string(content)

	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		if !strings.Contains(existing, pattern) {
			existing += pattern + "\n"
		}
	}

	if err := os.WriteFile(gitignorePath, []byte(existing), 0644); err != nil {
		return err
	}

	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would create VS Code settings")
		return nil
	}

	i.logger.LogSuccess("Updated .gitignore")
	return nil
}

func (i *Installer) setupVSCodeSettings() error {
	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would create VS Code settings")
		return nil
	}

	vscodeDir := filepath.Join(i.targetDir, ".vscode")
	settingsPath := filepath.Join(vscodeDir, "settings.json")

	if i.fileExists(settingsPath) && !i.flags.Force {
		return nil
	}

	if err := i.ensureDir(vscodeDir); err != nil {
		return err
	}

	settings := `{
    "github.copilot.chat.useAgentsMdFile": true
}
`

	if err := os.WriteFile(settingsPath, []byte(settings), 0644); err != nil {
		return err
	}

	i.logger.LogSuccess("Created VS Code settings")
	return nil
}

func (i *Installer) initializeGA() error {
	if i.flags.SkipGA {
		i.logger.LogInfo("GA initialization skipped by --skip-ga")
		return nil
	}
	if !i.flags.All && !i.flags.InstallGA {
		i.logger.LogInfo("GA initialization skipped (GA install not requested)")
		return nil
	}

	types := i.loadTypesConfig()
	if !types.BaseConfig.InitGA {
		i.logger.LogInfo("GA initialization disabled by base_config.init_ga")
		return nil
	}
	if !i.presetAllowsInitGA() {
		name, _ := i.activePreset()
		i.logger.LogInfo(fmt.Sprintf("GA initialization disabled by preset '%s'", name))
		return nil
	}

	if !i.commandExists("ga") {
		i.logger.LogInfo("GA command not available, skipping initialization")
		return nil
	}

	gaConfigPath := filepath.Join(i.targetDir, ".ga")
	gaConfigExists := i.fileExists(gaConfigPath)

	if i.flags.DryRun {
		if !gaConfigExists || i.flags.Force {
			i.logger.Log("DRY RUN: Would initialize GA in project")
		}
		i.logger.Log("DRY RUN: Would configure .ga template/provider")
		i.logger.Log("DRY RUN: Would install GA hooks")
		return nil
	}

	if !gaConfigExists || i.flags.Force {
		if err := i.runCommand("ga", "init"); err != nil {
			return fmt.Errorf("failed to initialize GA: %w", err)
		}
		i.logger.LogSuccess("GA initialized in project")
	} else {
		i.logger.LogInfo("GA already initialized in project")
	}

	if err := i.applyGAProjectTemplate(gaConfigPath); err != nil {
		return err
	}

	if err := i.applyGAProvider(gaConfigPath); err != nil {
		return err
	}

	if err := i.runCommand("ga", "install"); err != nil {
		return fmt.Errorf("failed to install GA hooks: %w", err)
	}
	i.logger.LogSuccess("GA hooks installed")

	return nil
}

func (i *Installer) resolveGATemplatePath() string {
	candidates := make([]string, 0, 5)
	candidates = append(candidates,
		i.ywaiCandidates(false, "ga/.ga.opencode-template")...,
	)
	if repoRoot := i.getRepoRoot(); repoRoot != "" {
		candidates = append(candidates, filepath.Join(repoRoot, "ywai", "ga", ".ga.opencode-template"))
	}
	candidates = append(candidates, filepath.Join(i.getGADir(), ".ga.opencode-template"))
	return i.firstExistingFile(uniqueCleanPaths(candidates)...)
}

func (i *Installer) applyGAProjectTemplate(gaConfigPath string) error {
	templatePath := i.resolveGATemplatePath()
	if templatePath == "" || !i.fileExists(gaConfigPath) {
		return nil
	}

	if err := i.copyFile(templatePath, gaConfigPath); err != nil {
		return fmt.Errorf("failed to apply GA template: %w", err)
	}
	i.logger.LogSuccess("Applied OpenCode template to .ga")
	return nil
}

func (i *Installer) applyGAProvider(gaConfigPath string) error {
	provider := strings.TrimSpace(i.provider)
	if provider == "" || strings.EqualFold(provider, "opencode") {
		return nil
	}
	if !i.fileExists(gaConfigPath) {
		return nil
	}

	content, err := os.ReadFile(gaConfigPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	replaced := false
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "PROVIDER=") {
			lines[idx] = fmt.Sprintf("PROVIDER=\"%s\"", provider)
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, fmt.Sprintf("PROVIDER=\"%s\"", provider))
	}

	result := strings.Join(lines, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	if err := os.WriteFile(gaConfigPath, []byte(result), 0o644); err != nil {
		return err
	}

	i.logger.LogSuccess("Provider set in .ga")
	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would apply lefthook.yml")
		return nil
	}

	return nil
}

func (i *Installer) applyLefthookConfig(relPath string) error {
	candidateRelPaths := i.typeDocCandidatePaths(i.projectType, "lefthook.yml", relPath)
	source := i.firstExistingFile(i.ywaiCandidates(false, candidateRelPaths...)...)
	if source == "" {
		return nil
	}

	dest := filepath.Join(i.targetDir, "lefthook.yml")
	if i.fileExists(dest) && !i.flags.Force {
		return nil
	}
	if err := i.copyFile(source, dest); err != nil {
		return err
	}
	i.logger.LogSuccess("Applied lefthook.yml")

	if i.commandExists("lefthook") {
		if err := i.runCommand("lefthook", "install"); err != nil {
			i.logger.LogWarning("Failed to install lefthook hooks")
		} else {
			i.logger.LogSuccess("Lefthook hooks installed")
		}
	} else {
	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would append agent templates")
		return nil
	}

		i.logger.LogInfo("Lefthook not installed, skipping hooks")
	}

	return nil
}

func (i *Installer) appendAgentsTemplates(relTemplates []string) error {
	if len(relTemplates) == 0 {
		return nil
	}
	agentsPath := filepath.Join(i.targetDir, "AGENTS.md")
	if !i.fileExists(agentsPath) {
		return nil
	}

	content, err := os.ReadFile(agentsPath)
	if err != nil {
		return err
	}
	existing := string(content)

	for _, rel := range relTemplates {
		normalized := normalizeTypesRelativePath(rel)
		if normalized == "" {
			continue
		}

		candidateRelPaths := make([]string, 0, 6)
		candidateRelPaths = append(candidateRelPaths, withSetupCompatFallback(normalized)...)
		baseName := filepath.Base(normalized)
		candidateRelPaths = append(candidateRelPaths,
			filepath.Join("templates", baseName),
			filepath.Join("setup", "lib", "templates", baseName),
		)

		source := i.firstExistingFile(i.ywaiCandidates(false, candidateRelPaths...)...)
		if source == "" {
			continue
		}

		tplData, readErr := os.ReadFile(source)
		if readErr != nil {
			continue
		}

		text := strings.TrimSpace(string(tplData))
		if text == "" {
			continue
		}
		if strings.Contains(existing, text) {
			continue
		}
		existing += "\n\n" + text + "\n"
	}

	return os.WriteFile(agentsPath, []byte(existing), 0644)
}
