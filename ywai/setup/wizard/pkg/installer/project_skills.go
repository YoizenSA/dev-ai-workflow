package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func (i *Installer) installTypeSkills() error {
	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would install type skills")
		return nil
	}

	types := i.loadTypesConfig()
	pt := i.projectType
	if pt == "" {
		pt = types.Default
	}

	typeConfig, ok := types.Types[pt]
	if !ok {
		return nil
	}
	typeSkills := uniqueStrings(append([]string{}, typeConfig.Skills...))
	typeSkills = uniqueStrings(append(baseSkillsForAllTypes, typeSkills...))
	if len(typeSkills) == 0 {
		return nil
	}

	repoRoot := i.getRepoRoot()
	skillsSrc := i.findSkillsSource(repoRoot)
	skillsTgt := i.getSkillsDir()
	if skillsSrc == "" {
		return nil
	}

	if err := i.ensureDir(skillsTgt); err != nil {
		return err
	}

	installed := 0
	missing := make([]string, 0)
	for _, skill := range typeSkills {
		srcPath := filepath.Join(skillsSrc, skill)
		destPath := filepath.Join(skillsTgt, skill)

		if !i.dirExists(srcPath) {
			missing = append(missing, skill)
			continue
		}
		if i.dirExists(destPath) && !i.flags.Force {
			continue
		}
		if i.dirExists(destPath) {
			if err := os.RemoveAll(destPath); err != nil {
				return err
			}
		}
		if err := i.copyDir(srcPath, destPath); err != nil {
			return err
		}
		installed++
	}

	if installed > 0 {
		i.logger.LogSuccess(fmt.Sprintf("Installed %d type skills for %s", installed, pt))
	}
	if len(missing) > 0 {
		i.logger.LogWarning(fmt.Sprintf("Missing skill directories in source (%s): %s", skillsSrc, strings.Join(missing, ", ")))
	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would copy shared skills")
		return nil
	}

	}
	return nil
}

func (i *Installer) copySharedSkills() error {
	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would copy shared skills")
		return nil
	}

	types := i.loadTypesConfig()
	repoRoot := i.getRepoRoot()
	skillsSrc := i.findSkillsSource(repoRoot)
	skillsTgt := i.getSkillsDir()

	if skillsSrc == "" {
		i.logger.LogInfo("No shared skills directory found")
		return nil
	}

	if err := i.ensureDir(skillsTgt); err != nil {
		return err
	}

	if types.BaseConfig.CopySharedSkills {
		copiedAll, err := i.copySkillEntries(skillsSrc, skillsTgt)
		if err != nil {
			return err
		}
		if copiedAll > 0 {
			i.logger.LogSuccess(fmt.Sprintf("Copied %d shared skill asset(s)", copiedAll))
		}
	}

	sharedSrc := filepath.Join(skillsSrc, "_shared")
	sharedDst := filepath.Join(skillsTgt, "_shared")
	if !i.dirExists(sharedSrc) {
		return nil
	}

	if err := i.ensureDir(sharedDst); err != nil {
		return err
	}

	copiedShared, err := i.copySkillEntries(sharedSrc, sharedDst)
	if err != nil {
		return err
	}
	if copiedShared > 0 {
		i.logger.LogSuccess(fmt.Sprintf("Copied %d skills/_shared asset(s)", copiedShared))
	}

	return nil
}

func (i *Installer) copySkillEntries(sourceDir, targetDir string) (int, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return 0, err
	}

	copied := 0
	for _, entry := range entries {
		name := entry.Name()
		srcPath := filepath.Join(sourceDir, name)
		destPath := filepath.Join(targetDir, name)

		if i.fileExists(destPath) || i.dirExists(destPath) {
			if !i.flags.Force {
				continue
			}
			if err := os.RemoveAll(destPath); err != nil {
				return copied, err
			}
		}

		if entry.IsDir() {
			if err := i.copyDir(srcPath, destPath); err != nil {
				return copied, err
			}
		} else {
			if err := i.copyFile(srcPath, destPath); err != nil {
				return copied, err
			}
		}

		copied++
	}

	return copied, nil
}

func (i *Installer) resolveLocalSkillsSetupScript() string {
	ext := "sh"
	if runtime.GOOS == "windows" {
		ext = "ps1"
	}

	candidates := []string{
		filepath.Join(i.getSkillsDir(), fmt.Sprintf("setup.%s", ext)),
	}

	if source := i.findSkillsSource(i.getRepoRoot()); source != "" {
		candidates = append(candidates, filepath.Join(source, fmt.Sprintf("setup.%s", ext)))
	}
	candidates = append(candidates,
		i.ywaiCandidates(false, fmt.Sprintf("skills/setup.%s", ext))...,
	)

	return i.firstExistingFile(uniqueCleanPaths(candidates)...)
}

func (i *Installer) runLocalSkillsSetup() error {
	script := i.resolveLocalSkillsSetupScript()
	if script == "" {
		return nil
	}

	if i.flags.DryRun {
		i.logger.LogInfo("DRY RUN: Would run local skills setup")
		return nil
	}

	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-File", script, "--all")
		cmd.Dir = i.targetDir
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("YWAI_PROJECT_TYPE=%s", i.getEffectiveProjectType()),
		)
		if err := i.runCommandWithCmd(cmd, "powershell", "-ExecutionPolicy", "Bypass", "-File", script, "--all"); err != nil {
			return fmt.Errorf("failed to run local skills setup: %w", err)
		}
	} else {
		cmd := exec.Command("bash", script, "--all")
		cmd.Dir = i.targetDir
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("YWAI_PROJECT_TYPE=%s", i.getEffectiveProjectType()),
		)
		if err := i.runCommandWithCmd(cmd, "bash", script, "--all"); err != nil {
			return fmt.Errorf("failed to run local skills setup: %w", err)
		}
	}

	i.logger.LogSuccess("Configured local skills")
	return nil
}

func (i *Installer) copyCommands() error {
	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would copy commands")
		return nil
	}

	types := i.loadTypesConfig()
	if !types.BaseConfig.CopyCommands {
		return nil
	}

	source := i.firstExistingDir(i.ywaiCandidates(false, "commands", "setup/commands")...)
	if source == "" {
		return nil
	}

	dest := filepath.Join(i.targetDir, "commands")
	sourceForSync := source

	if i.flags.DryRun {
		i.logger.LogInfo("DRY RUN: Would copy commands directory")
		i.logger.LogInfo("DRY RUN: Would sync commands to .github/prompts")
		i.logger.LogInfo("DRY RUN: Would sync commands to OpenCode commands")
		return nil
	}

	if i.dirExists(dest) {
		sourceForSync = dest
		if i.flags.Force {
			if err := os.RemoveAll(dest); err != nil {
				return err
			}
			if err := i.copyDir(source, dest); err != nil {
				return err
			}
			i.logger.LogSuccess("Copied commands directory")
		}
	} else {
		if err := i.copyDir(source, dest); err != nil {
			return err
		}
		i.logger.LogSuccess("Copied commands directory")
		sourceForSync = dest
	}

	if err := i.syncCommandsToGitHubPrompts(sourceForSync); err != nil {
		return err
	}

	if err := i.syncCommandsToOpenCode(sourceForSync); err != nil {
		return err
	}

	return nil
}

func (i *Installer) syncCommandsToGitHubPrompts(sourceDir string) error {
	githubPromptsDir := filepath.Join(i.targetDir, ".github", "prompts")
	copied, err := i.copyMarkdownFiles(sourceDir, githubPromptsDir)
	if err != nil {
		return err
	}
	if copied > 0 {
		i.logger.LogSuccess(fmt.Sprintf("Synced %d command file(s) to .github/prompts", copied))
	}
	return nil
}

func (i *Installer) syncCommandsToOpenCode(sourceDir string) error {
	home, _ := os.UserHomeDir()
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}
	opencodeCommandsDir := filepath.Join(xdgConfig, "opencode", "commands")

	copied, err := i.copyMarkdownFiles(sourceDir, opencodeCommandsDir)
	if err != nil {
		return err
	}
	if copied > 0 {
		i.logger.LogSuccess(fmt.Sprintf("Synced %d command file(s) to OpenCode commands", copied))
	}
	return nil
}

func (i *Installer) copyMarkdownFiles(sourceDir, destDir string) (int, error) {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return 0, err
	}

	if err := i.ensureDir(destDir); err != nil {
		return 0, err
	}

	copied := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}

		src := filepath.Join(sourceDir, name)
		dst := filepath.Join(destDir, name)

		if i.fileExists(dst) {
			if !i.flags.Force {
				continue
			}
			if err := os.Remove(dst); err != nil {
				return copied, err
			}
		}

		if err := i.copyFile(src, dst); err != nil {
			return copied, err
		}
		copied++
	}

	return copied, nil
}
