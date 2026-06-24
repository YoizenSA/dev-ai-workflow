package missions

import (
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func TestResolveExecution_UsesOnlyActiveOrchestratorProfileForOrchestratorRoles(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.ActiveOrchestratorProfile = "active"
	cfg.OrchestratorProfiles = map[string]config.OrchestratorModelProfile{
		"active": {
			DisplayName: "Active",
			RoleDefaults: config.RoleDefaults{
				"scout": {Agent: "active-agent", Model: "active/model", Fallbacks: []string{"active/fallback"}},
			},
		},
		"inactive": {
			DisplayName: "Inactive",
			RoleDefaults: config.RoleDefaults{
				"scout": {Agent: "inactive-agent", Model: "inactive/model", Fallbacks: []string{"inactive/fallback"}},
			},
		},
	}

	feature := &Feature{Role: "scout"}
	mission := &Mission{Model: "mission/model", ExecutionAgent: "mission-agent"}

	model, agent, chain := ResolveExecution(feature, mission, cfg)

	if model != "active/model" {
		t.Fatalf("expected active profile model, got %q", model)
	}
	if agent != "active-agent" {
		t.Fatalf("expected active profile agent, got %q", agent)
	}
	if len(chain) != 2 || chain[0] != "active/model" || chain[1] != "active/fallback" {
		t.Fatalf("expected chain from active profile only, got %#v", chain)
	}
	for _, item := range append(chain, model, agent) {
		if item == "inactive/model" || item == "inactive-agent" || item == "inactive/fallback" {
			t.Fatalf("inactive profile value leaked into runtime resolution: model=%q agent=%q chain=%#v", model, agent, chain)
		}
	}
}
