package plugins

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	exe, err := resolveGraftBinary()
	if err != nil {
		return err
	}
	entry, ok := mcp.CatalogByID("graft")
	if !ok {
		return fmt.Errorf("graft MCP entry missing from catalog")
	}

	// Only agents whose SettingsPaths entry is a JSON config with a shape
	// ywai understands (opencode "mcp", claude-code/pi
	// "mcpServers"). omp points at models.yml and the IDE agents use their
	// own file formats, so they are skipped.
	wired := false
	var failures []string
	for name, configPath := range agent.SettingsPaths() {
		switch name {
		case "opencode", "claude-code", "pi":
		default:
			continue
		}
		if configPath == "" {
			continue
		}
		// SettingsPaths resolves opencode with FindJSONCPath, which returns a
		// candidate path whether or not the file exists. Wiring an agent that
		// was never installed only produced a puzzling "cannot find the path"
		// warning, so skip what is not on disk.
		if _, err := os.Stat(configPath); err != nil {
			continue
		}
		if err := writeGraftMCPEntry(configPath, name, graftLaunchCommand(exe, entry.Command)); err != nil {
			// One unwritable config must not cost the others their MCP entry.
			// Returning here left the rest unwired, and since Go randomizes map
			// order, which agents got graft varied between runs.
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		wired = true
	}
	if !wired {
		if len(failures) > 0 {
			return fmt.Errorf("failed to wire graft MCP: %s", strings.Join(failures, "; "))
		}
		return fmt.Errorf("no agent configs found to wire graft MCP into")
	}
	if len(failures) > 0 {
		fmt.Printf("  Warning: graft MCP not wired for %s\n", strings.Join(failures, "; "))
	}
	return nil
}

// resolveGraftBinary returns the path to the graft executable. PATH comes
// first; when that misses it probes npm's global bin, because
// InstallGraftCLI runs `npm i -g` inside this same process and the process
// PATH is a snapshot taken at launch — on a machine where npm's global bin
// was not already on PATH (a fresh Node install), the binary exists but
// LookPath cannot see it until the next shell.
func resolveGraftBinary() (string, error) {
	if exe, err := exec.LookPath("graft"); err == nil {
		return exe, nil
	}
	if exe, ok := graftInNPMPrefix(); ok {
		return exe, nil
	}
	return "", fmt.Errorf("graft binary not found on PATH — it may be installed but not yet visible to this process; open a new shell and re-run, or install it with `npm i -g %s`", GraftNPMPackage)
}

// graftInNPMPrefix looks for graft in npm's global bin directory. On Windows
// npm puts the launchers directly in the prefix; elsewhere they live in
// prefix/bin.
func graftInNPMPrefix() (string, bool) {
	prefix, ok := npmGlobalPrefix()
	if !ok {
		return "", false
	}
	candidates := []string{filepath.Join(prefix, "bin", "graft")}
	if runtime.GOOS == "windows" {
		candidates = []string{filepath.Join(prefix, "graft.cmd"), filepath.Join(prefix, "graft.exe")}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c, true
		}
	}
	return "", false
}

// npmGlobalPrefix returns npm's global install prefix.
func npmGlobalPrefix() (string, bool) {
	out, err := exec.Command("npm", "prefix", "-g").Output()
	if err != nil {
		return "", false
	}
	prefix := strings.TrimSpace(string(out))
	return prefix, prefix != ""
}

// graftNodeEntrypoint locates the CLI's JavaScript entrypoint inside npm's
// global node_modules. Both npm launchers are thin wrappers that exec
// `node <this file>`, so calling node directly is equivalent.
func graftNodeEntrypoint() (string, bool) {
	prefix, ok := npmGlobalPrefix()
	if !ok {
		return "", false
	}
	js := filepath.Join(prefix, "node_modules", "@nanonets", "graft", "dist", "cli.js")
	if st, err := os.Stat(js); err == nil && !st.IsDir() {
		return js, true
	}
	return "", false
}

// graftLaunchCommand rewrites the catalog's argv so MCP hosts can actually
// spawn it on Windows, where none of npm's three launchers is reliably
// spawnable without a shell:
//
//   - `graft` (no extension) is a Unix shell script; a host that spawns
//     without a shell applies no PATHEXT, hits this file and fails ENOENT.
//   - `graft.cmd` is refused outright by Node-based hosts, which since
//     CVE-2024-27980 reject .cmd/.bat unless shell:true (EINVAL).
//
// Both launchers only exec `node <pkg>/dist/cli.js`, so the entry is written
// as node + that script: node.exe is a real executable every host can spawn
// directly. If the script cannot be located, fall back to the resolved
// launcher, which is still better than the bare name. Elsewhere the bare
// name is left alone so configs stay portable across machines.
func graftLaunchCommand(exe string, command []string) []string {
	if runtime.GOOS != "windows" || len(command) == 0 {
		return command
	}
	rest := append([]string{}, command[1:]...)
	if js, ok := graftNodeEntrypoint(); ok {
		return append([]string{"node", js}, rest...)
	}
	if exe == "" {
		return command
	}
	return append([]string{exe}, rest...)
}

// writeGraftMCPEntry writes the graft MCP server in the target's native
// shape. OpenCode requires type+command-array+enabled; Claude/pi
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
		servers := mcp.CollectOpenCodeServers(mcpMap)
		servers["graft"] = shape
		root[key] = mcp.WriteOpenCodeMCP(mcpMap, servers)
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
