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

func TestStore_AddActivity(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	session := s.CreateSession("test-project", "Activity test")
	del, err := s.CreateDelegation(session.ID, "dev", "Implement feature", nil)
	if err != nil {
		t.Fatalf("CreateDelegation failed: %v", err)
	}

	// Test adding a valid activity
	activity := &ActivityEvent{
		Type:    ActivityProgress,
		Content: "Starting implementation",
	}
	if err := s.AddActivity(del.ID, activity); err != nil {
		t.Fatalf("AddActivity failed: %v", err)
	}

	// Verify activity got an ID and timestamp
	if activity.ID == "" {
		t.Error("activity ID should be set")
	}
	if activity.CreatedAt.IsZero() {
		t.Error("activity CreatedAt should be set")
	}

	// Verify delegation's LatestActivity is updated
	gotDel, ok := s.GetDelegation(del.ID)
	if !ok {
		t.Fatal("delegation not found after adding activity")
	}
	if gotDel.LatestActivity != string(ActivityProgress) {
		t.Errorf("expected LatestActivity 'progress', got '%s'", gotDel.LatestActivity)
	}
	if gotDel.PendingAction {
		t.Error("PendingAction should be false for progress activity")
	}

	// Test adding a decision activity sets PendingAction
	decision := &ActivityEvent{
		Type:    ActivityDecision,
		Content: "Approve architecture?",
		Options: []string{"approve", "reject"},
	}
	if err := s.AddActivity(del.ID, decision); err != nil {
		t.Fatalf("AddActivity decision failed: %v", err)
	}

	gotDel, ok = s.GetDelegation(del.ID)
	if !ok {
		t.Fatal("delegation not found after decision")
	}
	if !gotDel.PendingAction {
		t.Error("PendingAction should be true after decision activity")
	}
	if gotDel.LatestActivity != string(ActivityDecision) {
		t.Errorf("expected LatestActivity 'decision', got '%s'", gotDel.LatestActivity)
	}

	// Test adding activity to non-existent delegation returns error
	if err := s.AddActivity("nonexistent", activity); err == nil {
		t.Error("expected error when adding activity to non-existent delegation")
	}

	// Test adding a blocked activity also sets PendingAction
	blocked := &ActivityEvent{
		Type:    ActivityBlocked,
		Content: "Waiting for access",
	}
	if err := s.AddActivity(del.ID, blocked); err != nil {
		t.Fatalf("AddActivity blocked failed: %v", err)
	}

	gotDel, ok = s.GetDelegation(del.ID)
	if !ok {
		t.Fatal("delegation not found after blocked")
	}
	if !gotDel.PendingAction {
		t.Error("PendingAction should remain true after blocked activity")
	}

	// Verify we have 3 activities total
	activities, err := s.GetActivities(del.ID)
	if err != nil {
		t.Fatalf("GetActivities failed: %v", err)
	}
	if len(activities) != 3 {
		t.Errorf("expected 3 activities, got %d", len(activities))
	}
}

func TestStore_GetActivities(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	session := s.CreateSession("test-project", "Activity test")
	del, err := s.CreateDelegation(session.ID, "dev", "Implement feature", nil)
	if err != nil {
		t.Fatalf("CreateDelegation failed: %v", err)
	}

	// Empty list for delegation with no activities
	activities, err := s.GetActivities(del.ID)
	if err != nil {
		t.Fatalf("GetActivities failed: %v", err)
	}
	if len(activities) != 0 {
		t.Errorf("expected 0 activities, got %d", len(activities))
	}

	// Add activities in chronological order
	a1 := &ActivityEvent{Type: ActivityProgress, Content: "First update"}
	_ = s.AddActivity(del.ID, a1)
	a2 := &ActivityEvent{Type: ActivityDecision, Content: "Architecture decision", Options: []string{"yes", "no"}}
	_ = s.AddActivity(del.ID, a2)
	a3 := &ActivityEvent{Type: ActivityQuestion, Content: "What database?", Options: []string{"pg", "mysql"}}
	_ = s.AddActivity(del.ID, a3)

	activities, err = s.GetActivities(del.ID)
	if err != nil {
		t.Fatalf("GetActivities failed: %v", err)
	}
	if len(activities) != 3 {
		t.Fatalf("expected 3 activities, got %d", len(activities))
	}

	// Verify chronological order (ascending by CreatedAt)
	if activities[0].Content != "First update" {
		t.Errorf("first activity should be 'First update', got '%s'", activities[0].Content)
	}
	if activities[1].Content != "Architecture decision" {
		t.Errorf("second activity should be 'Architecture decision', got '%s'", activities[1].Content)
	}
	if activities[2].Content != "What database?" {
		t.Errorf("third activity should be 'What database?', got '%s'", activities[2].Content)
	}

	// Verify chronological order is maintained
	if activities[0].CreatedAt.After(activities[1].CreatedAt) {
		t.Error("activities should be in chronological order: 0 before 1")
	}
	if activities[1].CreatedAt.After(activities[2].CreatedAt) {
		t.Error("activities should be in chronological order: 1 before 2")
	}

	// Non-existent delegation returns empty array (no error)
	activities, err = s.GetActivities("nonexistent")
	if err != nil {
		t.Errorf("GetActivities for non-existent delegation should not error, got: %v", err)
	}
	if len(activities) != 0 {
		t.Errorf("expected empty array for non-existent delegation, got %d activities", len(activities))
	}
}

func TestStore_ResolveActivity(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	session := s.CreateSession("test-project", "Activity test")
	del, err := s.CreateDelegation(session.ID, "dev", "Implement feature", nil)
	if err != nil {
		t.Fatalf("CreateDelegation failed: %v", err)
	}

	// Add a decision activity
	decision := &ActivityEvent{
		Type:    ActivityDecision,
		Content: "Approve design?",
		Options: []string{"approve", "reject"},
	}
	if err := s.AddActivity(del.ID, decision); err != nil {
		t.Fatalf("AddActivity failed: %v", err)
	}

	// Verify PendingAction is true
	gotDel, _ := s.GetDelegation(del.ID)
	if !gotDel.PendingAction {
		t.Error("PendingAction should be true before resolve")
	}

	// Resolve the activity
	resolved, err := s.ResolveActivity(del.ID, decision.ID, "approve")
	if err != nil {
		t.Fatalf("ResolveActivity failed: %v", err)
	}
	if resolved == nil {
		t.Fatal("expected non-nil activity on successful resolve")
	}
	if resolved.Resolution != "approve" {
		t.Errorf("expected resolution 'approve', got '%s'", resolved.Resolution)
	}
	if resolved.ResolvedAt == nil {
		t.Error("ResolvedAt should be set after resolve")
	}

	// Verify the activity is resolved
	activities, err := s.GetActivities(del.ID)
	if err != nil {
		t.Fatalf("GetActivities failed: %v", err)
	}
	if len(activities) == 0 {
		t.Fatal("expected at least one activity")
	}
	resolved2 := activities[0]
	if resolved2.Resolution != "approve" {
		t.Errorf("expected resolution 'approve', got '%s'", resolved2.Resolution)
	}
	if resolved2.ResolvedAt == nil {
		t.Error("ResolvedAt should be set after resolve")
	}

	// Verify PendingAction is now false since no more pending
	gotDel, _ = s.GetDelegation(del.ID)
	if gotDel.PendingAction {
		t.Error("PendingAction should be false after resolve")
	}

	// Test resolving non-existent delegation
	if _, err := s.ResolveActivity("nonexistent", decision.ID, "ok"); err == nil {
		t.Error("expected error when resolving activity in non-existent delegation")
	}

	// Test resolving non-existent activity
	if _, err := s.ResolveActivity(del.ID, "nonexistent", "ok"); err == nil {
		t.Error("expected error when resolving non-existent activity")
	}

	// Test re-resolving an already-resolved activity — should fail now.
	if _, err := s.ResolveActivity(del.ID, decision.ID, "again"); err == nil {
		t.Errorf("expected error on re-resolve, got nil")
	}
	// Verify the original resolution was NOT overwritten
	activities, _ = s.GetActivities(del.ID)
	if activities[0].Resolution != "approve" {
		t.Errorf("expected resolution 'approve', got '%s'", activities[0].Resolution)
	}
}

func TestStore_GetPendingDecisions(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	session := s.CreateSession("test-project", "Pending decisions test")
	del, err := s.CreateDelegation(session.ID, "dev", "Implement feature", nil)
	if err != nil {
		t.Fatalf("CreateDelegation failed: %v", err)
	}

	// Initially no pending decisions
	pending, err := s.GetPendingDecisions(session.ID)
	if err != nil {
		t.Fatalf("GetPendingDecisions failed: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending decisions, got %d", len(pending))
	}

	// Add a progress activity — should NOT appear in pending
	progress := &ActivityEvent{Type: ActivityProgress, Content: "Working..."}
	s.AddActivity(del.ID, progress)

	pending, _ = s.GetPendingDecisions(session.ID)
	if len(pending) != 0 {
		t.Error("progress type should not appear in pending decisions")
	}

	// Add a decision activity — SHOULD appear
	decision := &ActivityEvent{Type: ActivityDecision, Content: "Go/no-go?", Options: []string{"go", "no-go"}}
	s.AddActivity(del.ID, decision)

	pending, _ = s.GetPendingDecisions(session.ID)
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending decision, got %d", len(pending))
	}
	if pending[0].Type != ActivityDecision {
		t.Errorf("expected decision type, got %s", pending[0].Type)
	}

	// Add a question — SHOULD appear
	question := &ActivityEvent{Type: ActivityQuestion, Content: "Which library?", Options: []string{"A", "B"}}
	s.AddActivity(del.ID, question)

	pending, _ = s.GetPendingDecisions(session.ID)
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending, got %d", len(pending))
	}

	// Add a blocked — SHOULD appear
	blocked := &ActivityEvent{Type: ActivityBlocked, Content: "Need credentials"}
	s.AddActivity(del.ID, blocked)

	pending, _ = s.GetPendingDecisions(session.ID)
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending, got %d", len(pending))
	}

	// Resolve the decision
	_, _ = s.ResolveActivity(del.ID, decision.ID, "go")
	pending, _ = s.GetPendingDecisions(session.ID)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending after resolving one, got %d", len(pending))
	}

	// Resolve the question
	_, _ = s.ResolveActivity(del.ID, question.ID, "A")
	pending, _ = s.GetPendingDecisions(session.ID)
	if len(pending) != 1 {
		t.Errorf("expected 1 pending after resolving two, got %d", len(pending))
	}

	// Verify scoped to session — create another session with its own pending
	otherSession := s.CreateSession("other-project", "Other session")
	otherDel, _ := s.CreateDelegation(otherSession.ID, "qa", "Test", nil)
	otherDecision := &ActivityEvent{Type: ActivityDecision, Content: "Other decision"}
	s.AddActivity(otherDel.ID, otherDecision)

	// First session should still have 1 pending
	pending, _ = s.GetPendingDecisions(session.ID)
	if len(pending) != 1 {
		t.Errorf("session scope: expected 1 pending for first session, got %d", len(pending))
	}

	// Other session should have its own pending
	pending, _ = s.GetPendingDecisions(otherSession.ID)
	if len(pending) != 1 {
		t.Errorf("session scope: expected 1 pending for other session, got %d", len(pending))
	}
}
