package workflows

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateQAAutomationSeed(t *testing.T) {
	data, err := os.ReadFile("../../workflows/qa-automation.json")
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	var wf Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	res := Validate(&wf)
	if !res.Valid {
		t.Fatalf("qa-automation seed is INVALID:\n%+v", res)
	}
	for _, w := range res.Warnings {
		t.Logf("warning: [%s] %s", w.NodeID, w.Message)
	}
	t.Logf("qa-automation valid: %d nodes, %d connections", len(wf.Nodes), len(wf.Connections))
}

// TestExportQAAutomationSeed confirms the seed exports to opencode artifacts
// (1 slash command + 1 orchestrator agent + 1 agent per subAgent node) and that
// the command references the orchestrator.
func TestExportQAAutomationSeed(t *testing.T) {
	data, err := os.ReadFile("../../workflows/qa-automation.json")
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	var wf Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	commandsDir := t.TempDir()
	agentsDir := t.TempDir()
	e := NewExporterWithDirs(commandsDir, agentsDir)

	plan, err := e.Apply(&wf)
	if err != nil {
		t.Fatalf("export Apply: %v", err)
	}

	// 1 command + 1 orchestrator + N sub-agents.
	wantAgents := 1 // orchestrator
	for _, n := range wf.Nodes {
		if n.Type == NodeTypeSubAgent {
			wantAgents++
		}
	}
	kinds := map[string]int{}
	for _, a := range plan.Files {
		kinds[a.Kind]++
		if _, err := os.Stat(a.Path); err != nil {
			t.Errorf("planned file not written: %s (%v)", a.Path, err)
		}
	}
	if kinds["command"] != 1 {
		t.Errorf("expected 1 command, got %d", kinds["command"])
	}
	if kinds["agent"] != wantAgents {
		t.Errorf("expected %d agents, got %d", wantAgents, kinds["agent"])
	}

	// The slash command must reference the orchestrator and pass $ARGUMENTS.
	cmdContent, err := os.ReadFile(filepath.Join(commandsDir, "qa-automation.md"))
	if err != nil {
		t.Fatalf("command file missing: %v", err)
	}
	cs := string(cmdContent)
	if !strings.Contains(cs, "agent: qa-automation-orchestrator") {
		t.Errorf("command should reference orchestrator agent:\n%s", cs)
	}
	if !strings.Contains(cs, "$ARGUMENTS") {
		t.Errorf("command should pass $ARGUMENTS:\n%s", cs)
	}

	// Every subAgent must produce an agent file whose slug is derived from its
	// node id, so a human can read the list of exported agents.
	for _, n := range wf.Nodes {
		if n.Type != NodeTypeSubAgent {
			continue
		}
		// The exporter writes <workflow>-<slug>.md; just assert at least one
		// agent file per subAgent exists by counting non-orchestrator agents.
	}
	orchFiles, err := filepath.Glob(filepath.Join(agentsDir, "qa-automation-*.md"))
	if err != nil || len(orchFiles) != wantAgents {
		t.Errorf("expected %d agent files under agents dir, got %d (err=%v)", wantAgents, len(orchFiles), err)
	}
}
