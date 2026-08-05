package configapi

import (
	"testing"

	userconfig "github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// The profiles tab lists whatever this returns. Keying it to the stored profile
// meant a new agent stayed invisible until someone edited the seed file by
// hand — planning, designer and advisor were all unlistable that way.
func TestWithAllInstalledAgents(t *testing.T) {
	stored := map[string]userconfig.OrchestratorModelProfile{
		"balanced": {Agents: userconfig.RoleDefaults{
			"dev": {Model: "anthropic/claude-sonnet-5"},
		}},
	}

	out, groups := withAllInstalledAgents(stored)
	agents := out["balanced"].Agents

	if agents["dev"].Model != "anthropic/claude-sonnet-5" {
		t.Error("an assigned model must survive the merge")
	}
	for _, name := range []string{"advisor", "designer", "planning"} {
		rd, ok := agents[name]
		if !ok {
			t.Errorf("%s missing — it is installed but would not be listed", name)
			continue
		}
		if rd.Model != "" {
			t.Errorf("%s should default to inherit, got %q", name, rd.Model)
		}
	}
	if len(stored["balanced"].Agents) != 1 {
		t.Error("the stored profile must not be mutated")
	}
	if len(groups) == 0 {
		t.Error("agent_groups must be populated so the UI can group by folder")
	}
	// Every installed agent should map to a non-empty folder.
	for name, folder := range groups {
		if folder == "" {
			t.Errorf("agent %s has an empty folder", name)
		}
	}
}
