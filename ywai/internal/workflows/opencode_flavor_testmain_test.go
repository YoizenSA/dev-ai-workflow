package workflows

import (
	"os"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/testsandbox"
)

func TestMain(m *testing.M) {
	// Redirect every config path first: these tests reach write paths that
	// would otherwise land in the developer's live OpenCode config.
	_, cleanup := testsandbox.Isolate("ywai-workflows-test-home")
	code := m.Run()
	cleanup()
	os.Exit(code)
}
