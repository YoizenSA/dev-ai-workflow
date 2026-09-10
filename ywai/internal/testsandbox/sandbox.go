// Package testsandbox redirects the config paths a test might write to.
//
// It exists because clearing one variable is not enough and getting it half
// right is worse than leaving it alone. EntryTargetPath resolves an OpenCode
// config from OPENCODE_CONFIG_DIR first, then XDG_CONFIG_HOME, then HOME. A
// sandbox that pins HOME but leaves OPENCODE_CONFIG_DIR set writes into the
// live config of any host that sets it (Orca does); one that clears
// OPENCODE_CONFIG_DIR without pinning HOME just moves the damage to
// ~/.config/opencode instead. Both have happened, and both left a developer
// with test fixtures — "somebin", "fake-server", "myserver" — in the config
// their editor was actually using.
//
// Isolate covers the whole set at once so a future path cannot be missed in
// one package and remembered in another.
package testsandbox

import (
	"os"
	"path/filepath"
)

// Isolate points every config path a test can reach at a throwaway HOME and
// returns a cleanup func. Call it from TestMain, before m.Run.
//
// Per-test t.Setenv("HOME", ...) still works and simply re-homes inside this
// sandbox — those calls override, they do not escape.
func Isolate(prefix string) (home string, cleanup func()) {
	home, err := os.MkdirTemp("", prefix+"-*")
	if err != nil {
		// TestMain has no *testing.T; panic so this cannot pass unnoticed.
		panic("test sandbox: cannot create temp HOME: " + err.Error())
	}

	_ = os.Setenv("HOME", home)
	_ = os.Setenv("USERPROFILE", home) // Windows equivalent of HOME.
	_ = os.Unsetenv("XDG_CONFIG_HOME")
	// Checked before HOME by EntryTargetPath, so it has to go or the rest is
	// decoration.
	_ = os.Unsetenv("OPENCODE_CONFIG_DIR")

	// Pre-create the dir so a cold-install path has somewhere to land.
	_ = os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o755)

	return home, func() { _ = os.RemoveAll(home) }
}
