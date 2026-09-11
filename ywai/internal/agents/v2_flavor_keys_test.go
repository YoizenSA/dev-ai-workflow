package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// TestApplyDelegations_V2WritesAgentsKey pins P1-3a: on a v2 host the task
// map mirror lands under `agents`, and the legacy `agent` key does not
// coexist with it.
func TestApplyDelegations_V2WritesAgentsKey(t *testing.T) {
	t.Setenv("YWAI_OPENCODE", "opencode2")
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	agentsDir := filepath.Join(dir, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	doc := &DelegationsDoc{
		Agents: map[string]AgentDelegation{
			"dev": {Task: map[string]string{"qa": "allow"}},
		},
	}
	// ApplyDelegations only touches agents whose markdown is installed, so
	// the test must ship dev.md or the doc is filtered to nothing.
	if err := os.WriteFile(filepath.Join(agentsDir, "dev.md"), []byte("---\ndescription: writes code\n---\nBody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ApplyDelegations(configPath, agentsDir, doc); err != nil {
		t.Fatalf("ApplyDelegations: %v", err)
	}

	root, err := config.ReadJSONC(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := root["agent"]; ok {
		t.Fatalf("legacy agent key must not be written on a v2 host, got: %v", root)
	}
	agents, ok := root["agents"].(map[string]any)
	if !ok {
		t.Fatalf("agents key missing or not a map: %v", root)
	}
	dev, ok := agents["dev"].(map[string]any)
	if !ok {
		t.Fatalf("dev entry missing under agents: %v", agents)
	}
	perm, ok := dev["permission"].(map[string]any)
	if !ok {
		t.Fatalf("dev permission missing: %v", dev)
	}
	task, ok := perm["task"].(map[string]any)
	if !ok {
		t.Fatalf("dev permission.task missing: %v", perm)
	}
	if task["qa"] != "allow" {
		t.Errorf("task[qa] = %v, want allow", task["qa"])
	}
}

// TestStripYwaiAgentKeys JSON decode sanity for the uninstall path lives in
// cmd/ywai; here we only guard that the merged read never loses the legacy
// entries when both keys exist (P0-2 at the agents-package level).
func TestReadJSONCRoundtripKeepsBothAgentKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(path, []byte(`{
  "agents": {"a": {"x": 1}},
  "agent": {"b": {"y": 2}}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := config.ReadJSONC(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	var probe struct {
		Agents map[string]any `json:"agents"`
		Agent  map[string]any `json:"agent"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatal(err)
	}
	if len(probe.Agents) != 1 || len(probe.Agent) != 1 {
		t.Fatalf("both keys must survive a read/WriteJSONC roundtrip: %s", raw)
	}
}
