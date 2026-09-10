package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// SubagentStatuslineServerBundleName is the filename the sub-agent statusline
// server bundle is installed under, inside the auto-discovered plugins dir. It
// mirrors config.SubagentStatuslineServerBundleName: the installer reads the
// config constant, and the uninstall path in cmd/ywai uses this exported name.
// Keep the two values equal.
const SubagentStatuslineServerBundleName = "subagent-statusline-server.js"

// SubagentStatuslineTuiBundleName is the filename the sub-agent statusline TUI
// bundle is installed under, inside tui-plugins/. The bundle is built as .js
// and renamed to .tsx, matching the plain-source plugins the TUI host loads. It
// mirrors config.SubagentStatuslineTuiBundleName for the same uninstall path.
const SubagentStatuslineTuiBundleName = "subagent-statusline-tui.tsx"

// subagentStatuslineServerEntry reports whether a plugin entry references the
// sub-agent statusline server bundle. OpenCode accepts a plain path string and
// a map that names the package under "package" or "path"; both shapes must be
// dropped or v2 warns about the rejected path on every start.
func subagentStatuslineServerEntry(raw any) bool {
	switch v := raw.(type) {
	case string:
		return strings.Contains(v, config.SubagentStatuslineServerBundleName)
	case map[string]any:
		for _, key := range []string{"package", "path"} {
			if s, ok := v[key].(string); ok && strings.Contains(s, config.SubagentStatuslineServerBundleName) {
				return true
			}
		}
	}
	return false
}

// InstallSubagentStatusline vendors the full sub-agent statusline (server +
// TUI halves) into the opencode config at configPath. It is the v2 successor
// of both the published opencode-subagent-statusline (peer range stops at
// OpenCode 2) and the minimal ywai-statusline stand-in, which cannot show
// delegations. On v1 the published package still works and is installed by
// InstallPublishedSubAgentStatusline, so this one installs only on v2.
func InstallSubagentStatusline(configPath string) error {
	if !agent.OpenCodeIsV2() {
		return nil
	}
	serverBundle, err := config.SubagentStatuslineServerBundlePath()
	if err != nil {
		return err
	}
	tuiBundle, err := config.SubagentStatuslineTuiBundlePath()
	if err != nil {
		return err
	}
	return installSubagentStatuslineWithBundles(configPath, serverBundle, tuiBundle)
}

// installSubagentStatuslineWithBundles performs the copy + patch glue for
// installSubagentStatusline. Split out so it is unit testable without
// resolving the real embedded/source bundles. All steps are idempotent and
// preserve unrelated config keys.
func installSubagentStatuslineWithBundles(configPath, serverBundleSrc, tuiBundleSrc string) error {
	dir := filepath.Dir(configPath)

	// Server half: the v2 plugin host rejects an explicit absolute .js path,
	// so the bundle lands in the auto-discovered plugins dir (same
	// arrangement as installBackgroundAgentsV2).
	serverDest := filepath.Join(dir, AutoDiscoveredPluginsSubdir, config.SubagentStatuslineServerBundleName)
	if err := os.MkdirAll(filepath.Dir(serverDest), 0o755); err != nil {
		return fmt.Errorf("create plugins dir %s: %w", filepath.Dir(serverDest), err)
	}
	if err := copyFile(serverBundleSrc, serverDest); err != nil {
		return fmt.Errorf("copy sub-agent statusline server bundle: %w", err)
	}

	// Drop any stale explicit entry that references the bundle, whether it is a
	// plain path string or a map-form entry; otherwise v2 warns about the
	// rejected path on every start. Mirrors installBackgroundAgentsV2.
	root, err := config.ReadJSONC(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", configPath, err)
	}
	if err == nil {
		kept := make([]any, 0)
		for _, raw := range openCodePlugins(root) {
			if subagentStatuslineServerEntry(raw) {
				continue
			}
			kept = append(kept, raw)
		}
		writePlugins(root, kept)
		if err := config.WriteJSONC(configPath, root); err != nil {
			return fmt.Errorf("write %s: %w", configPath, err)
		}
	}

	// A stale v1-shaped copy must not keep loading the bundle twice.
	if err := os.Remove(filepath.Join(dir, ywaiPluginsSubdir, config.SubagentStatuslineServerBundleName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// TUI half: the host registers TUI plugins by absolute path in cli.json;
	// mouse capture must be on so row clicks work.
	tuiDest := filepath.Join(dir, tuiPluginsSubdir, config.SubagentStatuslineTuiBundleName)
	if err := os.MkdirAll(filepath.Dir(tuiDest), 0o755); err != nil {
		return fmt.Errorf("create tui-plugins dir %s: %w", filepath.Dir(tuiDest), err)
	}
	if err := copyFile(tuiBundleSrc, tuiDest); err != nil {
		return fmt.Errorf("copy sub-agent statusline TUI bundle: %w", err)
	}

	tuiConfig := filepath.Join(dir, tuiConfigName)
	if err := patchTuiPlugin(tuiConfig, tuiDest, true); err != nil {
		return err
	}

	// The TUI bundle imports solid-js/@opentui/solid, which Bun resolves from
	// the config dir upward; make them resolvable (best-effort). The same
	// dependency gap pre-exists for ywai-logo.tsx and the minimal statusline.
	ensureTuiPeerDependencies(dir)

	// The full monitor supersedes the minimal ywai-statusline: two footer
	// statuslines would fight for the same slot.
	return supersedeTuiStatusline(tuiConfig)
}

// tuiPeerDependencies are the packages the built sub-agent statusline TUI
// bundle imports without bundling (solid-js, @opentui/solid). Bun resolves
// them from the plugin file's directory upward, so a config dir without
// node_modules fails to load the plugin with "Cannot find package 'solid-js'".
// Versions are pinned with ^ to match what a working opencode install
// resolves (checked against a real install: solid-js 1.9.13,
// @opentui/solid 0.4.2).
var tuiPeerDependencies = map[string]string{
	"solid-js":       "^1.9.13",
	"@opentui/solid": "^0.4.2",
}

// runTuiPeerInstall installs the TUI peer dependencies in configDir. It is a
// variable so tests can inject success/failure without spawning a real package
// manager. bun install is tried first, npm install as a fallback; each is
// attempted exactly once and its output is discarded.
var runTuiPeerInstall = func(configDir string) error {
	if runPackageManagerQuiet(configDir, "bun") == nil {
		return nil
	}
	return runPackageManagerQuiet(configDir, "npm")
}

func runPackageManagerQuiet(configDir, name string) error {
	cmd := exec.Command(name, "install")
	cmd.Dir = configDir
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// nodeModulesHasSolidJS reports whether the config dir already carries a
// resolvable solid-js package (the probe that makes this whole step skip).
func nodeModulesHasSolidJS(solidJSPath string) bool {
	info, err := os.Stat(solidJSPath)
	return err == nil && info.IsDir()
}

// mergeTuiPeerDepsIntoPackageJSON creates or merges <configdir>/package.json
// so its dependencies include the TUI peer packages. Existing keys are never
// clobbered: a dependency that is already present (whatever its version) wins.
func mergeTuiPeerDepsIntoPackageJSON(packageJSONPath string) error {
	root := map[string]any{}
	if raw, err := os.ReadFile(packageJSONPath); err == nil {
		if err := json.Unmarshal(raw, &root); err != nil {
			return fmt.Errorf("parse %s: %w", packageJSONPath, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", packageJSONPath, err)
	}

	deps, _ := root["dependencies"].(map[string]any)
	if deps == nil {
		deps = map[string]any{}
	}
	changed := false
	for name, version := range tuiPeerDependencies {
		if existing, ok := deps[name].(string); ok && existing != "" {
			continue
		}
		deps[name] = version
		changed = true
	}
	if !changed {
		return nil
	}
	root["dependencies"] = deps

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", packageJSONPath, err)
	}
	out = append(out, '\n')
	return os.WriteFile(packageJSONPath, out, 0o644)
}

// ensureTuiPeerDependencies makes the TUI bundle's peer imports resolvable
// from the config dir. It never fails the install: a missing package manager,
// an unreadable package.json, or a failed install only prints a warning that
// names the exact command the user can run.
func ensureTuiPeerDependencies(configDir string) {
	if nodeModulesHasSolidJS(filepath.Join(configDir, "node_modules", "solid-js")) {
		return
	}
	packageJSONPath := filepath.Join(configDir, "package.json")
	if err := mergeTuiPeerDepsIntoPackageJSON(packageJSONPath); err != nil {
		fmt.Printf("  Warning: %v\n", err)
		return
	}
	if err := runTuiPeerInstall(configDir); err != nil {
		fmt.Printf("  Warning: could not install the sub-agent statusline TUI peers in %s; run: cd %s && bun add solid-js @opentui/solid\n", configDir, configDir)
	}
}

// supersedeTuiStatusline removes the minimal ywai-statusline (entry in the TUI
// config plus the bundled file) now that the full sub-agent statusline is
// installed. Idempotent: a missing entry or a missing file is a no-op, and no
// unrelated key or plugin entry is touched.
func supersedeTuiStatusline(tuiConfigPath string) error {
	root, err := config.ReadJSONC(tuiConfigPath)
	if errors.Is(err, os.ErrNotExist) {
		root = nil
	} else if err != nil {
		return fmt.Errorf("read %s: %w", tuiConfigPath, err)
	}
	if root != nil {
		plugins := openCodePlugins(root)
		kept := plugins[:0]
		for _, raw := range plugins {
			if s, ok := raw.(string); ok && filepath.Base(s) == config.TuiStatuslineBundleName {
				continue
			}
			kept = append(kept, raw)
		}
		if len(kept) != len(plugins) {
			writePlugins(root, kept)
			if err := config.WriteJSONC(tuiConfigPath, root); err != nil {
				return fmt.Errorf("write %s: %w", tuiConfigPath, err)
			}
		}
	}

	stale := filepath.Join(filepath.Dir(tuiConfigPath), tuiPluginsSubdir, config.TuiStatuslineBundleName)
	if err := os.Remove(stale); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove superseded statusline %s: %w", stale, err)
	}
	return nil
}
