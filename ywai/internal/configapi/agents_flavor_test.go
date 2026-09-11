package configapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pinOpenCodeFlavor lives in orchestration_policy_test.go.

// writeOpenCodeConfig writes opencode.json under a temp HOME layout.
func writeOpenCodeConfig(t *testing.T, home, content string) string {
	t.Helper()
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestApplyAgentModel_FallsThroughToMarkdownWithoutAgentMap pins P0-1: an
// opencode.json without an agent map must not stop the markdown update.
// ywai installs agents as markdown, so markdown-only agents are the normal
// case.
func TestApplyAgentModel_FallsThroughToMarkdownWithoutAgentMap(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	pinOpenCodeFlavor(t, "opencode2")

	writeOpenCodeConfig(t, home, `{"model": "prov/other"}`)

	agentsDir := filepath.Join(home, ".config", "opencode", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mdPath := filepath.Join(agentsDir, "dev.md")
	md := "---\ndescription: writes code\n---\nBody\n"
	if err := os.WriteFile(mdPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	if !applyAgentModel("dev", "prov/m1") {
		t.Fatalf("applyAgentModel = false, want true (markdown update)")
	}
	got, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "model: prov/m1") {
		t.Fatalf("markdown model not written:\n%s", got)
	}
}

// TestApplyAgentModel_MergesLegacyAgentKey pins P0-2 (agents): an entry stored
// only under the legacy key must survive a write, and the written file must
// hold a single agent key.
func TestApplyAgentModel_MergesLegacyAgentKey(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	pinOpenCodeFlavor(t, "opencode2")

	path := writeOpenCodeConfig(t, home, `{
  "agents": {"ask": {"mode": "subagent", "description": "answers"}},
  "agent": {"dev": {"mode": "subagent", "description": "writes code"}}
}`)

	if !applyAgentModel("dev", "prov/m2") {
		t.Fatalf("applyAgentModel = false, want true")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["agent"]; ok {
		t.Fatalf("legacy agent key must be deleted, got:\n%s", data)
	}
	var agents map[string]map[string]any
	if err := json.Unmarshal(root["agents"], &agents); err != nil {
		t.Fatal(err)
	}
	dev, ok := agents["dev"]
	if !ok {
		t.Fatalf("dev entry lost during write:\n%s", data)
	}
	if dev["model"] != "prov/m2" {
		t.Errorf("dev model = %v, want prov/m2", dev["model"])
	}
	if _, ok := agents["ask"]; !ok {
		t.Errorf("ask entry (v2 key) lost during write:\n%s", data)
	}
}

// TestApplyAgentModel_V1MergesV2Key is the v1 mirror of the merge test.
func TestApplyAgentModel_V1MergesV2Key(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	pinOpenCodeFlavor(t, "opencode")

	path := writeOpenCodeConfig(t, home, `{
  "agent": {"ask": {"mode": "subagent", "description": "answers"}},
  "agents": {"dev": {"mode": "subagent", "description": "writes code"}}
}`)

	if !applyAgentModel("dev", "prov/m3") {
		t.Fatalf("applyAgentModel = false, want true")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["agents"]; ok {
		t.Fatalf("v2 agents key must be deleted on a v1 host, got:\n%s", data)
	}
	var agents map[string]map[string]any
	if err := json.Unmarshal(root["agent"], &agents); err != nil {
		t.Fatal(err)
	}
	if agents["dev"]["model"] != "prov/m3" {
		t.Errorf("dev model = %v, want prov/m3", agents["dev"]["model"])
	}
	if _, ok := agents["ask"]; !ok {
		t.Errorf("ask entry (v1 key) lost during write:\n%s", data)
	}
}

// TestLookupProviderSection_MergesBothKeys pins P0-2 (providers): the merged
// view contains entries from both spellings, with the flavor's key winning.
func TestLookupProviderSection_MergesBothKeys(t *testing.T) {
	t.Run("v2", func(t *testing.T) {
		pinOpenCodeFlavor(t, "opencode2")
		config := map[string]json.RawMessage{
			"providers": json.RawMessage(`{"admin": {"name": "v2 admin"}, "only-v2": {"name": "v2"}}`),
			"provider":  json.RawMessage(`{"admin": {"name": "v1 admin"}, "only-v1": {"name": "v1"}}`),
		}
		section := lookupProviderSection(config)
		if section == nil {
			t.Fatal("lookupProviderSection = nil, want merged map")
		}
		if len(section) != 3 {
			t.Fatalf("merged = %v, want 3 providers", section)
		}
		var admin struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(section["admin"], &admin); err != nil {
			t.Fatal(err)
		}
		if admin.Name != "v2 admin" {
			t.Errorf("flavor key must win for admin, got %q", admin.Name)
		}
	})
	t.Run("v1", func(t *testing.T) {
		pinOpenCodeFlavor(t, "opencode")
		config := map[string]json.RawMessage{
			"providers": json.RawMessage(`{"admin": {"name": "v2 admin"}}`),
			"provider":  json.RawMessage(`{"admin": {"name": "v1 admin"}}`),
		}
		section := lookupProviderSection(config)
		var admin struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(section["admin"], &admin); err != nil {
			t.Fatal(err)
		}
		if admin.Name != "v1 admin" {
			t.Errorf("flavor key must win for admin, got %q", admin.Name)
		}
	})
	t.Run("empty", func(t *testing.T) {
		pinOpenCodeFlavor(t, "opencode2")
		if got := lookupProviderSection(map[string]json.RawMessage{}); got != nil {
			t.Errorf("lookupProviderSection = %v, want nil", got)
		}
	})
}

// TestDeleteProvider_MergedAndSingleKey pins P1-1 plus the providers half of
// P0-2: deleting a provider removes it from the merged view, keeps entries
// that lived only under the legacy key, and leaves a single provider key.
func TestDeleteProvider_MergedAndSingleKey(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	pinOpenCodeFlavor(t, "opencode2")

	path := writeOpenCodeConfig(t, home, `{
  "providers": {
    "kept": {"name": "Kept"},
    "doomed": {"name": "Doomed"}
  },
  "provider": {"legacy-only": {"name": "Legacy"}}
}`)

	req := httptest.NewRequest(http.MethodDelete, "/api/config/providers/doomed", nil)
	req.SetPathValue("name", "doomed")
	w := httptest.NewRecorder()
	(&Handlers{}).DeleteProvider(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["provider"]; ok {
		t.Fatalf("legacy provider key must be deleted, got:\n%s", data)
	}
	var providers map[string]any
	if err := json.Unmarshal(root["providers"], &providers); err != nil {
		t.Fatal(err)
	}
	if _, ok := providers["doomed"]; ok {
		t.Errorf("doomed provider must be removed, got %v", providers)
	}
	for _, want := range []string{"kept", "legacy-only"} {
		if _, ok := providers[want]; !ok {
			t.Errorf("provider %q lost during delete, got %v", want, providers)
		}
	}
}
