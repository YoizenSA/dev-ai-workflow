package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcppkg "github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
)

func writeCfg(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func readCfg(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("config is not valid JSON after write: %v\n%s", err, b)
	}
	return m
}

// The scoped @anthropic-ai/ package this used to point at does not exist on
// npm, so the entry has to carry the unscoped one or the server never spawns.
func TestInstallChromeDevToolsMCP_OpenCodeV2(t *testing.T) {
	dir := t.TempDir()
	path := writeCfg(t, dir, "opencode.json", `{"mcp":{"servers":{"graft":{"type":"local","command":["graft","mcp"]}}}}`)

	if err := InstallChromeDevToolsMCP(path, "opencode"); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Layout (nested under mcp.servers vs flat under mcp) depends on the
	// installed OpenCode major, so read it back through the same normalizer
	// the writer uses instead of pinning one shape.
	raw, _ := readCfg(t, path)["mcp"].(map[string]any)
	inner := mcppkg.CollectOpenCodeServers(raw)
	entry, ok := inner["chrome-devtools"].(map[string]any)
	if !ok {
		t.Fatalf("chrome-devtools not written: %v", inner)
	}
	if entry["type"] != "local" {
		t.Errorf("type = %v, want local", entry["type"])
	}
	argv, _ := entry["command"].([]any)
	if len(argv) == 0 || argv[0] != "npx" {
		t.Fatalf("command = %v, want an npx argv", entry["command"])
	}
	joined := ""
	for _, a := range argv {
		joined += a.(string) + " "
	}
	if want := "chrome-devtools-mcp@latest"; !contains(joined, want) {
		t.Errorf("command %q does not launch %q", joined, want)
	}
	if contains(joined, "@anthropic-ai/") {
		t.Errorf("command still points at the non-existent scoped package: %q", joined)
	}
	// The neighbour must survive.
	if _, ok := inner["graft"]; !ok {
		t.Error("installing chrome-devtools dropped the graft server")
	}
}

// Claude Code / pi split argv into command + args.
func TestInstallChromeDevToolsMCP_ClaudeFormat(t *testing.T) {
	dir := t.TempDir()
	path := writeCfg(t, dir, "claude.json", `{"mcpServers":{}}`)

	if err := InstallChromeDevToolsMCP(path, "claude-code"); err != nil {
		t.Fatalf("install: %v", err)
	}
	servers, _ := readCfg(t, path)["mcpServers"].(map[string]any)
	entry, ok := servers["chrome-devtools"].(map[string]any)
	if !ok {
		t.Fatalf("chrome-devtools not written: %v", servers)
	}
	if entry["command"] != "npx" {
		t.Errorf("command = %v, want npx", entry["command"])
	}
	args, _ := entry["args"].([]any)
	if len(args) != 2 || args[1] != "chrome-devtools-mcp@latest" {
		t.Errorf("args = %v, want [-y chrome-devtools-mcp@latest]", args)
	}
}

// Re-running install must not clobber a user's edited entry.
func TestInstallChromeDevToolsMCP_KeepsExisting(t *testing.T) {
	dir := t.TempDir()
	path := writeCfg(t, dir, "opencode.json",
		`{"mcp":{"servers":{"chrome-devtools":{"type":"local","command":["my-own-chrome"]}}}}`)

	if err := InstallChromeDevToolsMCP(path, "opencode"); err != nil {
		t.Fatalf("install: %v", err)
	}
	raw, _ := readCfg(t, path)["mcp"].(map[string]any)
	entry, _ := mcppkg.CollectOpenCodeServers(raw)["chrome-devtools"].(map[string]any)
	argv, _ := entry["command"].([]any)
	if len(argv) != 1 || argv[0] != "my-own-chrome" {
		t.Errorf("overwrote the user's entry: %v", entry["command"])
	}
}

func contains(h, n string) bool {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}
