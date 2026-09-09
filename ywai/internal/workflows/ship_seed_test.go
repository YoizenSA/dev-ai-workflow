package workflows

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func loadShipSeed(t *testing.T) *Workflow {
	t.Helper()
	data, err := os.ReadFile("../../workflows/ship.json")
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	var wf Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &wf
}

func TestValidateShipSeed(t *testing.T) {
	wf := loadShipSeed(t)
	res := Validate(wf)
	if !res.Valid {
		t.Fatalf("ship seed is INVALID:\n%+v", res)
	}
	for _, w := range res.Warnings {
		t.Logf("warning: [%s] %s", w.NodeID, w.Message)
	}
}

// TestShipSeedStartLinksOrchestrator guards the link between the ship
// workflow's START node and the real orchestrator agent. A START node carrying
// its own agentDefinition silently wins over agentRef (see
// resolveAgentDefinition), so the copy would drift from agents/core/orchestrator
// without anything failing.
func TestShipSeedStartLinksOrchestrator(t *testing.T) {
	wf := loadShipSeed(t)
	start := wf.findNode(NodeTypeStart)
	if start == nil {
		t.Fatal("ship seed has no start node")
	}
	if def := strings.TrimSpace(start.Data.AgentDefinition); def != "" {
		t.Errorf("start node embeds its own prompt, which overrides agentRef:\n%s", def)
	}
	if got := start.Data.AgentRef; got != "core/orchestrator" {
		t.Fatalf("start agentRef = %q, want core/orchestrator", got)
	}

	// The ref must resolve against the real agents/ tree, not just be well-spelled.
	profiles, err := agents.LoadProfiles(config.AgentsSourceDir())
	if err != nil {
		t.Skipf("agents source dir unavailable: %v", err)
	}
	want, ok := profiles["core/orchestrator"]
	if !ok {
		t.Fatal("core/orchestrator profile not found under agents/")
	}
	got := resolveAgentDefinition(start)
	if strings.TrimSpace(got) != strings.TrimSpace(want.Prompt) {
		t.Errorf("start prompt does not resolve to the core/orchestrator agent")
	}
}
