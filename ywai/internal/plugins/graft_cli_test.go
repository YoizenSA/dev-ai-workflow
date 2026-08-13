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
	got, _ := mcpMap["graft"].(map[string]any)
	if got["type"] != "local" {
		t.Errorf("type = %#v, want local", got["type"])
	}
	if got["enabled"] != true {
		t.Errorf("enabled = %#v, want true", got["enabled"])
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

func TestPlugins_GraftSurfacePresent(t *testing.T) {
	t.Log("InstallGraftCLI and WireGraftMCP are exported by internal/plugins")

	dir := t.TempDir()
	fake := filepath.Join(dir, "graft")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho graft 0.1.0\n"), 0o755); err != nil {
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
