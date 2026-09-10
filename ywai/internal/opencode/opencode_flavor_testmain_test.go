package opencode

import (
	"os"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/testsandbox"
)

// These tests assert the v1 bare routes (/agent, /provider). Without pinning
// they follow whichever OpenCode the developer has installed and fail on any
// machine with opencode2. api_path_test.go covers both flavors explicitly.
func TestMain(m *testing.M) {
	// Redirect every config path first: these tests reach write paths that
	// would otherwise land in the developer's live OpenCode config.
	_, cleanup := testsandbox.Isolate("ywai-opencode-test-home")
	os.Setenv(agent.OpenCodeOverrideEnv, "v1")
	code := m.Run()
	cleanup()
	os.Exit(code)
}
