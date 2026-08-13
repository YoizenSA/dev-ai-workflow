package gentlai

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Install must not upgrade an already-installed gentle-ai: that is `ywai
// update`'s job. Guards against reintroducing the implicit Upgrade() call.
//
// CurrentVersion() is a ywai-owned stub that always returns "", so the
// version-based guard cannot be exercised through it. Instead, a fake
// `gentle-ai` executable is placed FIRST on PATH and writes a counter file
// on every invocation: a compliant Install() must never exec it.
func TestInstallDoesNotUpgradeWhenPresent(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "invoked")
	fakeBin := filepath.Join(dir, "gentle-ai")
	script := "#!/bin/sh\ntouch " + counter + "\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, err := exec.LookPath("gentle-ai"); err != nil {
		t.Fatalf("test setup: fake gentle-ai must resolve on PATH, got %v", err)
	}

	if err := Install(); err != nil {
		t.Fatalf("Install() on an existing install must be a no-op, got %v", err)
	}

	if _, err := os.Stat(counter); err == nil {
		t.Fatal("Install() must not invoke gentle-ai; fake binary was executed (counter file exists)")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat counter: %v", err)
	}
}
