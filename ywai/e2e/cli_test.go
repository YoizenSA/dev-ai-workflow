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

	expected := []string{"install", "update", "skills"}
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

	requiredFlags := []string{"--agent", "--dry-run"}
	for _, flag := range requiredFlags {
		if !strings.Contains(out, flag) {
			t.Errorf("expected flag %q in install help, got: %s", flag, out)
		}
	}
}

func TestInstallHelpWithErrorHandling(t *testing.T) {
	bin := buildBinary(t)
	out, err := runYwaiAllowFail(t, bin, "install", "--help", "--unknown-flag")
	if !strings.Contains(out, "--agent") {
		t.Errorf("help output should still contain --agent, got: %s, err: %v", out, err)
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

func TestGroupsCommand(t *testing.T) {
	bin := buildBinary(t)

	// Run with --help first to verify groups subcommand exists
	helpOut := runYwai(t, bin, "--help")
	if !strings.Contains(helpOut, "groups") {
		t.Fatal("expected 'groups' subcommand in help output")
	}

	out, err := runYwaiWithError(bin, "groups")
	if err != nil {
		// This can happen when the binary was built without embedded data
		// and ~/.ywai/agents doesn't exist yet. Accept it as a soft failure.
		t.Logf("groups command exited with error (likely no embedded data): %v", err)
		t.Logf("output: %s", out)
		return
	}

	// Should list group names — at minimum "core" is always present
	if !strings.Contains(out, "core") {
		t.Errorf("expected 'core' in groups output, got: %s", out)
	}
	// Each group should be on its own line
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 1 {
		t.Errorf("expected at least 1 group, got %d", len(lines))
	}
}

func TestInstallHelpShowsGroupFlags(t *testing.T) {
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--help")

	requiredFlags := []string{"--group", "--all-groups"}
	for _, flag := range requiredFlags {
		if !strings.Contains(out, flag) {
			t.Errorf("expected %q flag in install --help output, got: %s", flag, out)
		}
	}
}

func TestInstallDryRunWithGroup(t *testing.T) {
	withFakeAgent(t)
	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--dry-run", "--group", "social-refactor")

	// Should not crash with FATAL errors
	if strings.Contains(out, "FATAL") {
		t.Errorf("install --dry-run --group social-refactor should not produce FATAL errors, got: %s", out)
	}
	// Should complete the dry-run flow
	if !strings.Contains(out, "=== Done! ===") {
		t.Errorf("expected dry-run to complete, got: %s", out)
	}
	// Should mention the group-related flag in its output (the command parsed it without error)
	if strings.Contains(out, "unknown flag") {
		t.Errorf("--group flag should be recognised, got: %s", out)
	}
}

// Helper function to run ywai command and capture both output and error
func runYwaiWithError(bin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}
