package e2e

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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

// The install step count is dynamic (see countApplySteps), so assert the
// numbered-progress format is well-formed instead of a magic count: every
// marker shares the same total, they run 1..total with no gaps, and the last
// equals the total.
func TestInstallShowsNumberedSteps(t *testing.T) {
	withFakeAgent(t)
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--dry-run")

	re := regexp.MustCompile(`\[(\d+)/(\d+)\]`)
	matches := re.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		t.Fatalf("expected numbered [cur/total] steps in install output, got: %s", out)
	}

	total := matches[0][2]
	for i, m := range matches {
		if m[2] != total {
			t.Errorf("step %d has total %s, expected consistent %s", i+1, m[2], total)
		}
		cur, _ := strconv.Atoi(m[1])
		if cur != i+1 {
			t.Errorf("expected step [%d/%s], got [%s/%s]", i+1, total, m[1], m[2])
		}
	}

	if last := matches[len(matches)-1][1]; last != total {
		t.Errorf("last step %s does not match total %s", last, total)
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
