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

	// minimal preset → skills only (no engram)
	want := []string{
		"install", "--agent", "claude-code",
		"--scope", "workspace",
		"--component", "skills",
		"--dry-run",
	}
	if !slices.Equal(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestPlanForPreset(t *testing.T) {
	t.Parallel()
	tests := []struct {
		preset        string
		wantEngram    bool
		wantEcosystem []string
	}{
		{"", true, []string{"skills", "context7", "permissions"}},
		{"full-gentleman", true, []string{"skills", "context7", "permissions"}},
		{"ecosystem-only", true, []string{"skills", "context7", "permissions"}},
		{"minimal", false, []string{"skills"}},
	}
	for _, tt := range tests {
		t.Run(tt.preset, func(t *testing.T) {
			t.Parallel()
			p := PlanForPreset(tt.preset)
			if p.IncludeEngram != tt.wantEngram {
				t.Fatalf("IncludeEngram = %v, want %v", p.IncludeEngram, tt.wantEngram)
			}
			if !slices.Equal(p.Ecosystem, tt.wantEcosystem) {
				t.Fatalf("Ecosystem = %v, want %v", p.Ecosystem, tt.wantEcosystem)
			}
		})
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

func TestInstallOptions_OptionalComponents(t *testing.T) {
	t.Parallel()

	off := InstallOptions{}
	if off.HasOptionalComponents() {
		t.Fatal("defaults should not install optional components")
	}
	if len(off.optionalComponents()) != 0 {
		t.Fatalf("optionalComponents = %v, want empty", off.optionalComponents())
	}

	sdd := InstallOptions{InstallSDD: true, SDDMode: "single"}
	if !sdd.HasOptionalComponents() {
		t.Fatal("expected optional SDD")
	}
	got := sdd.optionalComponents()
	want := []string{"sdd"}
	if !slices.Equal(got, want) {
		t.Fatalf("optionalComponents = %v, want %v", got, want)
	}
	if sdd.EffectiveSDDMode() != "single" {
		t.Fatalf("EffectiveSDDMode = %q, want single", sdd.EffectiveSDDMode())
	}
	defMode := InstallOptions{InstallSDD: true}.EffectiveSDDMode()
	if defMode != "multi" {
		t.Fatalf("default SDD mode = %q, want multi", defMode)
	}
}

func TestInstallOptions_BuildArgs_BaseNeverIncludesSDDOrPersona(t *testing.T) {
	t.Parallel()
	opts := InstallOptions{
		AgentName:  "opencode",
		InstallSDD: true,
	}
	args := opts.buildArgs(nil)
	for _, bad := range []string{"sdd", "persona"} {
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--component" && args[i+1] == bad {
				t.Fatalf("base buildArgs must not include %s: %v", bad, args)
			}
		}
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
