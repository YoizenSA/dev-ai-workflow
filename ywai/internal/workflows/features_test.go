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
