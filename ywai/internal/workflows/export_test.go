package workflows

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// exportFixture builds a small but representative workflow exercising the main
// node types.
func exportFixture() *Workflow {
	return &Workflow{
		Name:        "daily-task",
		Description: "A daily task workflow",
		Version:     "1.0.0",
		Nodes: []Node{
			{ID: "s", Type: NodeTypeStart, Name: "s"},
			{ID: "q", Type: NodeTypeAskUserQuestion, Name: "q", Data: NodeData{
				QuestionText: "What now?",
				Options: []QuestionOption{
					{Label: "report"},
					{Label: "news"},
				},
			}},
			{ID: "news", Type: NodeTypeSubAgent, Name: "news_briefing", Data: NodeData{
				AgentDescription: "News Briefing Agent",
				AgentDefinition:  "You research the latest news.",
				Prompt:           "Find today's top tech news.",
				Model:            "sonnet",
				Tools:            "read,webfetch",
			}},
			{ID: "p", Type: NodeTypePrompt, Name: "p", Data: NodeData{Prompt: "Write the report."}},
			{ID: "e", Type: NodeTypeEnd, Name: "e"},
		},
		Connections: []Connection{
			{From: "s", To: "q"},
			{From: "q", To: "news", FromPort: "branch-1"},
			{From: "q", To: "p", FromPort: "branch-0"},
			{From: "news", To: "e"},
			{From: "p", To: "e"},
		},
	}
}

func TestPlanGeneratesExpectedArtifacts(t *testing.T) {
	commandsDir := t.TempDir()
	agentsDir := t.TempDir()
	e := NewExporterWithDirs(commandsDir, agentsDir)

	plan, files, err := e.Plan(exportFixture())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	// Expect: 1 command + 1 orchestrator agent + 1 sub-agent.
	if len(plan.Files) != 3 {
		t.Fatalf("expected 3 artifacts, got %d: %+v", len(plan.Files), plan.Files)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 file contents, got %d", len(files))
	}

	// Verify kinds.
	kinds := map[string]int{}
	for _, a := range plan.Files {
		kinds[a.Kind]++
	}
	if kinds["command"] != 1 {
		t.Errorf("expected 1 command, got %d", kinds["command"])
	}
	if kinds["agent"] != 2 {
		t.Errorf("expected 2 agents (orchestrator + sub-agent), got %d", kinds["agent"])
	}
}

// TestOrchestratorHasTaskPermission verifies the generated orchestrator agent
// markdown includes a nested permission.task block with deny-all and an allow
// entry for each subAgent node.
func TestOrchestratorHasTaskPermission(t *testing.T) {
	commandsDir := t.TempDir()
	agentsDir := t.TempDir()
	e := NewExporterWithDirs(commandsDir, agentsDir)

	_, files, err := e.Plan(exportFixture())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	orchPath := filepath.Join(agentsDir, "daily-task-orchestrator.md")
	orch, ok := files[orchPath]
	if !ok {
		t.Fatalf("orchestrator markdown not found at %s", orchPath)
	}

	if !strings.Contains(orch, "permission:") {
		t.Errorf("orchestrator missing permission block:\n%s", orch)
	}
	if !strings.Contains(orch, "\n  task:") {
		t.Errorf("orchestrator missing nested permission.task block:\n%s", orch)
	}
	if !strings.Contains(orch, `"*": deny`) {
		t.Errorf("orchestrator permission.task missing deny-all:\n%s", orch)
	}
	if !strings.Contains(orch, "daily-task-news-briefing: allow") {
		t.Errorf("orchestrator permission.task missing subagent allow entry:\n%s", orch)
	}
}

func TestApplyWritesFilesToDisk(t *testing.T) {
	commandsDir := t.TempDir()
	agentsDir := t.TempDir()
	e := NewExporterWithDirs(commandsDir, agentsDir)

	plan, err := e.Apply(exportFixture())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if plan.DryRun {
		t.Fatal("Apply should clear DryRun")
	}

	// Every planned file must exist on disk.
	for _, a := range plan.Files {
		if _, err := os.Stat(a.Path); err != nil {
			t.Errorf("planned file not written: %s (%v)", a.Path, err)
		}
	}

	// The slash command must exist and reference the orchestrator.
	cmdPath := filepath.Join(commandsDir, "daily-task.md")
	content, err := os.ReadFile(cmdPath)
	if err != nil {
		t.Fatalf("command file missing: %v", err)
	}
	if !strings.Contains(string(content), "agent: daily-task-orchestrator") {
		t.Errorf("command should reference orchestrator agent:\n%s", content)
	}
	if !strings.Contains(string(content), "subtask: true") {
		t.Errorf("command should have subtask: true:\n%s", content)
	}
	if !strings.Contains(string(content), "$ARGUMENTS") {
		t.Errorf("command should pass $ARGUMENTS:\n%s", content)
	}

	// The orchestrator agent must exist with a Mermaid diagram + steps.
	orchPath := filepath.Join(agentsDir, "daily-task-orchestrator.md")
	orch, err := os.ReadFile(orchPath)
	if err != nil {
		t.Fatalf("orchestrator file missing: %v", err)
	}
	if !strings.Contains(string(orch), "description:") {
		t.Errorf("orchestrator missing frontmatter description:\n%s", orch)
	}
	if !strings.Contains(string(orch), "flowchart LR") {
		t.Errorf("orchestrator should embed a Mermaid diagram:\n%s", orch)
	}
	if !strings.Contains(string(orch), "Execution steps") {
		t.Errorf("orchestrator should have execution steps:\n%s", orch)
	}
	if !strings.Contains(string(orch), "task") {
		t.Errorf("orchestrator should mention the task tool:\n%s", orch)
	}

	// The sub-agent must exist and reference its task.
	subPath := filepath.Join(agentsDir, "daily-task-news-briefing.md")
	sub, err := os.ReadFile(subPath)
	if err != nil {
		t.Fatalf("sub-agent file missing: %v", err)
	}
	if !strings.Contains(string(sub), "News Briefing Agent") {
		t.Errorf("sub-agent should carry its description:\n%s", sub)
	}
	if !strings.Contains(string(sub), "Find today's top tech news.") {
		t.Errorf("sub-agent should carry its task prompt:\n%s", sub)
	}
}

func TestSubAgentSlugUniqueAndSafe(t *testing.T) {
	wf := &Workflow{
		Name: "deploy",
		Nodes: []Node{
			{ID: "a", Type: NodeTypeSubAgent, Name: "builder", Data: NodeData{Name: "Builder"}},
			{ID: "b", Type: NodeTypeSubAgent, Name: "tester", Data: NodeData{AgentDescription: "Tester"}},
		},
	}
	got := map[string]bool{}
	for _, n := range wf.Nodes {
		if n.Type != NodeTypeSubAgent {
			continue
		}
		slug := subAgentSlug(wf.Name, &n)
		if got[slug] {
			t.Errorf("duplicate slug: %s", slug)
		}
		got[slug] = true
		if !strings.HasPrefix(slug, "deploy-") {
			t.Errorf("slug should start with workflow name: %s", slug)
		}
	}
	if !got["deploy-builder"] || !got["deploy-tester"] {
		t.Errorf("unexpected slugs: %+v", got)
	}
}

func TestSanitizeSlug(t *testing.T) {
	cases := map[string]string{
		"News Briefing":  "news-briefing",
		"  Spaced  Out ": "spaced-out",
		"Weird/Chars!":   "weird-chars",
		"":               "",
		"---leading":     "leading",
		"TRAILING---":    "trailing",
		"news_briefing":  "news-briefing",
		"agent-1":        "agent-1",
	}
	for in, want := range cases {
		if got := sanitizeSlug(in); got != want {
			t.Errorf("sanitizeSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestImportDailyTaskWorkflow(t *testing.T) {
	// Minimal workflow JSON without a name to exercise derivation.
	raw := []byte(`{
		"id": "workflow-123",
		"version": "1.0.0",
		"nodes": [
			{"id":"s","type":"start","name":"s","position":{"x":0,"y":0},"data":{"label":"Start"}},
			{"id":"a","type":"subAgent","name":"agent-1","position":{"x":100,"y":0},"data":{"description":"D","agentDefinition":"ID","prompt":"T","model":"sonnet"}},
			{"id":"e","type":"end","name":"e","position":{"x":200,"y":0},"data":{"label":"End"}}
		],
		"connections": [
			{"from":"s","to":"a","fromPort":"out","toPort":"input"},
			{"from":"a","to":"e","fromPort":"out","toPort":"in"}
		],
		"createdAt":"2025-01-01T00:00:00Z",
		"updatedAt":"2025-01-01T00:00:00Z"
	}`)
	res, err := Import(raw, ImportOptions{Name: "imported"})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Workflow.Name != "imported" {
		t.Errorf("name = %q, want imported", res.Workflow.Name)
	}
	// start/end preserved from source.
	if res.Workflow.findNode(NodeTypeStart) == nil || res.Workflow.findNode(NodeTypeEnd) == nil {
		t.Error("import should preserve start/end nodes")
	}
	// Sub-agent carries its fields.
	a := res.Workflow.findNode(NodeTypeSubAgent)
	if a == nil || a.Data.AgentDescription != "D" || a.Data.Prompt != "T" {
		t.Errorf("sub-agent fields not preserved: %+v", a)
	}
}

func TestImportAddsMissingEndpoints(t *testing.T) {
	// A bare node with no start/end/connections.
	raw := []byte(`{
		"name":"bare","version":"1.0.0",
		"nodes":[{"id":"n","type":"prompt","name":"n","position":{"x":0,"y":0},"data":{"prompt":"hi"}}],
		"connections":[]
	}`)
	res, err := Import(raw, ImportOptions{})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if res.Workflow.findNode(NodeTypeStart) == nil {
		t.Error("import should add a start node when missing")
	}
	if res.Workflow.findNode(NodeTypeEnd) == nil {
		t.Error("import should add an end node when missing")
	}
}

func TestImportRejectsBadName(t *testing.T) {
	raw := []byte(`{"name":"BAD NAME","version":"1.0.0","nodes":[],"connections":[]}`)
	if _, err := Import(raw, ImportOptions{}); err == nil {
		t.Fatal("expected error for invalid name")
	}
}

// TestExportClaudeCodeTarget verifies the claude-code dialect: native frontmatter
// (name/description/tools/model) and a slash command without opencode's
// agent:/subtask: keys.
func TestExportClaudeCodeTarget(t *testing.T) {
	wf := exportFixture()
	cmdDir := t.TempDir()
	agentsDir := t.TempDir()
	e := NewExporterWithDirsForTarget(cmdDir, agentsDir, TargetClaudeCode)

	plan, files, err := e.Plan(wf)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.WorkflowName != "daily-task" {
		t.Fatalf("workflow name: %q", plan.WorkflowName)
	}

	subPath := filepath.Join(agentsDir, "daily-task-news-briefing.md")
	sub, ok := files[subPath]
	if !ok {
		t.Fatalf("sub-agent file missing: %s (have %v)", subPath, keys(files))
	}
	for _, want := range []string{"name: daily-task-news-briefing", "description: News Briefing Agent", "tools: read,webfetch", "model: sonnet"} {
		if !strings.Contains(sub, want) {
			t.Errorf("claude sub-agent missing %q in:\n%s", want, sub)
		}
	}
	if strings.Contains(sub, "permission") {
		t.Errorf("claude sub-agent must not carry opencode permission block:\n%s", sub)
	}

	cmd, ok := files[filepath.Join(cmdDir, "daily-task.md")]
	if !ok {
		t.Fatal("command file missing")
	}
	if strings.Contains(cmd, "subtask:") || strings.Contains(cmd, "agent:") {
		t.Errorf("claude command must omit agent:/subtask:\n%s", cmd)
	}
}

// TestExportSubAgentHandoffInjection verifies that the handoff contract is
// appended to sub-agent prompts for both opencode and claude-code targets,
// but is NOT injected into the orchestrator prompt.
func TestExportSubAgentHandoffInjection(t *testing.T) {
	t.Run("opencode", func(t *testing.T) {
		wf := exportFixture()
		cmdDir := t.TempDir()
		agentsDir := t.TempDir()
		e := NewExporterWithDirs(cmdDir, agentsDir)

		_, files, err := e.Plan(wf)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}

		subPath := filepath.Join(agentsDir, "daily-task-news-briefing.md")
		sub, ok := files[subPath]
		if !ok {
			t.Fatalf("sub-agent markdown not found at %s", subPath)
		}
		if !strings.Contains(sub, "**Status**:") {
			t.Errorf("opencode sub-agent should contain handoff section (**Status**:), got:\n%s", sub)
		}

		orchPath := filepath.Join(agentsDir, "daily-task-orchestrator.md")
		orch, ok := files[orchPath]
		if !ok {
			t.Fatalf("orchestrator markdown not found at %s", orchPath)
		}
		if strings.Contains(orch, "**Status**:") {
			t.Error("opencode orchestrator should NOT contain handoff section")
		}
	})

	t.Run("claude-code", func(t *testing.T) {
		wf := exportFixture()
		cmdDir := t.TempDir()
		agentsDir := t.TempDir()
		e := NewExporterWithDirsForTarget(cmdDir, agentsDir, TargetClaudeCode)

		_, files, err := e.Plan(wf)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}

		subPath := filepath.Join(agentsDir, "daily-task-news-briefing.md")
		sub, ok := files[subPath]
		if !ok {
			t.Fatalf("sub-agent markdown not found at %s", subPath)
		}
		if !strings.Contains(sub, "**Status**:") {
			t.Errorf("claude-code sub-agent should contain handoff section (**Status**:), got:\n%s", sub)
		}

		orchPath := filepath.Join(agentsDir, "daily-task-orchestrator.md")
		orch, ok := files[orchPath]
		if !ok {
			t.Fatalf("orchestrator markdown not found at %s", orchPath)
		}
		if strings.Contains(orch, "**Status**:") {
			t.Error("claude-code orchestrator should NOT contain handoff section")
		}
	})

	t.Run("custom-sections", func(t *testing.T) {
		wf := exportFixture()
		for i := range wf.Nodes {
			if wf.Nodes[i].ID == "news" {
				wf.Nodes[i].Data.Sections = "handoff-qa"
			}
		}
		cmdDir := t.TempDir()
		agentsDir := t.TempDir()
		e := NewExporterWithDirs(cmdDir, agentsDir)

		_, files, err := e.Plan(wf)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}

		subPath := filepath.Join(agentsDir, "daily-task-news-briefing.md")
		sub, ok := files[subPath]
		if !ok {
			t.Fatalf("sub-agent markdown not found at %s", subPath)
		}
		if !strings.Contains(sub, "@qa-orchestrator") {
			t.Errorf("custom-sections sub-agent should contain handoff-qa section (@qa-orchestrator), got:\n%s", sub)
		}
	})

	// Regression: the frontend persists Sections as a comma-separated string, so
	// the workflow JSON must decode into NodeData.Sections and export correctly.
	// A prior version typed Sections as []string, which failed to unmarshal the
	// CSV the UI sends, breaking save/load for any node with sections set.
	t.Run("csv-from-frontend-json", func(t *testing.T) {
		var nd NodeData
		if err := json.Unmarshal([]byte(`{"sections":"handoff-qa, context-gathering"}`), &nd); err != nil {
			t.Fatalf("NodeData must decode CSV sections from the frontend: %v", err)
		}
		if got := subAgentSectionList(nd.Sections); len(got) != 2 || got[0] != "handoff-qa" || got[1] != "context-gathering" {
			t.Errorf("subAgentSectionList(%q) = %v, want [handoff-qa context-gathering]", nd.Sections, got)
		}
		if got := subAgentSectionList(""); len(got) != 1 || got[0] != "handoff" {
			t.Errorf("empty sections must default to [handoff], got %v", got)
		}
	})
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
