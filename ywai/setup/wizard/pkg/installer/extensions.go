package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

func (i *Installer) installExtensions() error {
	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would install extensions")
		return nil
	}

	i.logger.LogStep("Installing extensions...")

	selected, hasConfig := i.selectedExtensionsByType()

	if err := i.installProjectExtensions(selected, hasConfig); err != nil {
		i.logger.LogWarning("Failed to install project extensions")
	}

	if err := i.installHooks(selected["hooks"], hasConfig); err != nil {
		i.logger.LogWarning("Failed to install hooks")
	}

	if err := i.installMCPs(selected["mcps"], hasConfig); err != nil {
		i.logger.LogWarning("Failed to install MCPs")
	}

	if err := i.installInstallSteps(selected["install-steps"], hasConfig); err != nil {
		i.logger.LogWarning("Failed to install install-steps")
	}

	if err := i.installCommands(selected["commands"], hasConfig); err != nil {
		i.logger.LogWarning("Failed to install commands")
	}

	i.logger.LogSuccess("Extensions installed")
	return nil
}

func (i *Installer) installVSCodeExtensions() error {
	if i.flags.SkipVSCode {
		return nil
	}

	if !i.commandExists("code") {
		i.logger.LogWarning("VS Code CLI not available, skipping extensions")
		return nil
	}

	i.logger.LogInfo("Installing VS Code extensions...")

	extensions := []string{"github.copilot", "github.copilot-chat"}

	for _, ext := range extensions {
		if i.flags.DryRun {
			i.logger.Log(fmt.Sprintf("DRY RUN: Would install VS Code extension: %s", ext))
			continue
		}

		if err := i.runCommand("code", "--install-extension", ext, "--force"); err != nil {
			i.logger.LogWarning(fmt.Sprintf("Could not install %s", ext))
		} else {
			i.logger.LogSuccess(fmt.Sprintf("Installed %s", ext))
		}
	}

	return nil
}

func (i *Installer) installProjectExtensions(selected map[string]map[string]bool, hasConfig bool) error {
	pt := i.getEffectiveProjectType()
	if hasConfig {
		i.logger.LogInfo(fmt.Sprintf("Using extension profile for type: %s", pt))
	}

	order := []string{"hooks", "mcps", "install-steps", "commands"}
	for _, extType := range order {
		names := selected[extType]
		if len(names) == 0 {
			continue
		}

		list := make([]string, 0, len(names))
		for extName := range names {
			list = append(list, extName)
		}
		sort.Strings(list)
		i.logger.LogInfo(fmt.Sprintf("%s: %s", formatExtensionTypeLabel(extType), strings.Join(list, ", ")))
	}

	return nil
}

func formatExtensionTypeLabel(extType string) string {
	switch extType {
	case "hooks":
		return "Hooks"
	case "mcps":
		return "MCPs"
	case "install-steps":
		return "Install steps"
	case "commands":
		return "Commands"
	default:
		return extType
	}
}

func (i *Installer) installHooks(allowed map[string]bool, hasConfig bool) error {
	// Skip hooks if flag is set
	if i.flags.SkipHooks {
		i.logger.LogInfo("Skipping hooks installation (--skip-hooks)")
		return nil
	}

	// Check if hooks are optional in config
	types := i.loadTypesConfig()
	if types.BaseConfig.OptionalHooks {
		i.logger.LogInfo("Hooks are optional, skipping installation (set --install-hooks to install)")
		return nil
	}

	extensionsRoot := i.getExtensionsRoot()
	if extensionsRoot == "" {
		return nil
	}
	hooksDir := filepath.Join(extensionsRoot, "hooks")

	if !i.dirExists(hooksDir) {
		return nil
	}
	if hasConfig && len(allowed) == 0 {
		return nil
	}

	i.logger.LogInfo("Installing hooks...")

	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		return err
	}

	installed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		hookName := entry.Name()
		if hasConfig && !allowed[hookName] {
			continue
		}

		srcPath := filepath.Join(hooksDir, hookName)
		destPath := filepath.Join(i.targetDir, ".ywai", "hooks", hookName)

		if i.flags.DryRun {
			i.logger.Log(fmt.Sprintf("DRY RUN: Would install hook: %s", hookName))
			i.logger.Log(fmt.Sprintf("DRY RUN: Would execute %s/install.sh", srcPath))
			installed++
			continue
		}

		if err := i.ensureDir(filepath.Dir(destPath)); err != nil {
			return err
		}

		if i.dirExists(destPath) && i.flags.Force {
			if err := os.RemoveAll(destPath); err != nil {
				return err
			}
		}
		if !i.dirExists(destPath) {
			if err := i.copyDir(srcPath, destPath); err != nil {
				return err
			}
		}

		if err := i.executeExtensionScript(srcPath); err != nil {
			i.logger.LogWarning(fmt.Sprintf("Hook %s script failed: %v", hookName, err))
		}
		installed++
	}

	if installed > 0 {
		i.logger.LogSuccess(fmt.Sprintf("Installed %d hooks", installed))
	}

	return nil
}

func (i *Installer) installMCPs(allowed map[string]bool, hasConfig bool) error {
	if i.flags.SkipMCPs {
		i.logger.LogInfo("Skipping MCPs (--skip-mcps)")
		return nil
	}

	extensionsRoot := i.getExtensionsRoot()
	if extensionsRoot == "" {
		return nil
	}
	mcpsDir := filepath.Join(extensionsRoot, "mcps")

	if !i.dirExists(mcpsDir) {
		return nil
	}
	if hasConfig && len(allowed) == 0 {
		return nil
	}

	if os.Getenv("YWAI_SKIP_MCPS") == "true" {
		i.logger.LogInfo("Skipping MCPs (YWAI_SKIP_MCPS=true)")
		return nil
	}

	i.logger.LogInfo("Installing MCPs...")

	entries, err := os.ReadDir(mcpsDir)
	if err != nil {
		return err
	}

	installed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		mcpName := entry.Name()
		if hasConfig && !allowed[mcpName] {
			continue
		}

		srcPath := filepath.Join(mcpsDir, mcpName)
		destPath := filepath.Join(i.targetDir, ".ywai", "mcps", mcpName)

		if i.flags.DryRun {
			i.logger.Log(fmt.Sprintf("DRY RUN: Would install MCP: %s", mcpName))
			i.logger.Log(fmt.Sprintf("DRY RUN: Would execute %s/install.sh", srcPath))
			installed++
			continue
		}

		if err := i.ensureDir(filepath.Dir(destPath)); err != nil {
			return err
		}

		if i.dirExists(destPath) && i.flags.Force {
			if err := os.RemoveAll(destPath); err != nil {
				return err
			}
		}
		if !i.dirExists(destPath) {
			if err := i.copyDir(srcPath, destPath); err != nil {
				return err
			}
		}

		if err := i.executeExtensionScript(srcPath); err != nil {
			i.logger.LogWarning(fmt.Sprintf("MCP %s script failed: %v", mcpName, err))
		}
		installed++
	}

	if installed > 0 {
		i.logger.LogSuccess(fmt.Sprintf("Installed %d MCPs", installed))
	}

	return nil
}

func (i *Installer) installInstallSteps(allowed map[string]bool, hasConfig bool) error {
	extensionsRoot := i.getExtensionsRoot()
	if extensionsRoot == "" {
		return nil
	}
	stepsDir := filepath.Join(extensionsRoot, "install-steps")

	if !i.dirExists(stepsDir) {
		return nil
	}
	if hasConfig && len(allowed) == 0 {
		return nil
	}

	i.logger.LogInfo("Installing install-steps...")

	entries, err := os.ReadDir(stepsDir)
	if err != nil {
		return err
	}

	installed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		stepName := entry.Name()
		if hasConfig && !allowed[stepName] {
			continue
		}

		srcPath := filepath.Join(stepsDir, stepName)
		destPath := filepath.Join(i.targetDir, ".ywai", "install-steps", stepName)

		if i.flags.DryRun {
			i.logger.Log(fmt.Sprintf("DRY RUN: Would install install-step: %s", stepName))
			i.logger.Log(fmt.Sprintf("DRY RUN: Would execute install-step: %s", stepName))
			installed++
			continue
		}

		if err := i.ensureDir(filepath.Dir(destPath)); err != nil {
			return err
		}
		if i.dirExists(destPath) && i.flags.Force {
			if err := os.RemoveAll(destPath); err != nil {
				return err
			}
		}
		if !i.dirExists(destPath) {
			if err := i.copyDir(srcPath, destPath); err != nil {
				return err
			}
		}

		if err := i.executeInstallStep(stepName, srcPath); err != nil {
			i.logger.LogWarning(fmt.Sprintf("Install-step %s failed: %v", stepName, err))
		}
		installed++
	}

	if installed > 0 {
		i.logger.LogSuccess(fmt.Sprintf("Installed %d install-steps", installed))
	}

	return nil
}

func (i *Installer) executeInstallStep(stepName, srcPath string) error {
	switch stepName {
	case "biome-baseline":
		if i.flags.SkipBiome {
			i.logger.LogInfo("Skipping biome-baseline (--skip-biome)")
			return nil
		}
		return i.installBiome()
	case "plannotator-setup":
		// Global-scope install-step: installs plannotator CLI and configures
		// detected agent tools (Gemini ~/.gemini, Claude, Copilot, Pi).
		// In GlobalOnly mode, pass /tmp so the script skips opencode.json edits.
		if i.flags.GlobalOnly {
			return i.executeExtensionScriptWithArgs(srcPath, "/tmp")
		}
		return i.executeExtensionScript(srcPath)
	case "metronous-setup":
		// Global-scope install-step: installs metronous CLI and configures
		// OpenCode telemetry (opt-in). Skips if InstallMetronous flag is false.
		if !i.flags.InstallMetronous {
			i.logger.LogInfo("Skipping metronous-setup (opt-in, not enabled)")
			return nil
		}
		return i.executeExtensionScript(srcPath)
	case "engram-setup":
		if i.flags.SkipEngram {
			i.logger.LogInfo("Skipping engram-setup (--skip-engram)")
			return nil
		}
		return i.executeExtensionScript(srcPath)
	case "sdd-engram-plugin":
		if i.flags.SkipSddEngramPlugin {
			i.logger.LogInfo("Skipping sdd-engram-plugin (--skip-sdd-engram-plugin)")
			return nil
		}
		return i.executeExtensionScript(srcPath)
	case "github-prompts", "slash-commands":
		if i.flags.SkipCommands {
			i.logger.LogInfo(fmt.Sprintf("Skipping %s (--skip-commands)", stepName))
			return nil
		}
		return i.executeExtensionScript(srcPath)
	case "global-agents":
		// Delegate to the in-process generator instead of running install.sh
		// to get consistent behavior across OS and preserve user-owned files.
		return i.UpdateGlobalAgents()
	default:
		return i.executeExtensionScript(srcPath)
	}
}

func (i *Installer) executeExtensionScript(srcPath string) error {
	return i.executeExtensionScriptWithArgs(srcPath, "")
}

func (i *Installer) executeExtensionScriptWithArgs(srcPath, extraArgs string) error {
	envVars := []string{
		fmt.Sprintf("YWAI_PROVIDER=%s", i.provider),
		fmt.Sprintf("YWAI_PROJECT_TYPE=%s", i.getEffectiveProjectType()),
	}

	if runtime.GOOS == "windows" {
		psScript := filepath.Join(srcPath, "install.ps1")
		if i.fileExists(psScript) {
			args := []string{"-ExecutionPolicy", "Bypass", "-File", psScript, "-TargetDir", i.targetDir}
			cmd := exec.Command("powershell", args...)
			cmd.Dir = srcPath
			cmd.Env = append(os.Environ(), envVars...)
			if err := i.runCommandWithCmd(cmd, "powershell", args...); err != nil {
				return fmt.Errorf("%w", err)
			}
			return nil
		}
	}

	script := filepath.Join(srcPath, "install.sh")
	if !i.fileExists(script) {
		return nil
	}

	args := []string{script, i.targetDir}
	if extraArgs != "" {
		args = append(args, extraArgs)
	}

	cmd := exec.Command("bash", args...)
	cmd.Dir = srcPath
	cmd.Env = append(os.Environ(), envVars...)
	if err := i.runCommandWithCmd(cmd, "bash", args...); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

func (i *Installer) selectedExtensionsByType() (map[string]map[string]bool, bool) {
	types := i.loadTypesConfig()
	pt := i.getEffectiveProjectType()

	selected := map[string]map[string]bool{
		"hooks":         {},
		"mcps":          {},
		"install-steps": {},
	}

	hasConfig := false
	for extType, extNames := range types.BaseConfig.Extensions {
		if _, ok := selected[extType]; !ok {
			selected[extType] = map[string]bool{}
		}
		for _, extName := range extNames {
			selected[extType][extName] = true
			hasConfig = true
		}
	}

	typeConfig, ok := types.Types[pt]
	if ok {
		hasConfig = true
		for extType, extNames := range typeConfig.Extensions {
			if _, ok := selected[extType]; !ok {
				selected[extType] = map[string]bool{}
			}
			for _, extName := range extNames {
				selected[extType][extName] = true
			}
		}
	}

	if i.flags.InstallGlobal {
		if i.presetAllowsGlobalAgents() {
			selected["install-steps"]["global-agents"] = true
		}
		if i.presetAllowsGlobalSkills() {
			selected["install-steps"]["global-skills"] = true
		}
		hasConfig = true
	}

	if i.flags.InstallMetronous {
		selected["install-steps"]["metronous-setup"] = true
		hasConfig = true
	}

	// Apply preset allow-lists (no-op for standard preset).
	for _, bucket := range []string{"hooks", "mcps", "install-steps"} {
		current := make([]string, 0, len(selected[bucket]))
		for name := range selected[bucket] {
			current = append(current, name)
		}
		filtered := i.filterByPreset(bucket, current)
		if len(filtered) != len(current) {
			newSet := map[string]bool{}
			for _, name := range filtered {
				newSet[name] = true
			}
			selected[bucket] = newSet
		}
	}

	// Respect preset's global-agents/global-skills gates even if --install-global
	// was not requested via flag (enforces minimal/full boundaries).
	if !i.presetAllowsGlobalAgents() {
		delete(selected["install-steps"], "global-agents")
	}
	if !i.presetAllowsGlobalSkills() {
		delete(selected["install-steps"], "global-skills")
	}

	return selected, hasConfig
}

func (i *Installer) getExtensionsRoot() string {
	return i.firstExistingDir(i.ywaiCandidates(true, "extensions")...)
}

func (i *Installer) getEffectiveProjectType() string {
	types := i.loadTypesConfig()
	pt := i.projectType
	if pt == "" {
		pt = i.inferProjectType()
	}
	if pt == "" {
		pt = types.Default
	}
	if _, ok := types.Types[pt]; !ok {
		pt = types.Default
	}
	i.projectType = pt
	return pt
}

func (i *Installer) installOpenCode() error {
	if !i.commandExists("npm") {
		i.logger.LogWarning("npm not available, skipping OpenCode CLI install")
		return nil
	}

	if i.commandExists("opencode") {
		i.logger.LogInfo("OpenCode CLI already installed")
		return nil
	}

	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would install OpenCode CLI")
		return nil
	}

	i.logger.Log("Installing OpenCode CLI...")
	if err := i.runCommand("npm", "install", "-g", "opencode-ai"); err != nil {
		home, _ := os.UserHomeDir()
		i.logger.LogWarning("Global npm install failed, retrying with user prefix ~/.local")
		if err2 := i.runCommand("npm", "install", "-g", "opencode-ai", "--prefix", filepath.Join(home, ".local")); err2 != nil {
			i.logger.LogWarning("Could not install OpenCode CLI")
			return nil
		}
	}

	i.logger.LogSuccess("OpenCode CLI installed")
	return nil
}

func (i *Installer) installCommands(allowed map[string]bool, hasConfig bool) error {
	extensionsRoot := i.getExtensionsRoot()
	if extensionsRoot == "" {
		return nil
	}
	commandsDir := filepath.Join(extensionsRoot, "commands")

	if !i.dirExists(commandsDir) {
		return nil
	}
	if hasConfig && len(allowed) == 0 {
		return nil
	}

	i.logger.LogInfo("Installing commands...")

	entries, err := os.ReadDir(commandsDir)
	if err != nil {
		return err
	}

	installed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		cmdName := entry.Name()
		if hasConfig && !allowed[cmdName] {
			continue
		}

		srcPath := filepath.Join(commandsDir, cmdName)
		destPath := filepath.Join(i.targetDir, ".ywai", "commands", cmdName)

		if i.flags.DryRun {
			i.logger.Log(fmt.Sprintf("DRY RUN: Would install command: %s", cmdName))
			installed++
			continue
		}

		if err := i.ensureDir(filepath.Dir(destPath)); err != nil {
			return err
		}

		if i.dirExists(destPath) && i.flags.Force {
			if err := os.RemoveAll(destPath); err != nil {
				return err
			}
		}
		if !i.dirExists(destPath) {
			if err := i.copyDir(srcPath, destPath); err != nil {
				return err
			}
		}

		if err := i.executeExtensionScript(srcPath); err != nil {
			i.logger.LogWarning(fmt.Sprintf("Command %s script failed: %v", cmdName, err))
		}
		installed++
	}

	if installed > 0 {
		i.logger.LogSuccess(fmt.Sprintf("Installed %d commands", installed))
	}

	return nil
}
