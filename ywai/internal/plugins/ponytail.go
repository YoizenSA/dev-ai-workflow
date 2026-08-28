package plugins

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// PonytailNPMPackage is retained to remove legacy OpenCode plugin entries.
// Ponytail's published OpenCode plugin is not compatible with OpenCode v2.
const PonytailNPMPackage = "@dietrichgebert/ponytail"

// PonytailClaudeMarketplaceSource is the GitHub owner/repo shorthand passed to
// `claude plugin marketplace add`.
const PonytailClaudeMarketplaceSource = "DietrichGebert/ponytail"

// PonytailClaudePluginID is the plugin@marketplace id for
// `claude plugin install`.
const PonytailClaudePluginID = "ponytail@ponytail"

// claudeCLI is the Claude Code binary name. Overridable in tests.
var claudeCLI = "claude"

// InstallPonytail installs the official ponytail plugin for the given agent.
//
//   - opencode / kilocode: removes the legacy Ponytail npm plugin entry. Ponytail
//     is not installed in OpenCode v2 plugin mode because its published plugin
//     contract is incompatible with the v2 loader.
//   - claude-code: runs non-interactive Claude CLI marketplace add + plugin install
//     (`claude plugin marketplace add DietrichGebert/ponytail` then
//     `claude plugin install ponytail@ponytail`). Both commands are idempotent.
//
// configPath is required for OpenCode-format agents; ignored for claude-code.
// Returns an error the caller should surface as a non-fatal warning.
func InstallPonytail(agentName, configPath string) error {
	switch agentName {
	case "opencode", "kilocode":
		return removeOpenCodePluginName(configPath, PonytailNPMPackage)
	case "claude-code":
		return installPonytailClaude()
	default:
		return fmt.Errorf("ponytail install not supported for agent %q", agentName)
	}
}

// SupportsPonytail reports whether ywai can install ponytail for the agent.
func SupportsPonytail(agentName string) bool {
	switch agentName {
	case "opencode", "kilocode", "claude-code":
		return true
	default:
		return false
	}
}

// installPonytailClaude adds the ponytail marketplace and installs the plugin
// via the Claude Code CLI (user scope).
func installPonytailClaude() error {
	if _, err := exec.LookPath(claudeCLI); err != nil {
		return fmt.Errorf("%s not found in PATH — install Claude Code, then run: %s plugin marketplace add %s && %s plugin install %s",
			claudeCLI, claudeCLI, PonytailClaudeMarketplaceSource, claudeCLI, PonytailClaudePluginID)
	}

	// marketplace add is idempotent when already declared ("already on disk").
	if out, err := runClaudePlugin("marketplace", "add", PonytailClaudeMarketplaceSource); err != nil {
		return fmt.Errorf("claude plugin marketplace add %s failed: %w%s",
			PonytailClaudeMarketplaceSource, err, formatCmdOutput(out))
	}

	// install is idempotent when already installed.
	if out, err := runClaudePlugin("install", PonytailClaudePluginID, "-s", "user"); err != nil {
		return fmt.Errorf("claude plugin install %s failed: %w%s",
			PonytailClaudePluginID, err, formatCmdOutput(out))
	}
	return nil
}

// runClaudePlugin runs `claude plugin <args...>` and returns combined output.
func runClaudePlugin(args ...string) (string, error) {
	full := append([]string{"plugin"}, args...)
	cmd := exec.Command(claudeCLI, full...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	cmd.Env = os.Environ()
	err := cmd.Run()
	return buf.String(), err
}

func formatCmdOutput(out string) string {
	out = strings.TrimSpace(out)
	if out == "" {
		return ""
	}
	return "\n" + out
}

// removeOpenCodePluginName removes pluginName from the config's "plugins"
// array while preserving all other plugin entries. It is idempotent.
func removeOpenCodePluginName(configPath, pluginName string) error {
	root := map[string]any{}
	if _, err := os.Stat(configPath); err == nil {
		var readErr error
		root, readErr = config.ReadJSONC(configPath)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", configPath, readErr)
		}
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	plugins := openCodePlugins(root)
	filtered := make([]any, 0, len(plugins))
	for _, plugin := range plugins {
		if name, ok := plugin.(string); ok && name == pluginName {
			continue
		}
		filtered = append(filtered, plugin)
	}
	writePlugins(root, filtered)

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
}
