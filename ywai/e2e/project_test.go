package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitReact(t *testing.T) {
	bin := buildBinary(t)
	targetDir := t.TempDir()

	out := runYwai(t, bin, "init", "react", "--dir", targetDir)

	if !strings.Contains(out, `initialized as "react"`) {
		t.Fatalf("expected init success, got: %s", out)
	}

	for _, file := range []string{"AGENTS.md", "REVIEW.md"} {
		path := filepath.Join(targetDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", file)
		}
	}
}

func TestInitAllTypes(t *testing.T) {
	types := []string{"generic", "react", "nest", "dotnet", "devops"}
	bin := buildBinary(t)

	for _, ptype := range types {
		t.Run(ptype, func(t *testing.T) {
			targetDir := t.TempDir()

			out := runYwai(t, bin, "init", ptype, "--dir", targetDir)

			if !strings.Contains(out, `initialized as "`+ptype+`"`) {
				t.Errorf("expected init %s success, got: %s", ptype, out)
			}

			agentsMd := filepath.Join(targetDir, "AGENTS.md")
			if _, err := os.Stat(agentsMd); os.IsNotExist(err) {
				t.Errorf("AGENTS.md missing for type %s", ptype)
			}
		})
	}
}

func TestInitInvalidType(t *testing.T) {
	bin := buildBinary(t)
	out, err := runYwaiAllowFail(t, bin, "init", "invalid-type")
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
	if !strings.Contains(out, "unknown project type") {
		t.Errorf("expected 'unknown project type' error, got: %s", out)
	}
}

func TestInitNoArgs(t *testing.T) {
	bin := buildBinary(t)
	_, err := runYwaiAllowFail(t, bin, "init")
	if err == nil {
		t.Fatal("expected error for missing args")
	}
}
