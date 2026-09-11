package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcppkg "github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/testsandbox"
)

func isolateHome(t *testing.T) {
	t.Helper()
	_, cleanup := testsandbox.Isolate("ywai-manifest-test")
	t.Cleanup(cleanup)
}

func writeOverride(t *testing.T, body string) string {
	t.Helper()
	path := ManifestPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The embedded manifest is the shipped policy: it must parse and every id it
// names must have an executor wired. This test pins the default entries so a
// policy change is a conscious test change too.
func TestLoadManifest_EmbeddedDefault(t *testing.T) {
	isolateHome(t)

	mf, warnings := LoadManifest()
	if len(warnings) != 0 {
		t.Fatalf("embedded manifest produced warnings: %v", warnings)
	}
	var ids []string
	for _, e := range mf.Install {
		ids = append(ids, e.ID)
	}
	want := []string{
		"background-agents", "vision-bridge", "advisor", "tui-logo",
		"chrome-devtools", "grafana", "microsoft-learn", "meta-devtools", "ponytail",
	}
	if len(ids) != len(want) {
		t.Fatalf("embedded manifest ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("embedded manifest ids = %v, want %v", ids, want)
		}
	}
}

// An external override fully replaces the embedded policy — that is the whole
// point: retuning what installs without rebuilding ywai.
func TestLoadManifest_OverrideReplacesEmbedded(t *testing.T) {
	isolateHome(t)
	writeOverride(t, `{"install":[{"id":"grafana"}]}`)

	mf, warnings := LoadManifest()
	if len(warnings) != 0 {
		t.Fatalf("valid override produced warnings: %v", warnings)
	}
	if len(mf.Install) != 1 || mf.Install[0].ID != "grafana" {
		t.Fatalf("override not applied: %+v", mf.Install)
	}
}

// An empty install list is valid policy: the user opted out of everything.
func TestLoadManifest_EmptyOverrideMeansInstallNothing(t *testing.T) {
	isolateHome(t)
	writeOverride(t, `{"install":[]}`)

	mf, warnings := LoadManifest()
	if len(warnings) != 0 {
		t.Fatalf("empty override produced warnings: %v", warnings)
	}
	if len(mf.Install) != 0 {
		t.Fatalf("install = %+v, want empty", mf.Install)
	}
}

// A bad hand edit must never half-apply: ywai falls back to the embedded
// policy and says so.
func TestLoadManifest_MalformedOverrideFallsBackToEmbedded(t *testing.T) {
	isolateHome(t)
	path := writeOverride(t, `{"install":[{"id":"no-such-thing"}]}`)

	mf, warnings := LoadManifest()
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly the override warning", warnings)
	}
	if !strings.Contains(warnings[0], path) {
		t.Errorf("warning %q does not name the override path", warnings[0])
	}
	if len(mf.Install) == 0 {
		t.Fatal("fell back to nothing instead of the embedded manifest")
	}
	for _, e := range mf.Install {
		if e.ID == "no-such-thing" {
			t.Fatal("the invalid override entry leaked into the resolved manifest")
		}
	}
}

func TestParseManifest_Rejects(t *testing.T) {
	cases := map[string]string{
		"unknown id":   `{"install":[{"id":"nope"}]}`,
		"duplicate id": `{"install":[{"id":"grafana"},{"id":"grafana"}]}`,
		"missing id":   `{"install":[{"agents":["opencode"]}]}`,
		"bad flavor":   `{"install":[{"id":"grafana","flavor":"v3"}]}`,
		"not json":     `{"install":`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseManifest([]byte(doc)); err == nil {
				t.Errorf("parseManifest(%s) accepted invalid document", doc)
			}
		})
	}
}

// Installers read an existing agent config (SettingsPaths only returns real
// files), so the run tests seed one first.
func seedConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunManifest_FlagAndAgentGating(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "opencode.json")
	seedConfig(t, configPath, `{"mcp":{}}`)
	mf := Manifest{Install: []ManifestEntry{
		{ID: "microsoft-learn", Flag: "mcp"},
		{ID: "ponytail", Agents: []string{"claude-code"}},
	}}

	// Neither applies: flag off, agent not in list. Silent — policy working.
	if got := RunManifest(mf, "opencode", configPath, nil); len(got) != 0 {
		t.Fatalf("results = %+v, want none", got)
	}

	// Flag on: the entry runs and really writes the config entry.
	if got := RunManifest(mf, "opencode", configPath, map[string]bool{"mcp": true}); len(got) != 1 {
		t.Fatalf("results = %+v, want exactly microsoft-learn", got)
	} else if got[0].ID != "microsoft-learn" || got[0].Err != nil {
		t.Fatalf("microsoft-learn result = %+v, want clean install", got[0])
	}
	mcpMap, _ := readJSONFile(t, configPath)["mcp"].(map[string]any)
	servers := mcppkg.CollectOpenCodeServers(mcpMap)
	if _, ok := servers["microsoft-learn"]; !ok {
		t.Fatalf("microsoft-learn not written: %v", servers)
	}
	if _, ok := servers["ponytail"]; ok {
		t.Fatal("ponytail must not install for an agent outside its list")
	}
}

// The v2 flavor gate surfaces as a skip reason, not a silent no-op and not an
// error: on v1 the entry is expected to stay off, and the output says why.
func TestRunManifest_V2OnlySkipsOnV1(t *testing.T) {
	// TestMain pins this package to v1.
	mf := Manifest{Install: []ManifestEntry{{ID: "background-agents", Flavor: "v2"}}}

	got := RunManifest(mf, "opencode", filepath.Join(t.TempDir(), "opencode.json"), nil)
	if len(got) != 1 || got[0].Skipped == "" || got[0].Err != nil {
		t.Fatalf("results = %+v, want one skipped entry", got)
	}
}

// Dispatch must really run the executor for a catalog MCP: one manifest entry
// in, a chrome-devtools entry in the agent config out.
func TestRunManifest_DispatchesToExecutor(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "claude.json")
	seedConfig(t, configPath, `{"mcpServers":{}}`)
	mf := Manifest{Install: []ManifestEntry{{ID: "chrome-devtools"}}}

	got := RunManifest(mf, "claude-code", configPath, nil)
	if len(got) != 1 || got[0].Err != nil {
		t.Fatalf("results = %+v, want clean chrome-devtools install", got)
	}
	servers, _ := readJSONFile(t, configPath)["mcpServers"].(map[string]any)
	if _, ok := servers["chrome-devtools"]; !ok {
		t.Fatalf("chrome-devtools not written: %v", servers)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
