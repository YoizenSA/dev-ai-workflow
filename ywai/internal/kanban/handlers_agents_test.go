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

// setTestHomeDir redirects the user home directory for the duration of the
// test. os.UserHomeDir() reads HOME on unix and USERPROFILE on Windows, so
// both must be set for these tests to resolve config paths under the temp
// dir on every CI runner.
func setTestHomeDir(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

// TestGetAgentGraph builds a fake ~/.config/opencode layout under a temp HOME
// and asserts the static delegation graph is derived correctly from each
// agent's permission.task map.
func TestGetAgentGraph(t *testing.T) {
	// HOME drives opencodeConfigPath() and agentsDir() (both read os.UserHomeDir).
	home := t.TempDir()
	setTestHomeDir(t, home)

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
	setTestHomeDir(t, home)

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

// --- Delegation Rules (sidecar JSON) tests ---

// writeSidecar writes a delegations.json sidecar under the temp HOME agents dir.
func writeSidecar(t *testing.T, payload string) {
	t.Helper()
	home := t.TempDir()
	setTestHomeDir(t, home)
	agentsDir := filepath.Join(home, ".config", "opencode", "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "delegations.json"), []byte(payload), 0o644)
}

const sidecarWithDefaults = `{
  "defaults": {
    "rules": [
      {"action":"Read to decide/verify (1-3 files)","inline":"Yes","delegate":"No"},
      {"action":"Write with analysis","inline":"No","delegate":"Yes"}
    ],
    "triggers": [
      {"name":"4-file rule","description":"delegate exploration"},
      {"name":"PR rule","description":"fresh review before PR"}
    ]
  },
  "agents": {
    "orchestrator": {"skip_rules": false},
    "dev": {"skip_rules": true},
    "architect": {"rules":[{"action":"Custom","inline":"No","delegate":"Yes"}]}
  }
}`

func TestGetDelegationRules_FromSidecar_Defaults(t *testing.T) {
	writeSidecar(t, sidecarWithDefaults)
	req := httptest.NewRequest(http.MethodGet, "/api/config/agents/orchestrator/delegation-rules", nil)
	req.SetPathValue("name", "orchestrator")
	w := httptest.NewRecorder()
	(&Handlers{}).GetDelegationRules(w, req)

	var resp delegationRulesResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.HasRules {
		t.Fatal("expected HasRules=true (defaults apply)")
	}
	if len(resp.Rules) != 2 {
		t.Errorf("expected 2 default rules, got %d", len(resp.Rules))
	}
	if len(resp.Triggers) != 2 {
		t.Errorf("expected 2 default triggers, got %d", len(resp.Triggers))
	}
}

func TestGetDelegationRules_SkipRules(t *testing.T) {
	writeSidecar(t, sidecarWithDefaults)
	req := httptest.NewRequest(http.MethodGet, "/api/config/agents/dev/delegation-rules", nil)
	req.SetPathValue("name", "dev")
	w := httptest.NewRecorder()
	(&Handlers{}).GetDelegationRules(w, req)

	var resp delegationRulesResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.HasRules {
		t.Error("expected HasRules=false for skip_rules agent")
	}
}

func TestGetDelegationRules_PerAgentOverride(t *testing.T) {
	writeSidecar(t, sidecarWithDefaults)
	req := httptest.NewRequest(http.MethodGet, "/api/config/agents/architect/delegation-rules", nil)
	req.SetPathValue("name", "architect")
	w := httptest.NewRecorder()
	(&Handlers{}).GetDelegationRules(w, req)

	var resp delegationRulesResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Rules) != 1 || resp.Rules[0].Action != "Custom" {
		t.Errorf("expected 1 override rule 'Custom', got %+v", resp.Rules)
	}
}

func TestGetDelegationRules_NoSidecar(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	os.MkdirAll(filepath.Join(home, ".config", "opencode", "agents"), 0o755)

	req := httptest.NewRequest(http.MethodGet, "/api/config/agents/orchestrator/delegation-rules", nil)
	req.SetPathValue("name", "orchestrator")
	w := httptest.NewRecorder()
	(&Handlers{}).GetDelegationRules(w, req)

	var resp delegationRulesResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.HasRules {
		t.Error("expected HasRules=false when no sidecar exists")
	}
}

func TestPutDelegationRules_WritesSidecarAndRendersMarkdown(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	agentsDir := filepath.Join(home, ".config", "opencode", "agents")
	os.MkdirAll(agentsDir, 0o755)
	// Seed an agent markdown so the PUT can render into it.
	os.WriteFile(filepath.Join(agentsDir, "orchestrator.md"), []byte("---\nmode: primary\n---\n\nprompt body."), 0o644)

	body := `{"rules":[{"action":"Read 1 file","inline":"Yes","delegate":"No"}],"triggers":[{"name":"4-file rule","description":"delegate"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/config/agents/orchestrator/delegation-rules", strings.NewReader(body))
	req.SetPathValue("name", "orchestrator")
	w := httptest.NewRecorder()
	(&Handlers{}).PutDelegationRules(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: %d %s", w.Code, w.Body.String())
	}

	// Sidecar written.
	sidecarData, _ := os.ReadFile(filepath.Join(agentsDir, "delegations.json"))
	if !strings.Contains(string(sidecarData), "Read 1 file") {
		t.Errorf("sidecar missing the rule: %s", sidecarData)
	}

	// Markdown re-rendered with the table.
	mdData, _ := os.ReadFile(filepath.Join(agentsDir, "orchestrator.md"))
	if !strings.Contains(string(mdData), "### Delegation Rules") {
		t.Errorf("markdown missing the section")
	}
	if !strings.Contains(string(mdData), "Read 1 file") {
		t.Errorf("markdown missing the rule row")
	}
}
