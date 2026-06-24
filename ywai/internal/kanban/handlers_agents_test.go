package kanban

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGetAgentGraph builds a fake ~/.config/opencode layout under a temp HOME
// and asserts the static delegation graph is derived correctly from each
// agent's permission.task map.
func TestGetAgentGraph(t *testing.T) {
	// HOME drives opencodeConfigPath() and agentsDir() (both read os.UserHomeDir).
	home := t.TempDir()
	t.Setenv("HOME", home)

	// opencode.json: a primary orchestrator with a dense task allow/deny map
	// (mirrors the gentle-orchestrator shape), a subagent "dev", and an "ask"
	// delegation to a reviewer subagent.
	config := `{
  "agent": {
    "orchestrator": {
      "mode": "primary",
      "description": "coordinates sub-agents",
      "permission": {
        "*": "deny",
        "question": "allow",
        "task": {
          "*": "deny",
          "dev": "allow",
          "reviewer": "ask"
        }
      }
    },
    "dev": {
      "mode": "subagent",
      "description": "writes code",
      "permission": {
        "*": "deny",
        "read": "allow",
        "edit": "allow",
        "task": "deny"
      }
    },
    "reviewer": {
      "mode": "subagent",
      "permission": {
        "read": "allow"
      }
    }
  }
}`
	ocPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(ocPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ocPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config/agents/graph", nil)
	w := httptest.NewRecorder()
	(&Handlers{}).GetAgentGraph(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp agentGraphResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Nodes: orchestrator, dev, reviewer (+ no ghosts — all targets exist).
	nodeByID := map[string]agentGraphNode{}
	for _, n := range resp.Nodes {
		nodeByID[n.ID] = n
	}
	for _, want := range []string{"orchestrator", "dev", "reviewer"} {
		if _, ok := nodeByID[want]; !ok {
			t.Errorf("expected node %q in graph, missing", want)
		}
	}
	if len(resp.Nodes) != 3 {
		t.Errorf("expected exactly 3 nodes, got %d: %+v", len(resp.Nodes), resp.Nodes)
	}

	// orchestrator: primary mode, wildcard deny on task.
	orch := nodeByID["orchestrator"]
	if orch.Mode != "primary" {
		t.Errorf("orchestrator mode = %q, want primary", orch.Mode)
	}
	if !orch.HasWildcard || orch.WildcardValue != "deny" {
		t.Errorf("orchestrator wildcard = (%v,%q), want (true,deny)", orch.HasWildcard, orch.WildcardValue)
	}

	// dev: task is a scalar "deny" -> normalized into wildcard.
	dev := nodeByID["dev"]
	if dev.Mode != "subagent" {
		t.Errorf("dev mode = %q, want subagent", dev.Mode)
	}
	if !dev.HasWildcard || dev.WildcardValue != "deny" {
		t.Errorf("dev wildcard = (%v,%q), want (true,deny)", dev.HasWildcard, dev.WildcardValue)
	}

	// Edges: orchestrator->dev (allow), orchestrator->reviewer (ask).
	wantEdge := map[string]string{
		"orchestrator->dev":      "allow",
		"orchestrator->reviewer": "ask",
	}
	gotEdge := map[string]string{}
	for _, e := range resp.Edges {
		gotEdge[e.ID] = e.Value
	}
	for id, val := range wantEdge {
		if got, ok := gotEdge[id]; !ok {
			t.Errorf("expected edge %q, missing", id)
		} else if got != val {
			t.Errorf("edge %q value = %q, want %q", id, got, val)
		}
	}
	if len(resp.Edges) != len(wantEdge) {
		t.Errorf("expected %d edges, got %d: %+v", len(wantEdge), len(resp.Edges), resp.Edges)
	}
}

// TestGetAgentGraph_GhostTarget verifies an edge target that is not a defined
// agent becomes a ghost node instead of a dangling reference.
func TestGetAgentGraph_GhostTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	config := `{
  "agent": {
    "orch": {
      "mode": "primary",
      "permission": {
        "task": { "missing-sub": "allow" }
      }
    }
  }
}`
	ocPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(ocPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ocPath, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config/agents/graph", nil)
	w := httptest.NewRecorder()
	(&Handlers{}).GetAgentGraph(w, req)

	var resp agentGraphResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var ghost *agentGraphNode
	for i := range resp.Nodes {
		if resp.Nodes[i].ID == "missing-sub" {
			ghost = &resp.Nodes[i]
		}
	}
	if ghost == nil {
		t.Fatalf("expected ghost node 'missing-sub', nodes: %+v", resp.Nodes)
	}
	if !ghost.Ghost {
		t.Errorf("ghost node should have Ghost=true, got %+v", ghost)
	}

	found := false
	for _, e := range resp.Edges {
		if e.Source == "orch" && e.Target == "missing-sub" && e.Value == "allow" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected edge orch->missing-sub (allow), edges: %+v", resp.Edges)
	}
}

// Fixture mirroring the gentle-orchestrator Delegation Rules + Triggers section.
const delegationRulesFixture = `---
description: test orchestrator
mode: primary
permission:
  question: allow
---

# Orchestrator

intro.

### Delegation Rules

core principle: does this inflate my context without need?

| Action | Inline | Delegate |
| ------ | ------ | -------- |
| Read to decide/verify (1-3 files) | Yes | No |
| Read to explore/understand (4+ files) | No | Yes |
| Write with analysis (multiple files, new logic) | No | Yes |

#### Mandatory Delegation Triggers

1. **4-file rule**: if understanding requires reading 4+ files, delegate.
2. **Multi-file write rule**: if implementation touches 2+ files, delegate one writer.
3. **PR rule**: before commit/push, run a fresh-context review.

### Cost and Context Balance

other content.`

// writeAgentFixture writes the delegationRulesFixture under a temp HOME agents
// dir and returns the agent name.
func writeAgentFixture(t *testing.T, name string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	agentsDir := filepath.Join(home, ".config", "opencode", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, name+".md"), []byte(delegationRulesFixture), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGetDelegationRules_ParsesTableAndTriggers(t *testing.T) {
	const name = "test-orch"
	writeAgentFixture(t, name)

	req := httptest.NewRequest(http.MethodGet, "/api/config/agents/"+name+"/delegation-rules", nil)
	req.SetPathValue("name", name)
	w := httptest.NewRecorder()
	(&Handlers{}).GetDelegationRules(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp delegationRulesResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.HasRules {
		t.Error("expected HasRules=true")
	}
	if len(resp.Rules) != 3 {
		t.Fatalf("expected 3 rules, got %d: %+v", len(resp.Rules), resp.Rules)
	}
	if resp.Rules[0].Action != "Read to decide/verify (1-3 files)" {
		t.Errorf("rule 0 action = %q", resp.Rules[0].Action)
	}
	if resp.Rules[0].Inline != "Yes" || resp.Rules[0].Delegate != "No" {
		t.Errorf("rule 0 inline/delegate = %q/%q", resp.Rules[0].Inline, resp.Rules[0].Delegate)
	}
	if len(resp.Triggers) != 3 {
		t.Fatalf("expected 3 triggers, got %d: %+v", len(resp.Triggers), resp.Triggers)
	}
	if resp.Triggers[0].Name != "4-file rule" {
		t.Errorf("trigger 0 name = %q", resp.Triggers[0].Name)
	}
}

func TestGetDelegationRules_AbsentSection(t *testing.T) {
	const name = "no-rules-agent"
	home := t.TempDir()
	t.Setenv("HOME", home)
	agentsDir := filepath.Join(home, ".config", "opencode", "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, name+".md"), []byte("---\nmode: subagent\n---\n\njust a prompt."), 0o644)

	req := httptest.NewRequest(http.MethodGet, "/api/config/agents/"+name+"/delegation-rules", nil)
	req.SetPathValue("name", name)
	w := httptest.NewRecorder()
	(&Handlers{}).GetDelegationRules(w, req)

	var resp delegationRulesResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.HasRules {
		t.Error("expected HasRules=false when section absent")
	}
}

func TestPutDelegationRules_RoundTrip(t *testing.T) {
	const name = "roundtrip-orch"
	writeAgentFixture(t, name)

	in := struct {
		Rules    []delegationRule    `json:"rules"`
		Triggers []delegationTrigger `json:"triggers"`
	}{
		Rules: []delegationRule{
			{Action: "Read 1 file", Inline: "Yes", Delegate: "No"},
			{Action: "Write multi-file", Inline: "No", Delegate: "Yes, together with the write"},
		},
		Triggers: []delegationTrigger{
			{Name: "4-file rule", Description: "delegate exploration when 4+ files."},
			{Name: "PR rule", Description: "fresh review before PR."},
		},
	}
	bodyBytes, _ := json.Marshal(in)
	req := httptest.NewRequest(http.MethodPut, "/api/config/agents/"+name+"/delegation-rules", strings.NewReader(string(bodyBytes)))
	req.SetPathValue("name", name)
	w := httptest.NewRecorder()
	(&Handlers{}).PutDelegationRules(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: %d %s", w.Code, w.Body.String())
	}

	// Re-read and verify the rules persisted.
	req2 := httptest.NewRequest(http.MethodGet, "/api/config/agents/"+name+"/delegation-rules", nil)
	req2.SetPathValue("name", name)
	w2 := httptest.NewRecorder()
	(&Handlers{}).GetDelegationRules(w2, req2)
	var resp delegationRulesResp
	json.Unmarshal(w2.Body.Bytes(), &resp)
	if len(resp.Rules) != 2 {
		t.Fatalf("expected 2 rules after round-trip, got %d", len(resp.Rules))
	}
	if resp.Rules[1].Delegate != "Yes, together with the write" {
		t.Errorf("delegate text not preserved: %q", resp.Rules[1].Delegate)
	}
	if len(resp.Triggers) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(resp.Triggers))
	}

	// Sibling section (Cost and Context Balance) must survive.
	mdPath := readAgentMarkdownPath(name)
	data, _ := os.ReadFile(mdPath)
	if !strings.Contains(string(data), "Cost and Context Balance") {
		t.Error("sibling section should survive the PUT")
	}
	// Frontmatter must survive.
	if !strings.HasPrefix(string(data), "---") {
		t.Error("frontmatter should be preserved")
	}
}

func TestParseDelegationRulesTable_PipeEscape(t *testing.T) {
	section := `| Action | Inline | Delegate |
| ------ | ------ | -------- |
| a \| b | Yes | No |`
	rules := parseDelegationRulesTable(section)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Action != "a | b" {
		t.Errorf("escaped pipe not handled: %q", rules[0].Action)
	}
}

func TestTriggerLineRegex(t *testing.T) {
	cases := map[string]bool{
		`1. **4-file rule**: if understanding requires...`: true,
		`2. **PR rule** - before commit`:                  true,
		`3. **Incident rule** — after wrong cwd`:          true,
		`- not a trigger`:                                 false,
		`plain text`:                                      false,
	}
	for line, want := range cases {
		got := triggerLineRegex.MatchString(line)
		if got != want {
			t.Errorf("MatchString(%q) = %v, want %v", line, got, want)
		}
	}
}
