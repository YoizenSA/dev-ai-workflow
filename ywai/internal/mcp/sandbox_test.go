package mcp

import (
	"os"
	"strings"
	"testing"
)

// The package sandbox is only real if it covers the variable EntryTargetPath
// reads FIRST. Orca sets OPENCODE_CONFIG_DIR, and while TestMain cleared only
// HOME and XDG_CONFIG_HOME the sandbox was a no-op on those machines: install
// tests wrote their fixtures ("somebin", "fake-server", "myserver") into the
// developer's live config. This fails if that hole is reopened.
func TestSandboxRedirectsEveryOpenCodePath(t *testing.T) {
	for _, name := range []string{"OPENCODE_CONFIG_DIR", "XDG_CONFIG_HOME"} {
		if v, ok := os.LookupEnv(name); ok && v != "" {
			t.Errorf("%s is set to %q inside the test sandbox: writes escape to the real config", name, v)
		}
	}

	path, err := EntryTargetPath("opencode")
	if err != nil {
		t.Fatalf("EntryTargetPath: %v", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	if !strings.HasPrefix(path, home) {
		t.Fatalf("install target %q is outside the sandboxed HOME %q", path, home)
	}
	if !strings.Contains(home, "ywai-mcp-test-home") {
		t.Fatalf("HOME %q is not the throwaway sandbox — tests would write to a real config", home)
	}
}
