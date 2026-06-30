package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAgentsMd_IncludesCuratedSections(t *testing.T) {
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
		"## Skills",
		"## Sub-Agents",
		"## Hooks",
		"mem_save",
		"Sub-Agent Launch Deduplication",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("AGENTS.md missing %q", want)
		}
	}
}

func TestWriteAgentsMd_ExcludesSDDAndPersona(t *testing.T) {
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
		"gentle-ai:engram-protocol",
		"gentle-ai:sdd-orchestrator",
		"gentle-ai:persona",
		"sdd-orchestrator",
		"SDD Workflow",
	} {
		if strings.Contains(content, forbidden) {
			t.Errorf("AGENTS.md must not contain %q (SDD/persona excluded by default)", forbidden)
		}
	}
}

func TestWriteAgentsMd_CreatesParentDirs(t *testing.T) {
	// Writing to a path whose parent does not yet exist should fail clearly
	// (os.WriteFile does not mkdir). This pins that contract so callers know
	// to create the config dir first — executeInstall already does.
	dir := t.TempDir()
	nested := filepath.Join(dir, "missing", "AGENTS.md")
	if err := WriteAgentsMd(nested); err == nil {
		t.Fatal("expected error when parent dir missing, got nil")
	}
}
