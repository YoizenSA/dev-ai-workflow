package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGroupAgentBasenamesUnknown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "groups.json"), []byte(`{"groups":{"core":{"agents":["dev"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := GroupAgentBasenames(dir, "nope")
	if err == nil {
		t.Fatal("expected unknown group")
	}
}

func TestGroupAgentBasenamesFlattens(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"groups":{"social-refactor":{"agents":["migration-planner","experiment/infra-docs"]}}}`
	if err := os.WriteFile(filepath.Join(dir, "groups.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := GroupAgentBasenames(dir, "social-refactor")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"migration-planner": true, "infra-docs": true}
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	for _, n := range got {
		if !want[n] {
			t.Fatalf("unexpected %q in %v", n, got)
		}
	}
}

func TestDisableGroupRefusesCore(t *testing.T) {
	_, err := DisableGroupAgents(t.TempDir(), "core", []string{"dev"})
	if err == nil {
		t.Fatal("expected refuse core")
	}
}

func TestDisableGroupRemovesOnlyThatGroupsFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dev.md"), []byte("core"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "migration-planner.md"), []byte("sr"), 0o644); err != nil {
		t.Fatal(err)
	}

	removed, err := DisableGroupAgents(dir, "social-refactor", []string{"migration-planner"})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != "migration-planner" {
		t.Fatalf("removed = %v", removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "dev.md")); err != nil {
		t.Fatal("core agent should remain")
	}
	if _, err := os.Stat(filepath.Join(dir, "migration-planner.md")); !os.IsNotExist(err) {
		t.Fatal("group agent should be gone")
	}
}
