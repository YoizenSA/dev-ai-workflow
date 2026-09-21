package agents

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func shellRules(t *testing.T, path string) map[string]any {
	t.Helper()
	root, err := config.ReadJSONC(path)
	if err != nil {
		t.Fatal(err)
	}
	perm, _ := root["permission"].(map[string]any)
	rules, _ := perm["shell"].(map[string]any)
	return rules
}

// The reported failure: a permission block with no shell key leaves the
// primary agent at "ask", which auto-denies `git push` in a headless session.
func TestEnsureRootShellPermissionAddsMissingShell(t *testing.T) {
	p := writeConfig(t, `{"permission":{"delegate":"allow","delegation_*":"allow"}}`)
	if err := EnsureRootShellPermission(p); err != nil {
		t.Fatal(err)
	}
	if got := shellRules(t, p)["git push*"]; got != "allow" {
		t.Errorf(`shell["git push*"] = %v, want "allow"`, got)
	}
	root, _ := config.ReadJSONC(p)
	perm := root["permission"].(map[string]any)
	if perm["delegate"] != "allow" {
		t.Errorf("existing permission keys must survive, got %v", perm)
	}
}

// A hand-narrowed pattern is a decision; re-running install must not undo it.
func TestEnsureRootShellPermissionKeepsUserChoices(t *testing.T) {
	p := writeConfig(t, `{"permission":{"shell":{"git push*":"deny"}}}`)
	if err := EnsureRootShellPermission(p); err != nil {
		t.Fatal(err)
	}
	if got := shellRules(t, p)["git push*"]; got != "deny" {
		t.Errorf(`shell["git push*"] = %v, want "deny" preserved`, got)
	}
}

// A blanket scalar is also a decision: no object is forced over it.
func TestEnsureRootShellPermissionLeavesScalar(t *testing.T) {
	p := writeConfig(t, `{"permission":{"shell":"allow"}}`)
	if err := EnsureRootShellPermission(p); err != nil {
		t.Fatal(err)
	}
	root, _ := config.ReadJSONC(p)
	perm := root["permission"].(map[string]any)
	if perm["shell"] != "allow" {
		t.Errorf(`shell = %v, want the scalar "allow" untouched`, perm["shell"])
	}
}
