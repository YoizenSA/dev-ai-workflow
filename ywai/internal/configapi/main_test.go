package configapi

// main_test.go — package-wide test isolation.
//
// These handlers resolve the OpenCode config through mcp.EntryTargetPath, which
// reads OPENCODE_CONFIG_DIR before HOME. Tests that pin only HOME therefore
// wrote into the live config of any host that sets it (Orca), which is exactly
// how a developer ends up with their MCP servers rewritten by a test run.

import (
	"os"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/testsandbox"
)

func TestMain(m *testing.M) {
	_, cleanup := testsandbox.Isolate("ywai-configapi-test-home")
	code := m.Run()
	cleanup()
	os.Exit(code)
}
