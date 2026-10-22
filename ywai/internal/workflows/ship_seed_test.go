package workflows

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// TestShipExport_SectionsAppearOnce guards against a linked node getting its
// agent's sections twice: resolveAgentDefinition already returns the prompt with
// the agent's sections, and the node lists the same sections again.
func TestShipExport_SectionsAppearOnce(t *testing.T) {
	wf := loadShipSeed(t)
	for _, target := range []string{TargetOpenCode, TargetClaudeCode} {
		e := newExporterWithDirsForTarget(t.TempDir(), t.TempDir(), target)
		_, files, err := e.Plan(wf)
		if err != nil {
			t.Fatalf("%s Plan: %v", target, err)
		}
		checked := 0
		for path, content := range files {
			base := filepath.Base(path)
			if !strings.HasPrefix(base, "ship-") {
				continue
			}
			checked++
			seen := map[string]int{}
			inFence := false
			for _, line := range strings.Split(content, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "```") {
					inFence = !inFence
				}
				if !inFence && strings.HasPrefix(line, "## ") {
					seen[line]++
				}
			}
			for heading, n := range seen {
				if n > 1 {
					t.Errorf("%s %s: %q appears %d times", target, base, heading, n)
				}
			}
		}
		if checked == 0 {
			t.Fatalf("%s: no ship-* agents exported", target)
		}
	}
}

// TestExport_MissingSectionFails makes an unknown section name an export error
// instead of a prompt silently missing its contract.
func TestExport_MissingSectionFails(t *testing.T) {
	wf := &Workflow{
		Name: "typo",
		Nodes: []Node{
			{ID: "s", Type: NodeTypeStart},
			{ID: "a", Type: NodeTypeSubAgent, Name: "worker",
				Data: NodeData{Name: "worker", AgentDefinition: "Work.", Sections: "no-such-section"}},
		},
		Connections: []Connection{{From: "s", To: "a"}},
	}
	e := newExporterWithDirs(t.TempDir(), t.TempDir())
	if _, _, err := e.Plan(wf); err == nil || !strings.Contains(err.Error(), "no-such-section") {
		t.Fatalf("Plan error = %v, want one naming the missing section", err)
	}
}

// TestSeedWorkflows_SectionsExist checks every seed node names real sections.
func TestSeedWorkflows_SectionsExist(t *testing.T) {
	paths, _ := filepath.Glob("../../workflows/*.json")
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var wf Workflow
		if err := json.Unmarshal(data, &wf); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for _, n := range wf.Nodes {
			for _, s := range subAgentSectionList(n.Data.Sections) {
				if _, err := os.Stat(filepath.Join(config.AgentsSourceDir(), "sections", s+".md")); err != nil {
					t.Errorf("%s node %s: section %q does not exist", filepath.Base(p), n.ID, s)
				}
			}
		}
	}
}

// TestSubAgentSectionList_LegacyHandoffQA keeps user workflows saved before the
// handoff-qa section was retired exporting: it reads as the standard handoff.
func TestSubAgentSectionList_LegacyHandoffQA(t *testing.T) {
	got := subAgentSectionList("handoff-qa,context-gathering")
	if len(got) != 2 || got[0] != "handoff" || got[1] != "context-gathering" {
		t.Fatalf("subAgentSectionList = %v, want [handoff context-gathering]", got)
	}
}
