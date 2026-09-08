package workflows

import (
	"os"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
)

// The exporter's golden output is the opencode v1 frontmatter schema. Without
// pinning these tests follow the developer's installed OpenCode and fail on any
// machine with opencode2.
func TestMain(m *testing.M) {
	os.Setenv(agent.OpenCodeOverrideEnv, "v1")
	os.Exit(m.Run())
}
