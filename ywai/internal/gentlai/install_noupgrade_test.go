package gentlai

import (
	"os/exec"
	"testing"
)

// Install must not upgrade an already-installed gentle-ai: that is `ywai
// update`'s job. Guards against reintroducing the implicit Upgrade() call.
func TestInstallDoesNotUpgradeWhenPresent(t *testing.T) {
	if !IsInstalled() {
		t.Skip("gentle-ai not installed")
	}
	before := CurrentVersion()
	if err := Install(); err != nil {
		t.Fatalf("Install() on an existing install must be a no-op, got %v", err)
	}
	if after := CurrentVersion(); after != before {
		t.Fatalf("Install() changed gentle-ai version %q → %q", before, after)
	}
	if _, err := exec.LookPath("gentle-ai"); err != nil {
		t.Fatalf("gentle-ai disappeared: %v", err)
	}
}
