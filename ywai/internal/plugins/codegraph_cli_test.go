package plugins

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseCodegraphVersion_FirstLine(t *testing.T) {
	// codegraph version prints the bare version first; newer builds append an
	// "Update available" banner line that must NOT be parsed as the version.
	got, err := parseCodegraphVersion([]byte("0.9.6\nUpdate available: run `codegraph upgrade`\n"))
	if err != nil {
		t.Fatalf("parseCodegraphVersion error: %v", err)
	}
	if got != "0.9.6" {
		t.Fatalf("version = %q, want 0.9.6", got)
	}
}

func TestParseCodegraphVersion_SkipsLeadingBlankLines(t *testing.T) {
	got, err := parseCodegraphVersion([]byte("\n\n   \n1.2.3\n"))
	if err != nil {
		t.Fatalf("parseCodegraphVersion error: %v", err)
	}
	if got != "1.2.3" {
		t.Fatalf("version = %q, want 1.2.3", got)
	}
}

func TestParseCodegraphVersion_EmptyOutput(t *testing.T) {
	if _, err := parseCodegraphVersion([]byte("   \n\n")); err == nil {
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
	if runtime.GOOS == "windows" {
		// os.UserHomeDir reads USERPROFILE on Windows, not HOME.
		t.Setenv("USERPROFILE", home)
	}

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
