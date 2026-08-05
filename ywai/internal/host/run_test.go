package host

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandArgsOpenCode(t *testing.T) {
	args, err := commandArgs(RunSpec{
		Host:   OpenCode,
		Agent:  "goal-orchestrator",
		Model:  "opencode-admin/grok-4.5",
		Prompt: "Deliver the goal.",
	})
	if err != nil {
		t.Fatalf("commandArgs: %v", err)
	}
	want := []string{"run", "--agent", "goal-orchestrator", "--model", "opencode-admin/grok-4.5", "Deliver the goal."}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("opencode args = %v, want %v", args, want)
	}
}

func TestCommandArgsPi(t *testing.T) {
	args, err := commandArgs(RunSpec{
		Host:   Pi,
		Agent:  "goal-orchestrator",
		Model:  "opencode-admin/grok-4.5",
		Prompt: "Deliver the goal.",
	})
	if err != nil {
		t.Fatalf("commandArgs: %v", err)
	}
	for _, arg := range args {
		if arg == "--agent" {
			t.Fatalf("pi args must not contain --agent: %v", args)
		}
	}
	wantAgentPath := filepath.Join(AgentsDir(Pi), "goal-orchestrator.md")
	want := []string{"--append-system-prompt", wantAgentPath, "--model", "opencode-admin/grok-4.5", "--print", "Deliver the goal."}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("pi args = %v, want %v", args, want)
	}
}

func TestCommandArgsOMP(t *testing.T) {
	args, err := commandArgs(RunSpec{
		Host:   OMP,
		Agent:  "goal-orchestrator",
		Model:  "opencode-admin/grok-4.5",
		Prompt: "Deliver the goal.",
	})
	if err != nil {
		t.Fatalf("commandArgs: %v", err)
	}
	for _, arg := range args {
		if arg == "--agent" {
			t.Fatalf("omp args must not contain --agent: %v", args)
		}
	}
	wantAgentPath := filepath.Join(AgentsDir(OMP), "goal-orchestrator.md")
	want := []string{"--append-system-prompt", wantAgentPath, "--model", "opencode-admin/grok-4.5", "-p", "Deliver the goal."}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("omp args = %v, want %v", args, want)
	}
}

func TestCommandArgsDefaults(t *testing.T) {
	args, err := commandArgs(RunSpec{Host: Pi})
	if err != nil {
		t.Fatalf("commandArgs: %v", err)
	}
	// Empty agent/model: no agent flag at all, default prompt, print mode.
	want := []string{"--print", "Run the workflow."}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("pi default args = %v, want %v", args, want)
	}

	if _, err := commandArgs(RunSpec{Host: Claude}); err == nil {
		t.Fatal("expected error for host without workflow run support")
	}
}
