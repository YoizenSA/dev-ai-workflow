package mcp

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
)

// v2Home pins the flavor to v2 and redirects the config path into a temp dir.
func v2Home(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("OPENCODE_CONFIG_DIR", "")
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")
	return home
}

func v2MCPSection(t *testing.T, home string) map[string]any {
	t.Helper()
	cfg := parseJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	section, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("cfg[mcp] = %v (%T), want map", cfg["mcp"], cfg["mcp"])
	}
	return section
}

// v2 reads servers from mcp.servers. Writing them flat at the root of mcp — the
// v1 layout — leaves them invisible, so the server never starts.
func TestWriteAgentConfig_V2NestsUnderServers(t *testing.T) {
	home := v2Home(t)

	entry, ok := CatalogByID("graft")
	if !ok {
		t.Fatal(`CatalogByID("graft") ok=false, want true`)
	}
	if _, err := WriteAgentConfig("opencode", "graft", BuildEntryShape("opencode", entry, nil)); err != nil {
		t.Fatalf("WriteAgentConfig: %v", err)
	}

	section := v2MCPSection(t, home)
	servers, nested := section["servers"].(map[string]any)
	if !nested {
		t.Fatalf("v2 must nest servers under mcp.servers, got %v", section)
	}
	if _, flat := section["graft"]; flat {
		t.Errorf("server also written flat at mcp.graft: %v", section)
	}
	got, ok := servers["graft"].(map[string]any)
	if !ok {
		t.Fatalf("mcp.servers[graft] = %v, want map", servers["graft"])
	}
	if got["type"] != "local" {
		t.Errorf(`type = %v, want "local"`, got["type"])
	}
	// v2 has no "enabled": an entry is on unless "disabled" says otherwise, so
	// writing enabled:true is dead weight and enabled:false does nothing at all.
	if _, bad := got["enabled"]; bad {
		t.Errorf(`v2 entry carries "enabled", which v2 ignores: %v`, got)
	}
}

// A disabled entry must say so in the spelling v2 understands.
func TestWriteAgentConfig_V2UsesDisabledNotEnabled(t *testing.T) {
	home := v2Home(t)

	shape := map[string]any{
		"type":    "local",
		"command": []any{"echo"},
		"enabled": false, // the v1 spelling, as read back from an existing file
	}
	if _, err := WriteAgentConfig("opencode", "off", shape); err != nil {
		t.Fatalf("WriteAgentConfig: %v", err)
	}

	servers, _ := v2MCPSection(t, home)["servers"].(map[string]any)
	got, ok := servers["off"].(map[string]any)
	if !ok {
		t.Fatalf("mcp.servers[off] missing: %v", servers)
	}
	if _, bad := got["enabled"]; bad {
		t.Errorf(`"enabled" must be translated away for v2: %v`, got)
	}
	if got["disabled"] != true {
		t.Errorf("disabled = %v, want true (enabled:false must survive the move)", got["disabled"])
	}
}

// Reading must accept whichever layout is on disk, so switching flavors does
// not lose the servers the other one wrote.
func TestCollectOpenCodeServers_ReadsBothLayouts(t *testing.T) {
	want := map[string]any{"a": map[string]any{"type": "local"}}

	flat := CollectOpenCodeServers(map[string]any{
		"a":       map[string]any{"type": "local"},
		"timeout": 5.0,
	})
	if !reflect.DeepEqual(flat, want) {
		t.Errorf("flat layout: got %v, want %v", flat, want)
	}

	nested := CollectOpenCodeServers(map[string]any{
		"servers": map[string]any{"a": map[string]any{"type": "local"}},
	})
	if !reflect.DeepEqual(nested, want) {
		t.Errorf("nested layout: got %v, want %v", nested, want)
	}
}

// Switching to v2 must migrate what v1 left flat, not duplicate it.
func TestWriteAgentConfig_V2MigratesFlatEntries(t *testing.T) {
	home := v2Home(t)

	// Seed a v1-shaped file, as an earlier v1 install would have left it.
	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeJSONFile(t, cfgPath, `{"mcp":{"legacy":{"type":"local","command":["echo"],"enabled":true}}}`)

	entry, _ := CatalogByID("graft")
	if _, err := WriteAgentConfig("opencode", "graft", BuildEntryShape("opencode", entry, nil)); err != nil {
		t.Fatalf("WriteAgentConfig: %v", err)
	}

	section := v2MCPSection(t, home)
	if _, flat := section["legacy"]; flat {
		t.Errorf("flat v1 entry left beside mcp.servers: %v", section)
	}
	servers, _ := section["servers"].(map[string]any)
	for _, id := range []string{"legacy", "graft"} {
		if _, ok := servers[id]; !ok {
			t.Errorf("mcp.servers[%s] missing after migration: %v", id, servers)
		}
	}
}
