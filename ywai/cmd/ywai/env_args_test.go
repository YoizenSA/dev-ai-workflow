package main

import (
	"slices"
	"strings"
	"testing"
)

func TestParseEnvRunArgs(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantArgv []string
		wantErr  bool
	}{
		{"colon form", []string{"hola", "--model:opencode-go/glm-5.3-flash", "--agent:ask"},
			[]string{"run", "--model", "opencode-go/glm-5.3-flash", "--agent", "ask", "hola"}, false},
		{"space and equals, words joined", []string{"--model", "zai/glm-5.3", "-a=ask", "hola", "mundo"},
			[]string{"run", "--model", "zai/glm-5.3", "--agent", "ask", "hola mundo"}, false},
		{"short -m and --auto", []string{"-m", "x/y", "fix it", "--auto"},
			[]string{"run", "--model", "x/y", "--auto", "fix it"}, false},
		{"double dash keeps literal text", []string{"hola", "--", "--model", "literal"},
			[]string{"run", "hola --model literal"}, false},
		{"passthrough value flag", []string{"describe", "--file", "a.png"},
			[]string{"run", "--file", "a.png", "describe"}, false},
		{"tui continue", []string{"-c"}, []string{"-c"}, false},
		{"tui plain", nil, nil, false},
		{"model without prompt", []string{"--model:x/y"}, nil, true},
		{"missing value", []string{"hola", "--agent"}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prompt, opts, err := parseEnvRunArgs(tc.args)
			if err == nil {
				var argv []string
				argv, err = envOpencodeArgv(prompt, opts)
				if err == nil && !slices.Equal(argv, tc.wantArgv) {
					t.Fatalf("argv = %q, want %q", argv, tc.wantArgv)
				}
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestResolveWorkflowRun(t *testing.T) {
	// name → {agent, body}; an empty agent marks a plain command.
	commands := map[string][2]string{
		"review-wf": {"review-wf-orchestrator", ""},
		"notes":     {"", "Write notes about $ARGUMENTS"},
		"todo":      {"", "List todos"},
		"empty":     {"", ""},
	}
	lookup := func(n string) (string, string, bool) {
		c, ok := commands[n]
		return c[0], c[1], ok
	}
	list := func() []string { return []string{"notes", "review-wf", "todo"} }

	cases := []struct {
		name      string
		args      []string
		wantArgv  []string
		wantWF    string
		wantErrIs string
	}{
		{"first word is a workflow", []string{"review-wf", "revisá", "el", "PR"},
			[]string{"run", "--agent", "review-wf-orchestrator", "revisá el PR"}, "review-wf", ""},
		{"slash form", []string{"/review-wf", "texto libre"},
			[]string{"run", "--agent", "review-wf-orchestrator", "texto libre"}, "review-wf", ""},
		{"flag form keeps the whole line as task", []string{"--workflow:review-wf", "texto", "libre"},
			[]string{"run", "--agent", "review-wf-orchestrator", "texto libre"}, "review-wf", ""},
		{"no task text falls back like the Run button", []string{"-w", "review-wf"},
			[]string{"run", "--agent", "review-wf-orchestrator", "Run the workflow."}, "review-wf", ""},
		{"explicit --agent wins", []string{"review-wf", "--agent", "ask", "hola"},
			[]string{"run", "--agent", "ask", "hola"}, "review-wf", ""},
		{"a plain command is not claimed by a bare word", []string{"notes", "algo"},
			[]string{"run", "notes algo"}, "", ""},
		{"flag form runs a plain command body with the task", []string{"-w", "notes", "algo"},
			[]string{"run", "Write notes about algo"}, "notes", ""},
		{"plain command without a task runs the bare body", []string{"--workflow", "notes"},
			[]string{"run", "Write notes about"}, "notes", ""},
		{"body without the placeholder appends the task", []string{"-w", "todo", "urgente"},
			[]string{"run", "List todos\n\nurgente"}, "todo", ""},
		{"a plain prompt is never hijacked", []string{"revisá", "el", "PR"},
			[]string{"run", "revisá el PR"}, "", ""},
		{"unknown workflow lists what exists", []string{"--workflow", "nope"}, nil, "", "available: notes, review-wf, todo"},
		{"empty plain command body", []string{"-w", "empty"}, nil, "", "no prompt body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prompt, opts, err := parseEnvRunArgs(tc.args)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			prompt, opts, err = resolveWorkflowRun(prompt, opts, lookup, list)
			if tc.wantErrIs != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrIs) {
					t.Fatalf("err = %v, want one naming %q", err, tc.wantErrIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if opts.Workflow != tc.wantWF {
				t.Errorf("workflow = %q, want %q", opts.Workflow, tc.wantWF)
			}
			argv, err := envOpencodeArgv(prompt, opts)
			if err != nil {
				t.Fatalf("argv: %v", err)
			}
			if !slices.Equal(argv, tc.wantArgv) {
				t.Errorf("argv = %q, want %q", argv, tc.wantArgv)
			}
		})
	}

	// An env with nothing exported explains how to export instead.
	_, _, err := resolveWorkflowRun(nil, envRunOpts{Workflow: "review-wf"},
		func(string) (string, string, bool) { return "", "", false }, func() []string { return nil })
	if err == nil || !strings.Contains(err.Error(), "export it") {
		t.Errorf("empty env err = %v", err)
	}
}

func TestWantsHelp(t *testing.T) {
	if !wantsHelp([]string{"--help"}) || !wantsHelp([]string{"x", "-h"}) {
		t.Error("help flag not detected")
	}
	if wantsHelp([]string{"--", "--help"}) {
		t.Error("--help after -- is prompt text, not a help request")
	}
}
