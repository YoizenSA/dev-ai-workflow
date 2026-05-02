package e2e

import (
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "--version")
	if !strings.Contains(out, "ywai version") {
		t.Fatalf("expected version output, got: %s", out)
	}
}

func TestHelp(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "--help")

	expected := []string{"install", "update", "init", "skills"}
	for _, cmd := range expected {
		if !strings.Contains(out, cmd) {
			t.Errorf("expected %q in help output", cmd)
		}
	}
}

func TestInstallHelp(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--help")
	if !strings.Contains(out, "--type") || !strings.Contains(out, "--agent") || !strings.Contains(out, "--dry-run") {
		t.Errorf("expected flags in install help, got: %s", out)
	}
}

func TestInitHelp(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "init", "--help")
	if !strings.Contains(out, "type") {
		t.Errorf("expected 'type' in init help, got: %s", out)
	}
}
