package e2e

import (
	"strings"
	"testing"
)

func TestInstallDryRunShowsSkillFilter(t *testing.T) {
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
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--dry-run")

	if strings.Contains(out, "Skills for") {
		t.Errorf("should not show skill filter when no type specified, got: %s", out)
	}
}

func TestInstallDryRunGenericShowsAllMessage(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--type", "generic", "--dry-run")

	if !strings.Contains(out, "linking all skills") {
		t.Errorf("expected 'linking all skills' for generic, got: %s", out)
	}
}

func TestInstallHas4Steps(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--dry-run")

	for _, step := range []string{"[1/4]", "[2/4]", "[3/4]", "[4/4]"} {
		if !strings.Contains(out, step) {
			t.Errorf("expected %s in install output", step)
		}
	}
}

func TestInstallNoGGA(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--dry-run")

	if strings.Contains(out, "GGA") {
		t.Errorf("should not mention GGA, got: %s", out)
	}
}
