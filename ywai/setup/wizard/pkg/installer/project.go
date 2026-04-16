package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var baseSkillsForAllTypes = []string{
	"git-commit",
	"skill-creator",
	"skill-sync",
}

func (i *Installer) configureProject() error {
	i.logger.LogStep("Configuring project...")

	if err := i.applyProjectType(); err != nil {
		return err
	}

	if err := i.installTypeSkills(); err != nil {
		i.logger.LogWarning("Failed to install type skills")
	}

	if err := i.copySharedSkills(); err != nil {
		i.logger.LogWarning("Failed to copy shared skills")
	}

	if err := i.runLocalSkillsSetup(); err != nil {
		i.logger.LogWarning("Failed to run local skills setup")
	}

	if err := i.copyCommands(); err != nil {
		i.logger.LogWarning("Failed to copy commands")
	}

	if err := i.updateGitignore(); err != nil {
		return err
	}

	if err := i.setupVSCodeSettings(); err != nil {
		return err
	}

	if err := i.initializeGA(); err != nil {
		i.logger.LogWarning("Failed to initialize GA")
	}

	i.logger.LogSuccess("Project configured")
	return nil
}

func (i *Installer) applyProjectType() error {
	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would apply project type")
		return nil
	}

	types := i.loadTypesConfig()

	pt := i.projectType
	if pt == "" {
		pt = i.inferProjectType()
		if pt == "" {
			pt = types.Default
		}
		i.logger.LogInfo(fmt.Sprintf("Inferred project type: %s", pt))
	}

	_, ok := types.Types[pt]
	if !ok {
		i.logger.LogWarning(fmt.Sprintf("Unknown project type '%s', falling back to default '%s'", pt, types.Default))
		pt = types.Default
	}
	i.projectType = pt

	i.logger.LogInfo(fmt.Sprintf("Applying project type: %s", pt))

	typeConfig := types.Types[pt]

	docs := []struct {
		relPath  string
		destName string
	}{
		{typeConfig.AgentsMD, "AGENTS.md"},
		{typeConfig.ReviewMD, "REVIEW.md"},
	}

	for _, doc := range docs {
		sourcePath := i.firstExistingFile(i.ywaiCandidates(false, i.typeDocCandidatePaths(pt, doc.destName, doc.relPath)...)...)
		destPath := filepath.Join(i.targetDir, doc.destName)

		if sourcePath == "" {
			i.logger.LogWarning(fmt.Sprintf("Template not found for %s (%s)", doc.destName, pt))
			continue
		}

		if !i.fileExists(destPath) || i.flags.Force {
			if i.fileExists(sourcePath) {
				if err := i.copyFile(sourcePath, destPath); err != nil {
					return err
				}
				i.logger.LogSuccess(fmt.Sprintf("Created %s", doc.destName))
			}
		} else {
			i.logger.LogInfo(fmt.Sprintf("%s already exists, skipping", doc.destName))
		}
	}

	if err := i.applyLefthookConfig(typeConfig.LefthookYML); err != nil {
		i.logger.LogWarning("Failed to apply lefthook.yml")
	}
	if err := i.appendAgentsTemplates(types.BaseConfig.AppendAgentsTemplates); err != nil {
		i.logger.LogWarning("Failed to append AGENTS templates")
	}

	i.logger.LogSuccess(fmt.Sprintf("Project type '%s' applied", pt))
	return nil
}

func (i *Installer) inferProjectType() string {
	packageJsonPath := filepath.Join(i.targetDir, "package.json")
	if i.fileExists(packageJsonPath) {
		return i.inferFromPackageJson(packageJsonPath)
	}

	pyprojectPath := filepath.Join(i.targetDir, "pyproject.toml")
	if i.fileExists(pyprojectPath) {
		return "python"
	}

	csprojFiles, _ := filepath.Glob(filepath.Join(i.targetDir, "*.csproj"))
	if len(csprojFiles) > 0 {
		return "dotnet"
	}

	dockerfile := filepath.Join(i.targetDir, "Dockerfile")
	if i.fileExists(dockerfile) {
		return i.inferFromDockerfile(dockerfile)
	}

	return "generic"
}

func (i *Installer) inferFromPackageJson(packageJsonPath string) string {
	data, err := os.ReadFile(packageJsonPath)
	if err != nil {
		return "generic"
	}

	content := string(data)
	// Check for NestJS
	if strings.Contains(content, "@nestjs/core") || strings.Contains(content, "nestjs") {
		if strings.Contains(content, "@angular") || strings.Contains(content, "angular") {
			return "nest-angular"
		}
		if strings.Contains(content, "react") || strings.Contains(content, "@react") {
			return "nest-react"
		}
		return "nest"
	}
	// Check for React
	if strings.Contains(content, "react") || strings.Contains(content, "@react") {
		return "react"
	}
	// Check for Angular (standalone)
	if strings.Contains(content, "@angular") || strings.Contains(content, "angular") {
		return "angular"
	}
	// Check for Node.js generic
	if strings.Contains(content, "node") || strings.Contains(content, "npm") {
		return "nest"
	}
	if strings.Contains(content, "python") || strings.Contains(content, "pip") {
		return "python"
	}
	if strings.Contains(content, "dotnet") || strings.Contains(content, "nuget") {
		return "dotnet"
	}

	return "generic"
}

func (i *Installer) inferFromDockerfile(dockerfile string) string {
	data, err := os.ReadFile(dockerfile)
	if err != nil {
		return "generic"
	}

	content := string(data)
	if strings.Contains(content, "node") || strings.Contains(content, "npm") {
		return "nest"
	}
	if strings.Contains(content, "python") || strings.Contains(content, "pip") {
		return "python"
	}
	if strings.Contains(content, "dotnet") || strings.Contains(content, "nuget") {
		return "dotnet"
	}

	return "generic"
}

func uniqueStrings(input []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(input))
	for _, value := range input {
		key := strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func normalizeTypesRelativePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "ywai/")
	path = strings.TrimPrefix(path, "setup/")
	return path
}

func withSetupCompatFallback(path string) []string {
	path = normalizeYWAIRelativePath(path)
	if path == "" {
		return nil
	}

	paths := []string{path}
	if !strings.HasPrefix(path, "setup/") {
		paths = append(paths, filepath.Join("setup", path))
	}
	return paths
}

func (i *Installer) typeDocCandidatePaths(projectType, docName, configuredRelPath string) []string {
	candidateRelPaths := make([]string, 0, 6)

	normalized := normalizeTypesRelativePath(configuredRelPath)
	if normalized != "" {
		candidateRelPaths = append(candidateRelPaths, withSetupCompatFallback(normalized)...)
	}

	candidateRelPaths = append(candidateRelPaths,
		filepath.Join("types", projectType, docName),
		filepath.Join("setup", "types", projectType, docName),
	)

	return uniqueCleanPaths(candidateRelPaths)
}

func (i *Installer) syncSkillMetadataTables() error {
	ext := "sh"
	if runtime.GOOS == "windows" {
		ext = "ps1"
	}

	syncScript := filepath.Join(i.getSkillsDir(), "skill-sync", "assets", fmt.Sprintf("sync.%s", ext))
	if !i.fileExists(syncScript) {
		return nil
	}

	if i.flags.DryRun {
		i.logger.LogInfo("DRY RUN: Would sync AGENTS metadata tables")
		return nil
	}

	var (
		cmd         *exec.Cmd
		commandName string
		args        []string
	)
	if runtime.GOOS == "windows" {
		commandName = "powershell"
		args = []string{"-ExecutionPolicy", "Bypass", "-File", syncScript}
		cmd = exec.Command(commandName, args...)
	} else {
		commandName = "bash"
		args = []string{syncScript}
		cmd = exec.Command(commandName, args...)
	}
	cmd.Dir = i.targetDir
	if err := i.runCommandWithCmd(cmd, commandName, args...); err != nil {
		return fmt.Errorf("failed to sync AGENTS metadata tables: %w", err)
	}

	i.logger.LogSuccess("Synced AGENTS metadata tables")
	return nil
}
