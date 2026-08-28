package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAgentsMd_OnlyTwoConcerns(t *testing.T) {
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
		"### Session close",
		"### After compaction",
		"## Sub-Agents",
		"### One launch per task",
		"### Skills: match by trigger, load by id",
		// Artifact writing style. It rides in ### Language on purpose: a style
		// rule applies while writing anything, so a skill would never load in
		// time, and a section of its own would break the two-concern scope.
		"ASD-STE100",
		"<available_skills>",
		"### Context protocol",
		"mem_save",
		"mem_session_summary",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("AGENTS.md missing %q", want)
		}
	}
}

// graft install writes its own AGENTS.md marker section (see
// plugins.WireGraftMCP). ywai must not author that surface too, or the two
// installers fight over the same block.
func TestWriteAgentsMd_LeavesGraftToItsOwnInstaller(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	if err := WriteAgentsMd(path); err != nil {
		t.Fatalf("WriteAgentsMd error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	for _, forbidden := range []string{"CODEGRAPH_START", "CODEGRAPH_END", "codegraph_explore"} {
		if strings.Contains(string(got), forbidden) {
			t.Errorf("AGENTS.md must not contain %q — graft owns that section", forbidden)
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
