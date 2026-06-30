package plugins

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// writeMockCodegraph writes a tiny executable at dir/codegraph that prints the
// given stdout when invoked. Used to exercise codegraphVersionFromBinary
// without depending on a real install.
func writeMockCodegraph(t *testing.T, dir, out string) string {
	t.Helper()
	bin := filepath.Join(dir, "codegraph")
	// A minimal shell script: ignore args, print the canned output.
	script := "#!/bin/sh\ncat <<'EOF'\n" + out + "\nEOF\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write mock codegraph: %v", err)
	}
	return bin
}

func TestCodegraphVersionFromBinary_ParsesFirstLine(t *testing.T) {
	dir := t.TempDir()
	// codegraph version prints the bare version first; newer builds append an
	// "Update available" banner line that must NOT be parsed as the version.
	bin := writeMockCodegraph(t, dir, "0.9.6\nUpdate available: run `codegraph upgrade`\n")
	got, err := codegraphVersionFromBinary(bin)
	if err != nil {
		t.Fatalf("codegraphVersionFromBinary error: %v", err)
	}
	if got != "0.9.6" {
		t.Fatalf("version = %q, want 0.9.6", got)
	}
}

func TestCodegraphVersionFromBinary_EmptyOutput(t *testing.T) {
	dir := t.TempDir()
	bin := writeMockCodegraph(t, dir, "")
	if _, err := codegraphVersionFromBinary(bin); err == nil {
		t.Fatal("expected error for empty version output, got nil")
	}
}

func TestCodegraphVersionFromBinary_MissingBinary(t *testing.T) {
	if _, err := codegraphVersionFromBinary(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
}

func TestCodegraphResolveBin_FallbackSymlink(t *testing.T) {
	// Force a fake HOME so the ~/.local/bin/codegraph fallback can be exercised
	// even when a real codegraph is (or is not) on PATH.
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Place a file at the fallback path and assert it resolves there.
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	fallback := filepath.Join(binDir, "codegraph")
	if err := os.WriteFile(fallback, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write fallback: %v", err)
	}

	got := codegraphResolveBin()
	// If a real codegraph is on PATH, exec.LookPath wins and returns that —
	// valid on a dev machine. Only assert the fallback path when PATH misses.
	if _, lookErr := exec.LookPath("codegraph"); lookErr != nil {
		if got != fallback {
			t.Fatalf("codegraphResolveBin = %q, want fallback %q", got, fallback)
		}
	}
}
