package installer

import (
	"fmt"

	"github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/installer/globalagents"
)

func (i *Installer) runAll() error {
	i.logActivePreset()

	if err := i.installGA(); err != nil {
		return err
	}

	if err := i.installSDD(); err != nil {
		return err
	}

	if err := i.installVSCodeExtensions(); err != nil {
		return err
	}

	if err := i.installOpenCode(); err != nil {
		return err
	}

	if err := i.configureProject(); err != nil {
		return err
	}

	if err := i.installExtensions(); err != nil {
		return err
	}

	if err := i.saveYWAIConfig(); err != nil {
		i.logger.LogWarning("Failed to save YWAI config: " + err.Error())
	}

	if err := i.copyCanonicalSDDModels(); err != nil {
		i.logger.LogWarning("Failed to copy sdd-models.json: " + err.Error())
	}

	if err := i.generateSkillRegistry(); err != nil {
		i.logger.LogWarning("Failed to generate skill-registry.md: " + err.Error())
	}

	return nil
}

// logActivePreset prints the active preset name when it is not the default.
func (i *Installer) logActivePreset() {
	name, _ := i.activePreset()
	if name != "" && name != "standard" {
		i.logger.LogInfo(fmt.Sprintf("Active install preset: %s", name))
	}
}

func (i *Installer) runSelected() error {
	i.logActivePreset()

	if !i.flags.SkipGA {
		if err := i.installGA(); err != nil {
			return err
		}
	}

	if i.flags.InstallSDD && !i.flags.SkipSDD {
		if err := i.installSDD(); err != nil {
			return err
		}
	}

	if i.flags.InstallVSCode && !i.flags.SkipVSCode {
		if err := i.installVSCodeExtensions(); err != nil {
			return err
		}
	}

	if err := i.installOpenCode(); err != nil {
		return err
	}

	if err := i.configureProject(); err != nil {
		return err
	}

	if err := i.installExtensions(); err != nil {
		return err
	}

	if err := i.saveYWAIConfig(); err != nil {
		i.logger.LogWarning("Failed to save YWAI config: " + err.Error())
	}

	if err := i.copyCanonicalSDDModels(); err != nil {
		i.logger.LogWarning("Failed to copy sdd-models.json: " + err.Error())
	}

	if err := i.generateSkillRegistry(); err != nil {
		i.logger.LogWarning("Failed to generate skill-registry.md: " + err.Error())
	}

	return nil
}

func (i *Installer) updateAll() error {
	i.logger.LogStep("Updating YWAI installation...")

	if err := i.updateGA(); err != nil {
		return err
	}

	if i.flags.InstallSDD || i.flags.All {
		if err := i.installSDD(); err != nil {
			return err
		}
	}

	if i.flags.InstallVSCode && !i.flags.SkipVSCode {
		if err := i.installVSCodeExtensions(); err != nil {
			return err
		}
	}

	if err := i.installOpenCode(); err != nil {
		return err
	}

	if err := i.configureProject(); err != nil {
		return err
	}

	if err := i.installExtensions(); err != nil {
		return err
	}

	i.logger.LogSuccess("YWAI update complete")
	return nil
}

func (i *Installer) checkForUpdates() error {
	currentVersion, err := i.versionResolver.GetInstalledVersion()
	if err != nil {
		return fmt.Errorf("failed to get installed version: %w", err)
	}

	if currentVersion == "" {
		i.logger.LogInfo("GA is not installed")
		return nil
	}

	hasUpdate, latestVersion, err := i.versionResolver.CheckForUpdates(currentVersion, i.channel)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if hasUpdate {
		i.logger.LogSuccess(fmt.Sprintf("Update available: %s → %s", currentVersion, latestVersion))
	} else {
		i.logger.LogInfo(fmt.Sprintf("GA is up to date: %s", currentVersion))
	}

	return nil
}

func (i *Installer) listTypes() error {
	types := i.loadTypesConfig()

	fmt.Println("Available project types:")
	fmt.Println("")

	for name, config := range types.Types {
		marker := " "
		if name == types.Default {
			marker = "*"
		}
		fmt.Printf("%s %s - %s\n", marker, name, config.Description)
	}

	fmt.Println("")
	fmt.Printf("Default: %s\n", types.Default)
	fmt.Println("")
	fmt.Println("Use with --type=<name>")

	return nil
}

func (i *Installer) listExtensions() error {
	types := i.loadTypesConfig()

	pt := i.projectType
	if pt == "" {
		pt = types.Default
	}

	typeConfig, ok := types.Types[pt]
	if !ok {
		return fmt.Errorf("project type '%s' not found", pt)
	}

	fmt.Printf("Extensions for project type '%s':\n", pt)
	fmt.Println("")

	if len(typeConfig.Extensions) == 0 {
		fmt.Println("No extensions configured for this type")
		return nil
	}

	for extType, extNames := range typeConfig.Extensions {
		fmt.Printf("%s:\n", extType)
		for _, extName := range extNames {
			fmt.Printf("  - %s\n", extName)
		}
		fmt.Println("")
	}

	return nil
}

func (i *Installer) UpdateGA() error {
	return i.installGA()
}

func (i *Installer) UpdateSDD() error {
	return i.installSDD()
}

func (i *Installer) UpdateGlobalAgents() error {
	srcDir := i.firstExistingDir(
		i.ywaiCandidates(false, "extensions/install-steps/global-agents")...,
	)
	if srcDir == "" {
		return fmt.Errorf("global-agents extension not found in any YWAI location")
	}

	skillsDir := i.firstExistingDir(i.ywaiCandidates(false, "skills")...)
	typesJSON := i.firstExistingFile(
		i.ywaiCandidates(true, "types/types.json")...,
	)

	if i.flags.GlobalOnly {
		i.logger.LogInfo("GlobalOnly mode: Installing global agents only")
	}

	gen := globalagents.Generator{
		ExtensionDir: srcDir,
		SkillsDir:    skillsDir,
		TypesJSON:    typesJSON,
		ProjectType:  i.getEffectiveProjectType(),
		Logger:       i.logger,
	}

	installed, err := gen.InstallAll()
	if err != nil {
		return err
	}
	if installed > 0 {
		i.logger.LogSuccess(fmt.Sprintf("Updated %d global agent file(s)", installed))
	}
	return nil
}

func (i *Installer) UpdateEngram() error {
	extDir := i.firstExistingDir(
		i.ywaiCandidates(false, "extensions/install-steps/engram-setup")...,
	)
	if extDir == "" {
		return fmt.Errorf("engram-setup extension not found in any YWAI location")
	}

	// In GlobalOnly mode, pass empty target dir to prevent repo writes
	if i.flags.GlobalOnly {
		return i.executeExtensionScriptWithArgs(extDir, "/tmp")
	}
	return i.executeExtensionScriptWithArgs(extDir, "")
}

func (i *Installer) UpdatePlannotator() error {
	extDir := i.firstExistingDir(
		i.ywaiCandidates(false, "extensions/install-steps/plannotator-setup")...,
	)
	if extDir == "" {
		return fmt.Errorf("plannotator-setup extension not found in any YWAI location")
	}

	// In GlobalOnly mode, pass /tmp to prevent repo writes (opencode.json edits).
	if i.flags.GlobalOnly {
		return i.executeExtensionScriptWithArgs(extDir, "/tmp")
	}
	return i.executeExtensionScriptWithArgs(extDir, "")
}

func (i *Installer) UpdateContext7() error {
	extDir := i.firstExistingDir(
		i.ywaiCandidates(false, "extensions/mcps/context7-mcp")...,
	)
	// In GlobalOnly mode, pass empty target dir to prevent repo writes
	if i.flags.GlobalOnly {
		return i.executeExtensionScriptWithArgs(extDir, "/tmp")
	}
	if extDir == "" {
		return fmt.Errorf("context7-mcp extension not found in any YWAI location")
	}

	return i.executeExtensionScriptWithArgs(extDir, "")
}

func (i *Installer) UpdateMetronous() error {
	extDir := i.firstExistingDir(
		i.ywaiCandidates(false, "extensions/install-steps/metronous-setup")...,
	)
	if extDir == "" {
		return fmt.Errorf("metronous-setup extension not found in any YWAI location")
	}

	// In GlobalOnly mode, pass /tmp to prevent repo writes
	if i.flags.GlobalOnly {
		return i.executeExtensionScriptWithArgs(extDir, "/tmp")
	}
	return i.executeExtensionScriptWithArgs(extDir, "")
}

func (i *Installer) UpdateSDDEngramPlugin() error {
	extDir := i.firstExistingDir(
		i.ywaiCandidates(false, "extensions/install-steps/sdd-engram-plugin")...,
	)
	if extDir == "" {
		return fmt.Errorf("sdd-engram-plugin extension not found in any YWAI location")
	}

	// In GlobalOnly mode, pass /tmp to prevent repo writes
	if i.flags.GlobalOnly {
		return i.executeExtensionScriptWithArgs(extDir, "/tmp")
	}
	return i.executeExtensionScriptWithArgs(extDir, "")
}
