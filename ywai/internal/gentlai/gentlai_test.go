package gentlai

import (
	"slices"
	"testing"
)

// ─── InstallOptions.buildArgs ─────────────────────────────────────────────

func TestInstallOptions_BuildArgs_Minimal(t *testing.T) {
	opts := InstallOptions{
		AgentName: "opencode",
	}
	args := opts.buildArgs(nil)

	want := []string{
		"install", "--agent", "opencode", "--scope", "global",
		"--component", "engram", "--component", "skills",
		"--component", "context7", "--component", "permissions",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestInstallOptions_BuildArgs_AllFlags(t *testing.T) {
	opts := InstallOptions{
		AgentName: "claude-code",
		Preset:    "minimal",
		Scope:     "workspace",
		DryRun:    true,
	}
	args := opts.buildArgs(nil)

	want := []string{
		"install", "--agent", "claude-code",
		"--scope", "workspace",
		"--component", "engram", "--component", "skills",
		"--component", "context7", "--component", "permissions",
		"--dry-run",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestInstallOptions_BuildArgs_AlwaysHasScope(t *testing.T) {
	opts := InstallOptions{
		AgentName: "opencode",
	}
	args := opts.buildArgs(nil)

	if !slices.Contains(args, "--scope") {
		t.Fatal("--scope should always be present")
	}
	idx := slices.Index(args, "--scope")
	if args[idx+1] != "global" {
		t.Fatalf("default scope = %q, want global", args[idx+1])
	}
}

func TestInstallOptions_BuildArgs_NoDryRunWhenFalse(t *testing.T) {
	opts := InstallOptions{
		AgentName: "opencode",
		DryRun:    false,
	}
	args := opts.buildArgs(nil)

	if slices.Contains(args, "--dry-run") {
		t.Fatal("--dry-run should not be present when false")
	}
}

// ─── InstallOptions defaults ──────────────────────────────────────────────

func TestInstallOptions_EffectiveScope_Default(t *testing.T) {
	opts := InstallOptions{}
	if got, want := opts.effectiveScope(), "global"; got != want {
		t.Fatalf("effectiveScope() = %q, want %q", got, want)
	}
}

func TestInstallOptions_EffectiveScope_Explicit(t *testing.T) {
	opts := InstallOptions{Scope: "workspace"}
	if got, want := opts.effectiveScope(), "workspace"; got != want {
		t.Fatalf("effectiveScope() = %q, want %q", got, want)
	}
}

// ─── Version parsing ──────────────────────────────────────────────────────

func TestParseVersion_Semver(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"gentle-ai version v0.5.0", "0.5.0"},
		{"v1.0.0-beta.1", "1.0.0-beta.1"},
		{"no version here", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := parseVersion(tt.input)
		if got != tt.want {
			t.Errorf("parseVersion(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
