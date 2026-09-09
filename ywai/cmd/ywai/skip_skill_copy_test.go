package main

import (
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
)

func TestSkipSkillCopy(t *testing.T) {
	opencode := agent.Agent{Name: "opencode"}
	claude := agent.Agent{Name: "claude-code"}

	if skip, _ := skipSkillCopy(opencode, []agent.Agent{opencode, claude}); !skip {
		t.Error("opencode reads ~/.claude/skills: its copy must be skipped when Claude Code is installed")
	}
	// Without Claude Code nobody else carries them, so opencode still needs its own copy.
	if skip, _ := skipSkillCopy(opencode, []agent.Agent{opencode}); skip {
		t.Error("opencode-only install must keep receiving the skills")
	}
	if skip, _ := skipSkillCopy(claude, []agent.Agent{opencode, claude}); skip {
		t.Error("claude-code is the source of truth, never skipped")
	}
}
