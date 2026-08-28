package control

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeOpencodeMCP(t *testing.T, home string, mcp map[string]any) {
	t.Helper()
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"mcp": mcp})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSetMcpEnabledTogglesInstalledServer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeOpencodeMCP(t, home, map[string]any{
		"graft": map[string]any{"type": "local", "enabled": true},
	})

	if err := SetMcpEnabled("graft", false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	list, err := ListInstalledMCP()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "graft" || list[0].Enabled {
		t.Fatalf("after disable: %+v", list)
	}

	if err := SetMcpEnabled("graft", true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	list, err = ListInstalledMCP()
	if err != nil {
		t.Fatal(err)
	}
	if !list[0].Enabled {
		t.Fatal("expected enabled after SetMcpEnabled(true)")
	}

	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	mcp := root["mcp"].(map[string]any)
	// v1: servers sit directly under mcp, never nested under mcp.servers.
	if _, nested := mcp["servers"]; nested {
		t.Fatalf("v1 must not nest under mcp.servers, got %v", mcp)
	}
	entry, ok := mcp["graft"].(map[string]any)
	if !ok {
		t.Fatalf("graft missing under mcp, got %v", mcp)
	}
	// The test disables then re-enables, so the persisted state is enabled:true
	// — and v1 requires the key to be present either way.
	if entry["enabled"] != true {
		t.Fatalf("enable must persist as enabled:true, got %v", entry)
	}
}

func TestSetMcpEnabledUnknownServer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	writeOpencodeMCP(t, home, map[string]any{})

	err := SetMcpEnabled("nope", false)
	if err == nil {
		t.Fatal("expected error for missing MCP")
	}
}
