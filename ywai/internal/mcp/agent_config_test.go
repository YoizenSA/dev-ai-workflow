package mcp

// agent_config_test.go — TDD slice 3 of the "Real MCP Install" plan.
//
// These tests pin the contract of the new file ywai/internal/mcp/agent_config.go
// that will encapsulate multi-format persistence of MCP server configs across
// the three agent targets ywai supports: opencode, pi, and claude-code.
//
// RED: the file does not exist yet, so the test binary will not compile. The
// expected compile errors are:
//
//	undefined: EntryTargetPath
//	undefined: BuildEntryShape
//	undefined: WriteAgentConfig
//	undefined: RemoveAgentConfig
//	undefined: ReadAgentConfig
//
// All five symbols are pinned by the tests in this file. @dev's job is to
// add the implementation that makes them compile and pass.
//
// Target / format reference (per the slice 3 brief):
//
//	opencode     → ~/.config/opencode/opencode.json     top-level "mcp"
//	              local:  {type:"local", command:[..], env:{}, enabled:true}
//	              remote: {type:"remote", url:"..", enabled:true}
//
//	pi           → ~/.pi/agent/mcp.json                  top-level "mcpServers"
//	              local:  {command:"exe", args:[..], env:{}, enabled:true}
//	              remote: {url:"..", enabled:true}
//
//	claude-code  → ~/.claude.json                       top-level "mcpServers"
//	              local:  {command:"exe", args:[..], env:{}, enabled:true}
//	              remote: {url:"..", enabled:true}
//
// Assumptions baked into the tests:
//
//   - EntryTargetPath honors $XDG_CONFIG_HOME for opencode only (pi and
//     claude use a fixed $HOME-based path that XDG does not redirect).
//   - BuildEntryShape returns map[string]any whose values may be either
//     []string or []any for slice fields (JSON round-trip is the only
//     honest way to compare these). The tests use a JSON-roundtrip helper
//     for slice-bearing comparisons and direct equality for the simple
//     scalar fields (type, command-as-string, url, enabled).
//   - WriteAgentConfig is atomic: write-to-tmp + os.Rename. We pin this
//     by stressing the path with concurrent writes and asserting the
//     final file is still valid JSONC. We cannot simulate a real crash,
//     so this is the best behavioral proxy.
//   - WriteAgentConfig creates missing parent directories and writes the
//     file with mode 0o600 (credentials in env demand this).
//   - ReadAgentConfig on a missing file returns an empty map[string]any
//     and no error — the install path will read+modify+write, and a
//     fresh install with no prior config must not be a special case.
//   - ReadAgentConfig on malformed JSON returns an error (no silent
//     fallback to {}; that would mask real corruption).
//   - RemoveAgentConfig is idempotent: removing an entry that does not
//     exist is a no-op, not an error.
//
// Tests use stdlib only and follow the conventions of the existing
// *_test.go files in this package (per-test sections separated by
// banner comments, AAA structure, descriptive test names).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"
)

// ─── helpers ──────────────────────────────────────────────────────────────

// setTestHomeDir redirects the user home directory for the duration of the
// test. os.UserHomeDir() reads HOME on unix and USERPROFILE on Windows, so
// both must be set for these tests to resolve config paths under the temp
// dir on every CI runner.
func setTestHomeDir(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

// shapeHasKey reports whether the shape map has the given top-level key.
// Used for the "must NOT have X" assertions on BuildEntryShape output.
func shapeHasKey(shape map[string]any, key string) bool {
	_, ok := shape[key]
	return ok
}

// shapeJSONEqual asserts that got and want serialize to the same JSON.
// This handles cases where the implementation may choose []string vs.
// []any for slice fields, or map[string]string vs. map[string]any for
// env maps — the JSON form is the externally observable contract.
func shapeJSONEqual(t *testing.T, got, want map[string]any) {
	t.Helper()
	g, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	w, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	if string(g) != string(w) {
		t.Errorf("shape JSON mismatch:\n  got:  %s\n  want: %s", g, w)
	}
}

// sortedKeys returns the keys of a map[string]any sorted lexicographically.
// Used for assertions that care about presence, not order.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// parseJSONFile reads a file and decodes it as JSON (not JSONC — we use
// plain JSON in the test fixtures). Returns the decoded value.
func parseJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return out
}

// writeJSONFile writes content as JSON to path, creating parent dirs as
// needed. The file is given mode 0o644 — the test fixtures are not the
// file the production WriteAgentConfig writes, so perms don't matter
// here.
func writeJSONFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// ─── EntryTargetPath ──────────────────────────────────────────────────────

// TestEntryTargetPath_Opencode_Default pins the default path for opencode
// when no XDG_CONFIG_HOME is set: $HOME/.config/opencode/opencode.json.
// This is the most common path in the wild (most Linux users and all
// macOS users have XDG unset, defaulting to ~/.config).
func TestEntryTargetPath_Opencode_Default(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	// Explicitly clear XDG_CONFIG_HOME even if the surrounding test
	// environment has it set — this test asserts the default path.
	t.Setenv("XDG_CONFIG_HOME", "")

	got, err := EntryTargetPath("opencode")
	if err != nil {
		t.Fatalf("EntryTargetPath(opencode) err = %v, want nil", err)
	}
	want := filepath.Join(home, ".config", "opencode", "opencode.json")
	if got != want {
		t.Errorf("EntryTargetPath(opencode) = %q, want %q", got, want)
	}
}

// TestEntryTargetPath_Opencode_WithXDG pins the XDG-redirected path.
// When XDG_CONFIG_HOME is set, the opencode config dir is rooted at
// $XDG_CONFIG_HOME/opencode/opencode.json instead of $HOME/.config/...
// This is the path power-user Linux distros (NixOS, Fedora Silverblue)
// and container-friendly setups use.
func TestEntryTargetPath_Opencode_WithXDG(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	// HOME should NOT affect the result when XDG is set. We still
	// point HOME somewhere predictable so a buggy implementation
	// that ignores XDG and falls back to HOME cannot accidentally
	// produce the same string.
	t.Setenv("HOME", "/this/home/is/ignored/when/xdg/set")

	got, err := EntryTargetPath("opencode")
	if err != nil {
		t.Fatalf("EntryTargetPath(opencode) err = %v, want nil", err)
	}
	want := filepath.Join(xdg, "opencode", "opencode.json")
	if got != want {
		t.Errorf("EntryTargetPath(opencode) with XDG = %q, want %q", got, want)
	}
}

// TestEntryTargetPath_Claude pins the claude-code path: $HOME/.claude.json.
// This is a fixed path that ignores XDG_CONFIG_HOME — claude's
// installer pins the location, so we mirror it exactly.
func TestEntryTargetPath_Claude(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	// XDG must not redirect the claude path.
	t.Setenv("XDG_CONFIG_HOME", "/this/xdg/is/ignored/for/claude")

	got, err := EntryTargetPath("claude-code")
	if err != nil {
		t.Fatalf("EntryTargetPath(claude-code) err = %v, want nil", err)
	}
	want := filepath.Join(home, ".claude.json")
	if got != want {
		t.Errorf("EntryTargetPath(claude-code) = %q, want %q", got, want)
	}
}

// TestEntryTargetPath_Pi pins the pi path: $HOME/.pi/agent/mcp.json.
// Same fixed-path contract as claude: no XDG redirection.
func TestEntryTargetPath_Pi(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "/this/xdg/is/ignored/for/pi")

	got, err := EntryTargetPath("pi")
	if err != nil {
		t.Fatalf("EntryTargetPath(pi) err = %v, want nil", err)
	}
	want := filepath.Join(home, ".pi", "agent", "mcp.json")
	if got != want {
		t.Errorf("EntryTargetPath(pi) = %q, want %q", got, want)
	}
}

// TestEntryTargetPath_Unknown pins the not-found path. ywai only ships
// the three targets above; passing an unknown string must surface as
// an error (not a silent empty path, not a panic). The error message
// is NOT pinned — only the failure mode.
func TestEntryTargetPath_Unknown(t *testing.T) {
	got, err := EntryTargetPath("vim")
	if err == nil {
		t.Errorf("EntryTargetPath(vim) err = nil, path = %q, want error", got)
	}
}

// TestAgentConfig_UnknownTarget pins the error pass-through: all three
// public functions (Write/Read/Remove) must surface an error (not a
// panic, not a silent empty path) when given a target outside the
// three supported agents. The error originates in EntryTargetPath; the
// pass-through path lives at lines 102-104 / 126-128 / 160-162 of
// agent_config.go. TestEntryTargetPath_Unknown exercises the producer
// (EntryTargetPath) directly but does not reach those pass-through
// blocks — this test does.
//
// Subtests cover all three functions in one test, mirroring the
// existing single-function test naming style.
func TestAgentConfig_UnknownTarget(t *testing.T) {
	t.Run("WriteAgentConfig", func(t *testing.T) {
		if _, err := WriteAgentConfig("vim", "id", map[string]any{"x": 1}); err == nil {
			t.Errorf("WriteAgentConfig(vim) err = nil, want error")
		}
	})
	t.Run("RemoveAgentConfig", func(t *testing.T) {
		if err := RemoveAgentConfig("vim", "id"); err == nil {
			t.Errorf("RemoveAgentConfig(vim) err = nil, want error")
		}
	})
	t.Run("ReadAgentConfig", func(t *testing.T) {
		if _, err := ReadAgentConfig("vim"); err == nil {
			t.Errorf("ReadAgentConfig(vim) err = nil, want error")
		}
	})
}

// ─── BuildEntryShape — opencode ───────────────────────────────────────────

// TestBuildEntryShape_Opencode_Local pins the opencode local entry shape
// using github as the canonical example. Opencode's format keeps the
// command as a single argv slice (not split into command+args) and tags
// the entry with type="local". With creds, the env map must contain the
// GITHUB_PERSONAL_ACCESS_TOKEN that github's spec requires.
func TestBuildEntryShape_Opencode_Local(t *testing.T) {
	github, ok := CatalogByID("github")
	if !ok {
		t.Fatal("CatalogByID(github) ok=false, want true (catalog regression)")
	}
	creds := map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "xxx"}

	got := BuildEntryShape("opencode", github, creds)

	// type must be "local" (string).
	if v, ok := got["type"].(string); !ok || v != "local" {
		t.Errorf("opencode github shape type = %v (%T), want \"local\" string",
			got["type"], got["type"])
	}
	// enabled must be true.
	if v, ok := got["enabled"].(bool); !ok || !v {
		t.Errorf("opencode github shape enabled = %v (%T), want true",
			got["enabled"], got["enabled"])
	}
	// command must be a slice with exactly the 3 argv tokens the
	// catalog pins. We compare via JSON round-trip so the test is
	// robust to []string vs. []any.
	want := map[string]any{
		"type":    "local",
		"command": []string{"npx", "-y", "@modelcontextprotocol/server-github"},
		"env":     map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "xxx"},
		"enabled": true,
	}
	shapeJSONEqual(t, got, want)
}

// TestBuildEntryShape_Opencode_Remote pins the opencode remote entry
// shape using context7. Remotes have type="remote" and url=..., with
// no command field at all (the runtime talks to the URL directly via
// HTTP — no subprocess to launch).
func TestBuildEntryShape_Opencode_Remote(t *testing.T) {
	context7, ok := CatalogByID("context7")
	if !ok {
		t.Fatal("CatalogByID(context7) ok=false, want true (catalog regression)")
	}

	got := BuildEntryShape("opencode", context7, nil)

	// type must be "remote".
	if v, ok := got["type"].(string); !ok || v != "remote" {
		t.Errorf("opencode context7 shape type = %v (%T), want \"remote\" string",
			got["type"], got["type"])
	}
	// url must be the catalog URL.
	if v, ok := got["url"].(string); !ok || v != "https://mcp.context7.com/mcp" {
		t.Errorf("opencode context7 shape url = %v (%T), want \"https://mcp.context7.com/mcp\"",
			got["url"], got["url"])
	}
	// enabled must be true.
	if v, ok := got["enabled"].(bool); !ok || !v {
		t.Errorf("opencode context7 shape enabled = %v (%T), want true",
			got["enabled"], got["enabled"])
	}
	// command MUST NOT appear in a remote shape — the runtime keys
	// off its absence to pick the HTTP transport.
	if shapeHasKey(got, "command") {
		t.Errorf("opencode context7 shape has command = %v, want absent (remote has no subprocess)",
			got["command"])
	}
}

// ─── BuildEntryShape — claude-code ────────────────────────────────────────

// TestBuildEntryShape_Claude_Local pins the claude-code local entry
// shape. Unlike opencode, claude splits the argv: the executable is
// in command (as a STRING, not a slice) and the rest goes in args
// (as a slice). The shape does NOT have a "type" field — claude
// infers transport from the presence of command vs. url.
func TestBuildEntryShape_Claude_Local(t *testing.T) {
	github, ok := CatalogByID("github")
	if !ok {
		t.Fatal("CatalogByID(github) ok=false, want true (catalog regression)")
	}
	creds := map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "xxx"}

	got := BuildEntryShape("claude-code", github, creds)

	// command must be a STRING (not a slice). claude's schema
	// pins this explicitly: exec-style, not argv-style.
	if v, ok := got["command"].(string); !ok || v != "npx" {
		t.Errorf("claude github shape command = %v (%T), want \"npx\" string",
			got["command"], got["command"])
	}
	// args must be a slice with the rest of the argv.
	wantArgs := []string{"-y", "@modelcontextprotocol/server-github"}
	gotArgs, ok := got["args"]
	if !ok {
		t.Errorf("claude github shape args missing, want %v", wantArgs)
	} else {
		gJSON, _ := json.Marshal(gotArgs)
		wJSON, _ := json.Marshal(wantArgs)
		if string(gJSON) != string(wJSON) {
			t.Errorf("claude github shape args = %s, want %s", gJSON, wJSON)
		}
	}
	// env must contain the creds.
	want := map[string]any{
		"command": "npx",
		"args":    wantArgs,
		"env":     map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "xxx"},
		"enabled": true,
	}
	shapeJSONEqual(t, got, want)
	// type must NOT appear (claude has no "type" tag).
	if shapeHasKey(got, "type") {
		t.Errorf("claude github shape has type = %v, want absent (claude infers from command/url)",
			got["type"])
	}
}

// TestBuildEntryShape_Claude_Remote pins the claude-code remote shape.
// Remotes have only url and enabled — no command, no args, no env.
func TestBuildEntryShape_Claude_Remote(t *testing.T) {
	context7, ok := CatalogByID("context7")
	if !ok {
		t.Fatal("CatalogByID(context7) ok=false, want true (catalog regression)")
	}

	got := BuildEntryShape("claude-code", context7, nil)

	if v, ok := got["url"].(string); !ok || v != "https://mcp.context7.com/mcp" {
		t.Errorf("claude context7 shape url = %v (%T), want \"https://mcp.context7.com/mcp\"",
			got["url"], got["url"])
	}
	if v, ok := got["enabled"].(bool); !ok || !v {
		t.Errorf("claude context7 shape enabled = %v (%T), want true",
			got["enabled"], got["enabled"])
	}
	if shapeHasKey(got, "command") {
		t.Errorf("claude context7 shape has command = %v, want absent (remote has no command)",
			got["command"])
	}
	if shapeHasKey(got, "args") {
		t.Errorf("claude context7 shape has args = %v, want absent (remote has no args)",
			got["args"])
	}
}

// ─── BuildEntryShape — pi ─────────────────────────────────────────────────

// TestBuildEntryShape_Pi_Local pins the pi local entry shape. The pi
// format mirrors claude-code exactly: command as a string, args as a
// slice, env, enabled, no "type" field. This test exists as a separate
// pin so a future divergence between pi and claude is caught
// immediately — the two formats are not the same by accident, they
// are the same by historical convention.
func TestBuildEntryShape_Pi_Local(t *testing.T) {
	github, ok := CatalogByID("github")
	if !ok {
		t.Fatal("CatalogByID(github) ok=false, want true (catalog regression)")
	}
	creds := map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "xxx"}

	got := BuildEntryShape("pi", github, creds)

	if v, ok := got["command"].(string); !ok || v != "npx" {
		t.Errorf("pi github shape command = %v (%T), want \"npx\" string",
			got["command"], got["command"])
	}
	want := map[string]any{
		"command": "npx",
		"args":    []string{"-y", "@modelcontextprotocol/server-github"},
		"env":     map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "xxx"},
		"enabled": true,
	}
	shapeJSONEqual(t, got, want)
}

// ─── BuildEntryShape — env omission ───────────────────────────────────────

// TestBuildEntryShape_NoCreds_OmitsEnv pins the no-credentials branch.
// A local entry whose creds map is nil (or empty) must NOT emit an env
// key — leaving env out is the JSON idiom for "no env vars", and some
// agent runtimes treat an empty env object as a contract violation
// (e.g. they require the field to be absent). github is the test
// vehicle because it has an env spec, so the no-creds path is
// meaningful (not vacuously true).
func TestBuildEntryShape_NoCreds_OmitsEnv(t *testing.T) {
	github, ok := CatalogByID("github")
	if !ok {
		t.Fatal("CatalogByID(github) ok=false, want true (catalog regression)")
	}

	got := BuildEntryShape("opencode", github, nil)

	if shapeHasKey(got, "env") {
		t.Errorf("opencode github shape (no creds) has env = %v, want absent",
			got["env"])
	}
	// Also pin the empty-map variant — both nil and map[string]string{}
	// should produce the same "no env" outcome. An implementation that
	// only handles nil but not the empty map would still leak env={}
	// for the empty-map case, which is the bug this subtest catches.
	got2 := BuildEntryShape("opencode", github, map[string]string{})
	if shapeHasKey(got2, "env") {
		t.Errorf("opencode github shape (empty creds) has env = %v, want absent",
			got2["env"])
	}
}

// ─── WriteAgentConfig + ReadAgentConfig — opencode ────────────────────────

// TestWriteAgentConfig_Opencode_PreservesSiblings pins the
// read-modify-write contract: when the opencode config already has
// sibling content (here, the mcp section has a pre-existing entry AND
// there is a top-level "otherKey" the install must NOT touch), writing
// a new entry must preserve both. This is the regression that catches
// a "rewrite the file from scratch" implementation that drops
// unrelated keys.
func TestWriteAgentConfig_Opencode_PreservesSiblings(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	initial := `{"mcp":{"existing":{"type":"local","command":["echo"]}},"otherKey":"value"}`
	writeJSONFile(t, cfgPath, initial)

	github, _ := CatalogByID("github")
	shape := BuildEntryShape("opencode", github, nil)

	returned, err := WriteAgentConfig("opencode", "github", shape)
	if err != nil {
		t.Fatalf("WriteAgentConfig err = %v, want nil", err)
	}
	if returned != cfgPath {
		t.Errorf("WriteAgentConfig returned path = %q, want %q", returned, cfgPath)
	}

	// Re-read via the file directly (not ReadAgentConfig — we are
	// testing the *file* state here, and ReadAgentConfig strips
	// the top-level wrapper).
	cfg := parseJSONFile(t, cfgPath)
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("opencode cfg[mcp] = %v (%T), want map[string]any",
			cfg["mcp"], cfg["mcp"])
	}
	if _, has := mcp["existing"]; !has {
		t.Errorf("mcp section lost pre-existing 'existing' entry; mcp = %v", mcp)
	}
	if _, has := mcp["github"]; !has {
		t.Errorf("mcp section missing newly-written 'github' entry; mcp = %v", mcp)
	}
	if other, _ := cfg["otherKey"].(string); other != "value" {
		t.Errorf("top-level otherKey = %q, want \"value\" (must be preserved)", other)
	}
}

// TestWriteAgentConfig_Opencode_Overwrites pins that writing the same
// entryID twice replaces, it does not duplicate. A naive
// "append-to-map" implementation would create two "github" keys
// (impossible in JSON, but the implementation might merge by key and
// still produce a buggy result with mixed fields). This test ensures
// the second write fully replaces the first.
func TestWriteAgentConfig_Opencode_Overwrites(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	initial := `{"mcp":{"github":{"type":"local","command":["old-cmd"],"env":{}}}}`
	writeJSONFile(t, cfgPath, initial)

	github, _ := CatalogByID("github")
	shape := BuildEntryShape("opencode", github, nil)

	if _, err := WriteAgentConfig("opencode", "github", shape); err != nil {
		t.Fatalf("WriteAgentConfig err = %v, want nil", err)
	}

	cfg := parseJSONFile(t, cfgPath)
	mcp := cfg["mcp"].(map[string]any)
	gh := mcp["github"].(map[string]any)
	cmd, _ := gh["command"].([]any)
	if len(cmd) == 0 {
		t.Fatalf("github command = %v, want non-empty after overwrite", cmd)
	}
	if cmd[0] == "old-cmd" {
		t.Errorf("github command[0] = %q, want the new value (old value not overwritten)",
			cmd[0])
	}
	if cmd[0] != "npx" {
		t.Errorf("github command[0] = %q, want \"npx\" (catalog's first argv token)",
			cmd[0])
	}
	// Critical: only ONE github entry, not two. With a map this is
	// impossible to have literally twice, but the test pins the
	// invariant that the mcp section size for this case is 1.
	if len(mcp) != 1 {
		t.Errorf("mcp section has %d entries, want 1 (overwrite must not duplicate): keys = %v",
			len(mcp), sortedKeys(mcp))
	}
}

// TestWriteAgentConfig_Opencode_CreatesFile pins the first-install
// path: the opencode config file does not exist yet, and the install
// must create it from scratch (no error, no panic). After the write,
// the file must exist and contain the new entry.
func TestWriteAgentConfig_Opencode_CreatesFile(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatalf("precondition: cfgPath must not exist, but os.Stat = %v", err)
	}

	github, _ := CatalogByID("github")
	shape := BuildEntryShape("opencode", github, nil)

	if _, err := WriteAgentConfig("opencode", "github", shape); err != nil {
		t.Fatalf("WriteAgentConfig err = %v, want nil", err)
	}

	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("post-write os.Stat = %v, want nil (file should exist)", err)
	}
	if info.Size() == 0 {
		t.Errorf("post-write file is empty, want non-empty JSON content")
	}

	cfg := parseJSONFile(t, cfgPath)
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("mcp section missing or wrong type: %v (%T)", cfg["mcp"], cfg["mcp"])
	}
	if _, has := mcp["github"]; !has {
		t.Errorf("mcp section missing 'github' entry; mcp = %v", mcp)
	}
}

// TestWriteAgentConfig_Opencode_CreatesDir pins the deeper case: not
// only does the file not exist, the parent directory does not exist
// either. The implementation must MkdirAll the chain
// $HOME/.config/opencode/ before writing. This is the cold-install
// case on a fresh $HOME.
func TestWriteAgentConfig_Opencode_CreatesDir(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	parentDir := filepath.Dir(cfgPath)
	if _, err := os.Stat(parentDir); !os.IsNotExist(err) {
		t.Fatalf("precondition: parent dir %s must not exist, but os.Stat = %v",
			parentDir, err)
	}

	github, _ := CatalogByID("github")
	shape := BuildEntryShape("opencode", github, nil)

	if _, err := WriteAgentConfig("opencode", "github", shape); err != nil {
		t.Fatalf("WriteAgentConfig err = %v, want nil", err)
	}

	if _, err := os.Stat(parentDir); err != nil {
		t.Errorf("post-write parent dir os.Stat = %v, want nil (dir should be created)", err)
	}
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("post-write file os.Stat = %v, want nil (file should be created)", err)
	}
}

// TestWriteAgentConfig_Opencode_FilePerms pins that the written file
// has mode 0o600. The file contains credentials (the env map has
// GITHUB_PERSONAL_ACCESS_TOKEN); world-readable mode 0o644 would leak
// the token to other local users. The umask may further restrict the
// effective perms, so we pin that the explicit mode is 0o600 — at
// minimum — by checking that the file is NOT world-readable
// (perm.Perm()&0o004 == 0).
func TestWriteAgentConfig_Opencode_FilePerms(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")

	github, _ := CatalogByID("github")
	creds := map[string]string{"GITHUB_PERSONAL_ACCESS_TOKEN": "super-secret"}
	shape := BuildEntryShape("opencode", github, creds)

	if _, err := WriteAgentConfig("opencode", "github", shape); err != nil {
		t.Fatalf("WriteAgentConfig err = %v, want nil", err)
	}

	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("os.Stat = %v, want nil", err)
	}
	mode := info.Mode().Perm()
	// The file MUST NOT be world-readable. A token-bearing file
	// that another local user can read is a credential leak.
	if mode&0o004 != 0 {
		t.Errorf("cfg file mode = %o, want no world-read bit (token leak risk)", mode)
	}
	// And the file MUST be readable/writable by the owner. 0o600
	// is the documented contract; 0o400 would block the next install.
	if mode&0o600 == 0 {
		t.Errorf("cfg file mode = %o, want owner read+write (0o6xx)", mode)
	}
}

// ─── WriteAgentConfig + ReadAgentConfig — claude-code ─────────────────────

// TestWriteAgentConfig_Claude_TopKey pins that claude-code writes under
// the top-level key "mcpServers", not "mcp" (which is opencode's
// key). A wrong top-level key would cause claude to silently ignore
// the entry on its next start.
func TestWriteAgentConfig_Claude_TopKey(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".claude.json")

	github, _ := CatalogByID("github")
	shape := BuildEntryShape("claude-code", github, nil)

	returned, err := WriteAgentConfig("claude-code", "github", shape)
	if err != nil {
		t.Fatalf("WriteAgentConfig err = %v, want nil", err)
	}
	if returned != cfgPath {
		t.Errorf("WriteAgentConfig returned path = %q, want %q", returned, cfgPath)
	}

	cfg := parseJSONFile(t, cfgPath)
	// claude uses "mcpServers", not "mcp".
	if shapeHasKey(cfg, "mcp") {
		t.Errorf("claude cfg has top-level 'mcp' key, want 'mcpServers'")
	}
	mcpServers, ok := cfg["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("claude cfg[mcpServers] = %v (%T), want map[string]any",
			cfg["mcpServers"], cfg["mcpServers"])
	}
	if _, has := mcpServers["github"]; !has {
		t.Errorf("claude mcpServers missing 'github' entry; mcpServers = %v", mcpServers)
	}
}

// TestWriteAgentConfig_Atomic pins the atomicity contract. We cannot
// simulate a real crash mid-write, so the best behavioral proxy is
// to stress the write path with N concurrent writers and assert that
// the final file is still valid JSON. A non-atomic write (plain
// os.WriteFile to the target path) would, under concurrency, produce
// a half-written file that fails to parse — that's the failure mode
// we are catching here.
//
// 50 goroutines all write the same entryID with distinct shapes. The
// final file must be valid JSON, must have a single "github" entry,
// and the entry must equal one of the shapes we wrote (so we know
// the write actually completed, not just landed on a no-op).
func TestWriteAgentConfig_Atomic(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")

	github, _ := CatalogByID("github")
	const N = 50
	shapes := make([]map[string]any, N)
	for i := 0; i < N; i++ {
		// Each writer uses a distinct env value, so any one
		// of them "wins" — but the final state must be
		// self-consistent (no torn write, no merge of two
		// different writes).
		creds := map[string]string{
			"GITHUB_PERSONAL_ACCESS_TOKEN": "tok-" + itoa(i),
		}
		shapes[i] = BuildEntryShape("opencode", github, creds)
	}

	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			if _, err := WriteAgentConfig("opencode", "github", shapes[i]); err != nil {
				t.Errorf("concurrent WriteAgentConfig[%d] err = %v", i, err)
			}
		}()
	}
	wg.Wait()

	// Final file MUST be valid JSON. This is the atomicity pin:
	// a non-atomic implementation would produce a half-written
	// file that fails to parse here.
	cfg := parseJSONFile(t, cfgPath)
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("cfg[mcp] = %v (%T), want map[string]any (atomicity violated?)",
			cfg["mcp"], cfg["mcp"])
	}
	gh, ok := mcp["github"].(map[string]any)
	if !ok {
		t.Fatalf("mcp[github] = %v (%T), want map[string]any", mcp["github"], mcp["github"])
	}
	// The final github entry must match one of the shapes we wrote.
	// If the file is a torn merge of two shapes, this equality will
	// fail. (JSON round-trip of []any vs []string is the same JSON,
	// so this equality is well-defined.)
	matched := false
	for _, s := range shapes {
		if reflect.DeepEqual(gh, s) {
			matched = true
			break
		}
	}
	if !matched {
		t.Errorf("final github entry = %v, want it to equal exactly one of the %d shapes we wrote "+
			"(torn write?)", gh, N)
	}
	// And the mcp section must have exactly one entry — the
	// concurrent writers all used the same entryID, so a final
	// count > 1 would be a bug.
	if len(mcp) != 1 {
		t.Errorf("mcp section has %d entries, want 1 (overwrite semantics): keys = %v",
			len(mcp), sortedKeys(mcp))
	}
}

// ─── ReadAgentConfig ──────────────────────────────────────────────────────

// TestReadAgentConfig_Opencode pins the read path: after writing two
// entries, ReadAgentConfig("opencode") returns the mcp section as a
// map[string]any. This is the symmetric counterpart of WriteAgentConfig
// — the install UI will read the current state to render the
// "already installed" badges on the catalog rows.
func TestReadAgentConfig_Opencode(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	initial := `{"mcp":{"github":{"type":"local","command":["npx"]},"context7":{"type":"remote","url":"https://mcp.context7.com/mcp"}}}`
	writeJSONFile(t, cfgPath, initial)

	section, err := ReadAgentConfig("opencode")
	if err != nil {
		t.Fatalf("ReadAgentConfig(opencode) err = %v, want nil", err)
	}
	if len(section) != 2 {
		t.Errorf("ReadAgentConfig(opencode) section has %d entries, want 2: keys = %v",
			len(section), sortedKeys(section))
	}
	for _, want := range []string{"github", "context7"} {
		if _, ok := section[want]; !ok {
			t.Errorf("ReadAgentConfig section missing %q; keys = %v",
				want, sortedKeys(section))
		}
	}
}

// TestReadAgentConfig_NoFile pins the cold-start path: the file does
// not exist (fresh install, no prior config). The function must
// return an empty map and NO error — the caller will treat this as
// "nothing installed yet" and proceed with the first install.
//
// We pin map[string]any{} (non-nil empty map) rather than nil. The
// install code will iterate the map without a nil check; returning
// nil would force every caller to write `if section == nil { ... }`
// which is exactly the kind of footgun this empty-map contract
// exists to prevent.
func TestReadAgentConfig_NoFile(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	section, err := ReadAgentConfig("opencode")
	if err != nil {
		t.Fatalf("ReadAgentConfig(opencode) on missing file err = %v, want nil", err)
	}
	if section == nil {
		t.Errorf("ReadAgentConfig(opencode) on missing file = nil, want non-nil empty map")
	}
	if len(section) != 0 {
		t.Errorf("ReadAgentConfig(opencode) on missing file has %d entries, want 0: %v",
			len(section), section)
	}
}

// TestReadAgentConfig_MalformedJSONC pins the failure path: the file
// exists but is not valid JSON. The function must surface an error
// rather than silently returning an empty map (which would mask real
// corruption). The install UI needs to know "config is broken" so
// it can prompt the user to repair or back it up.
func TestReadAgentConfig_MalformedJSONC(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	// Truncated JSON: open-brace never closed, plus garbage.
	writeJSONFile(t, cfgPath, `{"mcp":{"github":{"type":"local"`)

	if _, err := ReadAgentConfig("opencode"); err == nil {
		t.Errorf("ReadAgentConfig(malformed) err = nil, want error")
	}
}

// ─── RemoveAgentConfig ────────────────────────────────────────────────────

// TestRemoveAgentConfig_Opencode pins the basic remove path: write
// two entries, remove one, the other must remain. The mcp section
// must end with exactly one entry.
func TestRemoveAgentConfig_Opencode(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	initial := `{"mcp":{"github":{"type":"local","command":["x"]},"git":{"type":"local","command":["y"]}}}`
	writeJSONFile(t, cfgPath, initial)

	if err := RemoveAgentConfig("opencode", "github"); err != nil {
		t.Fatalf("RemoveAgentConfig(opencode, github) err = %v, want nil", err)
	}

	cfg := parseJSONFile(t, cfgPath)
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("mcp = %v (%T), want map[string]any", cfg["mcp"], cfg["mcp"])
	}
	if len(mcp) != 1 {
		t.Errorf("mcp has %d entries, want 1: keys = %v", len(mcp), sortedKeys(mcp))
	}
	if _, has := mcp["github"]; has {
		t.Errorf("mcp still has 'github' after remove; mcp = %v", mcp)
	}
	if _, has := mcp["git"]; !has {
		t.Errorf("mcp lost 'git' during remove; mcp = %v", mcp)
	}
}

// TestRemoveAgentConfig_NonExistent pins the idempotent remove
// contract: removing an entry that is not in the file is a no-op,
// not an error. The install UI may issue removes based on stale
// state; a non-existent entry must not surface as a user-visible
// error.
func TestRemoveAgentConfig_NonExistent(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	initial := `{"mcp":{"github":{"type":"local","command":["x"]}}}`
	writeJSONFile(t, cfgPath, initial)

	if err := RemoveAgentConfig("opencode", "never-installed"); err != nil {
		t.Errorf("RemoveAgentConfig on missing entry err = %v, want nil (idempotent)", err)
	}

	// The existing entry must NOT have been touched.
	cfg := parseJSONFile(t, cfgPath)
	mcp := cfg["mcp"].(map[string]any)
	if _, has := mcp["github"]; !has {
		t.Errorf("github entry lost during idempotent remove; mcp = %v", mcp)
	}
}

// TestRemoveAgentConfig_PreservesSiblings pins the read-modify-write
// symmetry: removing one entry must not affect other entries, must
// not affect other top-level keys in the file, and must keep the
// file syntactically valid. This is the uninstall-all-except-one
// path; a buggy implementation that rebuilds the file from scratch
// would silently drop other top-level keys (e.g. "$schema",
// "theme", plugin config).
func TestRemoveAgentConfig_PreservesSiblings(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	initial := `{"mcp":{"github":{"type":"local","command":["x"]},"git":{"type":"local","command":["y"]},"context7":{"type":"remote","url":"https://mcp.context7.com/mcp"}},"otherKey":"keep-me","$schema":"https://example.com/schema.json"}`
	writeJSONFile(t, cfgPath, initial)

	if err := RemoveAgentConfig("opencode", "github"); err != nil {
		t.Fatalf("RemoveAgentConfig err = %v, want nil", err)
	}

	cfg := parseJSONFile(t, cfgPath)
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("mcp = %v (%T), want map[string]any", cfg["mcp"], cfg["mcp"])
	}
	// github removed
	if _, has := mcp["github"]; has {
		t.Errorf("mcp still has 'github' after remove; mcp = %v", mcp)
	}
	// siblings intact
	for _, want := range []string{"git", "context7"} {
		if _, has := mcp[want]; !has {
			t.Errorf("mcp lost sibling %q during remove; mcp = %v", want, mcp)
		}
	}
	// top-level keys intact
	if other, _ := cfg["otherKey"].(string); other != "keep-me" {
		t.Errorf("top-level otherKey = %q, want \"keep-me\" (must survive remove)", other)
	}
	if schema, _ := cfg["$schema"].(string); schema != "https://example.com/schema.json" {
		t.Errorf("top-level $schema = %q, want preserved (must survive remove)", schema)
	}
}

// ─── Concurrencia ─────────────────────────────────────────────────────────

// TestConcurrentWrites_Opencode pins the parallel-install path: 5
// goroutines write 5 distinct entryIDs to the same opencode config
// concurrently. After all writers finish, the file must be valid
// JSON, must contain all 5 entries, and the entries must match the
// shapes we wrote (no torn writes, no dropped entries).
//
// This is a different stress from TestWriteAgentConfig_Atomic (which
// uses the SAME entryID to stress the overwrite path). Here the 5
// distinct entryIDs stress the read-modify-write merge: each writer
// reads the current file, adds its own entry, and writes back. A
// naive read-then-write implementation would lose entries (the
// reader sees a snapshot, the writer overwrites with the snapshot
// plus its entry, dropping the entries added by other writers in
// between).
func TestConcurrentWrites_Opencode(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")

	// Pre-seed the file with one existing entry to make sure the
	// concurrent writers also don't clobber it.
	seed := `{"mcp":{"existing":{"type":"local","command":["echo"]}}}`
	writeJSONFile(t, cfgPath, seed)

	entries := []string{"github", "git", "playwright", "context7", "engram"}
	shapes := make(map[string]map[string]any, len(entries))
	for _, id := range entries {
		entry, ok := CatalogByID(id)
		if !ok {
			t.Fatalf("CatalogByID(%q) ok=false, want true (catalog regression)", id)
		}
		shapes[id] = BuildEntryShape("opencode", entry, nil)
	}

	var wg sync.WaitGroup
	wg.Add(len(entries))
	for _, id := range entries {
		id := id
		shape := shapes[id]
		go func() {
			defer wg.Done()
			if _, err := WriteAgentConfig("opencode", id, shape); err != nil {
				t.Errorf("concurrent WriteAgentConfig(%q) err = %v", id, err)
			}
		}()
	}
	wg.Wait()

	// Final file must be valid JSON (no torn writes).
	cfg := parseJSONFile(t, cfgPath)
	mcp, ok := cfg["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("mcp = %v (%T), want map[string]any (concurrent write clobber?)",
			cfg["mcp"], cfg["mcp"])
	}

	// All 5 new entries must be present, AND the seeded "existing"
	// entry must NOT have been dropped.
	wantKeys := append([]string{"existing"}, entries...)
	sort.Strings(wantKeys)
	gotKeys := sortedKeys(mcp)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Errorf("mcp keys = %v, want %v (entries lost during concurrent merge)",
			gotKeys, wantKeys)
	}

	// Each new entry must match the shape we wrote for it.
	for id, want := range shapes {
		got, ok := mcp[id].(map[string]any)
		if !ok {
			t.Errorf("mcp[%q] = %v (%T), want map[string]any", id, mcp[id], mcp[id])
			continue
		}
		// JSON-roundtrip comparison to handle []string vs []any
		// for the command field, exactly like the BuildEntryShape
		// tests above.
		gJSON, _ := json.Marshal(got)
		wJSON, _ := json.Marshal(want)
		if string(gJSON) != string(wJSON) {
			t.Errorf("mcp[%q] JSON = %s, want %s (torn write?)",
				id, gJSON, wJSON)
		}
	}
}

// TestConcurrentReadWrite_Opencode pins the contract that a reader
// (ReadAgentConfig) running concurrently with a writer (WriteAgentConfig)
// never observes a torn or missing file. The writer holds
// lockFor(target) for the whole read-modify-write, but the reader does
// NOT acquire the lock — atomic rename (write-to-tmp + os.Rename) is
// the only thing keeping the read consistent at the FS level. This is
// a different stress from TestConcurrentWrites_Opencode (writers only)
// and TestWriteAgentConfig_Atomic (writers only): here we mix the two
// directions and assert the read path stays parseable under churn.
//
// If the atomic-rename contract regresses, the reader can see a
// half-renamed file and parseJSON-equivalent (ReadAgentConfig) will
// return an error. We assert: err == nil, section != nil, and the
// seeded "existing" entry is always present (the writer never touches
// it, so losing it would mean a torn read picked up an earlier state).
func TestConcurrentReadWrite_Opencode(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cfgPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	// Pre-seed: "existing" is the canary the reader checks on every
	// iteration. The writer never modifies it.
	seed := `{"mcp":{"existing":{"type":"local","command":["echo"]}}}`
	writeJSONFile(t, cfgPath, seed)

	// Two distinct shapes so a torn merge would be detectable as
	// "command" mixing values from A and B.
	shapeA := map[string]any{
		"type":    "local",
		"command": []any{"echo", "A"},
		"enabled": true,
	}
	shapeB := map[string]any{
		"type":    "local",
		"command": []any{"echo", "B"},
		"enabled": true,
	}

	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(2)

	// Writer: alternates shapeA and shapeB for `iterations` writes.
	// The final file is allowed to land on either shape; the contract
	// is only that no intermediate state is observable to the reader.
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			s := shapeA
			if i%2 == 1 {
				s = shapeB
			}
			if _, err := WriteAgentConfig("opencode", "stress", s); err != nil {
				t.Errorf("WriteAgentConfig iter=%d err = %v", i, err)
				return
			}
		}
	}()

	// Reader: must always see a parseable section with "existing"
	// (the writer does not touch "existing" — losing it would mean
	// the read picked up a torn or pre-existing state).
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			section, err := ReadAgentConfig("opencode")
			if err != nil {
				t.Errorf("ReadAgentConfig iter=%d err = %v (torn read?)", i, err)
				return
			}
			if section == nil {
				t.Errorf("ReadAgentConfig iter=%d section = nil, want non-nil map", i)
				return
			}
			if _, ok := section["existing"]; !ok {
				t.Errorf("ReadAgentConfig iter=%d lost 'existing'; section = %v", i, section)
				return
			}
		}
	}()

	wg.Wait()
}

// ─── itoa ─────────────────────────────────────────────────────────────────

// itoa is a tiny strconv-free integer-to-string used in the atomic
// test to label env values. Avoiding strconv keeps the test imports
// minimal (the existing tests in this package use a similar
// minimalist style).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
