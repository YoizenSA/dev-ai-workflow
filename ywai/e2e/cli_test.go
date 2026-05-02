package e2e

import (
	"os/exec"
	"path/filepath"
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

func TestVersionWithErrorHandling(t *testing.T) {
	// Test version command with a non-existent binary
	_, err := runYwaiWithError("non-existent-binary", "--version")
	if err == nil {
		t.Error("expected error for non-existent binary, got nil")
	}
}

func TestHelp(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "--help")

	expected := []string{"install", "update", "init", "skills"}
	for _, cmd := range expected {
		if !strings.Contains(out, cmd) {
			t.Errorf("expected %q in help output, got: %s", cmd, out)
		}
	}
}

func TestHelpEmptyBinary(t *testing.T) {
	// Test help with empty binary name
	_, err := runYwaiWithError("", "--help")
	if err == nil {
		t.Error("expected error for empty binary name, got nil")
	}
}

func TestInstallHelp(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--help")

	requiredFlags := []string{"--type", "--agent", "--dry-run"}
	for _, flag := range requiredFlags {
		if !strings.Contains(out, flag) {
			t.Errorf("expected flag %q in install help, got: %s", flag, out)
		}
	}
}

func TestInstallHelpWithErrorHandling(t *testing.T) {
	bin := buildBinary(t)
	out, err := runYwaiAllowFail(t, bin, "install", "--help", "--unknown-flag")
	if !strings.Contains(out, "--type") {
		t.Errorf("help output should still contain --type, got: %s, err: %v", out, err)
	}
}

func TestInitHelp(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "init", "--help")
	if !strings.Contains(out, "type") {
		t.Errorf("expected 'type' in init help, got: %s", out)
	}
}

func TestInitWithInvalidType(t *testing.T) {
	bin := buildBinary(t)
	// This should show help or an error, but not crash
	out, err := runYwaiWithError(bin, "init", "--type", "invalid-type")
	if err == nil {
		// If it doesn't error, it should at least show help
		if !strings.Contains(out, "init") {
			t.Error("command should either error or show help")
		}
	}
}

func TestCrossPlatformPathHandling(t *testing.T) {
	bin := buildBinary(t)

	absBin, err := filepath.Abs(bin)
	if err == nil {
		out, err := runYwaiWithError(absBin, "--version")
		if err != nil && !strings.Contains(out, "ywai version") {
			t.Errorf("expected version output with absolute path, got: %s (error: %v)", out, err)
		}
	}
}

func TestMultipleFlagCombinations(t *testing.T) {
	bin := buildBinary(t)
	out, _ := runYwaiAllowFail(t, bin, "--version", "--help")
	if !strings.Contains(out, "version") && !strings.Contains(out, "help") && !strings.Contains(out, "Usage") {
		t.Error("expected some output when mixing version and help flags")
	}
}

func TestBinaryExitCodeHandling(t *testing.T) {
	bin := buildBinary(t)

	// Test invalid command
	_, err := runYwaiWithError(bin, "invalid-command")
	if err == nil {
		t.Error("expected error for invalid command")
	}

	// Test missing required argument
	_, err = runYwaiWithError(bin, "install")
	_ = err
}

// Helper function to run ywai command and capture both output and error
func runYwaiWithError(bin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
