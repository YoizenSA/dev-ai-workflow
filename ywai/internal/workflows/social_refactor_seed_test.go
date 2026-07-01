package workflows

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSocialRefactorSeed(t *testing.T) {
	data, err := os.ReadFile("../../workflows/social-refactor.json")
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	var wf Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	res := Validate(&wf)
	if !res.Valid {
		t.Fatalf("social-refactor seed is INVALID:\n%+v", res)
	}
	for _, w := range res.Warnings {
		t.Logf("warning: [%s] %s", w.NodeID, w.Message)
	}
	t.Logf("social-refactor valid: %d nodes, %d connections", len(wf.Nodes), len(wf.Connections))
}

// TestExportSocialRefactorSeed confirms the seed exports to opencode artifacts
// (1 slash command + 1 orchestrator agent + 1 agent per subAgent node).
func TestExportSocialRefactorSeed(t *testing.T) {
	data, err := os.ReadFile("../../workflows/social-refactor.json")
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

	cmdContent, err := os.ReadFile(filepath.Join(commandsDir, "social-refactor.md"))
	if err != nil {
		t.Fatalf("command file missing: %v", err)
	}
	cs := string(cmdContent)
	if !strings.Contains(cs, "agent: social-refactor-orchestrator") {
		t.Errorf("command should reference orchestrator agent:\n%s", cs)
	}
	if !strings.Contains(cs, "$ARGUMENTS") {
		t.Errorf("command should pass $ARGUMENTS:\n%s", cs)
	}
}
