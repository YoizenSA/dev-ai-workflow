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
		"install", "--agent", "opencode", "--persona", "neutral", "--scope", "global",
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
		SDDMode:   "multi",
		Persona:   "gentleman",
		DryRun:    true,
	}
	args := opts.buildArgs(nil)

	want := []string{
		"install", "--agent", "claude-code",
		"--persona", "gentleman", "--scope", "workspace",
		"--component", "engram", "--component", "skills",
		"--component", "context7", "--component", "permissions",
		"--sdd-mode", "multi", "--dry-run",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestInstallOptions_BuildArgs_CustomPersona(t *testing.T) {
	opts := InstallOptions{
		AgentName: "cursor",
		Persona:   "custom",
	}
	args := opts.buildArgs(nil)

	if !slices.Contains(args, "--persona") {
		t.Fatal("missing --persona flag")
	}
	idx := slices.Index(args, "--persona")
	if args[idx+1] != "custom" {
		t.Fatalf("persona = %q, want custom", args[idx+1])
	}
	if !slices.Contains(args, "--component") {
		t.Fatal("missing --component flags")
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

func TestInstallOptions_BuildArgs_NoSDDModeWhenEmpty(t *testing.T) {
	opts := InstallOptions{
		AgentName: "opencode",
		SDDMode:   "",
	}
	args := opts.buildArgs(nil)

	if slices.Contains(args, "--sdd-mode") {
		t.Fatal("--sdd-mode should not be present when empty")
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

func TestInstallOptions_EffectivePersona_Default(t *testing.T) {
	opts := InstallOptions{}
	if got, want := opts.effectivePersona(), "neutral"; got != want {
		t.Fatalf("effectivePersona() = %q, want %q", got, want)
	}
}

func TestInstallOptions_EffectivePersona_Explicit(t *testing.T) {
	opts := InstallOptions{Persona: "gentleman"}
	if got, want := opts.effectivePersona(), "gentleman"; got != want {
		t.Fatalf("effectivePersona() = %q, want %q", got, want)
	}
}

// ─── SyncOptions.buildArgs ────────────────────────────────────────────────

func TestSyncOptions_BuildArgs_Empty(t *testing.T) {
	opts := SyncOptions{}
	args := opts.buildArgs()

	if !slices.Equal(args, []string{"sync"}) {
		t.Fatalf("empty sync args = %v, want [sync]", args)
	}
}

func TestSyncOptions_BuildArgs_AllFlags(t *testing.T) {
	opts := SyncOptions{
		SDDMode:       "multi",
		StrictTDD:     true,
		Profiles:      []string{"cheap:openrouter/qwen/qwen3-30b-a3b:free"},
		ProfilePhases: []string{"cheap:sdd-design:anthropic/claude-sonnet-4"},
		IncludePerms:  true,
		IncludeTheme:  true,
		DryRun:        true,
	}
	args := opts.buildArgs()

	want := []string{
		"sync",
		"--sdd-mode", "multi",
		"--strict-tdd",
		"--profile", "cheap:openrouter/qwen/qwen3-30b-a3b:free",
		"--profile-phase", "cheap:sdd-design:anthropic/claude-sonnet-4",
		"--include-permissions",
		"--include-theme",
		"--dry-run",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestSyncOptions_BuildArgs_MultipleProfiles(t *testing.T) {
	opts := SyncOptions{
		Profiles: []string{
			"cheap:openrouter/qwen/qwen3-30b-a3b:free",
			"premium:anthropic/claude-sonnet-4",
		},
	}
	args := opts.buildArgs()

	// Should have 2 --profile entries
	count := 0
	for _, arg := range args {
		if arg == "--profile" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 --profile flags, got %d", count)
	}
}

func TestSyncOptions_BuildArgs_NoStrictTDDWhenFalse(t *testing.T) {
	opts := SyncOptions{StrictTDD: false}
	args := opts.buildArgs()

	if slices.Contains(args, "--strict-tdd") {
		t.Fatal("--strict-tdd should not be present when false")
	}
}

func TestSyncOptions_BuildArgs_NoPermissionsWhenFalse(t *testing.T) {
	opts := SyncOptions{IncludePerms: false}
	args := opts.buildArgs()

	if slices.Contains(args, "--include-permissions") {
		t.Fatal("--include-permissions should not be present when false")
	}
}

func TestSyncOptions_BuildArgs_NoThemeWhenFalse(t *testing.T) {
	opts := SyncOptions{IncludeTheme: false}
	args := opts.buildArgs()

	if slices.Contains(args, "--include-theme") {
		t.Fatal("--include-theme should not be present when false")
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
