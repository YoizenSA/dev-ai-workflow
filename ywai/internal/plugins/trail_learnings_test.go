package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

const settingsWithTrailStop = `{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "node \"/h/.claude/helpers/graft-hooks.cjs\" session-start || exit 0"}]}],
    "Stop": [
      {"hooks": [{"type": "command", "command": "python3 /h/.ywai/claude-wrist-hook.py"}]},
      {"hooks": [{"type": "command", "command": "node \"/h/.claude/helpers/graft-hooks.cjs\" stop || exit 0", "timeout": 10}]},
      {"hooks": [{"type": "command", "command": "node \"${CLAUDE_PROJECT_DIR:-.}/.trail/hook.cjs\" stop || exit 0"}]}
    ]
  }
}`

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRemoveTrailStopHook(t *testing.T) {
	p := writeTemp(t, "settings.json", settingsWithTrailStop)

	changed, err := RemoveTrailStopHook(p)
	if err != nil || !changed {
		t.Fatalf("first run: changed=%v err=%v", changed, err)
	}
	data, _ := os.ReadFile(p)
	s := string(data)
	if strings.Contains(s, `graft-hooks.cjs\" stop`) || strings.Contains(s, `hook.cjs\" stop`) {
		t.Fatalf("trail stop hook still present:\n%s", s)
	}
	for _, keep := range []string{"session-start", "claude-wrist-hook.py"} {
		if !strings.Contains(s, keep) {
			t.Fatalf("lost unrelated hook %q:\n%s", keep, s)
		}
	}
	root, _ := config.ReadJSONC(p)
	if stop := root["hooks"].(map[string]any)["Stop"].([]any); len(stop) != 1 {
		t.Fatalf("want 1 Stop group left, got %d", len(stop))
	}

	changed, err = RemoveTrailStopHook(p)
	if err != nil || changed {
		t.Fatalf("second run must be a no-op: changed=%v err=%v", changed, err)
	}
}

func TestRemoveTrailStopHookAbsent(t *testing.T) {
	body := `{"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "python3 x.py"}]}]}}`
	p := writeTemp(t, "settings.json", body)
	changed, err := RemoveTrailStopHook(p)
	if err != nil || changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if data, _ := os.ReadFile(p); string(data) != body {
		t.Fatalf("file rewritten without a trail hook:\n%s", data)
	}
}

func TestUpsertTrailLearningsSection(t *testing.T) {
	handWritten := "# Rules\n\nkeep me\n\n## Trail learnings → Engram\n\n" + trailLearningsBody + "\n"
	p := writeTemp(t, "CLAUDE.md", handWritten)

	for i := 0; i < 2; i++ {
		if err := UpsertTrailLearningsSection(p); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := os.ReadFile(p)
	s := string(data)
	if n := strings.Count(s, "## Trail learnings → Engram"); n != 1 {
		t.Fatalf("want heading once, got %d:\n%s", n, s)
	}
	if !strings.Contains(s, "<!-- BEGIN ywai:trail-learnings -->") || !strings.Contains(s, "keep me") {
		t.Fatalf("missing managed block or lost content:\n%s", s)
	}
}

func TestUpsertTrailLearningsSectionFresh(t *testing.T) {
	p := writeTemp(t, "AGENTS.md", "# Agents\n")
	if err := UpsertTrailLearningsSection(p); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	if strings.Count(string(data), "mem_save") != 1 {
		t.Fatalf("section not written once:\n%s", data)
	}
}
