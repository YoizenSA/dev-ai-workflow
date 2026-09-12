package envprofile

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func testRoot(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Cleanup(func() { SetProfilesRootForTest("") })
	SetProfilesRootForTest(dir)
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"dev", "qa", "personal", "dev1", "my-profile"} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "Dev", "a/b", "..", "install", "profile", "env", "eval", "clean", "a_very_long_profile_name_over_32_chars_xx"} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", bad)
		}
	}
}

func TestCreateListDelete(t *testing.T) {
	testRoot(t)
	a, err := Create("dev", "dev")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.Port < 5800 || a.Port > 5899 {
		t.Errorf("Port = %d, want 5800-5899", a.Port)
	}
	for _, d := range []string{"agents", "commands", "skills"} {
		if st, err := os.Stat(filepath.Join(ProfilesDir(), "dev", "config", "opencode", d)); err != nil || !st.IsDir() {
			t.Errorf("config subdir %s missing: %v", d, err)
		}
	}
	b, err := Create("qa", "")
	if err != nil {
		t.Fatalf("Create qa: %v", err)
	}
	if b.Preset != "dev" {
		t.Errorf("Preset = %q, want default dev", b.Preset)
	}
	if a.Port == b.Port {
		t.Errorf("both profiles got port %d", a.Port)
	}
	if _, err := Create("dev", ""); err == nil {
		t.Errorf("duplicate Create = nil, want error")
	}
	list, err := List()
	if err != nil || len(list) != 2 || list[0].Name != "dev" || list[1].Name != "qa" {
		t.Fatalf("List = %v, %v", list, err)
	}
	if !Exists("dev") || Exists("nope") {
		t.Errorf("Exists mismatch")
	}
	if err := Delete("qa"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if Exists("qa") {
		t.Errorf("qa still exists after Delete")
	}
	if err := Delete("../escape"); err == nil {
		t.Errorf("Delete traversal = nil, want error")
	}
}

func TestEnvIsolation(t *testing.T) {
	testRoot(t)
	p, err := Create("dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	env := Env(p)
	for _, k := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "TMPDIR", "OPENCODE_CONFIG_DIR", "OPENCODE_DB", "OPENCODE_URL", "YWAI_PROFILE"} {
		if env[k] == "" {
			t.Errorf("Env missing %s", k)
		}
	}
	if env["YWAI_PROFILE"] != "dev" {
		t.Errorf("YWAI_PROFILE = %q", env["YWAI_PROFILE"])
	}
}

func TestPresetSpecMergesOverrides(t *testing.T) {
	testRoot(t)
	p, err := Create("dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	base, err := PresetSpec(p)
	if err != nil {
		t.Fatal(err)
	}
	baseGroups := PresetGroups(base)
	if len(baseGroups) == 0 {
		t.Skip("dev preset has no groups to override")
	}
	p.Overrides = &ProfileOverrides{Groups: []string{"core"}, Skills: []string{}, MCP: nil}
	merged, err := PresetSpec(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := PresetGroups(merged); len(got) != 1 || got[0] != "core" {
		t.Errorf("groups = %v, want [core]", got)
	}
	if got := PresetSkills(merged); got != nil {
		t.Errorf("skills = %v, want nil list", got)
	}
	if !PresetNone(merged, "skills") {
		t.Errorf("empty skills override must flag install-none")
	}
	if PresetNone(merged, "mcp") {
		t.Errorf("nil mcp override must inherit, not flag none")
	}
	p.Overrides.Groups = []string{}
	if got := PresetGroups(mustSpec(t, p)); len(got) != 1 || got[0] != "core" {
		t.Errorf("empty groups override = %v, want [core]", got)
	}
	p.Overrides.Groups = []string{"core"}
	// Manifest roundtrip keeps overrides.
	if err := p.SaveManifest(profileDir(p.Name)); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Get(p.Name)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Overrides == nil || len(reloaded.Overrides.Groups) != 1 {
		t.Errorf("overrides did not survive roundtrip: %+v", reloaded.Overrides)
	}
	if reloaded.Overrides != nil && (reloaded.Overrides.Skills == nil || reloaded.Overrides.MCP != nil) {
		t.Errorf("empty vs nil lists lost in roundtrip: %+v", reloaded.Overrides)
	}
}

func mustSpec(t *testing.T, p Profile) map[string]any {
	t.Helper()
	spec, err := PresetSpec(p)
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func TestPresets(t *testing.T) {
	names := Presets()
	for _, want := range []string{"code-review", "dev", "dev-cheap", "devops", "personal", "qa"} {
		if !slices.Contains(names, want) {
			t.Fatalf("presets = %v, missing %q", names, want)
		}
	}
	if _, err := Preset("qa"); err != nil {
		t.Fatalf("Preset(qa): %v", err)
	}
	if _, err := Preset("nope"); err == nil {
		t.Errorf("Preset(nope) = nil, want error")
	}
}
