package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// ─── AvailableNames ───────────────────────────────────────────────────────

func TestAvailableNames_ContainsAll15Agents(t *testing.T) {
	names := AvailableNames()
	expected := []string{
		"opencode", "claude-code", "cursor", "windsurf",
		"gemini-cli", "vscode-copilot", "codex",
		"kilocode", "kimi", "qwen-code", "antigravity", "kiro-ide",
		"openclaw", "trae-ide", "pi",
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

	expected := []string{"opencode", "kilocode", "windsurf", "gemini-cli"}
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
	os.MkdirAll(filepath.Dir(mcpPath), 0o755)
	os.WriteFile(mcpPath, []byte("{}"), 0o644)

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
