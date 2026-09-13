package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// LearnYwaiCommandName is the slash command that starts the official-docs tour.
const LearnYwaiCommandName = "learn-ywai.md"

// InstallLearnYwaiCommand writes /learn-ywai into each agent commands dir.
func InstallLearnYwaiCommand(commandsDirs ...string) error {
	src, err := learnYwaiCommandPath()
	if err != nil {
		return err
	}
	return installLearnYwaiCommandFrom(src, commandsDirs...)
}

func installLearnYwaiCommandFrom(src string, commandsDirs ...string) error {
	for _, dir := range commandsDirs {
		if dir == "" {
			continue
		}
		if err := installCommandMarkdown(src, dir, LearnYwaiCommandName); err != nil {
			return err
		}
	}
	return nil
}

func installCommandMarkdown(src, destDir, name string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create commands dir %s: %w", destDir, err)
	}
	if err := copyFile(src, filepath.Join(destDir, name)); err != nil {
		return fmt.Errorf("copy %s: %w", name, err)
	}
	return nil
}

func learnYwaiCommandPath() (string, error) {
	candidates := []string{
		filepath.Join(config.SkillsSourceDir(), "learn-ywai", "commands", LearnYwaiCommandName),
		filepath.Join(config.DataSkillsDir(), "learn-ywai", "commands", LearnYwaiCommandName),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("learn-ywai command markdown not found")
}

// DefaultLearnYwaiCommandDirs is every host ywai mirrors slash commands to.
func DefaultLearnYwaiCommandDirs() []string {
	return []string{
		config.OpenCodeCommandsDir(),
		config.ClaudeCommandsDir(),
		config.PiCommandsDir(),
		config.OmpCommandsDir(),
	}
}
