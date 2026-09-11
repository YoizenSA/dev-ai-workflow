package cleanup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sandboxHome builds a fake home directory with one of every retired artifact.
func sandboxHome(t *testing.T) (home, repo string) {
	t.Helper()
	home = t.TempDir()
	repo = t.TempDir()
	must := func(p, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oc := filepath.Join(home, ".config", "opencode")
	must(filepath.Join(home, ".claude", "skills", "skill-registry", "index.md"), "x")
	must(filepath.Join(home, ".claude", "skills", ".atl", "t"), "x")
	must(filepath.Join(oc, "skills", "skill-registry", "index.md"), "x")
	must(filepath.Join(oc, ".atl", "t"), "x")
	must(filepath.Join(oc, "agents", "qa-finder.md"), "old agent")
	must(filepath.Join(oc, "agents", "orchestrator.md"), "KEEP ME")
	must(filepath.Join(oc, "agents", "orchestrator.md.bak"), "old backup")
	must(filepath.Join(oc, "plugins", "engram.ts"), "x")
	must(filepath.Join(oc, "plugins", "subagent-statusline-server.js"), "x")
	must(filepath.Join(oc, "plugins", "subagent-statusline-tui", "x.js"), "x")
	must(filepath.Join(oc, "opencode-quota", "x.js"), "x")
	must(filepath.Join(repo, ".atl", "t"), "x")
	must(filepath.Join(repo, ".codegraph", "t"), "x")
	must(filepath.Join(repo, ".git", "HEAD"), "x")
	must(filepath.Join(oc, "opencode.json"), `{
  "plugins": ["@slkiser/opencode-quota", "background-agents.js", "@dietrichgebert/ponytail"],
  "plugin": ["@slkiser/opencode-quota"],
  "model": "keep-me"
}`)
	must(filepath.Join(oc, "cli.json"), `{"plugins": ["subagent-statusline-tui/server.js", "advisor.js"]}`)
	must(filepath.Join(home, ".claude", "settings.json"), `{
  "hooks": {
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "ywai skill-registry refresh"}]}],
    "PostToolUse": [{"hooks": [{"type": "command", "command": "echo keep"}]}]
  }
}`)
	return home, repo
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return root
}

func TestCleanDryRunChangesNothing(t *testing.T) {
	home, repo := sandboxHome(t)
	oc := filepath.Join(home, ".config", "opencode")

	actions := Run(Options{Home: home, Repo: repo, Apply: false})

	if len(actions) < 10 {
		t.Fatalf("expected a full report, got %d actions", len(actions))
	}
	for _, p := range []string{
		filepath.Join(oc, "agents", "qa-finder.md"),
		filepath.Join(oc, "agents", "orchestrator.md.bak"),
		filepath.Join(oc, "plugins", "engram.ts"),
		filepath.Join(home, ".claude", "skills", "skill-registry"),
		filepath.Join(repo, ".codegraph"),
	} {
		if !exists(p) {
			t.Errorf("dry run deleted %s", p)
		}
	}
	root := readJSON(t, filepath.Join(oc, "opencode.json"))
	if _, ok := root["plugins"]; !ok {
		t.Error("dry run rewrote opencode.json")
	}
}

func TestCleanApplyRemovesRetiredAndKeepsLive(t *testing.T) {
	home, repo := sandboxHome(t)
	oc := filepath.Join(home, ".config", "opencode")

	actions := Run(Options{Home: home, Repo: repo, Apply: true})
	if len(actions) == 0 {
		t.Fatal("expected actions")
	}

	// Removed.
	for _, p := range []string{
		filepath.Join(oc, "agents", "qa-finder.md"),
		filepath.Join(oc, "plugins", "engram.ts"),
		filepath.Join(oc, "plugins", "subagent-statusline-server.js"),
		filepath.Join(oc, "plugins", "subagent-statusline-tui"),
		filepath.Join(oc, "opencode-quota"),
		filepath.Join(oc, ".atl"),
		filepath.Join(oc, "skills", "skill-registry"),
		filepath.Join(home, ".claude", "skills", "skill-registry"),
		filepath.Join(repo, ".atl"),
		filepath.Join(repo, ".codegraph"),
	} {
		if exists(p) {
			t.Errorf("%s still exists after apply", p)
		}
	}

	// Kept.
	if !exists(filepath.Join(oc, "agents", "orchestrator.md")) {
		t.Error("live agent orchestrator.md was deleted")
	}
	if !exists(filepath.Join(repo, ".git", "HEAD")) {
		t.Error("repo walk entered .git")
	}

	// *.bak moved, not deleted.
	if exists(filepath.Join(oc, "agents", "orchestrator.md.bak")) {
		t.Error("backup was not moved out of the agents dir")
	}
	backup := filepath.Join(home, ".ywai", "agent-backups", "orchestrator.md.bak")
	data, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("backup missing after move: %v", err)
	}
	if string(data) != "old backup" {
		t.Error("backup content changed")
	}

	// opencode.json: quota + ponytail gone, live entry and other keys kept.
	root := readJSON(t, filepath.Join(oc, "opencode.json"))
	plugins, _ := root["plugins"].([]any)
	if len(plugins) != 1 || plugins[0] != "background-agents.js" {
		t.Errorf("plugins = %v, want [background-agents.js]", plugins)
	}
	if _, ok := root["plugin"]; ok {
		t.Error("empty legacy 'plugin' key should be dropped")
	}
	if root["model"] != "keep-me" {
		t.Error("unrelated keys must survive")
	}

	// cli.json: statusline entry gone, advisor kept.
	cli := readJSON(t, filepath.Join(oc, "cli.json"))
	cliPlugins, _ := cli["plugins"].([]any)
	if len(cliPlugins) != 1 || cliPlugins[0] != "advisor.js" {
		t.Errorf("cli.json plugins = %v, want [advisor.js]", cliPlugins)
	}

	// settings.json: retired hook gone, unrelated hook kept.
	settings := readJSON(t, filepath.Join(home, ".claude", "settings.json"))
	hooks := settings["hooks"].(map[string]any)
	if _, ok := hooks["UserPromptSubmit"]; ok {
		t.Error("retired UserPromptSubmit hook still present")
	}
	if _, ok := hooks["PostToolUse"]; !ok {
		t.Error("unrelated PostToolUse hook was removed")
	}
}

func TestCleanIsIdempotent(t *testing.T) {
	home, repo := sandboxHome(t)
	Run(Options{Home: home, Repo: repo, Apply: true})
	if again := Run(Options{Home: home, Repo: repo, Apply: true}); len(again) != 0 {
		t.Errorf("second pass reported %d actions, want 0: %v", len(again), again)
	}
}

func TestCleanSkipsUnparsableJSON(t *testing.T) {
	home := t.TempDir()
	oc := filepath.Join(home, ".config", "opencode")
	path := filepath.Join(oc, "opencode.json")
	must := func(p, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	must(path, "{\n  // JSONC comment\n  \"plugins\": [\"@slkiser/opencode-quota\"]\n}")
	Run(Options{Home: home, Repo: t.TempDir(), Apply: true})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "\"@slkiser/opencode-quota\""; !strings.Contains(string(data), want) {
		t.Errorf("unparsable config was modified; want it left alone (still containing %s)", want)
	}
}
