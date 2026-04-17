package main

import "testing"

func TestParseAgentMarkdown_BlockTools(t *testing.T) {
	md := `---
description: Senior engineer
mode: subagent
tools:
  read: true
  write: true
  edit: false
  bash: true
---
You are a senior engineer.
`

	desc, tools, prompt := parseAgentMarkdown(md)

	if desc != "Senior engineer" {
		t.Errorf("description = %q, want %q", desc, "Senior engineer")
	}
	if !tools["read"] || !tools["write"] || !tools["bash"] {
		t.Errorf("expected read/write/bash=true, got %v", tools)
	}
	if tools["edit"] {
		t.Errorf("edit must be false, got %v", tools["edit"])
	}
	if prompt != "You are a senior engineer." {
		t.Errorf("prompt = %q, want %q", prompt, "You are a senior engineer.")
	}
}

func TestParseAgentMarkdown_InlineTools(t *testing.T) {
	md := `---
description: QA agent
mode: subagent
tools: [read, bash]
---
Test things.
`

	desc, tools, prompt := parseAgentMarkdown(md)

	if desc != "QA agent" {
		t.Errorf("description = %q", desc)
	}
	if !tools["read"] || !tools["bash"] {
		t.Errorf("expected read/bash, got %v", tools)
	}
	if tools["write"] || tools["edit"] {
		t.Errorf("write/edit should not be set, got %v", tools)
	}
	if prompt != "Test things." {
		t.Errorf("prompt = %q", prompt)
	}
}

func TestRenderAgentToolsBlock(t *testing.T) {
	names := []string{"read", "write", "edit", "bash"}
	values := []bool{true, false, true, false}

	got := renderAgentToolsBlock(names, values)
	want := "tools:\n  read: true\n  write: false\n  edit: true\n  bash: false\n"
	if got != want {
		t.Errorf("renderAgentToolsBlock(...) =\n%q\nwant\n%q", got, want)
	}

	// All false -> empty string (keeps frontmatter minimal).
	allOff := renderAgentToolsBlock(names, []bool{false, false, false, false})
	if allOff != "" {
		t.Errorf("all-false should return empty string, got %q", allOff)
	}
}
