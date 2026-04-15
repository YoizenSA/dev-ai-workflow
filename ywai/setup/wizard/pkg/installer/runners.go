package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (i *Installer) runAll() error {
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

	return nil
}

func (i *Installer) runSelected() error {
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
	home, _ := os.UserHomeDir()
	agentsDir := filepath.Join(home, ".config", "opencode")

	srcDir := i.firstExistingDir(
		i.ywaiCandidates(false, "extensions/install-steps/global-agents")...,
	)
	if srcDir == "" {
		return fmt.Errorf("global-agents extension not found in any YWAI location")
	}

	tplDir := filepath.Join(srcDir, "templates")
	if !i.dirExists(tplDir) {
		return fmt.Errorf("global agent templates not found")
	}

	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(tplDir)
	if err != nil {
		return err
	}

	installed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		dest := filepath.Join(agentsDir, e.Name())
		if err := i.copyFile(filepath.Join(tplDir, e.Name()), dest); err != nil {
			i.logger.LogWarning(fmt.Sprintf("Failed to copy %s: %v", e.Name(), err))
			continue
		}
		installed++
	}

	if installed > 0 {
		i.logger.LogSuccess(fmt.Sprintf("Updated %d global agent(s)", installed))
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
