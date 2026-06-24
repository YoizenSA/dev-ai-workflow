package agents

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDelegations_FileAbsent(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadDelegations(dir)
	if err != nil {
		t.Fatalf("expected no error when file absent: %v", err)
	}
	if len(got.Agents) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got.Agents))
	}
}

func TestLoadDelegations_Parses(t *testing.T) {
	dir := t.TempDir()
	doc := `{"agents":{"orchestrator":{"task":{"*":"deny","dev":"allow"}},"dev":{"task":{"*":"deny"}}}}`
	os.WriteFile(filepath.Join(dir, DelegationsFile), []byte(doc), 0o644)

	got, err := LoadDelegations(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Agents["orchestrator"].Task["dev"] != "allow" {
		t.Errorf("orchestrator->dev = %q, want allow", got.Agents["orchestrator"].Task["dev"])
	}
	if got.Agents["dev"].Task["*"] != "deny" {
		t.Errorf("dev catch-all = %q, want deny", got.Agents["dev"].Task["*"])
	}
}

func applyTaskDelegationsForTest(t *testing.T, configPath, agentsDir string, tasks map[string]map[string]string) error {
	t.Helper()
	doc := &DelegationsDoc{Agents: make(map[string]AgentDelegation, len(tasks))}
	for agent, task := range tasks {
		doc.Agents[agent] = AgentDelegation{Task: task}
	}
	return ApplyDelegations(configPath, agentsDir, doc)
}

func TestApplyDelegations_CreatesAgentAndTaskMap(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	os.MkdirAll(dir, 0o755)

	delegations := map[string]map[string]string{
		"orchestrator": {"*": "deny", "dev": "allow"},
	}
	if err := applyTaskDelegationsForTest(t, configPath, dir, delegations); err != nil {
		t.Fatal(err)
	}

	root, _ := loadJSON(t, configPath)
	agents := root["agent"].(map[string]any)
	orch := agents["orchestrator"].(map[string]any)
	perm := orch["permission"].(map[string]any)
	task := perm["task"].(map[string]any)
	if task["dev"] != "allow" || task["*"] != "deny" {
		t.Errorf("task map = %+v, want dev=allow *=deny", task)
	}
}

func TestApplyDelegations_PreservesExistingScalars(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	// Pre-existing agent with a scalar permission block + a top-level model.
	seed := map[string]any{
		"agent": map[string]any{
			"orchestrator": map[string]any{
				"mode":  "primary",
				"model": "opencode-go/glm-5.1",
				"permission": map[string]any{
					"read": "allow",
					"edit": "deny",
				},
			},
		},
	}
	data, _ := json.MarshalIndent(seed, "", "  ")
	os.WriteFile(configPath, data, 0o644)

	delegations := map[string]map[string]string{
		"orchestrator": {"*": "deny", "dev": "allow"},
	}
	if err := applyTaskDelegationsForTest(t, configPath, dir, delegations); err != nil {
		t.Fatal(err)
	}

	root, _ := loadJSON(t, configPath)
	orch := root["agent"].(map[string]any)["orchestrator"].(map[string]any)
	perm := orch["permission"].(map[string]any)

	// Scalar permissions preserved.
	if perm["read"] != "allow" || perm["edit"] != "deny" {
		t.Errorf("scalar perms clobbered: %+v", perm)
	}
	// Model preserved.
	if orch["model"] != "opencode-go/glm-5.1" {
		t.Errorf("model lost: %+v", orch["model"])
	}
	// Task map added.
	task := perm["task"].(map[string]any)
	if task["dev"] != "allow" {
		t.Errorf("task map not written: %+v", task)
	}
}

func TestApplyDelegations_Idempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	os.MkdirAll(dir, 0o755)
	delegations := map[string]map[string]string{
		"orch": {"*": "deny", "dev": "allow"},
	}
	applyTaskDelegationsForTest(t, configPath, dir, delegations)
	first, _ := os.ReadFile(configPath)
	applyTaskDelegationsForTest(t, configPath, dir, delegations)
	second, _ := os.ReadFile(configPath)
	if string(first) != string(second) {
		t.Errorf("ApplyDelegations is not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestApplyDelegations_EmptyMapLeavesExistingTask(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	seed := map[string]any{
		"agent": map[string]any{
			"finder": map[string]any{
				"permission": map[string]any{
					"task": map[string]any{"*": "allow"},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(seed, "", "  ")
	os.WriteFile(configPath, data, 0o644)

	delegations := map[string]map[string]string{
		"finder": {}, // empty task map is ignored by the structured delegation doc
	}
	if err := applyTaskDelegationsForTest(t, configPath, dir, delegations); err != nil {
		t.Fatal(err)
	}
	root, _ := loadJSON(t, configPath)
	perm := root["agent"].(map[string]any)["finder"].(map[string]any)["permission"].(map[string]any)
	task := perm["task"].(map[string]any)
	if task["*"] != "allow" {
		t.Errorf("expected existing task map to remain unchanged, got %+v", task)
	}
}

func TestApplyDelegations_EmptyInputNoop(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	// File does not exist and no delegations -> must NOT create a file.
	if err := ApplyDelegations(configPath, dir, &DelegationsDoc{Agents: map[string]AgentDelegation{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(configPath); err == nil {
		t.Error("empty delegations should not create a config file")
	}
}

func TestRulesFor_Override(t *testing.T) {
	doc := &DelegationsDoc{
		Agents: map[string]AgentDelegation{
			"architect":    {Rules: []DelegationRule{{Action: "custom", Inline: "No", Delegate: "Yes"}}},
			"dev":          {SkipRules: true},
			"orchestrator": {},
		},
	}
	doc.Defaults.Rules = []DelegationRule{{Action: "default-rule", Inline: "Yes", Delegate: "No"}}

	// Override wins.
	r, ok := doc.RulesFor("architect")
	if !ok || len(r) != 1 || r[0].Action != "custom" {
		t.Errorf("expected override, got %v %+v", ok, r)
	}
	// Default applies.
	r, ok = doc.RulesFor("orchestrator")
	if !ok || len(r) != 1 || r[0].Action != "default-rule" {
		t.Errorf("expected default, got %v %+v", ok, r)
	}
	// SkipRules opts out.
	_, ok = doc.RulesFor("dev")
	if ok {
		t.Error("expected ok=false for SkipRules agent")
	}
}

func TestApplyDelegations_RendersMarkdownFromJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	agentsDir := filepath.Join(dir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "orchestrator.md"), []byte("---\nmode: primary\n---\n\nprompt."), 0o644)

	doc := &DelegationsDoc{Agents: map[string]AgentDelegation{
		"orchestrator": {},
	}}
	doc.Defaults.Rules = []DelegationRule{{Action: "Read 4+ files", Inline: "No", Delegate: "Yes"}}
	doc.Defaults.Triggers = []DelegationTrigger{{Name: "4-file rule", Description: "delegate exploration"}}

	ApplyDelegations(configPath, agentsDir, doc)

	md, _ := os.ReadFile(filepath.Join(agentsDir, "orchestrator.md"))
	if !strings.Contains(string(md), "### Delegation Rules") {
		t.Error("markdown missing the Delegation Rules heading")
	}
	if !strings.Contains(string(md), "Read 4+ files") {
		t.Error("markdown missing the rule row (should be generated from JSON)")
	}
	if !strings.Contains(string(md), "4-file rule") {
		t.Error("markdown missing the trigger")
	}
}

func TestApplyDelegations_PersistsSidecar(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	agentsDir := filepath.Join(dir, "agents")
	os.MkdirAll(agentsDir, 0o755)

	doc := &DelegationsDoc{Agents: map[string]AgentDelegation{
		"orchestrator": {Task: map[string]string{"*": "deny"}},
	}}
	doc.Defaults.Rules = []DelegationRule{{Action: "x", Inline: "Yes", Delegate: "No"}}
	ApplyDelegations(configPath, agentsDir, doc)

	sidecar, err := os.ReadFile(filepath.Join(agentsDir, DelegationsFile))
	if err != nil {
		t.Fatalf("sidecar not written: %v", err)
	}
	if !strings.Contains(string(sidecar), "orchestrator") {
		t.Errorf("sidecar missing agent: %s", sidecar)
	}
}

func TestApplyDelegations_WritesTaskMapIntoMarkdownFrontmatter(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	agentsDir := filepath.Join(dir, "agents")
	os.MkdirAll(agentsDir, 0o755)
	// Realistic installed frontmatter: a scalar "task: allow" bucket, no sub-map.
	md := "---\ndescription: orch\nmode: primary\npermission:\n  \"*\": deny\n  read: allow\n  task: allow\n---\n\nbody.\n"
	os.WriteFile(filepath.Join(agentsDir, "orchestrator.md"), []byte(md), 0o644)

	doc := &DelegationsDoc{Agents: map[string]AgentDelegation{
		"orchestrator": {Task: map[string]string{"*": "deny", "dev": "allow", "qa": "allow"}},
	}}
	ApplyDelegations(configPath, agentsDir, doc)

	out, _ := os.ReadFile(filepath.Join(agentsDir, "orchestrator.md"))
	got := string(out)
	// The scalar task bucket must become a nested allow/deny map.
	if strings.Contains(got, "  task: allow\n") {
		t.Errorf("scalar task bucket was not replaced:\n%s", got)
	}
	for _, want := range []string{"  task:\n", "    \"*\": deny", "    dev: allow", "    qa: allow"} {
		if !strings.Contains(got, want) {
			t.Errorf("frontmatter missing %q in:\n%s", want, got)
		}
	}
	// Other permission keys must be preserved.
	if !strings.Contains(got, "  read: allow") {
		t.Errorf("existing permission key lost:\n%s", got)
	}
}

func TestInjectTaskPermission_InsertsWhenMissing(t *testing.T) {
	md := "---\nmode: primary\npermission:\n  \"*\": deny\n  read: allow\n---\n\nbody."
	out, ok := injectTaskPermission(md, map[string]string{"*": "deny", "finder": "allow"})
	if !ok {
		t.Fatal("expected injection to succeed")
	}
	if !strings.Contains(out, "  task:\n") || !strings.Contains(out, "    finder: allow") {
		t.Errorf("task block not inserted:\n%s", out)
	}
}

func TestReadTaskPermission_Nested(t *testing.T) {
	md := "---\npermission:\n  \"*\": deny\n  task:\n    \"*\": deny\n    dev: allow\n---\n\nbody."
	got, ok := ReadTaskPermission(md)
	if !ok {
		t.Fatal("expected ok")
	}
	if got["*"] != "deny" || got["dev"] != "allow" {
		t.Errorf("unexpected task map: %v", got)
	}
}

func TestReadTaskPermission_Scalar(t *testing.T) {
	md := "---\npermission:\n  task: allow\n---\n\nbody."
	got, _ := ReadTaskPermission(md)
	if got["*"] != "allow" {
		t.Errorf("scalar task should map to {*: allow}, got %v", got)
	}
}

func TestInjectThenReadTaskPermission_RoundTrip(t *testing.T) {
	md := "---\npermission:\n  \"*\": deny\n  task: allow\n---\n\nbody."
	want := map[string]string{"*": "deny", "qa": "allow", "finder": "allow"}
	injected, ok := InjectTaskPermission(md, want)
	if !ok {
		t.Fatal("inject failed")
	}
	got, _ := ReadTaskPermission(injected)
	for k, v := range want {
		if got[k] != v {
			t.Errorf("round-trip mismatch for %q: got %q want %q\n%s", k, got[k], v, injected)
		}
	}
}

func TestInjectTaskPermission_NoPermissionBlock(t *testing.T) {
	md := "---\nmode: primary\n---\n\nbody."
	if _, ok := injectTaskPermission(md, map[string]string{"*": "deny"}); ok {
		t.Error("expected no-op when there is no permission block")
	}
}

func TestRenderRulesSection_EscapesPipes(t *testing.T) {
	out := renderRulesSection(
		[]DelegationRule{{Action: "a | b", Inline: "Yes", Delegate: "No"}},
		nil,
	)
	if !strings.Contains(out, `a \| b`) {
		t.Errorf("pipe not escaped in output: %s", out)
	}
}

func loadJSON(t *testing.T, path string) (map[string]any, error) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
