package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReconcileSeedWorkflowAgentLinks_RestoresOrchestratorOnStart(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "goal.json")

	// User copy: start detached (inline prompt, no agentRef) — what the Studio
	// "Detach and edit here" path produces.
	user := map[string]any{
		"id":   "goal",
		"name": "goal",
		"nodes": []any{
			map[string]any{
				"id":   "start",
				"type": "start",
				"data": map[string]any{
					"label":           "Feature request",
					"agentDefinition": "You are the technical lead…",
					"model":           "inherit",
				},
			},
			map[string]any{
				"id":   "finder",
				"type": "subAgent",
				"data": map[string]any{
					"name":     "finder",
					"agentRef": "core/finder",
				},
			},
		},
		"connections": []any{
			map[string]any{"from": "start", "to": "finder"},
		},
	}
	writeJSON(t, dst, user)

	seed := []byte(`{
  "id": "goal",
  "name": "goal",
  "nodes": [
    {
      "id": "start",
      "type": "start",
      "data": {
        "label": "Feature request",
        "agentRef": "core/orchestrator",
        "model": "inherit"
      }
    },
    {
      "id": "finder",
      "type": "subAgent",
      "data": {
        "name": "finder",
        "agentRef": "core/finder"
      }
    }
  ]
}`)

	if err := reconcileSeedWorkflowAgentLinks(dst, seed); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	nodes := got["nodes"].([]any)
	start := nodes[0].(map[string]any)
	data := start["data"].(map[string]any)
	if data["agentRef"] != "core/orchestrator" {
		t.Fatalf("agentRef=%v, want core/orchestrator", data["agentRef"])
	}
	if _, ok := data["agentDefinition"]; ok {
		t.Fatalf("agentDefinition still present: %v", data["agentDefinition"])
	}
	// Connections must survive (not clobbered by a full rewrite of seed).
	conns, ok := got["connections"].([]any)
	if !ok || len(conns) != 1 {
		t.Fatalf("connections not preserved: %v", got["connections"])
	}
}

func TestReconcileSeedWorkflowAgentLinks_NoopWhenAlreadyLinked(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "goal.json")
	user := map[string]any{
		"nodes": []any{
			map[string]any{
				"id":   "start",
				"type": "start",
				"data": map[string]any{"agentRef": "core/orchestrator"},
			},
		},
	}
	writeJSON(t, dst, user)
	before, _ := os.ReadFile(dst)
	seed := []byte(`{"nodes":[{"id":"start","type":"start","data":{"agentRef":"core/orchestrator"}}]}`)
	if err := reconcileSeedWorkflowAgentLinks(dst, seed); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(dst)
	if string(before) != string(after) {
		t.Fatalf("file rewritten without changes")
	}
}

func TestReconcileSeedWorkflowAgentLinks_ReappliesPromptAndTools(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "goal.json")
	user := map[string]any{
		"nodes": []any{
			map[string]any{
				"id":   "finder",
				"type": "subAgent",
				"data": map[string]any{
					"name":     "finder",
					"agentRef": "core/finder",
					"prompt":   "Explore the codebase using codegraph and context7.",
					"tools":    "read, grep, glob, codegraph_*, context7_*",
				},
			},
		},
	}
	writeJSON(t, dst, user)
	seed := []byte(`{"nodes":[{"id":"finder","type":"subAgent","data":{"agentRef":"core/finder","prompt":"Explore with Graft first.","tools":"read, grep, glob, graft_*, context7_*"}}]}`)
	if err := reconcileSeedWorkflowAgentLinks(dst, seed); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	data := got["nodes"].([]any)[0].(map[string]any)["data"].(map[string]any)
	if data["prompt"] != "Explore with Graft first." {
		t.Fatalf("prompt=%v, want seed prompt", data["prompt"])
	}
	if data["tools"] != "read, grep, glob, graft_*, context7_*" {
		t.Fatalf("tools=%v, want seed tools", data["tools"])
	}
}

func TestMigrateRetiredWorkflowVocab_RewritesCodegraph(t *testing.T) {
	in := []byte(`{
  "nodes": [{
    "id": "dev",
    "type": "subAgent",
    "data": {
      "prompt": "Explore using codegraph_explore then codegraph.",
      "tools": "read, codegraph_*, lsp, ast_grep, code_search"
    }
  }]
}`)
	out, changed, err := migrateRetiredWorkflowVocab(in)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected a rewrite")
	}
	s := string(out)
	if strings.Contains(strings.ToLower(s), "codegraph") {
		t.Fatalf("codegraph survived:\n%s", s)
	}
	if strings.Contains(s, `"lsp"`) || strings.Contains(s, "ast_grep") || strings.Contains(s, "code_search") {
		t.Fatalf("dropped v2 tools survived:\n%s", s)
	}
	if !strings.Contains(s, "graft_*") {
		t.Fatalf("graft_* missing:\n%s", s)
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Integration: when the real seed goal is available, SeedWorkflowsFrom repairs
// a detached start on the live DataWorkflowsDir copy if present.
func TestSeedWorkflowsFrom_RepairsDetachedGoalStart(t *testing.T) {
	src := filepath.Join(WorkflowsSourceDir(), "goal.json")
	seed, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("no repo seed: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "goal.json")
	// detached user copy
	user := []byte(`{
  "id": "goal",
  "name": "goal",
  "nodes": [{
    "id": "start",
    "type": "start",
    "data": {
      "label": "Feature request",
      "agentDefinition": "DETACHED",
      "model": "inherit"
    }
  }],
  "connections": [{"from":"start","to":"finder"}]
}`)
	if err := os.WriteFile(dst, user, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := reconcileSeedWorkflowAgentLinks(dst, seed); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(dst)
	var got map[string]any
	_ = json.Unmarshal(raw, &got)
	start := got["nodes"].([]any)[0].(map[string]any)["data"].(map[string]any)
	if start["agentRef"] != "core/orchestrator" {
		t.Fatalf("agentRef=%v", start["agentRef"])
	}
	if _, ok := start["agentDefinition"]; ok {
		t.Fatal("agentDefinition should be cleared")
	}
}
