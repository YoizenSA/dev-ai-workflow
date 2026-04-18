package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// copyCanonicalSDDModels copies ywai/config/sdd-models.json to .ywai/sdd-models.json
// in the target project so SDD skills and the GA review CLI can resolve a
// per-phase model without relying on the install location. Governed by
// base_config.copy_sdd_models.
func (i *Installer) copyCanonicalSDDModels() error {
	types := i.loadTypesConfig()
	if !types.BaseConfig.CopySDDModels {
		return nil
	}
	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would copy sdd-models.json to .ywai/")
		return nil
	}

	src := i.firstExistingFile(i.ywaiCandidates(false,
		"config/sdd-models.json",
		"ywai/config/sdd-models.json",
	)...)
	if src == "" {
		return nil
	}

	destDir := filepath.Join(i.targetDir, ".ywai")
	if err := i.ensureDir(destDir); err != nil {
		return err
	}
	dest := filepath.Join(destDir, "sdd-models.json")
	if i.fileExists(dest) && !i.flags.Force {
		return nil
	}
	if err := i.copyFile(src, dest); err != nil {
		return fmt.Errorf("copy sdd-models.json: %w", err)
	}
	i.logger.LogSuccess("Copied sdd-models.json to .ywai/")
	return nil
}

// generateSkillRegistry runs skills/skill-sync/assets/sync.sh --registry in
// the target project to emit .ywai/skill-registry.md. Silently no-ops on
// Windows when bash is unavailable and outside dry-run.
func (i *Installer) generateSkillRegistry() error {
	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would generate .ywai/skill-registry.md")
		return nil
	}

	script := filepath.Join(i.targetDir, "skills", "skill-sync", "assets", "sync.sh")
	if !i.fileExists(script) {
		return nil
	}

	bashPath := ""
	if runtime.GOOS == "windows" {
		// Prefer Git Bash if available; otherwise skip quietly.
		for _, cand := range []string{
			`C:\Program Files\Git\bin\bash.exe`,
			`C:\Program Files (x86)\Git\bin\bash.exe`,
		} {
			if i.fileExists(cand) {
				bashPath = cand
				break
			}
		}
		if bashPath == "" {
			if p, err := exec.LookPath("bash"); err == nil {
				bashPath = p
			}
		}
	} else {
		bashPath = "bash"
	}

	if bashPath == "" {
		i.logger.LogInfo("Skill registry generation skipped (bash not available)")
		return nil
	}

	cmd := exec.Command(bashPath, script, "--registry")
	cmd.Dir = i.targetDir
	cmd.Env = os.Environ()
	cmd.Stdout = i.out
	cmd.Stderr = i.out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("skill-sync --registry: %w", err)
	}
	return nil
}
