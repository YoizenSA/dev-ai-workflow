package agents

import (
	"os"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/testsandbox"
)

// These tests assert the opencode v1 frontmatter schema (nested "permission:"
// map). Without pinning they follow whichever OpenCode the developer has
// installed and fail on any machine with opencode2. v2 tests opt back in with
// t.Setenv(agent.OpenCodeOverrideEnv, "v2").
func TestMain(m *testing.M) {
	// Redirect every config path first: these tests reach write paths that
	// would otherwise land in the developer's live OpenCode config.
	_, cleanup := testsandbox.Isolate("ywai-agents-test-home")
	os.Setenv(agent.OpenCodeOverrideEnv, "v1")
	code := m.Run()
	cleanup()
	os.Exit(code)
}
