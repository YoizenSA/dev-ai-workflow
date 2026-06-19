package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const buildTimeout = 30 * time.Second

// buildBinary compiles the ywai binary and returns its path.
// It includes timeout protection and proper error handling.
func buildBinary(t *testing.T) string {
	t.Helper()
	binName := "ywai-test"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}

	repoRoot := repoRoot(t)
	binPath := filepath.Join(t.TempDir(), binName)

	// Set up build command with timeout
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, "./cmd/ywai")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Check if go command exists
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatalf("go compiler not found: %v", err)
	}

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("build timed out after %s", buildTimeout)
		}
		t.Fatalf("failed to build binary: %v", err)
	}

	// Verify the binary was created
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("binary not found after build: %v", err)
	}

	return binPath
}

// repoRoot finds the repository root by looking for go.mod file.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	maxDepth := 10 // Prevent infinite loops
	depth := 0

	for depth < maxDepth {
		modPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(modPath); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
		depth++
	}

	t.Fatal("could not find repo root (go.mod) within reasonable depth")
	return ""
}

// runYwai executes ywai command and returns its output.
// It fails the test if the command returns an error.
func runYwai(t *testing.T, bin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = repoRoot(t)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Command failed: %s %v", bin, args)
		t.Logf("Exit code: %v", err)
		t.Logf("Output: %s", out)
		t.Fatalf("ywai %s failed: %v\noutput: %s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// runYwaiAllowFail executes ywai command and returns both output and error.
// It does not fail the test if the command returns an error.
func runYwaiAllowFail(t *testing.T, bin string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = repoRoot(t)

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// buildCmd creates a command with proper configuration for testing.
func buildCmd(t *testing.T, bin string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = repoRoot(t)
	return cmd
}

// cleanupTempDir removes the temporary directory if it exists.
// Used in test cleanup functions.
func cleanupTempDir(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Logf("Warning: failed to clean up temp directory %s: %v", path, err)
	}
}
