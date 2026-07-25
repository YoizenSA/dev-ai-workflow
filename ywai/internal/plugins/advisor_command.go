package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// AdvisorCommandName is the slash command that drives the advisor's controls.
const AdvisorCommandName = "advisor.md"

// InstallAdvisorCommand writes the /advisor slash command into the agent's
// commands directory.
//
// The command is only a router: the actual reads and writes happen in the tools
// the advisor plugin registers, because a command file is a prompt and a prompt
// cannot change a setting. Installing it without the plugin would give the user
// a command that describes an action it cannot perform.
func InstallAdvisorCommand(commandsDir string) error {
	src, err := config.AdvisorCommandPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		return fmt.Errorf("create commands dir %s: %w", commandsDir, err)
	}
	dest := filepath.Join(commandsDir, AdvisorCommandName)
	if err := copyFile(src, dest); err != nil {
		return fmt.Errorf("copy advisor command: %w", err)
	}
	return nil
}
