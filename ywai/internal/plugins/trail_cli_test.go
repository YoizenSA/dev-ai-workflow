package plugins

// trail_cli_test.go — the Trail CLI installer, MCP wiring and graft → trail
// migration. npm and the binary are not exercised.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// Compile-time guards. Missing names → test binary fails to build,
// which Go reports as a test failure for every test in the package.
var (
	_ func() error = InstallTrailCLI
	_ func() error = WireTrailMCP
)

// TestPlugins_TrailSurfacePresent is the named runtime test that
// gives the suite a PASS once the compile-time guards succeed. It
// exercises the pure, testable seam of the slice (version parsing)
// without touching the npm / `trail mcp` binary seams.
func TestWriteTrailMCPEntry_OpenCodeShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(path, []byte(`{"mcp":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeTrailMCPEntry(path, "opencode", []string{"trail", "mcp"}); err != nil {
		t.Fatalf("writeTrailMCPEntry: %v", err)
	}

	assertOpenCodeTrailShape(t, path)
}

func TestWriteTrailMCPEntry_FlattensServersAndKeepsSiblings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	legacy := `{"mcp":{"timeout":15000,"context7":{"type":"remote","url":"https://x"},"trail":{"command":"trail","args":["mcp"]}}}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeTrailMCPEntry(path, "opencode", []string{"trail", "mcp"}); err != nil {
		t.Fatalf("writeTrailMCPEntry: %v", err)
	}

	assertOpenCodeTrailShape(t, path)
	root, err := config.ReadJSONC(path)
	if err != nil {
		t.Fatal(err)
	}
	mcpMap := root["mcp"].(map[string]any)
	if mcpMap["timeout"] != float64(15000) && mcpMap["timeout"] != 15000 {
		t.Fatalf("timeout not preserved: %#v", mcpMap["timeout"])
	}
	servers := mcpMap["servers"].(map[string]any)
	if _, ok := servers["context7"].(map[string]any); !ok {
		t.Fatalf("context7 missing from servers: %#v", servers)
	}
}

func TestWriteTrailMCPEntry_RepairsLegacyClaudeShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	legacy := `{"mcp":{"graft":{"command":"graft","args":["mcp"]}}}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeTrailMCPEntry(path, "opencode", []string{"trail", "mcp"}); err != nil {
		t.Fatalf("writeTrailMCPEntry: %v", err)
	}

	assertOpenCodeTrailShape(t, path)
}

func assertOpenCodeTrailShape(t *testing.T, path string) {
	t.Helper()
	root, err := config.ReadJSONC(path)
	if err != nil {
		t.Fatal(err)
	}
	mcpMap, _ := root["mcp"].(map[string]any)
	if mcpMap == nil {
		t.Fatal("missing mcp")
	}
	servers, ok := mcpMap["servers"].(map[string]any)
	if !ok {
		t.Fatalf("mcp.servers missing: %v", mcpMap)
	}
	got, _ := servers["trail"].(map[string]any)
	if got["type"] != "local" {
		t.Errorf("type = %#v, want local", got["type"])
	}
	if _, has := got["enabled"]; has {
		t.Errorf("enabled = %#v, want absent (v2 uses disabled)", got["enabled"])
	}
	cmd, ok := got["command"].([]any)
	if !ok {
		t.Fatalf("command = %#v, want [trail mcp] array", got["command"])
	}
	if len(cmd) != 2 || cmd[0] != "trail" || cmd[1] != "mcp" {
		t.Errorf("command = %#v, want [trail mcp]", cmd)
	}
	if _, hasArgs := got["args"]; hasArgs {
		t.Errorf("args must not be set on opencode trail entry: %#v", got)
	}
}

// On Windows the bare name `trail` resolves to the Unix shell script npm
// also drops in the prefix, so a host that spawns without a shell gets
// ENOENT. The wired entry must carry the resolved launcher instead.
func TestTrailLaunchCommand_SubstitutesResolvedBinaryOnWindows(t *testing.T) {
	exe := filepath.Join("C:\\", "npm", "trail.cmd")
	got := trailLaunchCommand(exe, []string{"trail", "mcp"})

	if runtime.GOOS != "windows" {
		if len(got) != 2 || got[0] != "trail" {
			t.Fatalf("non-windows must keep the bare name for portability: %#v", got)
		}
		return
	}
	// Either shape is correct, depending on whether the npm-installed
	// entrypoint could be located: [node <cli.js> mcp] is preferred, the
	// resolved launcher is the fallback. What must never survive is the
	// bare name, and the subcommand must be carried through.
	if got[0] == "trail" {
		t.Fatalf("bare name survived on windows — hosts cannot spawn it: %#v", got)
	}
	if got[len(got)-1] != "mcp" {
		t.Fatalf("subcommand lost: %#v", got)
	}
	if got[0] != "node" && got[0] != exe {
		t.Fatalf("argv[0] = %q, want node or %q", got[0], exe)
	}
}

// The catalog slice is shared state; rewriting argv in place would corrupt
// it for every later consumer.
func TestTrailLaunchCommand_DoesNotMutateInput(t *testing.T) {
	in := []string{"trail", "mcp"}
	trailLaunchCommand(filepath.Join("C:\\", "npm", "trail.cmd"), in)
	if in[0] != "trail" {
		t.Fatalf("input mutated: %#v", in)
	}
}

func TestTrailLaunchCommand_EmptyArgvIsLeftAlone(t *testing.T) {
	if got := trailLaunchCommand("x", nil); got != nil {
		t.Errorf("nil argv = %#v, want nil", got)
	}
}

// With no launcher resolved AND no npm entrypoint on disk there is nothing
// better to write, so the original argv must survive untouched rather than
// becoming a half-built command.
func TestTrailLaunchCommand_FallsBackWhenNothingResolves(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // hide npm, so trailNodeEntrypoint misses
	got := trailLaunchCommand("", []string{"trail", "mcp"})
	if len(got) != 2 || got[0] != "trail" || got[1] != "mcp" {
		t.Errorf("trailLaunchCommand = %#v, want [trail mcp]", got)
	}
}

func TestPlugins_TrailSurfacePresent(t *testing.T) {
	t.Log("InstallTrailCLI and WireTrailMCP are exported by internal/plugins")

	dir := t.TempDir()
	// trailVersionFromBinary execs the binary, so on Windows it must have an
	// extension LookPath/cmd can run — a shebang script is not launchable there.
	name := "trail"
	body := "#!/bin/sh\necho trail 0.1.0\n"
	if runtime.GOOS == "windows" {
		name += ".bat"
		body = "@echo trail 0.1.0\r\n"
	}
	fake := filepath.Join(dir, name)
	if err := os.WriteFile(fake, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := trailVersionFromBinary(fake)
	if err != nil {
		t.Fatalf("trailVersionFromBinary: %v", err)
	}
	if want := "trail 0.1.0"; got != want {
		t.Fatalf("trailVersionFromBinary = %q, want %q", got, want)
	}
}

func TestMigrateGraftMCPEntry_OpenCodeRewritesToTrail(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	path := filepath.Join(t.TempDir(), "opencode.json")
	legacy := `{"mcp":{"servers":{"graft":{"type":"local","command":["graft","mcp"]},"context7":{"type":"remote","url":"https://x"}}}}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := migrateGraftMCPEntry(path, "opencode")
	if err != nil || !changed {
		t.Fatalf("migrate = %v, %v; want true, nil", changed, err)
	}
	assertOpenCodeTrailShape(t, path)
	root, _ := config.ReadJSONC(path)
	servers := root["mcp"].(map[string]any)["servers"].(map[string]any)
	if _, has := servers["graft"]; has {
		t.Errorf("graft entry survived: %#v", servers)
	}
	if _, has := servers["context7"]; !has {
		t.Errorf("sibling context7 lost: %#v", servers)
	}

	// Idempotent: a second run finds no graft entry and writes nothing.
	before, _ := os.ReadFile(path)
	changed, err = migrateGraftMCPEntry(path, "opencode")
	if err != nil || changed {
		t.Fatalf("second migrate = %v, %v; want false, nil", changed, err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Errorf("second run rewrote the file")
	}
}

func TestMigrateGraftMCPEntry_ClaudeRewritesToTrail(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"graft":{"command":"graft","args":["mcp"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := migrateGraftMCPEntry(path, "claude-code")
	if err != nil || !changed {
		t.Fatalf("migrate = %v, %v", changed, err)
	}
	root, _ := config.ReadJSONC(path)
	servers := root["mcpServers"].(map[string]any)
	if _, has := servers["graft"]; has {
		t.Errorf("graft entry survived: %#v", servers)
	}
	trail, _ := servers["trail"].(map[string]any)
	if trail == nil || trail["command"] != "trail" {
		t.Fatalf("trail entry = %#v, want command trail", servers["trail"])
	}
	if args, _ := trail["args"].([]any); len(args) != 1 || args[0] != "mcp" {
		t.Errorf("args = %#v, want [mcp]", trail["args"])
	}
	if changed, _ := migrateGraftMCPEntry(path, "claude-code"); changed {
		t.Error("second run must be a no-op")
	}
}

func TestMigrateGraftMCPEntry_NoGraftIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(path, []byte(`{"mcp":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if changed, err := migrateGraftMCPEntry(path, "opencode"); err != nil || changed {
		t.Fatalf("migrate = %v, %v; want false, nil", changed, err)
	}
}
