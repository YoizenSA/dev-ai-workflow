package e2e

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func withFakeAgent(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	fakeBin := filepath.Join(tempDir, "claude")
	if runtime.GOOS == "windows" {
		fakeBin += ".exe"
	}
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatalf("failed to create fake agent binary: %v", err)
	}
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestInstallDryRunShowsAllSkills(t *testing.T) {
	withFakeAgent(t)
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--dry-run")

	if !strings.Contains(out, "Copying all ywai extra skills") {
		t.Errorf("expected 'Copying all ywai extra skills', got: %s", out)
	}
}

func TestInstallDryRunNoTypeShowsAllSkills(t *testing.T) {
	withFakeAgent(t)
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--dry-run")

	if strings.Contains(out, "Skills for") {
		t.Errorf("should not show skill filter when no type specified, got: %s", out)
	}
}

func TestInstallHas3Steps(t *testing.T) {
	withFakeAgent(t)
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--dry-run")

	for _, step := range []string{"[1/3]", "[2/3]", "[3/3]"} {
		if !strings.Contains(out, step) {
			t.Errorf("expected %s in install output", step)
		}
	}
}

func TestInstallOutputNoGentleAiAcronym(t *testing.T) {
	withFakeAgent(t)
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--dry-run")

	if strings.Contains(out, "GGA") {
		t.Errorf("should not mention GGA, got: %s", out)
	}
}
