package kanban

import (
	"testing"
)

func TestStorePersistence(t *testing.T) {
	dir := t.TempDir()

	// Create store, add data
	s1 := NewStore(dir)
	session := s1.CreateSession("test-project", "Test goal")
	del, _ := s1.CreateDelegation(session.ID, "dev", "Do work", nil)

	// Save
	if err := s1.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load into new store
	s2 := NewStore(dir)

	// Verify data survived
	gotSession, ok := s2.GetSession(session.ID)
	if !ok {
		t.Fatal("session not found after reload")
	}
	if gotSession.Goal != "Test goal" {
		t.Fatalf("expected goal 'Test goal', got '%s'", gotSession.Goal)
	}

	gotDel, ok := s2.GetDelegation(del.ID)
	if !ok {
		t.Fatal("delegation not found after reload")
	}
	if gotDel.TaskSummary != "Do work" {
		t.Fatalf("expected task 'Do work', got '%s'", gotDel.TaskSummary)
	}
}
