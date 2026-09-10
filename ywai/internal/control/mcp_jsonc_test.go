package control

import (
	"os"
	"path/filepath"
	"testing"
)

// OpenCode prefers opencode.jsonc when it exists, and a .jsonc carrying
// comments is not valid JSON. Reading it with encoding/json reported "no MCP
// servers" for a config full of them — the servers were installed, ywai was
// parsing the wrong thing.
func TestReadMcpConfigReadsJsoncWithComments(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{
  // the grafana box on the internal network
  "mcp": {
    "servers": {
      "grafana": { "type": "remote", "url": "https://grafana.example/mcp" },
      /* block comment */
      "graft": { "type": "local", "command": ["graft", "mcp"] }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "opencode.jsonc"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readMcpConfig()
	if err != nil {
		t.Fatalf("readMcpConfig: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("found %d servers, want 2 (grafana, graft): %v", len(got), got)
	}
	entry, ok := got["grafana"].(map[string]interface{})
	if !ok {
		t.Fatalf("grafana not detected: %v", got)
	}
	if entry["url"] != "https://grafana.example/mcp" {
		t.Errorf("url = %v, want the configured endpoint", entry["url"])
	}
}

// .jsonc wins over .json when both are present, the way OpenCode resolves it.
func TestReadMcpConfigPrefersJsoncOverJson(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"),
		[]byte(`{"mcp":{"servers":{"stale":{"type":"local"}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "opencode.jsonc"),
		[]byte(`{"mcp":{"servers":{"live":{"type":"local"}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readMcpConfig()
	if err != nil {
		t.Fatalf("readMcpConfig: %v", err)
	}
	if _, ok := got["live"]; !ok {
		t.Errorf("read the .json instead of the .jsonc OpenCode actually uses: %v", got)
	}
}
