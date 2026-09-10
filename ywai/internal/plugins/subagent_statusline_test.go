package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// seedMonitorBundles writes throwaway server + TUI sources and returns them
// alongside an opencode.json config path.
func seedMonitorBundles(t *testing.T, cfg map[string]any) (configPath, serverSrc, tuiSrc string) {
	t.Helper()
	configPath = writeAgentConfig(t, "opencode.json", cfg)
	serverSrc = filepath.Join(t.TempDir(), config.SubagentStatuslineServerBundleName)
	if err := os.WriteFile(serverSrc, []byte("server bundle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tuiSrc = filepath.Join(t.TempDir(), config.SubagentStatuslineTuiSrcBundleName)
	if err := os.WriteFile(tuiSrc, []byte("tui bundle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return configPath, serverSrc, tuiSrc
}

// On v2 the full monitor vendors both halves, registers the TUI bundle in the
// client config, strips stale explicit server entries, and supersedes the
// minimal ywai-statusline (entry + file).
func TestInstallSubagentStatusline_V2InstallsBothHalves(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
	stubTuiPeerInstall(t)
	dir := t.TempDir()
	legacyCopy := filepath.Join(dir, ywaiPluginsSubdir, config.SubagentStatuslineServerBundleName)
	if err := os.MkdirAll(filepath.Dir(legacyCopy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyCopy, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	minimalDest := filepath.Join(dir, tuiPluginsSubdir, config.TuiStatuslineBundleName)
	if err := os.MkdirAll(filepath.Dir(minimalDest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(minimalDest, []byte("minimal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "opencode.json")
	writeJSON(t, configPath, map[string]any{
		"plugin": []any{
			"/abs/path/subagent-statusline-server.js",
			"other.js",
		},
		"theme": "dark",
		"mcp":   map[string]any{"keep-alive": map[string]any{}},
	})
	tuiPath := filepath.Join(dir, tuiConfigName)
	writeJSON(t, tuiPath, map[string]any{
		"plugins": []any{minimalDest, "other-tui.js"},
		"theme":   "night",
	})
	serverSrc := filepath.Join(t.TempDir(), config.SubagentStatuslineServerBundleName)
	if err := os.WriteFile(serverSrc, []byte("server bundle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tuiSrc := filepath.Join(t.TempDir(), config.SubagentStatuslineTuiSrcBundleName)
	if err := os.WriteFile(tuiSrc, []byte("tui bundle\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installSubagentStatuslineWithBundles(configPath, serverSrc, tuiSrc); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Server half lands in the auto-discovered dir.
	got, err := os.ReadFile(filepath.Join(dir, autoDiscoveredPluginsSubdir, config.SubagentStatuslineServerBundleName))
	if err != nil || string(got) != "server bundle\n" {
		t.Errorf("server bundle = %q, err = %v; want vendored copy", string(got), err)
	}
	// Stale v1-shaped copy is gone so the bundle cannot load twice.
	if _, err := os.Stat(legacyCopy); !os.IsNotExist(err) {
		t.Errorf("stale v1 copy still present (err = %v)", err)
	}
	// Stale explicit entry is gone; unrelated entries survive under the v2 key.
	root := readConfigRoot(t, configPath)
	entries, ok := root["plugins"].([]any)
	if !ok {
		t.Fatalf("v2 must keep the plugins key, got %v", root)
	}
	if containsString(entries, "/abs/path/subagent-statusline-server.js") {
		t.Errorf("stale explicit server entry left behind: %v", entries)
	}
	if !containsString(entries, "other.js") {
		t.Errorf("unrelated entry dropped: %v", entries)
	}
	if root["theme"] != "dark" {
		t.Errorf("opencode theme dropped: %v", root)
	}
	if _, ok := root["mcp"]; !ok {
		t.Errorf("opencode mcp dropped: %v", root)
	}

	// TUI half lands in tui-plugins/ and is registered; mouse for row clicks.
	tuiDest := filepath.Join(dir, tuiPluginsSubdir, config.SubagentStatuslineTuiBundleName)
	if got, err := os.ReadFile(tuiDest); err != nil || string(got) != "tui bundle\n" {
		t.Errorf("tui bundle = %q, err = %v; want vendored copy", string(got), err)
	}
	tuiRoot := readConfigRoot(t, tuiPath)
	tuiEntries, ok := tuiRoot["plugins"].([]any)
	if !ok {
		t.Fatalf("v2 tui config must register under the plugins key, got %v", tuiRoot)
	}
	if !containsString(tuiEntries, tuiDest) {
		t.Errorf("tui plugins = %v, want entry %s", tuiEntries, tuiDest)
	}
	if !containsString(tuiEntries, "other-tui.js") {
		t.Errorf("unrelated tui entry dropped: %v", tuiEntries)
	}
	if tuiRoot["mouse"] != true {
		t.Errorf("tui mouse = %v, want true (needed for row clicks)", tuiRoot["mouse"])
	}
	if tuiRoot["theme"] != "night" {
		t.Errorf("tui theme dropped: %v", tuiRoot)
	}

	// The minimal stand-in is superseded: entry and file both gone.
	for _, e := range tuiEntries {
		if s, ok := e.(string); ok && filepath.Base(s) == config.TuiStatuslineBundleName {
			t.Errorf("superseded minimal statusline still registered: %v", tuiEntries)
		}
	}
	if _, err := os.Stat(minimalDest); !os.IsNotExist(err) {
		t.Errorf("superseded minimal statusline file still present (err = %v)", err)
	}
}

// A stale explicit server entry can be a map that names the bundle under
// "package" or "path"; both shapes must be stripped like the string form,
// while unrelated map entries keep their keys.
func TestInstallSubagentStatusline_StripsMapFormServerEntry(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
	stubTuiPeerInstall(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	writeJSON(t, configPath, map[string]any{
		"plugin": []any{
			map[string]any{"package": "subagent-statusline-server.js", "enabled": true},
			map[string]any{"path": "/abs/subagent-statusline-server.js"},
			map[string]any{"package": "unrelated-package", "enabled": true},
			"other.js",
		},
	})
	serverSrc := filepath.Join(t.TempDir(), config.SubagentStatuslineServerBundleName)
	if err := os.WriteFile(serverSrc, []byte("server bundle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tuiSrc := filepath.Join(t.TempDir(), config.SubagentStatuslineTuiSrcBundleName)
	if err := os.WriteFile(tuiSrc, []byte("tui bundle\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := installSubagentStatuslineWithBundles(configPath, serverSrc, tuiSrc); err != nil {
		t.Fatalf("install: %v", err)
	}

	root := readConfigRoot(t, configPath)
	entries, ok := root["plugins"].([]any)
	if !ok {
		t.Fatalf("v2 must write the plugins key, got %v", root)
	}
	if len(entries) != 2 {
		t.Fatalf("plugins = %v, want the two unrelated entries", entries)
	}
	var keptMap bool
	for _, e := range entries {
		m, ok := e.(map[string]any)
		if ok && m["package"] == "unrelated-package" && m["enabled"] == true {
			keptMap = true
		}
	}
	if !keptMap {
		t.Errorf("unrelated map entry dropped or altered: %v", entries)
	}
	if !containsString(entries, "other.js") {
		t.Errorf("unrelated string entry dropped: %v", entries)
	}
}

// On v1 the published package still works, so the vendored port stays out.
func TestInstallSubagentStatusline_V1IsNoOp(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v1")
	configPath, serverSrc, tuiSrc := seedMonitorBundles(t, map[string]any{})
	dir := filepath.Dir(configPath)

	if err := InstallSubagentStatusline(configPath); err != nil {
		t.Fatalf("install: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, autoDiscoveredPluginsSubdir, config.SubagentStatuslineServerBundleName)); !os.IsNotExist(err) {
		t.Errorf("server bundle vendored on v1 (err = %v)", err)
	}
	if _, err := os.Stat(filepath.Join(dir, tuiPluginsSubdir, config.SubagentStatuslineTuiBundleName)); !os.IsNotExist(err) {
		t.Errorf("tui bundle vendored on v1 (err = %v)", err)
	}
	_ = serverSrc
	_ = tuiSrc
}

// Re-running an install must not duplicate entries, must not fail, and must
// leave every file byte-identical.
func TestInstallSubagentStatusline_Idempotent(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
	stubTuiPeerInstall(t)
	configPath, serverSrc, tuiSrc := seedMonitorBundles(t, map[string]any{})
	dir := filepath.Dir(configPath)
	serverDest := filepath.Join(dir, autoDiscoveredPluginsSubdir, config.SubagentStatuslineServerBundleName)
	tuiDest := filepath.Join(dir, tuiPluginsSubdir, config.SubagentStatuslineTuiBundleName)
	tuiPath := filepath.Join(dir, tuiConfigName)

	if err := installSubagentStatuslineWithBundles(configPath, serverSrc, tuiSrc); err != nil {
		t.Fatalf("install 1: %v", err)
	}
	first := map[string]string{
		configPath: readBytes(t, configPath),
		tuiPath:    readBytes(t, tuiPath),
		serverDest: readBytes(t, serverDest),
		tuiDest:    readBytes(t, tuiDest),
	}

	if err := installSubagentStatuslineWithBundles(configPath, serverSrc, tuiSrc); err != nil {
		t.Fatalf("install 2: %v", err)
	}
	for path, want := range first {
		if got := readBytes(t, path); got != want {
			t.Errorf("%s changed between installs:\n want %q\n got  %q", path, want, got)
		}
	}

	root := readConfigRoot(t, configPath)
	entries, _ := root["plugins"].([]any)
	if len(entries) != 0 {
		t.Errorf("opencode plugins = %v, want no explicit server entries", entries)
	}
	tuiRoot := readConfigRoot(t, tuiPath)
	tuiEntries, _ := tuiRoot["plugins"].([]any)
	count := 0
	for _, e := range tuiEntries {
		if s, ok := e.(string); ok && filepath.Base(s) == config.SubagentStatuslineTuiBundleName {
			count++
		}
	}
	if count != 1 {
		t.Errorf("tui bundle registered %d times (%v), want exactly 1", count, tuiEntries)
	}
}

func readBytes(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// Supersede is safe when there is nothing to supersede and preserves others.
func TestSupersedeTuiStatusline_PreservesOthers(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
	dir := t.TempDir()
	tuiPath := filepath.Join(dir, tuiConfigName)
	writeJSON(t, tuiPath, map[string]any{
		"plugins": []any{"keep.js"},
		"mouse":   true,
	})

	if err := supersedeTuiStatusline(tuiPath); err != nil {
		t.Fatalf("supersede: %v", err)
	}

	root := readConfigRoot(t, tuiPath)
	entries, _ := root["plugins"].([]any)
	if len(entries) != 1 || entries[0] != "keep.js" {
		t.Errorf("tui plugins = %v, want [keep.js]", entries)
	}
	if root["mouse"] != true {
		t.Errorf("mouse = %v, want preserved true", root["mouse"])
	}
}

// Supersede on a machine without a TUI config is a no-op, not an error.
func TestSupersedeTuiStatusline_MissingConfig(t *testing.T) {
	if err := supersedeTuiStatusline(filepath.Join(t.TempDir(), tuiConfigName)); err != nil {
		t.Fatalf("supersede missing config: %v", err)
	}
}

// ─── TUI peer dependencies (finding F3) ─────────────────────────────────

// tuiPeerInstallRecorder replaces runTuiPeerInstall with a stub that records
// the config dirs it was asked to install into and returns rec.err (nil by
// default). No test ever spawns bun/npm or touches the network.
type tuiPeerInstallRecorder struct {
	dirs []string
	err  error
}

func stubTuiPeerInstall(t *testing.T) *tuiPeerInstallRecorder {
	t.Helper()
	original := runTuiPeerInstall
	rec := &tuiPeerInstallRecorder{}
	runTuiPeerInstall = func(configDir string) error {
		rec.dirs = append(rec.dirs, configDir)
		return rec.err
	}
	t.Cleanup(func() { runTuiPeerInstall = original })
	return rec
}

func readPackageJSONDeps(t *testing.T, dir string) map[string]any {
	t.Helper()
	var root map[string]any
	readJSON(t, filepath.Join(dir, "package.json"), &root)
	deps, _ := root["dependencies"].(map[string]any)
	return deps
}

// A clean config dir (no node_modules, no package.json) gets a package.json
// with both TUI peers pinned, and the package manager runs once on that dir.
func TestEnsureTuiPeerDependencies_CreatesPackageJsonWhenMissing(t *testing.T) {
	rec := stubTuiPeerInstall(t)
	dir := t.TempDir()

	ensureTuiPeerDependencies(dir)

	deps := readPackageJSONDeps(t, dir)
	if deps["solid-js"] != "^1.9.13" {
		t.Errorf("solid-js = %v, want ^1.9.13", deps["solid-js"])
	}
	if deps["@opentui/solid"] != "^0.4.2" {
		t.Errorf("@opentui/solid = %v, want ^0.4.2", deps["@opentui/solid"])
	}
	if len(rec.dirs) != 1 || rec.dirs[0] != dir {
		t.Errorf("package manager dirs = %v, want [%s]", rec.dirs, dir)
	}
}

// An existing package.json keeps every key: unrelated deps survive and an
// already-present peer keeps its own version (merge, never clobber).
func TestEnsureTuiPeerDependencies_MergesExistingPackageJson(t *testing.T) {
	rec := stubTuiPeerInstall(t)
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, "package.json"), map[string]any{
		"name": "user-config",
		"dependencies": map[string]any{
			"@dietrichgebert/ponytail": "^4.9.0",
			"solid-js":                 "^1.8.0",
		},
	})

	ensureTuiPeerDependencies(dir)

	var root map[string]any
	readJSON(t, filepath.Join(dir, "package.json"), &root)
	if root["name"] != "user-config" {
		t.Errorf("name clobbered: %v", root["name"])
	}
	deps := readPackageJSONDeps(t, dir)
	if deps["@dietrichgebert/ponytail"] != "^4.9.0" {
		t.Errorf("unrelated dep dropped: %v", deps)
	}
	if deps["solid-js"] != "^1.8.0" {
		t.Errorf("existing solid-js must win, got %v", deps["solid-js"])
	}
	if deps["@opentui/solid"] != "^0.4.2" {
		t.Errorf("@opentui/solid = %v, want ^0.4.2", deps["@opentui/solid"])
	}
	if len(rec.dirs) != 1 {
		t.Errorf("package manager dirs = %v, want one install", rec.dirs)
	}
}

// When <configdir>/node_modules/solid-js already resolves, nothing is written
// and no package manager runs: npm-plugin machines are left untouched.
func TestEnsureTuiPeerDependencies_NodeModulesPresentSkips(t *testing.T) {
	rec := stubTuiPeerInstall(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "node_modules", "solid-js"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(dir, "package.json"), map[string]any{
		"name":         "user-config",
		"dependencies": map[string]any{"@dietrichgebert/ponytail": "^4.9.0"},
	})
	before := readBytes(t, filepath.Join(dir, "package.json"))

	ensureTuiPeerDependencies(dir)

	if got := readBytes(t, filepath.Join(dir, "package.json")); got != before {
		t.Errorf("package.json changed although solid-js resolves:\n want %q\n got  %q", before, got)
	}
	if len(rec.dirs) != 0 {
		t.Errorf("package manager ran although solid-js resolves: %v", rec.dirs)
	}
}

// A failed install is a warning, never an install error: the statusline still
// installs and the package.json stays correct for a later manual bun add.
func TestEnsureTuiPeerDependencies_PmFailureWarnsAndNeverFailsInstall(t *testing.T) {
	rec := stubTuiPeerInstall(t)
	rec.err = os.ErrPermission
	dir := t.TempDir()

	ensureTuiPeerDependencies(dir)

	deps := readPackageJSONDeps(t, dir)
	if deps["solid-js"] != "^1.9.13" {
		t.Errorf("solid-js = %v, want ^1.9.13", deps["solid-js"])
	}
	if len(rec.dirs) != 1 {
		t.Errorf("package manager dirs = %v, want one attempt", rec.dirs)
	}
}
