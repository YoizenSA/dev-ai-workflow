package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// InstallSkillCommands copies every commands/*.md shipped by a skill into
// each agent commands dir. Skills own their slash commands; install mirrors
// them. A skill without a commands/ dir is a no-op, not an error.
func InstallSkillCommands(skill string, commandsDirs ...string) error {
	srcDir, err := skillCommandsPath(skill)
	if err != nil {
		return err
	}
	return installSkillCommandsFrom(srcDir, commandsDirs...)
}

func installSkillCommandsFrom(srcDir string, commandsDirs ...string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("read skill commands: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		src := filepath.Join(srcDir, e.Name())
		for _, dir := range commandsDirs {
			if dir == "" {
				continue
			}
			if err := installCommandMarkdown(src, dir, e.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}

func skillCommandsPath(skill string) (string, error) {
	candidates := []string{
		filepath.Join(config.SkillsSourceDir(), skill, "commands"),
		filepath.Join(config.DataSkillsDir(), skill, "commands"),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s command markdown not found", skill)
}
