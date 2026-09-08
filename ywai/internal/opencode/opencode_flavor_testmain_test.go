package opencode

import (
	"os"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
)

// These tests assert the v1 bare routes (/agent, /provider). Without pinning
// they follow whichever OpenCode the developer has installed and fail on any
// machine with opencode2. api_path_test.go covers both flavors explicitly.
func TestMain(m *testing.M) {
	os.Setenv(agent.OpenCodeOverrideEnv, "v1")
	os.Exit(m.Run())
}
