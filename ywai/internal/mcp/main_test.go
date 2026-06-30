package mcp

// main_test.go — package-wide test isolation for the mcp package.
//
// Several tests in this package (e.g. installer_test.go) call the production
// Install() path, which persists MCP entries via WriteAgentConfig ->
// EntryTargetPath. That function resolves the target file from os.UserHomeDir()
// (and XDG_CONFIG_HOME). Without isolation here, those tests would write test
// fixtures (e.g. the "fake-server" / "somebin" / "myserver" entries) into the
// developer's REAL ~/.config/opencode/opencode.json, corrupting their opencode
// MCP setup.
//
// TestMain sandboxes every test in the package under a throwaway HOME and
// clears XDG_CONFIG_HOME so EntryTargetPath always resolves under that temp
// HOME, regardless of which test (or future test) reaches the write path.
//
// Per-test t.Setenv("HOME", ...) calls elsewhere in this package remain valid
// and simply re-home within this same sandbox — they override, they don't
// escape it.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	// Create ONE temp HOME for the whole package. os.MkdirTemp instead of
	// t.TempDir() because TestMain has no *testing.T. It is intentionally
	// leaked — the test process is short-lived and the OS reclaims /tmp.
	home, err := os.MkdirTemp("", "ywai-mcp-test-home-*")
	if err != nil {
		// Cannot use t.Fatal here; panic so the failure is unmissable.
		panic("mcp test sandbox: failed to create temp HOME: " + err.Error())
	}

	// Sandboxing both vars covers every code path in EntryTargetPath:
	//   - opencode honors XDG_CONFIG_HOME when set (cleared here -> falls
	//     back to $HOME/.config/opencode/opencode.json under temp HOME).
	//   - pi / claude-code use $HOME directly.
	_ = os.Setenv("HOME", home)
	_ = os.Setenv("USERPROFILE", home) // Windows equivalent of HOME.
	_ = os.Unsetenv("XDG_CONFIG_HOME")

	// Pre-create the opencode config dir so cold-install paths (the ones that
	// previously polluted the real file) don't even need to MkdirAll into the
	// real tree. Harmless if tests never touch it.
	_ = os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o755)

	code := m.Run()

	// Best-effort cleanup; not required for correctness, keeps /tmp tidy.
	_ = os.RemoveAll(home)

	os.Exit(code)
}
