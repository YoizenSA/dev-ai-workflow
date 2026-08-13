package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveRetiredCLIs_DeletesIndex(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".codegraph"), 0o755); err != nil {
		t.Fatal(err)
	}

	removed, err := RemoveRetiredCLIs(dir, true)
	if err != nil || len(removed) == 0 {
		t.Fatalf("dry-run: removed=%v err=%v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".codegraph")); err != nil {
		t.Fatalf("dry-run deleted the index: %v", err)
	}

	if _, err := RemoveRetiredCLIs(dir, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".codegraph")); !os.IsNotExist(err) {
		t.Fatalf("index still present: %v", err)
	}
}

func TestRemoveRetiredCLIs_NothingToDo(t *testing.T) {
	removed, err := RemoveRetiredCLIs(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) > 0 && removed[0] != codegraphNPMPackage+" (global npm)" {
		t.Fatalf("unexpected removals: %v", removed)
	}
}
