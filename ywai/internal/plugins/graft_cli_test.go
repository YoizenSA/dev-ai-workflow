package plugins

// graft_cli_test.go — RED tests for slice 2: replace the CodeGraph
// CLI installer with the Graft CLI installer in the plugins package.
//
// Contracts under test (acceptance item 3):
//   - The plugins package exposes InstallGraftCLI and WireGraftMCP.
//     These are the slice-2 replacements for InstallCodegraphCLI and
//     WireCodegraphMCP.
//
// Implementation note: in Go, "this function is exported by package X"
// has no public runtime reflection API. The standard, minimal-cost
// RED technique is a compile-time reference: if the name does not
// exist, this file fails to build and the test run reports the
// failure for exactly the right reason. Once the dev adds the names,
// the file compiles and the test reports PASS.
//
// We deliberately do NOT call the functions here — they execute
// `npm install` / `graft mcp`, which is the seam the slice contract
// tells us not to exercise.

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
	_ func() error = InstallGraftCLI
	_ func() error = WireGraftMCP
)

// TestPlugins_GraftSurfacePresent is the named runtime test that
// gives the suite a PASS once the compile-time guards succeed. It
// exercises the pure, testable seam of the slice (version parsing)
// without touching the npm / `graft mcp` binary seams.
func TestWriteGraftMCPEntry_OpenCodeShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(path, []byte(`{"mcp":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeGraftMCPEntry(path, "opencode", []string{"graft", "mcp"}); err != nil {
		t.Fatalf("writeGraftMCPEntry: %v", err)
	}

	assertOpenCodeGraftShape(t, path)
}

func TestWriteGraftMCPEntry_FlattensServersAndKeepsSiblings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	legacy := `{"mcp":{"timeout":15000,"context7":{"type":"remote","url":"https://x"},"graft":{"command":"graft","args":["mcp"]}}}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeGraftMCPEntry(path, "opencode", []string{"graft", "mcp"}); err != nil {
		t.Fatalf("writeGraftMCPEntry: %v", err)
	}

	assertOpenCodeGraftShape(t, path)
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

func TestWriteGraftMCPEntry_RepairsLegacyClaudeShape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	legacy := `{"mcp":{"graft":{"command":"graft","args":["mcp"]}}}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeGraftMCPEntry(path, "opencode", []string{"graft", "mcp"}); err != nil {
		t.Fatalf("writeGraftMCPEntry: %v", err)
	}

	assertOpenCodeGraftShape(t, path)
}

func assertOpenCodeGraftShape(t *testing.T, path string) {
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
	got, _ := servers["graft"].(map[string]any)
	if got["type"] != "local" {
		t.Errorf("type = %#v, want local", got["type"])
	}
	if _, has := got["enabled"]; has {
		t.Errorf("enabled = %#v, want absent (v2 uses disabled)", got["enabled"])
	}
	cmd, ok := got["command"].([]any)
	if !ok {
		t.Fatalf("command = %#v, want [graft mcp] array", got["command"])
	}
	if len(cmd) != 2 || cmd[0] != "graft" || cmd[1] != "mcp" {
		t.Errorf("command = %#v, want [graft mcp]", cmd)
	}
	if _, hasArgs := got["args"]; hasArgs {
		t.Errorf("args must not be set on opencode graft entry: %#v", got)
	}
}

// On Windows the bare name `graft` resolves to the Unix shell script npm
// also drops in the prefix, so a host that spawns without a shell gets
// ENOENT. The wired entry must carry the resolved launcher instead.
func TestGraftLaunchCommand_SubstitutesResolvedBinaryOnWindows(t *testing.T) {
	exe := filepath.Join("C:\\", "npm", "graft.cmd")
	got := graftLaunchCommand(exe, []string{"graft", "mcp"})

	if runtime.GOOS != "windows" {
		if len(got) != 2 || got[0] != "graft" {
			t.Fatalf("non-windows must keep the bare name for portability: %#v", got)
		}
		return
	}
	// Either shape is correct, depending on whether the npm-installed
	// entrypoint could be located: [node <cli.js> mcp] is preferred, the
	// resolved launcher is the fallback. What must never survive is the
	// bare name, and the subcommand must be carried through.
	if got[0] == "graft" {
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
func TestGraftLaunchCommand_DoesNotMutateInput(t *testing.T) {
	in := []string{"graft", "mcp"}
	graftLaunchCommand(filepath.Join("C:\\", "npm", "graft.cmd"), in)
	if in[0] != "graft" {
		t.Fatalf("input mutated: %#v", in)
	}
}

func TestGraftLaunchCommand_EmptyArgvIsLeftAlone(t *testing.T) {
	if got := graftLaunchCommand("x", nil); got != nil {
		t.Errorf("nil argv = %#v, want nil", got)
	}
}

// With no launcher resolved AND no npm entrypoint on disk there is nothing
// better to write, so the original argv must survive untouched rather than
// becoming a half-built command.
func TestGraftLaunchCommand_FallsBackWhenNothingResolves(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // hide npm, so graftNodeEntrypoint misses
	got := graftLaunchCommand("", []string{"graft", "mcp"})
	if len(got) != 2 || got[0] != "graft" || got[1] != "mcp" {
		t.Errorf("graftLaunchCommand = %#v, want [graft mcp]", got)
	}
}

func TestPlugins_GraftSurfacePresent(t *testing.T) {
	t.Log("InstallGraftCLI and WireGraftMCP are exported by internal/plugins")

	dir := t.TempDir()
	// graftVersionFromBinary execs the binary, so on Windows it must have an
	// extension LookPath/cmd can run — a shebang script is not launchable there.
	name := "graft"
	body := "#!/bin/sh\necho graft 0.1.0\n"
	if runtime.GOOS == "windows" {
		name += ".bat"
		body = "@echo graft 0.1.0\r\n"
	}
	fake := filepath.Join(dir, name)
	if err := os.WriteFile(fake, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := graftVersionFromBinary(fake)
	if err != nil {
		t.Fatalf("graftVersionFromBinary: %v", err)
	}
	if want := "graft 0.1.0"; got != want {
		t.Fatalf("graftVersionFromBinary = %q, want %q", got, want)
	}
}
