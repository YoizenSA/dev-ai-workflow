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

func TestStore_DeleteSession(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	// Create a session
	session := s.CreateSession("my-project", "Delete test")
	if session == nil {
		t.Fatal("expected session to be created")
	}

	// Create delegations for this session
	del1, err := s.CreateDelegation(session.ID, "dev", "Task 1", nil)
	if err != nil {
		t.Fatalf("CreateDelegation failed: %v", err)
	}
	del2, err := s.CreateDelegation(session.ID, "qa", "Task 2", nil)
	if err != nil {
		t.Fatalf("CreateDelegation failed: %v", err)
	}

	// Create another session to ensure it's not affected
	otherSession := s.CreateSession("other", "Keep me")
	otherDel, err := s.CreateDelegation(otherSession.ID, "dev", "Other task", nil)
	if err != nil {
		t.Fatalf("CreateDelegation for other session failed: %v", err)
	}

	// Delete the first session
	if err := s.DeleteSession(session.ID); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Verify session is gone
	if _, ok := s.GetSession(session.ID); ok {
		t.Error("expected session to be deleted")
	}

	// Verify delegations of deleted session are gone
	if _, ok := s.GetDelegation(del1.ID); ok {
		t.Error("expected delegation del1 to be deleted with session")
	}
	if _, ok := s.GetDelegation(del2.ID); ok {
		t.Error("expected delegation del2 to be deleted with session")
	}

	// Verify other session and its delegation are untouched
	if _, ok := s.GetSession(otherSession.ID); !ok {
		t.Error("expected other session to still exist")
	}
	if _, ok := s.GetDelegation(otherDel.ID); !ok {
		t.Error("expected other delegation to still exist")
	}

	// Verify delete of non-existent session returns error
	if err := s.DeleteSession("nonexistent"); err == nil {
		t.Error("expected error when deleting non-existent session")
	}
}
