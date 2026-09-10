package mcp

// main_test.go — package-wide test isolation for the mcp package.
//
// Several tests here (installer_test.go among them) call the production
// Install() path, which persists entries through WriteAgentConfig ->
// EntryTargetPath. That resolves a real config file, so without isolation the
// fixtures below — "somebin", "fake-server", "myserver" — land in the
// developer's live OpenCode config and break their MCP setup.
//
// The redirection lives in internal/testsandbox because getting it half right
// is worse than not doing it: EntryTargetPath reads OPENCODE_CONFIG_DIR before
// HOME, so pinning only HOME leaves hosts that set it (Orca) exposed, and
// clearing only the var moves the writes to ~/.config/opencode instead.

import (
	"os"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/testsandbox"
)

func TestMain(m *testing.M) {
	_, cleanup := testsandbox.Isolate("ywai-mcp-test-home")
	code := m.Run()
	cleanup()
	os.Exit(code)
}
