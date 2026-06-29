package workflows

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateName(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
	}{
		{"deploy", true},
		{"daily-task-workflow", true},
		{"my_workflow_2", true},
		{"", false},      // empty
		{"UPPER", false}, // uppercase
		{"with space", false},
		{"with.dot", false},
		{"中文", false}, // non-ascii
	}
	for _, c := range cases {
		err := ValidateName(c.name)
		if (err == nil) != c.ok {
			t.Errorf("ValidateName(%q): ok=%v, want %v (err=%v)", c.name, err == nil, c.ok, err)
		}
	}
}

func TestStoreCRUD(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	wf := &Workflow{
		Name:        "deploy",
		Description: "Deploy something",
		Version:     "1.0.0",
		Nodes: []Node{
			{ID: "s", Type: NodeTypeStart, Name: "s"},
			{ID: "e", Type: NodeTypeEnd, Name: "e"},
		},
		Connections: []Connection{{From: "s", To: "e"}},
	}

	// Create.
	if err := s.Create(wf); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// File exists on disk.
	if _, err := os.Stat(filepath.Join(dir, "deploy.json")); err != nil {
		t.Fatalf("expected file on disk: %v", err)
	}

	// Duplicate Create fails.
	if err := s.Create(wf); err == nil {
		t.Fatal("expected ErrWorkflowExists on duplicate Create")
	}

	// Load.
	got, err := s.Load("deploy")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Name != "deploy" || len(got.Nodes) != 2 {
		t.Fatalf("loaded workflow mismatch: %+v", got)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatal("timestamps should be set")
	}

	// List.
	summaries, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(summaries) != 1 || summaries[0].Name != "deploy" {
		t.Fatalf("List mismatch: %+v", summaries)
	}

	// Save (update).
	got.Description = "updated"
	if err := s.Save(got); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got2, _ := s.Load("deploy")
	if got2.Description != "updated" {
		t.Fatalf("Save did not persist: %q", got2.Description)
	}

	// Rename.
	if err := s.Rename("deploy", "release"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if _, err := s.Load("deploy"); err == nil {
		t.Fatal("old name should be gone after rename")
	}
	if _, err := s.Load("release"); err != nil {
		t.Fatalf("new name should exist: %v", err)
	}

	// Delete.
	if err := s.Delete("release"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Load("release"); err == nil {
		t.Fatal("workflow should be gone after Delete")
	}
}

func TestStoreLoadMissing(t *testing.T) {
	s := NewStore(t.TempDir())
	if _, err := s.Load("nope"); err == nil {
		t.Fatal("expected error loading missing workflow")
	}
}

func TestStoreRejectsBadName(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Create(&Workflow{Name: "Bad Name"}); err == nil {
		t.Fatal("expected error for bad name")
	}
}
