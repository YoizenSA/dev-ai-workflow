package agents

import (
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
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

// v2 rejects the v1 "permission:" map as an unknown key and falls back to the
// legacy decode, which drops the permissions and pastes the whole frontmatter
// into the system prompt. The list form is not cosmetic.
func TestBuildOpenCodeMarkdown_V2UsesPermissionsList(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")

	md := BuildOpenCodeMarkdown("tester", v2Profile())

	if !strings.Contains(md, "\npermissions:\n") {
		t.Fatalf("v2 markdown must carry a permissions list, got:\n%s", md)
	}
	if strings.Contains(md, "\npermission:\n") {
		t.Errorf("v1 permission map must not be written for v2:\n%s", md)
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

func TestBuildOpenCodeMarkdown_V1KeepsPermissionMap(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v1")

	md := BuildOpenCodeMarkdown("tester", v2Profile())

	if !strings.Contains(md, "\npermission:\n") {
		t.Fatalf("v1 markdown must keep the nested permission map, got:\n%s", md)
	}
	if strings.Contains(md, "\npermissions:\n") {
		t.Errorf("v2 permissions list must not leak into a v1 file:\n%s", md)
	}
}

// Delegation gating lives in the markdown, so a v2 file whose subagent rules
// are never rewritten leaves every delegation target allowed.
func TestInjectTaskPermission_V2RewritesSubagentRules(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v2")

	md := BuildOpenCodeMarkdown("orchestrator", v2Profile())
	got, ok := injectTaskPermission(md, map[string]string{"dev": "allow", "qa": "deny"})
	if !ok {
		t.Fatalf("injection reported no-op on a v2 file:\n%s", md)
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

// A v1 file must keep using the map path, untouched by the v2 branch.
func TestInjectTaskPermission_V1StillUsesMap(t *testing.T) {
	t.Setenv(agent.OpenCodeOverrideEnv, "v1")

	md := BuildOpenCodeMarkdown("orchestrator", v2Profile())
	got, ok := injectTaskPermission(md, map[string]string{"dev": "allow"})
	if !ok {
		t.Fatalf("injection reported no-op on a v1 file:\n%s", md)
	}
	if !strings.Contains(got, "task:") {
		t.Errorf("v1 task map missing:\n%s", got)
	}
}
