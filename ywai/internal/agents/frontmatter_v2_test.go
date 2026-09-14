package agents

import (
	"strings"
	"testing"
)

func v2Profile() AgentProfile {
	return AgentProfile{
		Description: "Test agent",
		Mode:        "subagent",
		Prompt:      "body",
		Permission: map[string]string{
			"read":  "allow",
			"edit":  "deny",
			"bash":  "deny",
			"skill": "deny",
		},
	}
}

// The list form is the schema OpenCode 2 enforces; the map form is an unknown
// key that makes it fall back to the legacy decode and paste the frontmatter
// into the system prompt. This is not cosmetic.
func TestBuildOpenCodeMarkdown_UsesPermissionsList(t *testing.T) {
	md := BuildOpenCodeMarkdown("tester", v2Profile())

	if !strings.Contains(md, "\npermissions:\n") {
		t.Fatalf("markdown must carry a permissions list, got:\n%s", md)
	}
	if strings.Contains(md, "\npermission:\n") {
		t.Errorf("v1-style permission map must not be written:\n%s", md)
	}
	for _, want := range []string{"  - action:", "    resource:", "    effect:"} {
		if !strings.Contains(md, want) {
			t.Errorf("rendered rules missing %q in:\n%s", want, md)
		}
	}
	// The body must still follow the frontmatter, not be swallowed by it.
	if !strings.HasSuffix(strings.TrimSpace(md), "body") {
		t.Errorf("prompt body lost:\n%s", md)
	}
}

// Delegation gating lives in the markdown, so a file whose subagent rules
// are never rewritten leaves every delegation target allowed.
func TestInjectTaskPermission_RewritesSubagentRules(t *testing.T) {
	md := BuildOpenCodeMarkdown("orchestrator", v2Profile())
	got, ok := injectTaskPermission(md, map[string]string{"dev": "allow", "qa": "deny"})
	if !ok {
		t.Fatalf("injection reported no-op on a permissions file:\n%s", md)
	}
	if !strings.Contains(got, "subagent") {
		t.Errorf("no subagent rule written:\n%s", got)
	}
	if !strings.Contains(got, "dev") {
		t.Errorf("allowed target missing:\n%s", got)
	}
	// Non-subagent rules must survive: the list is order-sensitive and the
	// other rules are what deny everything else.
	if !strings.Contains(got, "action:") || strings.Count(got, "- action:") < 2 {
		t.Errorf("existing rules dropped during injection:\n%s", got)
	}
}
