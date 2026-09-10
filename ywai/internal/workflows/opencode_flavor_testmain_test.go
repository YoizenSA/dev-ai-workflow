package workflows

import (
	"os"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/testsandbox"
)

// The exporter's golden output is the opencode v1 frontmatter schema. Without
// pinning these tests follow the developer's installed OpenCode and fail on any
// machine with opencode2.
func TestMain(m *testing.M) {
	// Redirect every config path first: these tests reach write paths that
	// would otherwise land in the developer's live OpenCode config.
	_, cleanup := testsandbox.Isolate("ywai-workflows-test-home")
	os.Setenv(agent.OpenCodeOverrideEnv, "v1")
	code := m.Run()
	cleanup()
	os.Exit(code)
}
