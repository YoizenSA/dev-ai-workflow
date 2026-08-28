package plugins

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
)

// GraftNPMPackage is the npm package that ships the `graft` CLI.
// Upstream: https://github.com/nanonets/graft
const GraftNPMPackage = "@nanonets/graft"

// GraftInfo reports whether the `graft` CLI is on PATH and, if so, its
// version as printed by `graft --version`. installed is decided by
// exec.LookPath; version is "" when the binary exists but does not report a
// parseable version.
func GraftInfo() (version string, installed bool) {
	exe, err := exec.LookPath("graft")
	if err != nil {
		return "", false
	}
	if v, err := graftVersionFromBinary(exe); err == nil {
		return v, true
	}
	return "", true
}

// graftVersionFromBinary runs `graft --version` and returns the first
// non-empty line. Extracted for testability.
func graftVersionFromBinary(exe string) (string, error) {
	out, err := exec.Command(exe, "--version").Output()
	if err != nil {
		return "", err
	}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		return line, nil
	}
	return "", fmt.Errorf("no version line in graft --version output")
}

// InstallGraftCLI installs the `graft` CLI globally via npm. It is
// non-fatal: if npm is missing or the install fails, the returned error
// names the manual fallback so the caller can surface a single
// actionable warning.
//
// Install order:
//  1. Idempotent short-circuit: if `graft` is already on PATH, do nothing.
//  2. `npm i -g @nanonets/graft`.
func InstallGraftCLI() error {
	if _, err := exec.LookPath("graft"); err == nil {
		return nil
	}
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("graft install failed: npm is not installed — install Node.js, then run `npm i -g %s`", GraftNPMPackage)
	}
	cmd := exec.Command("npm", "i", "-g", GraftNPMPackage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install %s failed: %w", GraftNPMPackage, err)
	}
	return nil
}

// WireGraftMCP wires the graft MCP server into each configured agent by
// writing the `graft mcp` entry into the agent's config (opencode-style
// `mcp` key, or Claude Code / pi `mcpServers` key). ywai writes the entry
// natively instead of delegating to `graft init` so no instruction files
// are rewritten. Non-fatal: returns an error for the caller to surface as
// a warning.
//
// If the graft binary cannot be resolved, the error tells the user to run
// InstallGraftCLI first.
func WireGraftMCP() error {
	if _, err := exec.LookPath("graft"); err != nil {
		return fmt.Errorf("graft binary not found — install it first (it should have been installed in the previous step)")
	}
	entry, ok := mcp.CatalogByID("graft")
	if !ok {
		return fmt.Errorf("graft MCP entry missing from catalog")
	}

	// Only agents whose SettingsPaths entry is a JSON config with a shape
	// ywai understands (opencode/kilocode "mcp", claude-code/pi
	// "mcpServers"). omp points at models.yml and the IDE agents use their
	// own file formats, so they are skipped.
	wired := false
	for name, configPath := range agent.SettingsPaths() {
		switch name {
		case "opencode", "kilocode", "claude-code", "pi":
		default:
			continue
		}
		if configPath == "" {
			continue
		}
		if err := writeGraftMCPEntry(configPath, name, entry.Command); err != nil {
			return fmt.Errorf("failed to wire graft MCP for %s: %w", name, err)
		}
		wired = true
	}
	if !wired {
		return fmt.Errorf("no agent configs found to wire graft MCP into")
	}
	return nil
}

// writeGraftMCPEntry writes the graft MCP server in the target's native
// shape. OpenCode/kilocode require type+command-array+enabled; Claude/pi
// use command+args. Always overwrites so a previous Claude-shaped write
// cannot leave OpenCode unable to boot.
func writeGraftMCPEntry(configPath, agentName string, command []string) error {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return err
	}
	key := mcpConfigKey(agentName)
	mcpMap, _ := root[key].(map[string]any)
	if mcpMap == nil {
		mcpMap = map[string]any{}
	}
	entry := mcp.CatalogEntry{Type: "local", Command: command}
	shape := mcp.BuildEntryShape(mcpShapeTarget(agentName), entry, nil)
	if key == "mcp" {
		servers := collectOpenCodeServers(mcpMap)
		servers["graft"] = shape
		root[key] = flattenOpenCodeMCP(mcpMap, servers)
	} else {
		mcpMap["graft"] = shape
		root[key] = mcpMap
	}
	return config.WriteJSONC(configPath, root)
}

// mcpShapeTarget maps hosts that share OpenCode's mcp object to the
// "opencode" BuildEntryShape branch (type: local, command as argv).
func mcpShapeTarget(agentName string) string {
	if mcpConfigKey(agentName) == "mcp" {
		return "opencode"
	}
	return agentName
}
