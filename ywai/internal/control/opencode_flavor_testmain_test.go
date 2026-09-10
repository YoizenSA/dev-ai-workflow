package control

import (
	"os"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/testsandbox"
)

// These tests assert the opencode v1 config layout, which is what ywai wrote
// before the flavor switch existed. Without pinning, they follow whichever
// OpenCode the developer has installed and fail on any machine with opencode2.
// A v2 test opts back in with t.Setenv(agent.OpenCodeOverrideEnv, "v2").
func TestMain(m *testing.M) {
	// Redirect every config path first: these tests reach write paths that
	// would otherwise land in the developer's live OpenCode config.
	_, cleanup := testsandbox.Isolate("ywai-control-test-home")
	os.Setenv(agent.OpenCodeOverrideEnv, "v1")
	code := m.Run()
	cleanup()
	os.Exit(code)
}
