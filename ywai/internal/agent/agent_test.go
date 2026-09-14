package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// ─── AvailableNames ───────────────────────────────────────────────────────

func TestAvailableNames_ContainsAllKnownAgents(t *testing.T) {
	names := AvailableNames()
	expected := []string{
		"opencode", "claude-code", "cursor", "windsurf",
		"gemini-cli", "vscode-copilot", "codex",
		"kimi", "qwen-code", "antigravity", "kiro-ide",
		"openclaw", "trae-ide", "pi", "omp",
	}

	if len(names) != len(expected) {
		t.Fatalf("AvailableNames() returned %d agents, want %d", len(names), len(expected))
	}

	for _, name := range expected {
		if !slices.Contains(names, name) {
			t.Fatalf("AvailableNames() missing agent %q", name)
		}
	}
}

func TestKnownAgents_OpenCodeBinaryIsOpenCode2(t *testing.T) {
	for _, ka := range KnownAgents {
		if ka.Name != "opencode" {
			continue
		}
		if ka.Binary != "opencode2" {
			t.Fatalf("opencode Binary = %q, want opencode2 (OpenCode v2; no v1 fallback)", ka.Binary)
		}
		return
	}
	t.Fatal("opencode not found in KnownAgents")
}

func TestDetect_PrefersOpenCode2Binary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("OPENCODE_CONFIG_DIR", "")

	binDir := t.TempDir()
	v1 := filepath.Join(binDir, "opencode")
	v2 := filepath.Join(binDir, "opencode2")
	if runtime.GOOS == "windows" {
		v1 += ".exe"
		v2 += ".exe"
	}
	if err := os.WriteFile(v1, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v2, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	found := Detect()
	var oc *Agent
	for i := range found {
		if found[i].Name == "opencode" {
			oc = &found[i]
			break
		}
	}
	if oc == nil {
		t.Fatal("expected opencode agent when opencode2 is on PATH")
	}
	if oc.BinaryName != v2 {
		t.Fatalf("opencode BinaryName = %q, want %q (must prefer opencode2, not v1 opencode)", oc.BinaryName, v2)
	}
}

func TestAvailableNames_NoDuplicates(t *testing.T) {
	names := AvailableNames()
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Fatalf("duplicate agent name: %q", n)
		}
		seen[n] = true
	}
}

// ─── KnownAgents paths ───────────────────────────────────────────────────

func TestKnownAgents_WindsurfUsesCodeiumPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	for _, ka := range KnownAgents {
		if ka.Name != "windsurf" {
			continue
		}
		got := ka.SkillsPath()
		want := filepath.Join(home, ".codeium", "windsurf", "skills")
		if got != want {
			t.Fatalf("windsurf SkillsPath = %q, want %q", got, want)
		}
		return
	}
	t.Fatal("windsurf not found in KnownAgents")
}

func TestKnownAgents_KimiUsesHomeKimi(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	for _, ka := range KnownAgents {
		if ka.Name != "kimi" {
			continue
		}
		got := ka.SkillsPath()
		want := filepath.Join(home, ".kimi", "skills")
		if got != want {
			t.Fatalf("kimi SkillsPath = %q, want %q", got, want)
		}
		return
	}
	t.Fatal("kimi not found in KnownAgents")
}

func TestKnownAgents_OpenClawExists(t *testing.T) {
	found := false
	for _, ka := range KnownAgents {
		if ka.Name == "openclaw" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("openclaw not found in KnownAgents")
	}
}

func TestKnownAgents_TraeExists(t *testing.T) {
	found := false
	for _, ka := range KnownAgents {
		if ka.Name == "trae-ide" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("trae-ide not found in KnownAgents")
	}
}

func TestKnownAgents_PiExists(t *testing.T) {
	found := false
	for _, ka := range KnownAgents {
		if ka.Name == "pi" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("pi not found in KnownAgents")
	}
}

func TestKnownAgents_OmpExists(t *testing.T) {
	for _, ka := range KnownAgents {
		if ka.Name == "omp" {
			if ka.Binary != "omp" {
				t.Fatalf("omp binary = %q, want omp", ka.Binary)
			}
			return
		}
	}
	t.Fatal("omp not found in KnownAgents")
}

func TestKnownAgents_EachHasSkillsPath(t *testing.T) {
	for _, ka := range KnownAgents {
		path := ka.SkillsPath()
		if path == "" {
			t.Fatalf("agent %q has empty SkillsPath", ka.Name)
		}
	}
}

func TestKnownAgents_EachNameIsInAvailableNames(t *testing.T) {
	names := AvailableNames()
	for _, ka := range KnownAgents {
		if !slices.Contains(names, ka.Name) {
			t.Fatalf("agent %q in KnownAgents but not in AvailableNames", ka.Name)
		}
	}
}

// ─── Desktop agents (no binary) ───────────────────────────────────────────

func TestKnownAgents_DesktopAgentsHaveEmptyBinary(t *testing.T) {
	desktopAgents := map[string]bool{
		"windsurf":    true,
		"antigravity": true,
		"trae-ide":    true,
	}

	for _, ka := range KnownAgents {
		if desktopAgents[ka.Name] && ka.Binary != "" {
			t.Fatalf("desktop agent %q should have empty Binary, got %q", ka.Name, ka.Binary)
		}
	}
}

// ─── SettingsPaths ────────────────────────────────────────────────────────

func TestSettingsPaths_ReturnsMap(t *testing.T) {
	paths := SettingsPaths()
	if paths == nil {
		t.Fatal("SettingsPaths() returned nil")
	}

	expected := []string{"opencode", "windsurf", "gemini-cli", "pi"}
	for _, name := range expected {
		if _, ok := paths[name]; !ok {
			t.Fatalf("SettingsPaths() missing %q", name)
		}
	}
}

func TestSettingsPaths_OpenCodePrefersJSONC(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "") // CI runners export it; it outranks HOME

	// Without any file, should fall back to .json
	paths := SettingsPaths()
	want := filepath.Join(home, ".config", "opencode", "opencode.json")
	if paths["opencode"] != want {
		t.Fatalf("opencode path = %q, want %q", paths["opencode"], want)
	}

	// Create .jsonc — should be preferred
	jsonc := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	if err := os.MkdirAll(filepath.Dir(jsonc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonc, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths = SettingsPaths()
	wantJSONC := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	if paths["opencode"] != wantJSONC {
		t.Fatalf("opencode path with .jsonc = %q, want %q", paths["opencode"], wantJSONC)
	}
}

func TestSettingsPaths_WindsurfReturnsEmptyIfNotExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	paths := SettingsPaths()
	if paths["windsurf"] != "" {
		t.Fatalf("windsurf should be empty when mcp_config.json doesn't exist, got %q", paths["windsurf"])
	}
}

func TestSettingsPaths_WindsurfReturnsPathIfExists(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses unix paths")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)

	mcpPath := filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")
	_ = os.MkdirAll(filepath.Dir(mcpPath), 0o755)
	if err := os.WriteFile(mcpPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write mcp_config.json: %v", err)
	}

	paths := SettingsPaths()
	want := mcpPath
	if paths["windsurf"] != want {
		t.Fatalf("windsurf path = %q, want %q", paths["windsurf"], want)
	}
}

// ─── Detect ───────────────────────────────────────────────────────────────

func TestDetect_FindsAgentByConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Create windsurf config dir + skills dir (desktop app, detected by config dir)
	windsurfDir := filepath.Join(home, ".codeium", "windsurf")
	skillsDir := filepath.Join(windsurfDir, "skills")
	os.MkdirAll(skillsDir, 0o755)

	agents := Detect()
	found := false
	for _, a := range agents {
		if a.Name == "windsurf" {
			found = true
			if a.SkillsDir != filepath.Join(windsurfDir, "skills") {
				t.Fatalf("windsurf skills = %q, want %q", a.SkillsDir, filepath.Join(windsurfDir, "skills"))
			}
			break
		}
	}
	if !found {
		t.Fatal("windsurf not detected via config dir")
	}
}

// ─── FindByName ───────────────────────────────────────────────────────────

func TestFindByName_ReturnsErrorForUnknownAgent(t *testing.T) {
	_, err := FindByName("nonexistent-agent")
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

// ─── FindOpenCode ──────────────────────────────────────────────────────────

// fakeBinName returns the file name a fake executable must have so
// exec.LookPath resolves it: on Windows the extension is required (PATHEXT),
// on POSIX a shebang script with the exact name is enough.
func fakeBinName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".bat"
	}
	return name
}

// writeFakeBin materializes a fake executable in dir. The content is never
// executed by these tests — LookPath only checks existence — but each GOOS
// gets the file shape it can actually resolve.
func writeFakeBin(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, fakeBinName(name))
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFindOpenCode_ResolvesOpencode2(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	writeFakeBin(t, dir, "opencode2")
	// HOME + USERPROFILE override keeps real well-known dirs (~/.opencode/bin)
	// out of the resolution so the test exercises PATH injection only.
	// USERPROFILE matters on Windows, where os.UserHomeDir reads it, not HOME.
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PATH", dir)

	path, bin := FindOpenCode()
	if bin != "opencode2" {
		t.Fatalf("FindOpenCode bin = %q, want opencode2 (the only supported host)", bin)
	}
	if filepath.Base(path) != fakeBinName("opencode2") {
		t.Fatalf("FindOpenCode path = %q, want an opencode2 binary", path)
	}
}

// GateOpenCodeV2 is the ADR-0001 minimum-version gate: a machine with only the
// retired v1 `opencode` binary must get the withdrawal notice, not silent v2
// config writes.
func TestGateOpenCodeV2_ErrorsOnV1Only(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	writeFakeBin(t, dir, "opencode")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PATH", dir)

	err := GateOpenCodeV2()
	if err == nil {
		t.Fatal("v1-only machine must be gated with an error")
	}
	for _, want := range []string{"OpenCode 2", "opencode2", "0001-drop-opencode-v1-support"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("gate error must mention %q, got: %v", want, err)
		}
	}
}

func TestGateOpenCodeV2_PassesOnV2(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	writeFakeBin(t, dir, "opencode2")
	writeFakeBin(t, dir, "opencode")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PATH", dir)

	if err := GateOpenCodeV2(); err != nil {
		t.Fatalf("GateOpenCodeV2() = %v, want nil when opencode2 is installed", err)
	}
}

func TestGateOpenCodeV2_NoBinaryPasses(t *testing.T) {
	// Neither binary installed: not a gate error — the caller reports its own
	// not-found error.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PATH", t.TempDir())

	if err := GateOpenCodeV2(); err != nil {
		t.Fatalf("GateOpenCodeV2() = %v, want nil when neither binary exists", err)
	}
}

func TestOpenCodeBinaryNameNeverEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir reads this on Windows
	t.Setenv("PATH", t.TempDir()) // binary not installed
	if got := OpenCodeBinaryName(); got == "" {
		t.Error("callers that only need a name must never get an empty string")
	}
	_ = os.Getenv("PATH")
}

func TestFindOpenCode_Missing(t *testing.T) {
	// Neutralize PATH, HOME and USERPROFILE so neither PATH lookup nor the
	// well-known install dirs can see a really installed binary on any GOOS.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PATH", t.TempDir())

	path, bin := FindOpenCode()
	if path != "" || bin != "" {
		t.Fatalf("FindOpenCode() = (%q, %q), want empty when neither binary exists", path, bin)
	}
}
