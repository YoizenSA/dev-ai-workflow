package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestV1PermissionFromRules_KeepsTaskMap(t *testing.T) {
	got := V1PermissionFromRules([]PermissionRule{
		{Action: "edit", Resource: "*", Effect: "allow"},
		{Action: "shell", Resource: "*", Effect: "deny"},
		{Action: "subagent", Resource: "*", Effect: "deny"},
		{Action: "subagent", Resource: "finder", Effect: "allow"},
	})
	if got["edit"] != "allow" || got["bash"] != "deny" {
		t.Fatalf("broad rules = %#v", got)
	}
	task, ok := got["task"].(map[string]string)
	if !ok || task["*"] != "deny" || task["finder"] != "allow" {
		t.Fatalf("task = %#v", got["task"])
	}
}

func TestRewriteOpenCodeJSONV1Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	initial := `{
  "model": "opencode-admin/kept",
  "agent": {
    "orchestrator": {
      "model": "x",
      "permissions": [
        {"action": "read", "resource": "*", "effect": "allow"},
        {"action": "subagent", "resource": "finder", "effect": "allow"}
      ]
    },
    "dev": {"permission": {"read": "allow"}}
  }
}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := RewriteOpenCodeJSONV1Permissions(path)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rewritten = %d, want 1", n)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["model"]; !ok {
		t.Fatal("unrelated top-level keys must survive")
	}
	var agents map[string]map[string]json.RawMessage
	if err := json.Unmarshal(root["agent"], &agents); err != nil {
		t.Fatal(err)
	}
	if _, ok := agents["orchestrator"]["permissions"]; ok {
		t.Fatal("v2 permissions array must be gone")
	}
	var perm map[string]json.RawMessage
	if err := json.Unmarshal(agents["orchestrator"]["permission"], &perm); err != nil {
		t.Fatal(err)
	}
	if _, ok := perm["task"]; !ok {
		t.Fatalf("task map missing: %s", perm)
	}
}
