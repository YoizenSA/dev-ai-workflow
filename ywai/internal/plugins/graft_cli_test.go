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

func TestWriteGraftMCPEntry_NestsUnderServersAndLiftsSiblings(t *testing.T) {
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
	if _, ok := mcpMap["context7"]; ok {
		t.Fatal("context7 must be lifted into mcp.servers")
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
	if _, flat := mcpMap["graft"].(map[string]any); flat {
		t.Fatal("graft must not be a sibling of mcp.servers")
	}
	servers, ok := mcpMap["servers"].(map[string]any)
	if !ok {
		t.Fatalf("mcp.servers missing: %v", mcpMap)
	}
	got, _ := servers["graft"].(map[string]any)
	if got["type"] != "local" {
		t.Errorf("type = %#v, want local", got["type"])
	}
	// v2: servers are enabled by default; an "enabled" flag must not be written.
	if _, has := got["enabled"]; has {
		t.Errorf("enabled = %#v, want absent (v2: absent = enabled)", got["enabled"])
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
