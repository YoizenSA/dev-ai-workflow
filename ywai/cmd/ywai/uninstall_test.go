package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// uninstall deletes files. Every predicate below decides whether something is
// ywai's or the user's, so each one is pinned against the "looks like ours but
// is not" case — a false positive here is data loss.

func writeJSONFile(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return out
}

func TestYwaiSkillsIn_OnlyClaimsMarkedCopies(t *testing.T) {
	skillsDir := t.TempDir()

	// ywai's: a copied directory carrying the marker.
	ours := filepath.Join(skillsDir, "docker")
	if err := os.MkdirAll(ours, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ours, ywaiSkillMarker), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// The user's: same shape, no marker. Must survive.
	theirs := filepath.Join(skillsDir, "my-skill")
	if err := os.MkdirAll(theirs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(theirs, "SKILL.md"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The dangerous case: the user wrote a skill with a name ywai also ships.
	collision := filepath.Join(skillsDir, "tdd")
	if err := os.MkdirAll(collision, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ywaiSkillsIn(skillsDir)
	if len(got) != 1 || filepath.Base(got[0]) != "docker" {
		t.Fatalf("expected only the marked skill, got %v", got)
	}
}

func TestYwaiSkillsIn_IgnoresLinksOutsideYwai(t *testing.T) {
	skillsDir := t.TempDir()
	elsewhere := t.TempDir()

	link := filepath.Join(skillsDir, "external")
	if err := os.Symlink(elsewhere, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if got := ywaiSkillsIn(skillsDir); len(got) != 0 {
		t.Fatalf("a link outside ywai's skills dir must not be claimed, got %v", got)
	}
}

func TestUninstallStripYwaiConfigRefs_KeepsForeignPlugins(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "opencode.json")
	writeJSONFile(t, cfg, map[string]any{
		"model": "provider/model",
		"plugins": []any{
			"/home/u/.config/opencode/ywai-plugins/vision-bridge.js",
			"/home/u/.config/opencode/plugins/their-own.js",
			"/home/u/.config/opencode/ywai-plugins/background-agents-v2.js",
		},
	})

	if n := countYwaiConfigRefs(cfg); n != 2 {
		t.Fatalf("countYwaiConfigRefs = %d, want 2", n)
	}
	if err := stripYwaiConfigRefs(cfg); err != nil {
		t.Fatalf("stripYwaiConfigRefs: %v", err)
	}

	root := readJSONFile(t, cfg)
	plugins, _ := root["plugins"].([]any)
	if len(plugins) != 1 || plugins[0] != "/home/u/.config/opencode/plugins/their-own.js" {
		t.Fatalf("foreign plugin must survive, got %v", plugins)
	}
	if _, ok := root["plugin"]; ok {
		t.Error("must not write v1 plugin key")
	}
	if root["model"] != "provider/model" {
		t.Errorf("unrelated keys must be preserved, got %v", root["model"])
	}
}

func TestUninstallStripYwaiConfigRefs_DropsEmptyArray(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "opencode.json")
	writeJSONFile(t, cfg, map[string]any{
		"plugins": []any{"/x/ywai-plugins/vision-bridge.js"},
	})

	if err := stripYwaiConfigRefs(cfg); err != nil {
		t.Fatalf("stripYwaiConfigRefs: %v", err)
	}
	root := readJSONFile(t, cfg)
	if _, ok := root["plugins"]; ok {
		t.Error("an emptied plugins array should be removed, not left as []")
	}
	if _, ok := root["plugin"]; ok {
		t.Error("must not write v1 plugin key")
	}
}

func TestUninstallStripYwaiConfigRefs_DrainsLegacyPluginKey(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "opencode.json")
	writeJSONFile(t, cfg, map[string]any{
		"plugins": []any{
			"/home/u/.config/opencode/plugins/their-own.js",
		},
		"plugin": []any{
			"/home/u/.config/opencode/ywai-plugins/vision-bridge.js",
			"/home/u/.config/opencode/plugins/legacy-keep.js",
		},
	})

	if n := countYwaiConfigRefs(cfg); n != 1 {
		t.Fatalf("countYwaiConfigRefs = %d, want 1 (legacy ywai entry)", n)
	}
	if err := stripYwaiConfigRefs(cfg); err != nil {
		t.Fatalf("stripYwaiConfigRefs: %v", err)
	}

	root := readJSONFile(t, cfg)
	if _, ok := root["plugin"]; ok {
		t.Error("leftover v1 plugin key must be deleted")
	}
	plugins, _ := root["plugins"].([]any)
	if len(plugins) != 2 {
		t.Fatalf("v2 plugins must keep foreign + drained leftover, got %v", plugins)
	}
	got := map[string]bool{}
	for _, v := range plugins {
		s, _ := v.(string)
		got[s] = true
	}
	if !got["/home/u/.config/opencode/plugins/their-own.js"] || !got["/home/u/.config/opencode/plugins/legacy-keep.js"] {
		t.Fatalf("expected both foreign plugin paths, got %v", plugins)
	}
}

func TestUninstallStripYwaiAgentKeys_KeepsUserAgents(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "opencode.json")
	writeJSONFile(t, cfg, map[string]any{
		"agents": map[string]any{
			"dev":          map[string]any{"mode": "subagent"},
			"orchestrator": map[string]any{"mode": "primary"},
			"my-agent":     map[string]any{"mode": "primary"},
		},
		"model": "keep-me",
	})
	owned := map[string]bool{"dev": true, "orchestrator": true}

	if keys := ywaiAgentKeysWith(cfg, owned); len(keys) != 2 {
		t.Fatalf("ywaiAgentKeysWith = %v, want 2 owned keys", keys)
	}
	if err := stripYwaiAgentKeysWith(cfg, owned); err != nil {
		t.Fatalf("stripYwaiAgentKeysWith: %v", err)
	}

	root := readJSONFile(t, cfg)
	agents, _ := root["agents"].(map[string]any)
	if len(agents) != 1 {
		t.Fatalf("expected only the user's agent to remain, got %v", agents)
	}
	if _, ok := agents["my-agent"]; !ok {
		t.Error("the user's agent must survive")
	}
	if _, ok := root["agent"]; ok {
		t.Error("must not write v1 agent key")
	}
	if root["model"] != "keep-me" {
		t.Error("unrelated keys must be preserved")
	}
}

func TestUninstallStripYwaiAgentKeys_DrainsLegacyAgentKey(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "opencode.json")
	writeJSONFile(t, cfg, map[string]any{
		"agents": map[string]any{
			"dev":      map[string]any{"mode": "subagent"},
			"my-agent": map[string]any{"mode": "primary"},
		},
		"agent": map[string]any{
			"orchestrator": map[string]any{"mode": "primary"},
			"legacy-keep":  map[string]any{"mode": "primary"},
		},
	})
	owned := map[string]bool{"dev": true, "orchestrator": true}

	if keys := ywaiAgentKeysWith(cfg, owned); len(keys) != 2 {
		t.Fatalf("ywaiAgentKeysWith = %v, want 2 owned keys across v1+v2", keys)
	}
	if err := stripYwaiAgentKeysWith(cfg, owned); err != nil {
		t.Fatalf("stripYwaiAgentKeysWith: %v", err)
	}

	root := readJSONFile(t, cfg)
	if _, ok := root["agent"]; ok {
		t.Error("leftover v1 agent key must be deleted")
	}
	agents, _ := root["agents"].(map[string]any)
	if len(agents) != 2 {
		t.Fatalf("expected user agents from both keys, got %v", agents)
	}
	if _, ok := agents["my-agent"]; !ok {
		t.Error("v2 user agent must survive")
	}
	if _, ok := agents["legacy-keep"]; !ok {
		t.Error("drained leftover user agent must survive under agents")
	}
}

func TestInstallsAgentsAsJSONKeys_OnlyOpenCodeFormats(t *testing.T) {
	// Guards against widening the JSON-key deletion to agents ywai never
	// installs into that way — gemini-cli ships its own "agent" object.
	for _, name := range []string{"opencode", "kilocode"} {
		if !installsAgentsAsJSONKeys(name) {
			t.Errorf("%s should use the JSON-key install path", name)
		}
	}
	for _, name := range []string{"gemini-cli", "windsurf", "claude-code", "pi", "omp", "cursor", "codex"} {
		if installsAgentsAsJSONKeys(name) {
			t.Errorf("%s must not have its config's agent object touched", name)
		}
	}
}

func TestProfileDirsFor_KilocodeHasNoFileDir(t *testing.T) {
	// kilocode installs profiles as JSON keys; returning opencode's agents dir
	// here would delete opencode's profiles twice and report them as kilocode's.
	if dirs := profileDirsFor("kilocode", "/home/u"); len(dirs) != 0 {
		t.Errorf("kilocode should have no profile directory, got %v", dirs)
	}
	if dirs := profileDirsFor("opencode", "/home/u"); len(dirs) != 1 {
		t.Errorf("opencode should have exactly one profile directory, got %v", dirs)
	}
	if dirs := profileDirsFor("omp", "/home/u"); len(dirs) != 1 ||
		dirs[0] != "/home/u/.omp/agent/agents" {
		t.Errorf("omp profile dir = %v, want [~/.omp/agent/agents]", dirs)
	}
}

func TestYwaiProfileFilesIn_IgnoresUnknownAgents(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"dev.md", "my-own-agent.md", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// Without a resolvable ywai source dir the function must claim nothing
	// rather than guess — failing closed is the safe direction here.
	for _, path := range ywaiProfileFilesIn(dir) {
		if filepath.Base(path) == "my-own-agent.md" || filepath.Base(path) == "notes.txt" {
			t.Errorf("must not claim %s", path)
		}
	}
}
