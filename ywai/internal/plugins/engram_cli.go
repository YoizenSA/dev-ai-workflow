package plugins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
)

// engramMCPHosts are the agent configs ywai can write an engram MCP entry into.
var engramMCPHosts = map[string]bool{
	"opencode":    true,
	"pi":          true,
	"omp":         true,
	"claude-code": true,
}

// engramSetupHosts get the official `engram setup <agent>` pass after the
// MCP entry is written. claude-code is omitted: that setup is interactive.
// omp has no setup command in Engram. opencode is omitted on purpose: its
// setup drops plugins/engram.ts, a v1-shaped plugin opencode2 rejects
// ("Plugin must export a default definition…"); engram runs as MCP only.
var engramSetupHosts = map[string]bool{
	"pi": true,
}

var (
	runEngramSetup     = defaultRunEngramSetup
	engramSetupPresent = defaultEngramSetupPresent
)

func defaultRunEngramSetup(agent string) error {
	cmd := exec.Command("engram", "setup", agent)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func defaultEngramSetupPresent(host string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	switch host {
	case "pi":
		_, err = os.Stat(filepath.Join(home, ".pi", "agent", "npm", "node_modules", "gentle-engram"))
		return err == nil
	default:
		return false
	}
}

// removeLegacyEngramPlugin deletes the plugins/engram.ts that `engram setup
// opencode` used to drop into the (possibly env-scoped) opencode config dir.
// Only the Engram adapter is removed: a different file with that name stays.
func removeLegacyEngramPlugin() {
	path := filepath.Join(config.OpenCodeConfigDir(), "plugins", "engram.ts")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	src := string(data)
	if !strings.Contains(src, "OpenCode plugin adapter") && !strings.Contains(src, "export const Engram") {
		return
	}
	if err := os.Remove(path); err != nil {
		fmt.Printf("  Warning: could not remove legacy engram plugin %s: %v\n", path, err)
		return
	}
	fmt.Println("  Removed legacy plugins/engram.ts (engram runs as MCP only)")
}

// WireEngramMCP writes the catalog `engram mcp` entry into each supported
// host config (opencode, pi, omp, claude-code). For pi it also runs
// `engram setup` so the official package lands; opencode gets MCP only.
func WireEngramMCP(hosts []string) error {
	if _, err := exec.LookPath("engram"); err != nil {
		return fmt.Errorf("engram binary not found — install it first")
	}
	entry, ok := mcp.CatalogByID("engram")
	if !ok {
		return fmt.Errorf("engram MCP entry missing from catalog")
	}

	var wired []string
	for _, host := range hosts {
		if !engramMCPHosts[host] {
			continue
		}
		shape := mcp.BuildEntryShape(host, entry, nil)
		if _, err := mcp.WriteAgentConfig(host, "engram", shape); err != nil {
			return fmt.Errorf("failed to wire engram MCP for %s: %w", host, err)
		}
		if host == "opencode" {
			removeLegacyEngramPlugin()
		}
		if engramSetupHosts[host] && !engramSetupPresent(host) {
			if err := runEngramSetup(host); err != nil {
				fmt.Printf("  Warning: engram setup %s failed: %v\n", host, err)
			}
		}
		wired = append(wired, host)
	}
	if len(wired) == 0 {
		return fmt.Errorf("no supported agent hosts to wire engram MCP into")
	}
	return nil
}
