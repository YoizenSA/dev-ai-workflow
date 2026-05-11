package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAgentDetectionWithFakeBinary(t *testing.T) {
	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "claude")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	if err := os.WriteFile(binaryPath, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatalf("Failed to create fake binary: %v", err)
	}

	oldPath := os.Getenv("PATH")
	newPath := tempDir + string(os.PathListSeparator) + oldPath
	os.Setenv("PATH", newPath)
	defer os.Setenv("PATH", oldPath)

	bin := buildBinary(t)
	out := runYwai(t, bin, "install", "--dry-run")
	if !strings.Contains(out, "claude-code") {
		t.Errorf("expected claude-code in detected agents, got: %s", out)
	}
}

func TestAgentErrorHandling(t *testing.T) {
	bin := buildBinary(t)

	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", oldPath)

	out, err := runYwaiAllowFail(t, bin, "install", "--dry-run")

	// With the fallback to detect by config dir, agents may be detected
	// even if their binary is not in PATH
	if strings.Contains(out, "claude-code") || strings.Contains(out, "antigravity") {
		// Agent detected by config dir - this is expected behavior now
		if err != nil {
			t.Errorf("expected no error when agent detected by config dir, got: %v", err)
		}
		return
	}

	// No agent detected by config dir either - should fail
	if err == nil {
		t.Error("expected error with empty PATH and no config dir")
	}
	if !strings.Contains(out, "no supported agents detected") {
		t.Errorf("expected 'no supported agents detected', got: %s", out)
	}
}

func TestBinarySearch(t *testing.T) {
	t.Run("find go binary", func(t *testing.T) {
		path, err := exec.LookPath("go")
		if err != nil {
			t.Skip("go not found in PATH")
		}
		if path == "" {
			t.Error("expected non-empty path for go")
		}
	})

	t.Run("find non-existent binary", func(t *testing.T) {
		_, err := exec.LookPath("binary-that-does-not-exist-12345")
		if err == nil {
			t.Error("expected error for non-existent binary")
		}
	})
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}
