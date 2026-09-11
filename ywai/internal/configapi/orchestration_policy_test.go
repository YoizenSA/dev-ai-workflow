package configapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	userconfig "github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// opencodeOrchestratorMD is the shape InstallOpenCodeMarkdown writes for the
// orchestrator: an ordered permissions rule list with edit/shell allowed,
// plus a body.
const opencodeOrchestratorMD = `---
description: Technical lead
mode: all
permissions:
  - action: read
    resource: "*"
    effect: allow
  - action: edit
    resource: "*"
    effect: allow
  - action: shell
    resource: "*"
    effect: allow
---

# Orchestrator

Triage body.
`

// v2Rule asserts one full v2 rule block inside rendered frontmatter.
func v2Rule(action, resource, effect string) string {
	return fmt.Sprintf("  - action: %s\n    resource: %s\n    effect: %s", action, resource, effect)
}

// TestApplyOrchestrationPolicy_DeepDeniesSoloWriteAndInjectsSection verifies
// the profile-driven orchestration policy materializes into the installed
// opencode orchestrator: edit/write flip to deny, a generated policy section
// is injected once, and unrelated permissions survive.
func TestApplyOrchestrationPolicy_DeepDeniesSoloWriteAndInjectsSection(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)

	agentsDir := filepath.Join(home, ".config", "opencode", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "orchestrator.md"), []byte(opencodeOrchestratorMD), 0o644); err != nil {
		t.Fatal(err)
	}

	false_ := false
	hops := 2
	deep := userconfig.OrchestrationPolicy{
		DefaultMode:           "full",
		AllowSoloWrite:        &false_,
		MaxHopsBeforeEscalate: &hops,
		RequireReview:         "always",
	}
	if !applyOrchestrationPolicy(deep) {
		t.Fatal("expected policy apply to update the opencode orchestrator")
	}

	got, err := os.ReadFile(filepath.Join(agentsDir, "orchestrator.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(got)

	for _, rule := range []string{v2Rule("edit", `"*"`, "deny"), v2Rule("shell", `"*"`, "deny")} {
		if !strings.Contains(content, rule) {
			t.Errorf("expected rule %q after deep activation, got:\n%s", rule, content)
		}
	}
	if !strings.Contains(content, v2Rule("read", `"*"`, "allow")) {
		t.Error("read must stay allow when solo-write is denied")
	}
	if !strings.Contains(content, "**default_mode**: full") || !strings.Contains(content, "**allow_solo_write**: false") {
		t.Errorf("expected deep policy values in the generated section, got:\n%s", content)
	}
	if strings.Count(content, orchestrationPolicyMarkerStart) != 1 {
		t.Errorf("policy section must appear exactly once, got %d markers", strings.Count(content, orchestrationPolicyMarkerStart))
	}

	// Allow side: switching to a solo-write profile restores edit/write and the
	// full bash block (nested "*": allow, not a leftover deny), replacing the
	// section in place (no stacking).
	true_ := true
	fast := userconfig.OrchestrationPolicy{DefaultMode: "solo", AllowSoloWrite: &true_, RequireReview: "never"}
	if !applyOrchestrationPolicy(fast) {
		t.Fatal("expected policy re-apply to update the orchestrator")
	}
	got2, err := os.ReadFile(filepath.Join(agentsDir, "orchestrator.md"))
	if err != nil {
		t.Fatal(err)
	}
	content2 := string(got2)
	for _, rule := range []string{v2Rule("edit", `"*"`, "allow"), v2Rule("shell", `"*"`, "allow")} {
		if !strings.Contains(content2, rule) {
			t.Errorf("expected restored rule %q after solo-write profile, got:\n%s", rule, content2)
		}
	}
	if strings.Contains(content2, v2Rule("edit", `"*"`, "deny")) || strings.Contains(content2, v2Rule("shell", `"*"`, "deny")) {
		t.Error("broad edit/shell denies must not survive a solo-write profile")
	}
	if !strings.Contains(content2, "**default_mode**: solo") {
		t.Errorf("expected solo policy section, got:\n%s", content2)
	}
	if strings.Count(content2, orchestrationPolicyMarkerStart) != 1 {
		t.Error("re-applying the policy must not stack sections")
	}
}

// TestApplyOrchestrationPolicy_StripsToolsOnPiStyleMarkdown verifies the
// pi/omp/claude "tools:" frontmatter list loses edit/write when solo-write is
// denied, and the policy section is still injected.
func TestApplyOrchestrationPolicy_StripsToolsOnPiStyleMarkdown(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)

	agentsDir := filepath.Join(home, ".pi", "agent", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	piMD := `---
name: orchestrator
description: >
  Technical lead
tools: read,edit,write,bash,glob,grep
---

Body.
`
	if err := os.WriteFile(filepath.Join(agentsDir, "orchestrator.md"), []byte(piMD), 0o644); err != nil {
		t.Fatal(err)
	}

	false_ := false
	deep := userconfig.OrchestrationPolicy{DefaultMode: "full", AllowSoloWrite: &false_}
	if !applyOrchestrationPolicy(deep) {
		t.Fatal("expected policy apply to update the pi orchestrator")
	}

	got, err := os.ReadFile(filepath.Join(agentsDir, "orchestrator.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(got)
	if !strings.Contains(content, "tools: read,bash,glob,grep") {
		t.Errorf("edit/write must be stripped from the tools list, got:\n%s", content)
	}
	if !strings.Contains(content, orchestrationPolicyMarkerStart) {
		t.Error("policy section must be injected on pi-style markdown too")
	}

	// Claude-style capitalized tool list is stripped the same way.
	claudeMD := "---\nname: orchestrator\ndescription: >\n  Technical lead\ntools: Read,Edit,Write,Bash\n---\n\nBody.\n"
	claudeDir := filepath.Join(home, ".claude", "agents")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claudeDir, "orchestrator.md"), []byte(claudeMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if !applyOrchestrationPolicy(deep) {
		t.Fatal("expected policy apply to update the claude orchestrator")
	}
	gotClaude, err := os.ReadFile(filepath.Join(claudeDir, "orchestrator.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gotClaude), "tools: Read,Bash") {
		t.Errorf("Edit/Write must be stripped from the claude tools list, got:\n%s", string(gotClaude))
	}

	// Reversibility: a deny → allow cycle re-adds edit/write to the tools list.
	true_ := true
	solo := userconfig.OrchestrationPolicy{DefaultMode: "solo", AllowSoloWrite: &true_}
	if !applyOrchestrationPolicy(solo) {
		t.Fatal("expected policy re-apply to update the pi orchestrator")
	}
	gotBack, err := os.ReadFile(filepath.Join(agentsDir, "orchestrator.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gotBack), "tools: read,edit,write,bash,glob,grep") {
		t.Errorf("edit/write must be restored after re-allowing solo write, got:\n%s", string(gotBack))
	}
}

// TestApplyOmpModelRoles verifies the profile → omp modelRoles mapping:
// sourced roles are set from the active profile, unrelated keys and roles
// survive, and omp being absent is a no-op.
func TestApplyOmpModelRoles(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)

	ompDir := filepath.Join(home, ".omp", "agent")
	if err := os.MkdirAll(ompDir, 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `theme:
  dark: titanium
setupVersion: 1
modelRoles:
  advisor: openai-codex/gpt-5.6-sol:high
  vision: opencode-go/gpt-5.6-luna
  default: opencode-go/deepseek-v4-flash
defaultThinkingLevel: auto
`
	path := filepath.Join(ompDir, "config.yml")
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	profile := userconfig.OrchestratorModelProfile{
		Agents: userconfig.RoleDefaults{
			"orchestrator": {Model: "opencode-admin/grok-4.5"},
			"qa":           {Model: "opencode-admin/minimax-m3"},
			"architect":    {Model: "opencode-admin/grok-4.5"},
			"designer":     {Model: "opencode-admin/kimi-k3"},
			"advisor":      {Model: "opencode-admin/grok-4.5"},
			"dev":          {Model: "opencode-admin/deepseek-v4-flash"},
		},
	}
	if !applyOmpModelRoles(profile) {
		t.Fatal("expected config.yml update when omp is installed")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for role, want := range map[string]string{
		"default":  "grok-4.5",   // ← orchestrator
		"smol":     "minimax-m3", // ← qa
		"plan":     "grok-4.5",   // ← architect
		"designer": "kimi-k3",
		"advisor":  "grok-4.5",
		"commit":   "deepseek-v4-flash", // ← dev
	} {
		if !strings.Contains(content, role+": "+want) {
			t.Errorf("modelRoles.%s = want %q, got:\n%s", role, want, content)
		}
	}
	// Unrelated keys and unsourced roles survive.
	for _, keep := range []string{"setupVersion: 1", "vision: opencode-go/gpt-5.6-luna", "defaultThinkingLevel: auto"} {
		if !strings.Contains(content, keep) {
			t.Errorf("config.yml lost %q, got:\n%s", keep, content)
		}
	}

	// No-op when omp is not installed.
	home2 := t.TempDir()
	setTestHomeDir(t, home2)
	if applyOmpModelRoles(profile) {
		t.Error("applyOmpModelRoles must be a no-op when ~/.omp/agent is absent")
	}

	// Explicit profile overrides win over the derivation and are written
	// verbatim (provider id included); unsourced roles can be added.
	home3 := t.TempDir()
	setTestHomeDir(t, home3)
	ompDir3 := filepath.Join(home3, ".omp", "agent")
	if err := os.MkdirAll(ompDir3, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ompDir3, "config.yml"), []byte("setupVersion: 1\nmodelRoles:\n  default: old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	overridden := profile.Clone()
	overridden.OmpModelRoles = map[string]string{
		"default": "opencode-go/deepseek-v4-flash", // verbatim, provider included
		"vision":  "opencode-go/gpt-5.6-luna",      // not in the mapping table
	}
	if !applyOmpModelRoles(overridden) {
		t.Fatal("expected override apply to update config.yml")
	}
	data3, err := os.ReadFile(filepath.Join(ompDir3, "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	content3 := string(data3)
	if !strings.Contains(content3, "default: opencode-go/deepseek-v4-flash") {
		t.Errorf("explicit override must be written verbatim, got:\n%s", content3)
	}
	if !strings.Contains(content3, "vision: opencode-go/gpt-5.6-luna") {
		t.Errorf("unsourced role override must be written, got:\n%s", content3)
	}
	// Derived roles still appear alongside the overrides.
	if !strings.Contains(content3, "smol: minimax-m3") {
		t.Errorf("derived roles must survive next to overrides, got:\n%s", content3)
	}
}

// TestApplyOrchestrationPolicy_MigratesLegacyPermissionMap is the migration
// test for the agent section key: a v1-shaped opencode.json (`agent` key, flat
// `permission` map, nested task object) read through the kept readers is
// rewritten in the v2 interpretation — merged under `agents`, permissions as
// the ordered rule array, delegation preserved as subagent rules.
func TestApplyOrchestrationPolicy_MigratesLegacyPermissionMap(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	configDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ocJSON := `{
  "agent": {
    "orchestrator": {
      "model": "opencode-admin/kept",
      "permission": {
        "read": "allow",
        "edit": "allow",
        "bash": "allow",
        "task": {"*": "deny", "finder": "allow"}
      }
    }
  }
}`
	if err := os.WriteFile(filepath.Join(configDir, "opencode.json"), []byte(ocJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	false_ := false
	deep := userconfig.OrchestrationPolicy{DefaultMode: "full", AllowSoloWrite: &false_}
	if !applyOrchestrationPolicy(deep) {
		t.Fatal("expected policy apply to migrate and flip opencode.json")
	}
	data, err := os.ReadFile(filepath.Join(configDir, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.Contains(got, `"permission"`) {
		t.Fatalf("legacy permission map must be rewritten as a permissions array, got:\n%s", got)
	}
	var cfg struct {
		Agent  map[string]json.RawMessage `json:"agent"`
		Agents map[string]json.RawMessage `json:"agents"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Agent) != 0 {
		t.Fatalf("legacy agent key must be gone after migration, got:\n%s", got)
	}
	var orch struct {
		Model       string `json:"model"`
		Permissions []struct {
			Action   string `json:"action"`
			Resource string `json:"resource"`
			Effect   string `json:"effect"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal(cfg.Agents["orchestrator"], &orch); err != nil {
		t.Fatal(err)
	}
	if orch.Model != "opencode-admin/kept" {
		t.Errorf("model lost during migration: %q", orch.Model)
	}
	effectByAction := map[string]string{}
	for _, r := range orch.Permissions {
		if r.Resource == "*" {
			if _, dup := effectByAction[r.Action]; !dup {
				effectByAction[r.Action] = r.Effect
			}
		}
		if r.Action == "subagent" && r.Resource == "finder" && r.Effect != "allow" {
			t.Errorf("delegation to finder must survive migration as an allow rule: %+v", r)
		}
	}
	for action, want := range map[string]string{"edit": "deny", "shell": "deny", "read": "allow"} {
		if effectByAction[action] != want {
			t.Errorf("migrated %s rule = %q, want %q", action, effectByAction[action], want)
		}
	}
	if _, hasWildcard := effectByAction["subagent"]; !hasWildcard {
		t.Errorf("subagent catch-all rule missing after migration:\n%s", got)
	}
}

// TestApplyOrchestrationPolicy_KeepsV2PermissionsArray pins the v2 branch of
// applyOrchestrationPolicyToOpenCodeJSON: the ordered `permissions` rule array
// is the native v2 shape, so a deep activation must flip effects inside the
// array instead of downgrading it to the legacy v1 `permission` map. The v1
// `agent` key must stay absent.
func TestApplyOrchestrationPolicy_KeepsV2PermissionsArray(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)
	configDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ocJSON := `{
  "agents": {
    "orchestrator": {
      "mode": "primary",
      "system": "Lead.",
      "permissions": [
        {"action": "read", "resource": "*", "effect": "allow"},
        {"action": "edit", "resource": "*", "effect": "allow"},
        {"action": "shell", "resource": "*", "effect": "allow"},
        {"action": "subagent", "resource": "finder", "effect": "allow"}
      ]
    }
  }
}`
	if err := os.WriteFile(filepath.Join(configDir, "opencode.json"), []byte(ocJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	false_ := false
	deep := userconfig.OrchestrationPolicy{DefaultMode: "full", AllowSoloWrite: &false_}
	if !applyOrchestrationPolicy(deep) {
		t.Fatal("expected policy apply to flip the v2 permissions array")
	}
	data, err := os.ReadFile(filepath.Join(configDir, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, `"permissions"`) {
		t.Fatalf("v2 shape must keep the permissions array, got:\n%s", got)
	}
	if strings.Contains(got, `"permission":`) {
		t.Fatalf("v2 shape must not keep the v1 permission map, got:\n%s", got)
	}
	if !strings.Contains(got, `"agents"`) {
		t.Fatalf("v2 shape must use the agents key, got:\n%s", got)
	}
	var cfg struct {
		Agents map[string]struct {
			Permissions []struct {
				Action   string `json:"action"`
				Effect   string `json:"effect"`
				Resource string `json:"resource"`
			} `json:"permissions"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	perms := cfg.Agents["orchestrator"].Permissions
	effects := map[string]string{}
	for _, r := range perms {
		if r.Resource == "*" {
			effects[r.Action] = r.Effect
		}
	}
	for _, action := range []string{"edit", "shell"} {
		if effects[action] != "deny" {
			t.Errorf("permissions[%s] = %q, want deny after deep activation (rules=%+v)", action, effects[action], perms)
		}
	}
	if effects["read"] != "allow" {
		t.Errorf("permissions[read] = %q, want allow (rules=%+v)", effects["read"], perms)
	}
}
