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

// TrailNPMPackage is the npm package that ships the `trail` CLI.
// Upstream: https://github.com/trailhq/Trail-Code
const TrailNPMPackage = "@trailhq/trail-code"

// LegacyGraftNPMPackage is the package Trail was renamed from. ywai uninstalls
// it on install/update so the stale `trail` binary does not shadow trail.
const LegacyGraftNPMPackage = "@nanonets/graft"

// trailAllowScripts lists the packages whose install scripts must run: without
// them npm 11+ skips the native tree-sitter builds and trail cannot parse code.
var trailAllowScripts = []string{
	TrailNPMPackage, "tree-sitter", "tree-sitter-go", "tree-sitter-java",
	"tree-sitter-kotlin", "tree-sitter-php", "tree-sitter-python",
	"@davisvaughan/tree-sitter-r", "tree-sitter-typescript", "tree-sitter-javascript",
}

// TrailInfo reports whether the `trail` CLI is on PATH and, if so, its
// version as printed by `trail --version`.
func TrailInfo() (version string, installed bool) {
	exe, err := exec.LookPath("trail")
	if err != nil {
		return "", false
	}
	if v, err := trailVersionFromBinary(exe); err == nil {
		return v, true
	}
	return "", true
}

// trailVersionFromBinary runs `<exe> --version` and returns the first
// non-empty line.
func trailVersionFromBinary(exe string) (string, error) {
	out, err := exec.Command(exe, "--version").Output()
	if err != nil {
		return "", err
	}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			return line, nil
		}
	}
	return "", fmt.Errorf("no version line in trail --version output")
}

// InstallTrailCLI installs the `trail` CLI globally via npm and removes the
// legacy `@nanonets/trail` package. Non-fatal: the returned error names the
// manual fallback.
func InstallTrailCLI() error {
	if _, err := exec.LookPath("npm"); err != nil {
		if _, err := exec.LookPath("trail"); err == nil {
			return nil
		}
		return fmt.Errorf("trail install failed: npm is not installed — install Node.js, then run `npm i -g %s`", TrailNPMPackage)
	}
	uninstallLegacyGraft()
	if _, err := exec.LookPath("trail"); err == nil {
		return nil
	}
	args := []string{"i", "-g", TrailNPMPackage, "--allow-scripts=" + strings.Join(trailAllowScripts, ",")}
	if err := runNPM(args...); err != nil {
		// ponytail: npm older than the allow-scripts flag rejects it; retry bare
		// (those versions run install scripts by default anyway).
		if err2 := runNPM("i", "-g", TrailNPMPackage); err2 != nil {
			return fmt.Errorf("npm install %s failed: %w", TrailNPMPackage, err2)
		}
	}
	return nil
}

// uninstallLegacyGraft removes @nanonets/graft when npm lists it globally.
// Best effort: a failure only leaves a stale binary behind.
func uninstallLegacyGraft() {
	if err := exec.Command("npm", "ls", "-g", "--depth=0", LegacyGraftNPMPackage).Run(); err != nil {
		return // not installed (npm ls exits non-zero)
	}
	fmt.Printf("  Removing legacy %s (renamed to %s)...\n", LegacyGraftNPMPackage, TrailNPMPackage)
	if err := runNPM("uninstall", "-g", LegacyGraftNPMPackage); err != nil {
		fmt.Printf("  Warning: npm uninstall %s failed: %v\n", LegacyGraftNPMPackage, err)
	}
}

func runNPM(args ...string) error {
	cmd := exec.Command("npm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// WireTrailMCP wires the trail MCP server into each configured agent by
// writing the `trail mcp` entry into the agent's config (opencode-style
// `mcp` key, or Claude Code / pi `mcpServers` key). ywai writes the entry
// natively instead of delegating to `trail init` so no instruction files
// are rewritten. Non-fatal: returns an error for the caller to surface as
// a warning.
//
// If the trail binary cannot be resolved, the error tells the user to run
// InstallTrailCLI first.
func WireTrailMCP() error {
	exe, err := resolveTrailBinary()
	if err != nil {
		return err
	}
	entry, ok := mcp.CatalogByID("trail")
	if !ok {
		return fmt.Errorf("trail MCP entry missing from catalog")
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
		if err := writeTrailMCPEntry(configPath, name, trailLaunchCommand(exe, entry.Command)); err != nil {
			// One unwritable config must not cost the others their MCP entry.
			// Returning here left the rest unwired, and since Go randomizes map
			// order, which agents got trail varied between runs.
			failures = append(failures, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		wired = true
	}
	if !wired {
		if len(failures) > 0 {
			return fmt.Errorf("failed to wire trail MCP: %s", strings.Join(failures, "; "))
		}
		return fmt.Errorf("no agent configs found to wire trail MCP into")
	}
	if len(failures) > 0 {
		fmt.Printf("  Warning: trail MCP not wired for %s\n", strings.Join(failures, "; "))
	}
	return nil
}

// resolveTrailBinary returns the path to the trail executable. PATH comes
// first; when that misses it probes npm's global bin, because
// InstallTrailCLI runs `npm i -g` inside this same process and the process
// PATH is a snapshot taken at launch — on a machine where npm's global bin
// was not already on PATH (a fresh Node install), the binary exists but
// LookPath cannot see it until the next shell.
func resolveTrailBinary() (string, error) {
	if exe, err := exec.LookPath("trail"); err == nil {
		return exe, nil
	}
	if exe, ok := trailInNPMPrefix(); ok {
		return exe, nil
	}
	return "", fmt.Errorf("trail binary not found on PATH — it may be installed but not yet visible to this process; open a new shell and re-run, or install it with `npm i -g %s`", TrailNPMPackage)
}

// trailInNPMPrefix looks for trail in npm's global bin directory. On Windows
// npm puts the launchers directly in the prefix; elsewhere they live in
// prefix/bin.
func trailInNPMPrefix() (string, bool) {
	prefix, ok := npmGlobalPrefix()
	if !ok {
		return "", false
	}
	candidates := []string{filepath.Join(prefix, "bin", "trail")}
	if runtime.GOOS == "windows" {
		candidates = []string{filepath.Join(prefix, "trail.cmd"), filepath.Join(prefix, "trail.exe")}
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

// trailNodeEntrypoint locates the CLI's JavaScript entrypoint inside npm's
// global node_modules. Both npm launchers are thin wrappers that exec
// `node <this file>`, so calling node directly is equivalent.
func trailNodeEntrypoint() (string, bool) {
	prefix, ok := npmGlobalPrefix()
	if !ok {
		return "", false
	}
	js := filepath.Join(prefix, "node_modules", "@trailhq", "trail-code", "dist", "bin", "trail.js")
	if st, err := os.Stat(js); err == nil && !st.IsDir() {
		return js, true
	}
	return "", false
}

// trailLaunchCommand rewrites the catalog's argv so MCP hosts can actually
// spawn it on Windows, where none of npm's three launchers is reliably
// spawnable without a shell:
//
//   - `trail` (no extension) is a Unix shell script; a host that spawns
//     without a shell applies no PATHEXT, hits this file and fails ENOENT.
//   - `trail.cmd` is refused outright by Node-based hosts, which since
//     CVE-2024-27980 reject .cmd/.bat unless shell:true (EINVAL).
//
// Both launchers only exec `node <pkg>/dist/cli.js`, so the entry is written
// as node + that script: node.exe is a real executable every host can spawn
// directly. If the script cannot be located, fall back to the resolved
// launcher, which is still better than the bare name. Elsewhere the bare
// name is left alone so configs stay portable across machines.
func trailLaunchCommand(exe string, command []string) []string {
	if runtime.GOOS != "windows" || len(command) == 0 {
		return command
	}
	rest := append([]string{}, command[1:]...)
	if js, ok := trailNodeEntrypoint(); ok {
		return append([]string{"node", js}, rest...)
	}
	if exe == "" {
		return command
	}
	return append([]string{exe}, rest...)
}

// MigrateGraftMCP rewrites a legacy `graft` MCP entry (Trail's old name) into
// a `trail` entry in every installed opencode / claude-code / pi config. It
// runs on install and update regardless of the preset mcp[] filter, touches
// only configs that still carry `graft`, and is therefore idempotent.
func MigrateGraftMCP() []string {
	var migrated []string
	for name, configPath := range agent.SettingsPaths() {
		switch name {
		case "opencode", "claude-code", "pi":
		default:
			continue
		}
		if configPath == "" {
			continue
		}
		if _, err := os.Stat(configPath); err != nil {
			continue
		}
		ok, err := migrateGraftMCPEntry(configPath, name)
		if err != nil {
			fmt.Printf("  Warning: graft → trail MCP migration failed for %s: %v\n", name, err)
		} else if ok {
			migrated = append(migrated, name)
		}
	}
	return migrated
}

// migrateGraftMCPEntry replaces a `graft` entry with `trail mcp`; reports
// whether anything changed.
func migrateGraftMCPEntry(configPath, agentName string) (bool, error) {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return false, err
	}
	mcpMap, _ := root[mcpConfigKey(agentName)].(map[string]any)
	present := false
	if mcpConfigKey(agentName) == "mcp" {
		_, present = mcp.CollectOpenCodeServers(mcpMap)["graft"]
	} else {
		_, present = mcpMap["graft"]
	}
	if !present {
		return false, nil
	}
	exe, _ := resolveTrailBinary()
	return true, writeTrailMCPEntry(configPath, agentName, trailLaunchCommand(exe, []string{"trail", "mcp"}))
}

// writeTrailMCPEntry writes the trail MCP server in the target's native
// shape and drops any legacy `graft` entry. OpenCode requires
// type+command-array; Claude/pi use command+args. Always overwrites so a
// previous Claude-shaped write cannot leave OpenCode unable to boot.
func writeTrailMCPEntry(configPath, agentName string, command []string) error {
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
		delete(servers, "graft")
		servers["trail"] = shape
		root[key] = mcp.WriteOpenCodeMCP(mcpMap, servers)
	} else {
		delete(mcpMap, "graft")
		mcpMap["trail"] = shape
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
