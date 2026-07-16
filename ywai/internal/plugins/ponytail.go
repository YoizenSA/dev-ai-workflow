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

// PonytailNPMPackage is the official OpenCode plugin package for ponytail.
// OpenCode resolves npm package names listed in the config "plugin" array.
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
//   - opencode / kilocode: appends PonytailNPMPackage to the config "plugin" array
//     (OpenCode resolves the npm package at load time; no global npm install).
//   - claude-code: runs non-interactive Claude CLI marketplace add + plugin install
//     (`claude plugin marketplace add DietrichGebert/ponytail` then
//     `claude plugin install ponytail@ponytail`). Both commands are idempotent.
//
// configPath is required for OpenCode-format agents; ignored for claude-code.
// Returns an error the caller should surface as a non-fatal warning.
func InstallPonytail(agentName, configPath string) error {
	switch agentName {
	case "opencode", "kilocode":
		return patchOpenCodePluginName(configPath, PonytailNPMPackage)
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

// patchOpenCodePluginName appends pluginName to the config's "plugin" array
// if it is not already present. Creates the config file when missing.
func patchOpenCodePluginName(configPath, pluginName string) error {
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

	plugins, _ := root["plugin"].([]any)
	if !containsPluginPath(plugins, pluginName) {
		plugins = append(plugins, pluginName)
	}
	root["plugin"] = plugins

	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
}
