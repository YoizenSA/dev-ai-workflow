package envprofile

import (
	"slices"
	"strings"
	"testing"
)

// coreAgents mirrors agents/groups.json "core": a preset default_agent must
// be one of them, since every non-bare preset installs core.
var coreAgents = []string{"orchestrator", "ask", "dev", "qa", "architect", "designer", "advisor", "reviewer", "devops", "finder", "memory", "planning"}

// Builtin presets are data shipped to every user: a typo in an MCP id, agent
// or model only shows up as a silently skipped install, so pin their shape.
func TestBuiltinPresetsAreConsistent(t *testing.T) {
	mcp := FilterableMCPIDs()
	for _, name := range Presets() {
		if !IsBuiltinPreset(name) {
			continue
		}
		spec, err := Preset(name)
		if err != nil {
			t.Fatal(err)
		}
		if spec["name"] != name {
			t.Errorf("%s: name field = %v", name, spec["name"])
		}
		if d, _ := spec["description"].(string); strings.TrimSpace(d) == "" {
			t.Errorf("%s: empty description", name)
		}
		for _, id := range PresetMCPIDs(spec) {
			if !slices.Contains(mcp, id) {
				t.Errorf("%s: mcp %q is not filterable (%v)", name, id, mcp)
			}
		}
		if IsBareSpec(spec) {
			continue
		}
		if a := PresetDefaultAgent(spec); a != "" && !slices.Contains(coreAgents, a) && !strings.Contains(a, "/") && !strings.HasPrefix(a, "qa-") {
			t.Errorf("%s: default_agent %q is not a core agent", name, a)
		}
		if m := PresetDefaultModel(spec); m != "" && !strings.Contains(m, "/") {
			t.Errorf("%s: default_model %q must be provider/model", name, m)
		}
	}
}

func TestNewPresetsCopyProviders(t *testing.T) {
	for _, name := range []string{"devops", "dev-cheap", "code-review"} {
		spec, err := Preset(name)
		if err != nil {
			t.Fatal(err)
		}
		if !PresetCopyProviders(spec) {
			t.Errorf("%s: copy_global_providers must default to true", name)
		}
	}
}
