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
	servers, ok := mcp["servers"].(map[string]any)
	if !ok {
		t.Fatalf("write must nest under mcp.servers, got %v", mcp)
	}
	if _, has := mcp["graft"]; has {
		t.Fatal("graft must not remain a sibling of servers")
	}
	entry := servers["graft"].(map[string]any)
	if _, has := entry["enabled"]; has {
		t.Fatal("v2 must not write enabled")
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
