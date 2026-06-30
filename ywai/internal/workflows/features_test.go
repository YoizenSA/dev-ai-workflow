package workflows

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestSlashCommandOptionsEmittedInCommand verifies the advanced slash-command
// frontmatter fields are emitted when SlashCommandOptions is set, and omitted
// when it is not (default output unchanged).
func TestSlashCommandOptionsEmittedInCommand(t *testing.T) {
	cmdDir := t.TempDir()
	agentsDir := t.TempDir()
	e := NewExporterWithDirs(cmdDir, agentsDir)

	wf := exportFixture()
	wf.SlashCommandOptions = &SlashCommandOptions{
		AllowedTools:          "read,webfetch",
		Model:                 "sonnet",
		Context:               "fork",
		ArgumentHint:          "[topic]",
		DisableModelInvocation: true,
		Hooks: &WorkflowHooks{
			Stop: []HookEntry{{
				Matcher: "",
				Hooks:   []HookAction{{Type: "command", Command: "echo done"}},
			}},
		},
	}

	_, files, err := e.Plan(wf)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	cmd := files[filepath.Join(cmdDir, "daily-task.md")]
	for _, want := range []string{
		"allowed-tools: read,webfetch",
		"model: sonnet",
		"context: fork",
		// argument-hint contains "[", so yamlQuote wraps it; both shapes are valid.
		`argument-hint: "[topic]"`,
		"disable-model-invocation: true",
		"hooks:",
		"Stop:",
		"type: command",
		"command: echo done",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("command missing %q:\n%s", want, cmd)
		}
	}
}

// TestSlashCommandOptionsOmittedByDefault ensures a workflow without options
// produces the same minimal frontmatter as before.
func TestSlashCommandOptionsOmittedByDefault(t *testing.T) {
	cmdDir := t.TempDir()
	agentsDir := t.TempDir()
	e := NewExporterWithDirs(cmdDir, agentsDir)

	_, files, err := e.Plan(exportFixture())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	cmd := files[filepath.Join(cmdDir, "daily-task.md")]
	for _, banned := range []string{"allowed-tools:", "argument-hint:", "hooks:", "disable-model-invocation"} {
		if strings.Contains(cmd, banned) {
			t.Errorf("command should not contain %q when no options set:\n%s", banned, cmd)
		}
	}
}

// TestMcpStepRendersThreeModes checks the orchestrator prompt renders each MCP
// configuration mode with the right instruction.
func TestMcpStepRendersThreeModes(t *testing.T) {
	cases := []struct {
		name   string
		data   NodeData
		wants  []string
		banned []string
	}{
		{
			name:  "aiToolSelection",
			data:  NodeData{McpMode: MCPModeAIToolSelection, Server: "github", TaskDescription: "create an issue"},
			wants: []string{"AI tool selection", "github", "create an issue", "tools/list"},
		},
		{
			name:  "aiParameterConfig",
			data:  NodeData{McpMode: MCPModeAIParameterConfig, Server: "github", Tool: "create_issue", AIParams: "fill from repo"},
			wants: []string{"Call MCP tool", "github/create_issue", "fill from repo"},
		},
		{
			name:  "manualParameterConfig",
			data:  NodeData{McpMode: MCPModeManualParameterConfig, Server: "github", Tool: "list_repos"},
			wants: []string{"Call MCP tool", "github/list_repos"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n := &Node{Type: NodeTypeMCP, Data: c.data}
			got := mcpStep(n)
			for _, w := range c.wants {
				if !strings.Contains(got, w) {
					t.Errorf("missing %q in:\n%s", w, got)
				}
			}
			for _, b := range c.banned {
				if strings.Contains(got, b) {
					t.Errorf("should not contain %q in:\n%s", b, got)
				}
			}
		})
	}
}

// TestMcpStepEmptyModeDefaultsToAIParameterConfig: a node with no Mode (legacy
// workflow) must behave like aiParameterConfig for backward compat.
func TestMcpStepEmptyModeDefaultsToAIParameterConfig(t *testing.T) {
	n := &Node{Type: NodeTypeMCP, Data: NodeData{Server: "s", Tool: "t", AIParams: "p"}}
	got := mcpStep(n)
	if !strings.Contains(got, "Call MCP tool") {
		t.Errorf("empty mode should render as aiParameterConfig:\n%s", got)
	}
	if strings.Contains(got, "AI tool selection") {
		t.Errorf("empty mode should NOT render as aiToolSelection:\n%s", got)
	}
}

// TestValidateMCPModeAcceptsValidAndRejectsInvalid checks the mode enum.
func TestValidateMCPModeAcceptsValidAndRejectsInvalid(t *testing.T) {
	validModes := []string{"", MCPModeManualParameterConfig, MCPModeAIParameterConfig, MCPModeAIToolSelection}
	for _, m := range validModes {
		wf := &Workflow{
			Name:    "w",
			Version: "1.0.0",
			Nodes: []Node{
				{ID: "s", Type: NodeTypeStart, Name: "s"},
				{ID: "m", Type: NodeTypeMCP, Name: "m", Data: NodeData{McpMode: m, Server: "x"}},
				{ID: "e", Type: NodeTypeEnd, Name: "e"},
			},
			Connections: []Connection{{From: "s", To: "m"}, {From: "m", To: "e"}},
		}
		res := Validate(wf)
		// Should not produce an "invalid mode" error for any valid mode.
		for _, iss := range res.Errors {
			if strings.Contains(iss.Message, "invalid mode") {
				t.Errorf("mode %q flagged invalid: %s", m, iss.Message)
			}
		}
	}

	// Invalid mode must error.
	wf := &Workflow{
		Name:    "w",
		Version: "1.0.0",
		Nodes: []Node{
			{ID: "s", Type: NodeTypeStart, Name: "s"},
			{ID: "m", Type: NodeTypeMCP, Name: "m", Data: NodeData{McpMode: "bogus", Server: "x"}},
			{ID: "e", Type: NodeTypeEnd, Name: "e"},
		},
		Connections: []Connection{{From: "s", To: "m"}, {From: "m", To: "e"}},
	}
	res := Validate(wf)
	found := false
	for _, iss := range res.Errors {
		if strings.Contains(iss.Message, "invalid mode") {
			found = true
		}
	}
	if !found {
		t.Error("expected an 'invalid mode' error for bogus mode")
	}
}

// TestValidateSlashCommandOptions checks model/context enums.
func TestValidateSlashCommandOptions(t *testing.T) {
	bad := &Workflow{
		Name:               "w",
		Version:            "1.0.0",
		Nodes:              []Node{{ID: "s", Type: NodeTypeStart, Name: "s"}, {ID: "e", Type: NodeTypeEnd, Name: "e"}},
		Connections:        []Connection{{From: "s", To: "e"}},
		SlashCommandOptions: &SlashCommandOptions{Model: "bogus", Context: "bogus"},
	}
	res := Validate(bad)
	badModel, badCtx := false, false
	for _, iss := range res.Errors {
		if strings.Contains(iss.Message, "not a valid model") {
			badModel = true
		}
		if strings.Contains(iss.Message, "not valid") && strings.Contains(iss.Message, "context") {
			badCtx = true
		}
	}
	if !badModel {
		t.Error("expected model enum error")
	}
	if !badCtx {
		t.Error("expected context enum error")
	}
}

// TestEstimatedTokensPopulated checks the chars/4 estimate lands on ExportPlan.
func TestEstimatedTokensPopulated(t *testing.T) {
	cmdDir := t.TempDir()
	agentsDir := t.TempDir()
	e := NewExporterWithDirs(cmdDir, agentsDir)
	plan, _, err := e.Plan(exportFixture())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.EstimatedTokens <= 0 {
		t.Errorf("expected positive EstimatedTokens, got %d", plan.EstimatedTokens)
	}
}

// TestEstimateTokensHeuristic pins the ceil(chars/4) math.
func TestEstimateTokensHeuristic(t *testing.T) {
	cases := map[int]int{
		0:   0,
		1:   1,
		4:   1,
		5:   2,
		8:   2,
		100: 25,
	}
	for chars, want := range cases {
		s := strings.Repeat("a", chars)
		if got := estimateTokens(s); got != want {
			t.Errorf("estimateTokens(len=%d) = %d, want %d", chars, got, want)
		}
	}
}

// TestConversationHistoryRoundTrips verifies a workflow with a conversation
// history serializes and reloads preserving the messages.
func TestConversationHistoryRoundTrips(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	wf := &Workflow{
		Name:    "chat",
		Version: "1.0.0",
		Nodes: []Node{
			{ID: "s", Type: NodeTypeStart, Name: "s"},
			{ID: "e", Type: NodeTypeEnd, Name: "e"},
		},
		Connections: []Connection{{From: "s", To: "e"}},
		ConversationHistory: &ConversationHistory{
			SchemaVersion: "1.0.0",
			MaxIterations: 20,
			Messages: []ConversationMessage{
				{ID: "1", Sender: "user", Content: "add a node"},
				{ID: "2", Sender: "ai", Content: "done"},
			},
		},
	}
	if err := s.Create(wf); err != nil {
		t.Fatalf("Create: %v", err)
	}
	loaded, err := s.Load("chat")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ConversationHistory == nil || len(loaded.ConversationHistory.Messages) != 2 {
		t.Fatalf("conversation history not preserved: %+v", loaded.ConversationHistory)
	}
}

// TestToolsToPermissions verifies the CSV → permission map conversion, including
// bucket expansion and the :deny suffix.
func TestToolsToPermissions(t *testing.T) {
	cases := []struct {
		name     string
		csv      string
		defaults string
		want     map[string]string
	}{
		{
			name:     "empty uses defaults",
			csv:      "",
			defaults: "read,task",
			want:     map[string]string{"read": "allow", "task": "allow"},
		},
		{
			name:     "explicit tools override defaults",
			csv:      "read,edit,bash",
			defaults: "read,task",
			want:     map[string]string{"read": "allow", "edit": "allow", "bash": "allow"},
		},
		{
			name:     "deny suffix blocks a tool",
			csv:      "read,edit,bash:deny",
			defaults: "read",
			want:     map[string]string{"read": "allow", "edit": "allow", "bash": "deny"},
		},
		{
			name:     "mcp bucket expands to wildcards",
			csv:      "read,mcp",
			defaults: "read",
			want: map[string]string{
				"read":          "allow",
				"codegraph_*":   "allow",
				"context7_*":    "allow",
				"ywai-kanban_*": "allow",
			},
		},
		{
			name:     "delegate bucket expands",
			csv:      "read,delegate",
			defaults: "read",
			want: map[string]string{
				"read":          "allow",
				"delegate":      "allow",
				"delegation_*":  "allow",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := toolsToPermissions(c.csv, c.defaults)
			if len(got) != len(c.want) {
				t.Fatalf("len mismatch: got %d (%+v), want %d (%+v)", len(got), got, len(c.want), c.want)
			}
			for k, v := range c.want {
				if got[k] != v {
					t.Errorf("perm[%q] = %q, want %q (full: %+v)", k, got[k], v, got)
				}
			}
		})
	}
}

// TestDelegationMapFromOutgoing verifies that a subAgent's delegation map is
// derived from its outgoing edges to other subAgents.
func TestDelegationMapFromOutgoing(t *testing.T) {
	wf := &Workflow{
		Name: "w",
		Nodes: []Node{
			{ID: "s", Type: NodeTypeStart},
			{ID: "dev", Type: NodeTypeSubAgent},
			{ID: "qa", Type: NodeTypeSubAgent},
			{ID: "rev", Type: NodeTypeSubAgent},
			{ID: "e", Type: NodeTypeEnd},
		},
		Connections: []Connection{
			{From: "dev", To: "qa"},   // dev → qa: allowed
			{From: "dev", To: "rev"},  // dev → rev: allowed
			{From: "qa", To: "e"},     // qa → end: not a subAgent, ignored
			{From: "rev", To: "e"},    // rev → end: no subAgent targets
		},
	}
	subAgentIDs := map[string]string{
		"dev": "w-dev",
		"qa":  "w-qa",
		"rev": "w-rev",
	}

	// dev has edges to qa + rev → both allowed.
	devMap := delegationMapFromOutgoing(wf, "dev", subAgentIDs)
	if devMap["*"] != "deny" || devMap["w-qa"] != "allow" || devMap["w-rev"] != "allow" {
		t.Errorf("dev map wrong: %+v", devMap)
	}

	// rev has no edges to subAgents → only deny.
	revMap := delegationMapFromOutgoing(wf, "rev", subAgentIDs)
	if revMap["*"] != "deny" || len(revMap) != 1 {
		t.Errorf("rev map should be deny-only: %+v", revMap)
	}
}

// TestDelegationMapWithDelegateTo verifies the border case: a sub-agent with
// delegateTo="finder" can delegate to finder even without a visible edge.
func TestDelegationMapWithDelegateTo(t *testing.T) {
	wf := &Workflow{
		Name: "w",
		Nodes: []Node{
			{ID: "s", Type: NodeTypeStart},
			{ID: "dev", Type: NodeTypeSubAgent, Data: NodeData{DelegateTo: "finder"}},
			{ID: "finder", Type: NodeTypeSubAgent},
			{ID: "e", Type: NodeTypeEnd},
		},
		Connections: []Connection{
			{From: "s", To: "dev"},
			{From: "dev", To: "e"}, // no edge to finder
		},
	}
	subAgentIDs := map[string]string{
		"dev":    "w-dev",
		"finder": "w-finder",
	}
	// dev has no edge to finder, but delegateTo="finder" grants it.
	devMap := delegationMapFromOutgoing(wf, "dev", subAgentIDs)
	if devMap["w-finder"] != "allow" {
		t.Errorf("dev should delegate to finder via delegateTo: %+v", devMap)
	}
	if devMap["*"] != "deny" {
		t.Errorf("dev should still deny everything else: %+v", devMap)
	}
}

// TestDelegationMapWithExternalAgent verifies delegateTo with an external agent
// name (not a workflow node) passes through verbatim.
func TestDelegationMapWithExternalAgent(t *testing.T) {
	wf := &Workflow{
		Name: "w",
		Nodes: []Node{
			{ID: "s", Type: NodeTypeStart},
			{ID: "dev", Type: NodeTypeSubAgent, Data: NodeData{DelegateTo: "memory, ask"}},
			{ID: "e", Type: NodeTypeEnd},
		},
		Connections: []Connection{
			{From: "s", To: "dev"},
			{From: "dev", To: "e"},
		},
	}
	subAgentIDs := map[string]string{"dev": "w-dev"}
	devMap := delegationMapFromOutgoing(wf, "dev", subAgentIDs)
	if devMap["memory"] != "allow" || devMap["ask"] != "allow" {
		t.Errorf("dev should delegate to external memory+ask: %+v", devMap)
	}
}

// TestOrchestratorExportHasDelegationMap verifies the exported orchestrator .md
// contains a nested permission.task with the workflow's sub-agents and "*": deny.
func TestOrchestratorExportHasDelegationMap(t *testing.T) {
	wf := &Workflow{
		Name:    "deploy",
		Version: "1.0.0",
		Nodes: []Node{
			{ID: "s", Type: NodeTypeStart, Name: "s"},
			{ID: "dev", Type: NodeTypeSubAgent, Name: "dev"},
			{ID: "qa", Type: NodeTypeSubAgent, Name: "qa"},
			{ID: "e", Type: NodeTypeEnd, Name: "e"},
		},
		Connections: []Connection{
			{From: "s", To: "dev"},
			{From: "dev", To: "qa"},
			{From: "qa", To: "e"},
		},
	}
	e := NewExporterWithDirs(t.TempDir(), t.TempDir())
	_, files, err := e.Plan(wf)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	orchPath := ""
	for p := range files {
		if strings.HasSuffix(p, "deploy-orchestrator.md") {
			orchPath = p
		}
	}
	if orchPath == "" {
		t.Fatal("orchestrator file not found")
	}
	orch := files[orchPath]
	for _, want := range []string{
		`"*": deny`,
		"task:",
		"deploy-dev: allow",
		"deploy-qa: allow",
	} {
		if !strings.Contains(orch, want) {
			t.Errorf("orchestrator missing %q:\n%s", want, orch)
		}
	}
}

// TestSubAgentExportHasDelegationFromEdges verifies that a subAgent's delegation
// map reflects its outgoing edges (not all subAgents).
func TestSubAgentExportHasDelegationFromEdges(t *testing.T) {
	wf := &Workflow{
		Name:    "w",
		Version: "1.0.0",
		Nodes: []Node{
			{ID: "s", Type: NodeTypeStart, Name: "s"},
			{ID: "dev", Type: NodeTypeSubAgent, Name: "dev"},
			{ID: "qa", Type: NodeTypeSubAgent, Name: "qa"},
			{ID: "rev", Type: NodeTypeSubAgent, Name: "rev"},
			{ID: "e", Type: NodeTypeEnd, Name: "e"},
		},
		Connections: []Connection{
			{From: "dev", To: "qa"},
			{From: "qa", To: "rev"},
		},
	}
	e := NewExporterWithDirs(t.TempDir(), t.TempDir())
	_, files, err := e.Plan(wf)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	// dev should be able to delegate to qa (w-qa) but NOT to rev.
	devPath := ""
	for p := range files {
		if strings.HasSuffix(p, "w-dev.md") {
			devPath = p
		}
	}
	if devPath == "" {
		t.Fatal("dev agent file not found")
	}
	dev := files[devPath]
	if !strings.Contains(dev, "w-qa: allow") {
		t.Errorf("dev should delegate to qa:\n%s", dev)
	}
	if strings.Contains(dev, "w-rev: allow") {
		t.Errorf("dev should NOT delegate to rev (no edge):\n%s", dev)
	}
}
