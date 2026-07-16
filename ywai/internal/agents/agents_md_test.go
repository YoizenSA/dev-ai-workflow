package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAgentsMd_OnlyThreeConcerns(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	if err := WriteAgentsMd(path); err != nil {
		t.Fatalf("WriteAgentsMd error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	content := string(got)

	for _, want := range []string{
		"## Engram Persistent Memory",
		"PROACTIVE SAVE TRIGGERS",
		"SESSION CLOSE PROTOCOL",
		"AFTER COMPACTION",
		"## Sub-Agents — Strategy",
		"Sub-Agent Launch Deduplication",
		"Sub-Agent Launch Pattern",
		"Skill Resolution Feedback",
		"Sub-Agent Context Protocol",
		"## CodeGraph",
		"<!-- CODEGRAPH_START -->",
		"<!-- CODEGRAPH_END -->",
		"mem_save",
		"codegraph_explore",
		"CodeGraph Rules of Thumb",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("AGENTS.md missing %q", want)
		}
	}
}

func TestWriteAgentsMd_ExcludesNonOwnedSections(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	if err := WriteAgentsMd(path); err != nil {
		t.Fatalf("WriteAgentsMd error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	content := string(got)

	for _, forbidden := range []string{
		"## Skills\n",
		"## Hooks\n",
		"gentle-ai:engram-protocol",
		"gentle-ai:sdd-orchestrator",
		"gentle-ai:persona",
		"sdd-orchestrator",
		"SDD Workflow",
		"review-readability",
		"Contextual Skill Loading",
		"Agent Trigger Rules",
	} {
		if strings.Contains(content, forbidden) {
			t.Errorf("AGENTS.md must not contain %q", forbidden)
		}
	}
}

func TestWriteAgentsMd_CreatesParentDirs(t *testing.T) {
	// Writing to a path whose parent does not yet exist should fail clearly
	// (os.WriteFile does not mkdir). Callers create the config dir first.
	dir := t.TempDir()
	nested := filepath.Join(dir, "missing", "AGENTS.md")
	if err := WriteAgentsMd(nested); err == nil {
		t.Fatal("expected error when parent dir missing, got nil")
	}
}
