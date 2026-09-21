package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

func shellRules(t *testing.T, cfgPath string) map[string]any {
	t.Helper()
	root, err := config.ReadJSONC(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	perm, _ := root["permission"].(map[string]any)
	rules, _ := perm["shell"].(map[string]any)
	return rules
}

// resolve mirrors opencode's rule resolution: the LAST matching rule wins, and
// an unmatched command falls back to "ask" (which a headless session cannot
// answer). Patterns are read in serialized order, the order opencode receives
// them in via Object.entries.
func resolve(t *testing.T, cfgPath, command string) string {
	t.Helper()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	rules := shellRules(t, cfgPath)
	effect := "ask"
	for _, pattern := range serializedShellPatterns(t, data) {
		if globMatch(t, pattern, command) {
			effect = rules[pattern].(string)
		}
	}
	return effect
}

// globMatch reproduces opencode's matcher byte for byte: the pattern is escaped
// as a regex except for * and ?, which become .* and ., then anchored and run
// with dotall. A plain path.Match would not do — it refuses to cross "/", so
// "rm -rf*" would silently fail to match "rm -rf /tmp/x" and the gate tests
// would pass for the wrong reason.
func globMatch(t *testing.T, pattern, command string) bool {
	t.Helper()
	var b strings.Builder
	for _, r := range pattern {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '.', '+', '^', '$', '{', '}', '(', ')', '|', '[', ']', '\\':
			b.WriteString("\\" + string(r))
		default:
			b.WriteRune(r)
		}
	}
	expr := b.String()
	if strings.HasSuffix(expr, " .*") {
		expr = strings.TrimSuffix(expr, " .*") + "( .*)?"
	}
	re, err := regexp.Compile("(?s)^" + expr + "$")
	if err != nil {
		t.Fatalf("pattern %q: %v", pattern, err)
	}
	return re.MatchString(command)
}

// serializedShellPatterns returns the shell pattern keys in the order they were
// written to disk, which is the order opencode will apply them in.
func serializedShellPatterns(t *testing.T, data []byte) []string {
	t.Helper()
	marker := `"shell": {`
	i := strings.Index(string(data), marker)
	if i < 0 {
		t.Fatal("no shell block written")
	}
	var keys []string
	for _, line := range strings.Split(string(data)[i+len(marker):], "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "}") {
			break
		}
		if k, _, ok := strings.Cut(line, `":`); ok {
			keys = append(keys, strings.TrimPrefix(k, `"`))
		}
	}
	return keys
}

// The reported failure: with no shell rule the primary agent falls back to
// "ask", so even a read-only grep is denied in a headless session.
func TestEnsureRootShellPermissionAllowsOrdinaryCommands(t *testing.T) {
	p := writeConfig(t, `{"permission":{"delegate":"allow","delegation_*":"allow"}}`)
	if err := EnsureRootShellPermission(p); err != nil {
		t.Fatal(err)
	}
	// "cat /etc/hosts" is the control: it contains a slash, so it only lands on
	// "allow" if the broad "*" really matched. Without it a fallback bug would
	// hide behind commands that happen to match nothing.
	for _, cmd := range []string{"grep -rn foo .", "git push origin main", "npm test", "cat /etc/hosts"} {
		if got := resolve(t, p, cmd); got != "allow" {
			t.Errorf("%q = %q, want allow", cmd, got)
		}
	}
	root, _ := config.ReadJSONC(p)
	if root["permission"].(map[string]any)["delegate"] != "allow" {
		t.Error("existing permission keys must survive")
	}
}

// Destructive commands stay gated. "ask" is the ceiling the format allows:
// one prompt in the TUI, a refusal in headless.
func TestEnsureRootShellPermissionGatesDestructiveCommands(t *testing.T) {
	p := writeConfig(t, `{}`)
	if err := EnsureRootShellPermission(p); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range []string{
		"git push origin main --force",
		"git reset --hard HEAD~3",
		"rm -rf /tmp/x",
		"sudo systemctl stop nginx",
		"npm publish --access public",
		"terraform destroy -auto-approve",
	} {
		if got := resolve(t, p, cmd); got != "ask" {
			t.Errorf("%q = %q, want ask", cmd, got)
		}
	}
}

// findLast means the broad "*" must be serialized BEFORE the narrow patterns,
// or it would override every gate above. Go sorts map keys and "*" sorts first;
// this pins that ordering so a future edit cannot silently break the gates.
func TestRootShellRulesSerializeWildcardFirst(t *testing.T) {
	data, err := json.MarshalIndent(map[string]any{"shell": rootShellRules}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	keys := serializedShellPatterns(t, data)
	if len(keys) == 0 || keys[0] != "*" {
		t.Fatalf("first shell pattern = %q, want \"*\"", keys)
	}
}

// A hand-narrowed pattern is a decision; re-running install must not undo it.
func TestEnsureRootShellPermissionKeepsUserChoices(t *testing.T) {
	p := writeConfig(t, `{"permission":{"shell":{"*":"deny"}}}`)
	if err := EnsureRootShellPermission(p); err != nil {
		t.Fatal(err)
	}
	if got := shellRules(t, p)["*"]; got != "deny" {
		t.Errorf(`shell["*"] = %v, want "deny" preserved`, got)
	}
}

// A blanket scalar is also a decision: no object is forced over it.
func TestEnsureRootShellPermissionLeavesScalar(t *testing.T) {
	p := writeConfig(t, `{"permission":{"shell":"allow"}}`)
	if err := EnsureRootShellPermission(p); err != nil {
		t.Fatal(err)
	}
	root, _ := config.ReadJSONC(p)
	if root["permission"].(map[string]any)["shell"] != "allow" {
		t.Error("scalar shell permission must be left untouched")
	}
}
