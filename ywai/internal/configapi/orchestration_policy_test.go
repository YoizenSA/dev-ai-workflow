package configapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	userconfig "github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// opencodeOrchestratorMD is the shape InstallOpenCodeMarkdown writes for the
// orchestrator: a permission: block with edit/write allowed, plus a body.
const opencodeOrchestratorMD = `---
description: Technical lead
mode: all
permission:
  bash:
    "*": allow
  edit: allow
  write: allow
  read: allow
---

# Orchestrator

Triage body.
`

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

	for _, want := range []string{"  edit: deny", "  write: deny", "  bash: deny"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in permission block, got:\n%s", want, content)
		}
	}
	if !strings.Contains(content, "  read: allow") {
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
	for _, want := range []string{"  edit: allow", "  write: allow"} {
		if !strings.Contains(content2, want) {
			t.Errorf("expected %q restored after solo-write profile, got:\n%s", want, content2)
		}
	}
	if !strings.Contains(content2, "  bash:\n") || !strings.Contains(content2, `"*": allow`) {
		t.Errorf("bash must be restored to the nested allow block, got:\n%s", content2)
	}
	if strings.Contains(content2, "  bash: deny") {
		t.Error("bash: deny must not survive a solo-write profile")
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
}

// TestApplyOrchestrationPolicy_FlipsOpenCodeJSON mirrors the edit/write/bash
// flip into the legacy opencode.json agent entry so the two sources of truth
// cannot disagree after a deep activation.
func TestApplyOrchestrationPolicy_FlipsOpenCodeJSON(t *testing.T) {
	home := t.TempDir()
	setTestHomeDir(t, home)

	configDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ocJSON := `{
  "agent": {
    "orchestrator": {
      "mode": "primary",
      "permission": {
        "read": "allow",
        "edit": "allow",
        "write": "allow",
        "bash": "allow"
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
		t.Fatal("expected policy apply to flip opencode.json")
	}

	data, err := os.ReadFile(filepath.Join(configDir, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"edit", "write", "bash"} {
		if !strings.Contains(string(data), `"`+key+`": "deny"`) {
			t.Errorf("expected %s: deny in opencode.json after deep activation, got:\n%s", key, string(data))
		}
	}
	if !strings.Contains(string(data), `"read": "allow"`) {
		t.Error("read must stay allow in opencode.json")
	}
}
