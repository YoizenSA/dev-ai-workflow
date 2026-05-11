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
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", oldPath) })
	os.Setenv("PATH", tempDir+string(os.PathListSeparator)+oldPath)
}

func TestInstallDryRunShowsSkillFilter(t *testing.T) {
	withFakeAgent(t)
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--type", "react", "--dry-run")

	if !strings.Contains(out, "Skills for react:") {
		t.Errorf("expected skill filter line, got: %s", out)
	}
	if !strings.Contains(out, "react-19") {
		t.Errorf("expected react-19 in filtered output, got: %s", out)
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

func TestInstallDryRunGenericShowsAllMessage(t *testing.T) {
	withFakeAgent(t)
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--type", "generic", "--dry-run")

	if !strings.Contains(out, "copying all ywai extra skills") {
		t.Errorf("expected 'copying all ywai extra skills' for generic, got: %s", out)
	}
}

func TestInstallHas4Steps(t *testing.T) {
	withFakeAgent(t)
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--dry-run")

	for _, step := range []string{"[1/4]", "[2/4]", "[3/4]", "[4/4]"} {
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
