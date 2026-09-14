package envprofile

import (
	"slices"
	"strings"
	"testing"
)

func TestSavePresetCopyWithEnvOverrides(t *testing.T) {
	testRoot(t)
	ov := &ProfileOverrides{Groups: []string{"core"}, Skills: []string{}, MCP: []string{"graft"}}
	if err := SavePresetCopy("dev", "dev-lite", "trimmed dev", ov); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(Presets(), "dev-lite") {
		t.Fatalf("Presets() = %v, want dev-lite listed", Presets())
	}
	if IsBuiltinPreset("dev-lite") {
		t.Error("copy must not be builtin")
	}
	spec := mustSpec(t, Profile{Name: "x", Preset: "dev-lite"})
	if got := PresetGroups(spec); !slices.Equal(got, []string{"core"}) {
		t.Errorf("groups = %v", got)
	}
	if !PresetNone(spec, "skills") {
		t.Error("empty skills override must persist as none in the preset")
	}
	if got := PresetMCPIDs(spec); !slices.Equal(got, []string{"graft"}) {
		t.Errorf("mcp = %v", got)
	}
	if got := PresetDefaultAgent(spec); got != "orchestrator" {
		t.Errorf("non-list keys must copy from the base, default_agent = %q", got)
	}
	// An env override on top of a none-preset replaces the none flag.
	env := Profile{Name: "x", Preset: "dev-lite", Overrides: &ProfileOverrides{Skills: []string{"tdd"}}}
	if spec := mustSpec(t, env); PresetNone(spec, "skills") {
		t.Error("env skills list must clear the preset none flag")
	}

	if err := SavePresetCopy("dev", "dev-lite", "", nil); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("duplicate name err = %v", err)
	}
	if err := SavePresetCopy("dev", "qa", "", nil); err == nil {
		t.Error("a copy must never shadow a builtin")
	}
}

func TestRenameAndDeletePreset(t *testing.T) {
	testRoot(t)
	if err := SavePresetCopy("qa", "strict", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Create("lane", "strict"); err != nil {
		t.Fatal(err)
	}
	if err := RenamePreset("strict", "very-strict"); err != nil {
		t.Fatal(err)
	}
	p, err := Get("lane")
	if err != nil {
		t.Fatal(err)
	}
	if p.Preset != "very-strict" {
		t.Errorf("env preset = %q, want repointed to very-strict", p.Preset)
	}
	if _, err := Preset("strict"); err == nil {
		t.Error("old preset name must be gone after rename")
	}
	if err := DeletePreset("very-strict"); err == nil || !strings.Contains(err.Error(), "in use") {
		t.Errorf("delete in-use err = %v", err)
	}
	if err := RenamePreset("dev", "dev2"); err == nil {
		t.Error("builtin rename must fail")
	}
	if err := DeletePreset("dev"); err == nil {
		t.Error("builtin delete must fail")
	}
	if err := Delete("lane"); err != nil {
		t.Fatal(err)
	}
	if err := DeletePreset("very-strict"); err != nil {
		t.Errorf("delete unused preset: %v", err)
	}
}
