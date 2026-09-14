package plugins

import (
	"testing"

	mcppkg "github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
)

// Grafana ships blank because its endpoint is per-network. Blank AND enabled
// would be a server the agent tries and fails to reach on every start, so the
// entry has to arrive switched off.
func TestInstallGrafanaMCP_BlankAndDisabled(t *testing.T) {
	dir := t.TempDir()
	path := writeCfg(t, dir, "opencode.json", `{"mcp":{"servers":{}}}`)

	if err := InstallGrafanaMCP(path, "opencode"); err != nil {
		t.Fatalf("install: %v", err)
	}

	raw, _ := readCfg(t, path)["mcp"].(map[string]any)
	entry, ok := mcppkg.CollectOpenCodeServers(raw)["grafana"].(map[string]any)
	if !ok {
		t.Fatalf("grafana not written: %v", raw)
	}
	if entry["type"] != "remote" {
		t.Errorf("type = %v, want remote", entry["type"])
	}
	if u, _ := entry["url"].(string); u != "" {
		t.Errorf("url = %q, want empty — no endpoint belongs in the repo", u)
	}
	enabled, hasEnabled := entry["enabled"].(bool)
	disabled, hasDisabled := entry["disabled"].(bool)
	off := (hasEnabled && !enabled) || (hasDisabled && disabled)
	if !off {
		t.Errorf("grafana installed switched on with no URL: %v", entry)
	}
}

// A URL the user already set must survive the next install.
func TestInstallGrafanaMCP_KeepsConfiguredURL(t *testing.T) {
	dir := t.TempDir()
	path := writeCfg(t, dir, "opencode.json",
		`{"mcp":{"servers":{"grafana":{"type":"remote","url":"https://grafana.example/mcp"}}}}`)

	if err := InstallGrafanaMCP(path, "opencode"); err != nil {
		t.Fatalf("install: %v", err)
	}
	raw, _ := readCfg(t, path)["mcp"].(map[string]any)
	entry, _ := mcppkg.CollectOpenCodeServers(raw)["grafana"].(map[string]any)
	if entry["url"] != "https://grafana.example/mcp" {
		t.Errorf("install wiped the configured endpoint: %v", entry["url"])
	}
}
