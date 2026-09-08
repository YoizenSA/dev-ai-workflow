package agents

import (
	"os"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
)

// These tests assert the opencode v1 frontmatter schema (nested "permission:"
// map). Without pinning they follow whichever OpenCode the developer has
// installed and fail on any machine with opencode2. v2 tests opt back in with
// t.Setenv(agent.OpenCodeOverrideEnv, "v2").
func TestMain(m *testing.M) {
	os.Setenv(agent.OpenCodeOverrideEnv, "v1")
	os.Exit(m.Run())
}
