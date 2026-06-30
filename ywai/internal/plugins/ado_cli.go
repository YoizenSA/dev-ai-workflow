package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// AdoNPMPackage is the npm package that ships the `ado` CLI. ywai installs only
// the CLI globally; the agent then drives Azure DevOps through the `ado` skill
// (Bash) instead of loading 22 plugin tools into its context on every request.
const AdoNPMPackage = "@cioffinahuel/opencode-ado"

// adoPluginPackageNames are the npm spec prefixes that may appear in agent
// configs from past installs. The package was renamed (@nahuelcio →
// @cioffinahuel), so cleanup must match both the legacy and current names to
// fully unregister the in-process plugin.
//
// ywai no longer installs the Azure DevOps plugin: it ships the `ado` skill
// instead, which drives Azure DevOps through the `ado` CLI (Bash) so its tools
// are not loaded into the agent's context on every request. These helpers exist
// only to remove leftover plugin entries from older installs.
var adoPluginPackageNames = []string{
	"@nahuelcio/opencode-ado",
	"@cioffinahuel/opencode-ado",
}

// InstallAdoCLI installs the `ado` CLI globally via npm. It is non-fatal: a
// missing npm or a failed install returns an error the caller should surface as
// a warning, pointing the user at the manual install step in the `ado` skill.
//
// --legacy-peer-deps is required: the package's transitive dependency
// @opencode-ai/plugin pulls in @opentui/core with a peerOptional range that
// conflicts under npm's strict resolver (npm 7+), blocking a plain install.
// This flag is the standard workaround for global CLI installs.
func InstallAdoCLI() error {
	if _, err := exec.LookPath("npm"); err != nil {
		return fmt.Errorf("npm not found in PATH — install Node.js, then run `npm i -g %s --legacy-peer-deps`", AdoNPMPackage)
	}

	cmd := exec.Command("npm", "i", "-g", AdoNPMPackage, "--legacy-peer-deps")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install %s failed: %w", AdoNPMPackage, err)
	}
	return nil
}

// AdoCLIInfo reports whether the `ado` CLI is installed and, if so, its version
// read from the npm package's package.json. It does NOT call `ado --version`:
// the current package does not implement that flag (it exits 1 and prints help).
// Instead `installed` is decided by exec.LookPath, and the version is read from
// the package.json next to the resolved binary (the `ado` on PATH is a symlink
// into the npm global node_modules). version is "" when the binary is on PATH
// but its package.json can't be located (e.g. a non-npm install).
func AdoCLIInfo() (version string, installed bool) {
	exe, err := exec.LookPath("ado")
	if err != nil {
		return "", false
	}
	v, _ := adoVersionFromBinary(exe)
	return v, true
}

// adoVersionFromBinary resolves `exe` to its real path (following symlinks) and
// walks up the directory tree until it finds a package.json whose "version"
// field it returns. Extracted for testability (see ado_cli_test.go).
func adoVersionFromBinary(exe string) (string, error) {
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		real = exe // best effort: fall back to the given path
	}
	dir := filepath.Dir(real)
	for i := 0; i < 16; i++ { // bounded walk; a package.json must be close
		pkg := filepath.Join(dir, "package.json")
		if v, err := readVersionFromPackageJSON(pkg); err == nil {
			return v, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir { // reached filesystem root
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("no package.json found above %s", real)
}

// readVersionFromPackageJSON reads the "version" field from a package.json.
// Returns an error if the file is missing, unparseable, or has no version.
func readVersionFromPackageJSON(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", err
	}
	if strings.TrimSpace(pkg.Version) == "" {
		return "", fmt.Errorf("package.json has no version field")
	}
	return strings.TrimSpace(pkg.Version), nil
}

// adoLatestFn is the function used to fetch the latest published version from
// the npm registry. Indirected through a package var so tests can stub it
// without hitting the network (mirrors internal/versionfile.latestFn).
var adoLatestFn = func() (string, error) { return adoLatestFromRegistry() }

// AdoCLILatestVersion returns the latest version of the `ado` CLI published to
// npm. It runs `npm view <pkg> version` with a bounded timeout. Returns an
// error (non-fatal for callers) when npm is missing or the registry is
// unreachable (offline).
func AdoCLILatestVersion() (string, error) {
	return adoLatestFn()
}

// adoLatestFromRegistry shells out to `npm view @cioffinahuel/opencode-ado
// version` and trims the output. The trailing newline from npm is stripped.
func adoLatestFromRegistry() (string, error) {
	if _, err := exec.LookPath("npm"); err != nil {
		return "", fmt.Errorf("npm not found in PATH: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "npm", "view", AdoNPMPackage, "version").Output()
	if err != nil {
		return "", fmt.Errorf("npm view %s version: %w", AdoNPMPackage, err)
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "", fmt.Errorf("npm view returned empty version")
	}
	return v, nil
}

// RemoveAdoPluginFromConfig unregisters the in-process Azure DevOps plugin from
// an agent's config file so its tools are no longer loaded into context. It is
// idempotent and safe to call on configs that never had the plugin.
//
//   - opencode-format configs (opencode/kilocode): drops any entry from the
//     "plugin" array that references the ADO package (as a bare string spec or a
//     ["<pkg>", {...}] pair).
//   - claude-code / pi configs: drops any matching spec from the "packages"
//     array.
func RemoveAdoPluginFromConfig(configPath, agentName string) error {
	if configPath == "" {
		return nil
	}
	if _, err := os.Stat(configPath); err != nil {
		return nil // nothing to clean
	}

	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", configPath, err)
	}

	key := mcpConfigKey(agentName) // "mcpServers" for claude-code/pi, "mcp" otherwise
	var changed bool
	if key == "mcpServers" {
		changed = removeFromPackagesArray(root, adoPluginPackageNames)
	} else {
		changed = removeFromPluginArray(root, adoPluginPackageNames)
	}

	if !changed {
		return nil
	}
	if err := config.WriteJSONC(configPath, root); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}
	return nil
}

// RemoveAdoPluginConfigFile deletes the standalone ADO plugin config that
// older ywai installs wrote at ~/.config/opencode/ado-plugin.json. Missing is
// a no-op.
func RemoveAdoPluginConfigFile() error {
	path := adoPluginConfigFilePath()
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// adoPluginConfigFilePath returns ~/.config/opencode/ado-plugin.json.
func adoPluginConfigFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "opencode", "ado-plugin.json")
}

// removeFromPluginArray strips any ADO entry from root["plugin"], which is an
// opencode array of either bare plugin specs or ["<spec>", {opts}] pairs.
// Reports whether the root was modified.
func removeFromPluginArray(root map[string]any, packages []string) bool {
	raw, ok := root["plugin"].([]any)
	if !ok || len(raw) == 0 {
		return false
	}

	filtered := make([]any, 0, len(raw))
	changed := false
	for _, entry := range raw {
		if isAdoPluginEntry(entry, packages) {
			changed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if changed {
		if len(filtered) == 0 {
			delete(root, "plugin")
		} else {
			root["plugin"] = filtered
		}
	}
	return changed
}

// isAdoPluginEntry reports whether a "plugin" array element references one of
// the ADO package names, as either a bare spec string or a ["<spec>", opts] pair.
func isAdoPluginEntry(entry any, packages []string) bool {
	if spec, ok := entry.(string); ok {
		return matchesAdoPackage(spec, packages)
	}
	pair, ok := entry.([]any)
	if !ok || len(pair) == 0 {
		return false
	}
	spec, ok := pair[0].(string)
	if !ok {
		return false
	}
	return matchesAdoPackage(spec, packages)
}

// removeFromPackagesArray strips any ADO spec from root["packages"], the array
// Pi uses to list npm plugins. Reports whether the root was modified.
func removeFromPackagesArray(root map[string]any, packages []string) bool {
	raw, ok := root["packages"].([]any)
	if !ok || len(raw) == 0 {
		return false
	}

	filtered := make([]any, 0, len(raw))
	changed := false
	for _, entry := range raw {
		spec, ok := entry.(string)
		if ok && matchesAdoPackage(spec, packages) {
			changed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if changed {
		if len(filtered) == 0 {
			delete(root, "packages")
		} else {
			root["packages"] = filtered
		}
	}
	return changed
}

// matchesAdoPackage reports whether a plugin spec string (e.g.
// "npm:@nahuelcio/opencode-ado" or "@cioffinahuel/opencode-ado") refers to one
// of the known ADO package names, accounting for the "npm:" prefix and optional
// @version / @tag suffix.
func matchesAdoPackage(spec string, packages []string) bool {
	s := strings.TrimPrefix(spec, "npm:")
	// Drop an optional @version suffix (e.g. "@cioffinahuel/opencode-ado@1.2.3").
	// The package scope itself contains '@', so only trim after the final '/'.
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		base, ver := s[:idx+1], s[idx+1:]
		if at := strings.Index(ver, "@"); at >= 0 {
			s = base + ver[:at]
		}
	}
	for _, pkg := range packages {
		if s == pkg {
			return true
		}
	}
	return false
}
