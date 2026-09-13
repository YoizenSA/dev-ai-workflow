package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/envprofile"
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
			"/home/u/.config/opencode/ywai-plugins/background-agents.js",
		},
	})

	if n := countYwaiConfigRefs(cfg); n != 2 {
		t.Fatalf("countYwaiConfigRefs = %d, want 2", n)
	}
	if err := stripYwaiConfigRefs(cfg); err != nil {
		t.Fatalf("stripYwaiConfigRefs: %v", err)
	}

	root := readJSONFile(t, cfg)
	plugins, _ := root["plugin"].([]any)
	if len(plugins) != 1 || plugins[0] != "/home/u/.config/opencode/plugins/their-own.js" {
		t.Fatalf("foreign plugin must survive, got %v", plugins)
	}
	if _, ok := root["plugins"]; ok {
		t.Error("must not write the v2 plugins key")
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
	if _, ok := root["plugin"]; ok {
		t.Error("an emptied plugins array should be removed, not left as []")
	}
	if _, ok := root["plugins"]; ok {
		t.Error("must not write the v2 plugins key")
	}
}

func TestUninstallStripYwaiConfigRefs_DrainsV2PluginsKey(t *testing.T) {
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
	if _, ok := root["plugins"]; ok {
		t.Error("leftover v2 plugins key must be deleted")
	}
	plugins, _ := root["plugin"].([]any)
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

// countStatuslineRefs/stripStatuslineRefs remove the vendored TUI bundle entry
// from cli.json. They must strip the entry while preserving the key spelling
// the config already carried and every unrelated entry.
func TestUninstallStatuslineRefs_PreservesKeySpellingAndOthers(t *testing.T) {
	statusline := "/home/u/.config/opencode/tui-plugins/" + retiredStatuslineTuiBundle
	cases := []struct {
		name    string
		config  map[string]any
		wantKey string
		want    []any
	}{
		{
			name: "v2 plugins key only",
			config: map[string]any{
				"plugins": []any{statusline, "/home/u/other-tui.js"},
			},
			wantKey: "plugins",
			want:    []any{"/home/u/other-tui.js"},
		},
		{
			name: "v1 plugin key only",
			config: map[string]any{
				"plugin": []any{statusline, "/home/u/other-tui.js"},
			},
			wantKey: "plugin",
			want:    []any{"/home/u/other-tui.js"},
		},
		{
			name: "both keys drain under plugin",
			config: map[string]any{
				"plugin":  []any{statusline, "/home/u/legacy-keep.js"},
				"plugins": []any{"/home/u/v2-keep.js"},
			},
			wantKey: "plugin",
			want:    []any{"/home/u/legacy-keep.js", "/home/u/v2-keep.js"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.config["theme"] = "dark"
			cfg := filepath.Join(t.TempDir(), "cli.json")
			writeJSONFile(t, cfg, tc.config)

			if n := countStatuslineRefs(cfg); n != 1 {
				t.Fatalf("countStatuslineRefs = %d, want 1", n)
			}
			if err := stripStatuslineRefs(cfg); err != nil {
				t.Fatalf("stripStatuslineRefs: %v", err)
			}

			root := readJSONFile(t, cfg)
			got, ok := root[tc.wantKey].([]any)
			if !ok {
				t.Fatalf("key %q missing after strip, got %v", tc.wantKey, root)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("%s = %v, want %v", tc.wantKey, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("%s = %v, want %v", tc.wantKey, got, tc.want)
				}
			}

			// The other spelling must not be written back.
			other := "plugins"
			if tc.wantKey == "plugins" {
				other = "plugin"
			}
			if _, ok := root[other]; ok {
				t.Errorf("must not write the %q key", other)
			}
			if root["theme"] != "dark" {
				t.Errorf("unrelated key dropped: %v", root)
			}
			if containsAnyString(got, statusline) {
				t.Errorf("statusline entry survived: %v", got)
			}
		})
	}
}

// containsAnyString reports whether slice holds want as a string element.
func containsAnyString(slice []any, want string) bool {
	for _, v := range slice {
		if s, ok := v.(string); ok && s == want {
			return true
		}
	}
	return false
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
		t.Error("the legacy agent key must not be written")
	}
	if root["model"] != "keep-me" {
		t.Error("unrelated keys must be preserved")
	}
}

func TestUninstallStripYwaiAgentKeys_MergesLegacyAgentKey(t *testing.T) {
	// The merged survivors land back under `agents`, and the legacy `agent`
	// key must not coexist with it.
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
	if _, ok := root["agent"]; ok {
		t.Error("legacy agent key must be deleted on a v2 host")
	}
}

func TestInstallsAgentsAsJSONKeys_OnlyOpenCodeFormats(t *testing.T) {
	// Guards against widening the JSON-key deletion to agents ywai never
	// installs into that way — gemini-cli ships its own "agent" object.
	if !installsAgentsAsJSONKeys("opencode") {
		t.Error("opencode should use the JSON-key install path")
	}
	for _, name := range []string{"kilocode", "gemini-cli", "windsurf", "claude-code", "pi", "omp", "cursor", "codex"} {
		if installsAgentsAsJSONKeys(name) {
			t.Errorf("%s must not have its config's agent object touched", name)
		}
	}
}

func TestProfileDirsFor_FileDirs(t *testing.T) {
	if dirs := profileDirsFor("opencode", "/home/u"); len(dirs) != 1 {
		t.Errorf("opencode should have exactly one profile directory, got %v", dirs)
	}
	// profileDirsFor joins with the platform separator, so the expected value
	// must be built the same way: the fake home plus .omp/agent/agents.
	want := filepath.Join("/home/u", ".omp", "agent", "agents")
	if dirs := profileDirsFor("omp", "/home/u"); len(dirs) != 1 || dirs[0] != want {
		t.Errorf("omp profile dir = %v, want [%s]", dirs, want)
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

// --- sessions survive uninstall ---

// makeEnvWithDB creates a real profile (temp profiles root required) holding a
// fake session database with sidecars, the shape opencode leaves on disk.
func makeEnvWithDB(t *testing.T, name string) envprofile.Profile {
	t.Helper()
	p, err := envprofile.Create(name, "dev")
	if err != nil {
		t.Fatalf("create env %s: %v", name, err)
	}
	dbDir := filepath.Join(envprofile.Dirs(p)["data"], "opencode")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"opencode.db", "opencode.db-wal", "opencode.db-shm"} {
		if err := os.WriteFile(filepath.Join(dbDir, f), []byte("db"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func TestArchiveEnvSessionDBInto_MovesDBAndSidecars(t *testing.T) {
	envprofile.SetProfilesRootForTest(t.TempDir())
	p := makeEnvWithDB(t, "dev")
	base := t.TempDir()

	dst, err := archiveEnvSessionDBInto(base, p)
	if err != nil || dst == "" {
		t.Fatalf("archive: dst=%q err=%v", dst, err)
	}
	for _, f := range []string{"opencode.db", "opencode.db-wal", "opencode.db-shm"} {
		if _, err := os.Stat(filepath.Join(dst, f)); err != nil {
			t.Errorf("%s not archived: %v", f, err)
		}
	}
	if _, err := os.Stat(envSessionDBPath(p)); !os.IsNotExist(err) {
		t.Error("the original database must be gone after archiving")
	}
}

func TestArchiveEnvSessionDBInto_NoDatabaseIsNoOp(t *testing.T) {
	envprofile.SetProfilesRootForTest(t.TempDir())
	p, err := envprofile.Create("dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()

	dst, err := archiveEnvSessionDBInto(base, p)
	if err != nil || dst != "" {
		t.Fatalf("an env without a database has nothing to keep, got dst=%q err=%v", dst, err)
	}
	if entries, _ := os.ReadDir(base); len(entries) != 0 {
		t.Errorf("no archive directory should be created, got %v", entries)
	}
}

func TestPurgeRemovals_KeepsSessionsThenPurges(t *testing.T) {
	dataDir := t.TempDir()
	envprofile.SetProfilesRootForTest(filepath.Join(dataDir, "profiles"))
	p := makeEnvWithDB(t, "dev")
	base := t.TempDir()

	removals := purgeRemovals(dataDir, base, false, []envprofile.Profile{p})
	if len(removals) != 3 {
		t.Fatalf("want stop, archive and purge steps, got %d", len(removals))
	}
	// Order is the guarantee: archive before the directory removal.
	if removals[1].kind != kindData || removals[2].kind != kindData {
		t.Fatalf("expected the two data steps last, got %s then %s", removals[1].label, removals[2].label)
	}
	for _, r := range removals {
		if err := r.apply(); err != nil {
			t.Fatalf("step %q: %v", r.label, err)
		}
	}

	hits, err := filepath.Glob(filepath.Join(base, "dev-*", "opencode.db"))
	if err != nil || len(hits) != 1 {
		t.Fatalf("archived session db not found: hits=%v err=%v", hits, err)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("data directory still present after purge: %v", err)
	}
}

func TestPurgeRemovals_RefusesPurgeWhenArchiveFails(t *testing.T) {
	dataDir := t.TempDir()
	envprofile.SetProfilesRootForTest(filepath.Join(dataDir, "profiles"))
	p := makeEnvWithDB(t, "qa")
	// A file where the archive directory must be created makes every move fail.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	removals := purgeRemovals(dataDir, blocker, false, []envprofile.Profile{p})
	if err := removals[0].apply(); err != nil {
		t.Fatalf("stop step: %v", err)
	}
	if err := removals[1].apply(); err == nil {
		t.Fatal("archive step should have failed")
	}
	if err := removals[2].apply(); err == nil {
		t.Fatal("purge must refuse while sessions are un-archived")
	}
	if _, err := os.Stat(envSessionDBPath(p)); err != nil {
		t.Fatalf("the session database must survive a failed archive: %v", err)
	}
}

func TestPurgeRemovals_DiscardDeletesSessions(t *testing.T) {
	dataDir := t.TempDir()
	envprofile.SetProfilesRootForTest(filepath.Join(dataDir, "profiles"))
	p := makeEnvWithDB(t, "dev")
	base := t.TempDir()

	removals := purgeRemovals(dataDir, base, true, []envprofile.Profile{p})
	if len(removals) != 2 {
		t.Fatalf("discard must skip the archive step, got %d steps", len(removals))
	}
	for _, r := range removals {
		if err := r.apply(); err != nil {
			t.Fatalf("step %q: %v", r.label, err)
		}
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("data directory still present: %v", err)
	}
}

func TestPurgeRemovals_GuardsManifestlessEnvs(t *testing.T) {
	// A directory without a manifest (crash mid-create, manual tampering) must
	// still block the purge: the tree scan, not the manifest list, decides.
	dataDir := t.TempDir()
	envprofile.SetProfilesRootForTest(filepath.Join(dataDir, "profiles"))
	orphanDB := filepath.Join(dataDir, "profiles", "orphan", "data", "opencode")
	if err := os.MkdirAll(orphanDB, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphanDB, "opencode.db"), []byte("db"), 0o644); err != nil {
		t.Fatal(err)
	}

	removals := purgeRemovals(dataDir, t.TempDir(), false, nil)
	if len(removals) != 1 {
		t.Fatalf("no environments means a single purge step, got %d", len(removals))
	}
	if err := removals[0].apply(); err == nil {
		t.Fatal("purge must refuse while an orphaned session database remains")
	}
	if _, err := os.Stat(filepath.Join(orphanDB, "opencode.db")); err != nil {
		t.Fatalf("orphaned sessions must survive: %v", err)
	}
}

func TestParseSelection_NumbersRangesAndErrors(t *testing.T) {
	nums, err := parseSelection("1 3", 5)
	if err != nil || len(nums) != 2 || nums[0] != 1 || nums[1] != 3 {
		t.Fatalf("parseSelection(\"1 3\") = %v, %v", nums, err)
	}
	nums, err = parseSelection("2,4", 5)
	if err != nil || len(nums) != 2 || nums[0] != 2 || nums[1] != 4 {
		t.Fatalf("parseSelection(\"2,4\") = %v, %v", nums, err)
	}
	nums, err = parseSelection("2-4", 5)
	if err != nil || len(nums) != 3 || nums[0] != 2 || nums[2] != 4 {
		t.Fatalf("parseSelection(\"2-4\") = %v, %v", nums, err)
	}
	for _, bad := range []string{"0", "6", "x", "3-x", ""} {
		if _, err := parseSelection(bad, 5); err == nil {
			t.Errorf("parseSelection(%q) must fail", bad)
		}
	}
}

func TestRunUninstallProfile_ArchivesSessionDB(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	envprofile.SetProfilesRootForTest(t.TempDir())
	makeEnvWithDB(t, "dev")

	if err := runUninstallProfile("dev", false, true, false); err != nil {
		t.Fatalf("runUninstallProfile: %v", err)
	}
	if envprofile.Exists("dev") {
		t.Error("environment must be gone")
	}
	hits, err := filepath.Glob(filepath.Join(home, removedSessionsDirName, "dev-*", "opencode.db"))
	if err != nil || len(hits) != 1 {
		t.Fatalf("session database not archived under the home: hits=%v err=%v", hits, err)
	}
}
