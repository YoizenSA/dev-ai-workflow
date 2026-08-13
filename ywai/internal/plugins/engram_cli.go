package plugins

import (
	"fmt"
	"os"
	"os/exec"

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
// omp has no setup command in Engram.
var engramSetupHosts = map[string]bool{
	"opencode": true,
	"pi":       true,
}

var runEngramSetup = defaultRunEngramSetup

func defaultRunEngramSetup(agent string) error {
	cmd := exec.Command("engram", "setup", agent)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// WireEngramMCP writes the catalog `engram mcp` entry into each supported
// host config (opencode, pi, omp, claude-code). For opencode and pi it
// also runs `engram setup` so the official plugin/package lands.
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
		if engramSetupHosts[host] {
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
