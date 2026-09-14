package envprofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// frontKeysAfter returns the lines that follow `key:` inside the frontmatter
// up to the next top-level key, so tests can assert where rules landed.
func linesAfterKey(t *testing.T, content, key string) []string {
	t.Helper()
	lines := strings.Split(content, "\n")
	var out []string
	in := false
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "---" {
			break
		}
		top := !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "- ")
		if top {
			in = strings.HasPrefix(l, key+":")
			continue
		}
		if in {
			out = append(out, l)
		}
	}
	return out
}

// The model profile step writes `model:` after `permissions:`; rules must
// still land inside permissions, never under model (invalid YAML → opencode
// drops the whole frontmatter and every permission with it).
func TestAppendShellDeniesStaysInsidePermissions(t *testing.T) {
	in := "---\ndescription: \"x\"\nmode: all\npermissions:\n  - action: shell\n    resource: \"*\"\n    effect: allow\nmodel: zai/glm\n---\n\nbody\n"
	out, ok := appendShellDenies(in, []string{"git commit*"})
	if !ok {
		t.Fatal("ok = false")
	}
	if got := linesAfterKey(t, out, "model"); len(got) != 0 {
		t.Fatalf("lines dangling under model: %q\n%s", got, out)
	}
	perm := strings.Join(linesAfterKey(t, out, "permissions"), "\n")
	if !strings.Contains(perm, `resource: "git commit*"`) {
		t.Fatalf("rule not inside permissions:\n%s", out)
	}
	if !strings.HasSuffix(out, "---\n\nbody\n") {
		t.Fatalf("body changed:\n%s", out)
	}
}

// Files an older ywai already broke (rules under `model:`) are healed.
func TestAppendShellDeniesHealsDanglingRules(t *testing.T) {
	broken := "---\nmode: all\npermissions:\n  - action: read\n    resource: \"*\"\n    effect: allow\nmodel: zai/glm\n  - action: shell\n    resource: \"git commit*\"\n    effect: deny\n---\nbody\n"
	out, ok := appendShellDenies(broken, []string{"git commit*", "git push*"})
	if !ok {
		t.Fatal("ok = false")
	}
	if got := linesAfterKey(t, out, "model"); len(got) != 0 {
		t.Fatalf("still dangling under model: %q\n%s", got, out)
	}
	perm := strings.Join(linesAfterKey(t, out, "permissions"), "\n")
	if strings.Count(perm, `"git commit*"`) != 1 || !strings.Contains(perm, `"git push*"`) {
		t.Fatalf("permissions block wrong:\n%s", perm)
	}
}

// Block scalars (description: >) keep their indented text.
func TestAppendShellDeniesKeepsBlockScalars(t *testing.T) {
	in := "---\ndescription: >\n  multi line\n  text\npermissions:\n  - action: read\n    resource: \"*\"\n    effect: allow\n---\nbody"
	out, _ := appendShellDenies(in, []string{"rm *"})
	if got := linesAfterKey(t, out, "description"); len(got) != 2 {
		t.Fatalf("description lines = %q", got)
	}
}

func TestAppendDenyBashToAgentsHealsFiles(t *testing.T) {
	dir := t.TempDir()
	broken := "---\nmode: all\npermissions:\n  - action: read\n    resource: \"*\"\n    effect: allow\nmodel: m\n  - action: shell\n    resource: \"git push*\"\n    effect: deny\n---\nbody\n"
	path := filepath.Join(dir, "reviewer.md")
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := AppendDenyBashToAgents(dir, []string{"git push*"}, nil)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v, want 1 healed file", n, err)
	}
	data, _ := os.ReadFile(path)
	if got := linesAfterKey(t, string(data), "model"); len(got) != 0 {
		t.Fatalf("file not healed:\n%s", data)
	}
}

// Daily-dev/QA strip git commit/push from every agent. An empty (non-nil)
// only list means append to nobody and strip from everybody.
func TestAppendDenyBashStripsCommitFromEveryone(t *testing.T) {
	dir := t.TempDir()
	locked := "---\nmode: all\npermissions:\n  - action: shell\n    resource: \"*\"\n    effect: allow\n  - action: shell\n    resource: \"git commit*\"\n    effect: deny\n  - action: shell\n    resource: \"git push*\"\n    effect: deny\n---\nPrompt.\n"
	for _, name := range []string{"orchestrator.md", "dev.md", "qa-dev.md", "qa-orchestrator.md", "ask.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(locked), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	n, err := AppendDenyBashToAgents(dir, []string{"git commit*", "git push*"}, []string{})
	if err != nil {
		t.Fatalf("AppendDenyBashToAgents: %v", err)
	}
	if n != 5 {
		t.Fatalf("changed = %d, want 5", n)
	}
	for _, name := range []string{"orchestrator.md", "dev.md", "qa-dev.md", "qa-orchestrator.md", "ask.md"} {
		got, _ := os.ReadFile(filepath.Join(dir, name))
		if strings.Contains(string(got), `resource: "git commit*"`) || strings.Contains(string(got), `resource: "git push*"`) {
			t.Fatalf("%s still denied commit/push:\n%s", name, got)
		}
		if !strings.Contains(string(got), `resource: "*"`) {
			t.Fatalf("%s lost shell allow:\n%s", name, got)
		}
	}
}
