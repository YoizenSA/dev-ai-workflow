package control

import (
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/kanban"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
)

// --- helpers -----------------------------------------------------------

func newTestStores(t *testing.T) (*kanban.Store, *missions.MissionsStore) {
	t.Helper()
	dir := t.TempDir()

	ks := kanban.NewStore(filepath.Join(dir, "kanban.json"))
	ms := missions.NewMissionsStore(dir)

	return ks, ms
}

func seedMission(t *testing.T, ms *missions.MissionsStore, id string) {
	t.Helper()
	m := &missions.Mission{
		ID:      id,
		Name:    "Build auth with JWT",
		Project: "demo",
		Status:  missions.MissionActive,
		Features: []missions.Feature{
			{ID: "feat-1", Description: "Implement /login endpoint", SkillName: "api", Status: missions.FeaturePending},
			{ID: "feat-2", Description: "Write auth tests", SkillName: "tests", Status: missions.FeaturePending, Preconditions: []string{"feat-1"}},
		},
	}
	if err := ms.SaveMission(m); err != nil {
		t.Fatalf("save mission: %v", err)
	}
}

// --- Tests -------------------------------------------------------------

func TestKanbanProjector_FirstEventCreatesSession(t *testing.T) {
	ks, ms := newTestStores(t)
	seedMission(t, ms, "m1")

	p := NewKanbanProjector(ks, ms)

	// First event for unknown mission → should create session + delegations
	p.Project("feature_status_changed", map[string]interface{}{
		"missionId": "m1",
		"featureId": "feat-1",
		"status":    missions.FeatureInProgress,
	})

	p.mu.Lock()
	sid, ok := p.sessions["m1"]
	p.mu.Unlock()

	if !ok {
		t.Fatal("expected session to be created for mission m1")
	}

	sess, found := ks.GetSession(sid)
	if !found || sess == nil {
		t.Fatalf("session %s not found in kanban store", sid)
	}
	if sess.Project != "demo" {
		t.Errorf("expected project 'demo', got %q", sess.Project)
	}

	delegs := ks.ListDelegations(sid)
	if len(delegs) != 2 {
		t.Fatalf("expected 2 delegations, got %d", len(delegs))
	}
}

func TestKanbanProjector_StatusUpdateMovesCard(t *testing.T) {
	ks, ms := newTestStores(t)
	seedMission(t, ms, "m1")

	p := NewKanbanProjector(ks, ms)

	// Create session
	p.Project("feature_status_changed", map[string]interface{}{
		"missionId": "m1",
		"featureId": "feat-1",
		"status":    missions.FeatureInProgress,
	})

	p.mu.Lock()
	delegID := p.delegs["m1"]["feat-1"]
	p.mu.Unlock()

	if delegID == "" {
		t.Fatal("expected delegation for feat-1")
	}

	// Now complete the feature
	p.Project("feature_status_changed", map[string]interface{}{
		"missionId": "m1",
		"featureId": "feat-1",
		"status":    missions.FeatureCompleted,
	})

	deleg, found := ks.GetDelegation(delegID)
	if !found || deleg == nil {
		t.Fatalf("delegation %s not found", delegID)
	}
	if deleg.Status != "completed" {
		t.Errorf("expected status 'completed', got %q", deleg.Status)
	}
}

func TestKanbanProjector_InProgressMapsCorrectly(t *testing.T) {
	ks, ms := newTestStores(t)
	seedMission(t, ms, "m1")

	p := NewKanbanProjector(ks, ms)

	// Send in_progress event
	p.Project("feature_status_changed", map[string]interface{}{
		"missionId": "m1",
		"featureId": "feat-1",
		"status":    missions.FeatureInProgress,
	})

	p.mu.Lock()
	delegID := p.delegs["m1"]["feat-1"]
	p.mu.Unlock()

	deleg, found := ks.GetDelegation(delegID)
	if !found || deleg == nil {
		t.Fatalf("delegation not found")
	}

	// The critical check: in_progress → MissionActive → MapFSMToKanbanColumn → "in_progress"
	// NOT "backlog" (which was the bug before the fix)
	col := deleg.DerivedColumn()
	if col != "in_progress" {
		t.Errorf("BUG: in_progress mapped to %q instead of 'in_progress'", col)
	}
}

func TestKanbanProjector_IdempotentDuplicateEvent(t *testing.T) {
	ks, ms := newTestStores(t)
	seedMission(t, ms, "m1")

	p := NewKanbanProjector(ks, ms)

	// First event
	p.Project("feature_status_changed", map[string]interface{}{
		"missionId": "m1",
		"featureId": "feat-1",
		"status":    missions.FeatureInProgress,
	})

	// Duplicate event (same mission, same feature)
	p.Project("feature_status_changed", map[string]interface{}{
		"missionId": "m1",
		"featureId": "feat-1",
		"status":    missions.FeatureInProgress,
	})

	// Should still have exactly 1 session and 2 delegations (not 4)
	p.mu.Lock()
	sid := p.sessions["m1"]
	p.mu.Unlock()

	delegs := ks.ListDelegations(sid)
	if len(delegs) != 2 {
		t.Errorf("expected 2 delegations (idempotent), got %d", len(delegs))
	}
}

func TestKanbanProjector_IgnoresNonFeatureEvents(t *testing.T) {
	ks, ms := newTestStores(t)
	p := NewKanbanProjector(ks, ms)

	p.Project("mission_started", map[string]interface{}{
		"missionId": "m1",
	})

	p.mu.Lock()
	_, ok := p.sessions["m1"]
	p.mu.Unlock()

	if ok {
		t.Error("expected no session for non-feature event")
	}
}

func TestKanbanProjector_FeatureStatusTypeAssertion(t *testing.T) {
	ks, ms := newTestStores(t)
	seedMission(t, ms, "m1")

	p := NewKanbanProjector(ks, ms)

	// Simulate the real engine payload where status is FeatureStatus (named type),
	// NOT a plain string. This is the exact bug that was caught in code review:
	// .(string) assertion fails because the dynamic type is FeatureStatus.
	p.Project("feature_status_changed", map[string]interface{}{
		"missionId": "m1",
		"featureId": "feat-1",
		"status":    missions.FeatureInProgress, // FeatureStatus type, not string
	})

	p.mu.Lock()
	delegID := p.delegs["m1"]["feat-1"]
	p.mu.Unlock()

	deleg, found := ks.GetDelegation(delegID)
	if !found || deleg == nil {
		t.Fatal("delegation not found — type assertion failed for FeatureStatus")
	}

	// Must be "active" (MissionActive), not empty string
	if deleg.Status == "" {
		t.Error("BUG: FeatureStatus type assertion failed, status is empty")
	}
	if deleg.Status != "active" {
		t.Errorf("expected status 'active' (MissionActive), got %q", deleg.Status)
	}
}

func TestMapFeatureStatusToMissionStatus(t *testing.T) {
	tests := []struct {
		feature  missions.FeatureStatus
		expected missions.MissionStatus
	}{
		{missions.FeaturePending, missions.MissionPending},
		{missions.FeatureInProgress, missions.MissionActive},
		{missions.FeatureCompleted, missions.MissionCompleted},
		{missions.FeatureFailed, missions.MissionFailed},
		{missions.FeatureCancelled, missions.MissionCancelled},
		{"unknown", missions.MissionPending},
	}

	for _, tt := range tests {
		got := mapFeatureStatusToMissionStatus(string(tt.feature))
		if got != tt.expected {
			t.Errorf("mapFeatureStatusToMissionStatus(%q) = %q, want %q", tt.feature, got, tt.expected)
		}
	}
}

func TestNormalizeAgent(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"api", "dev"},
		{"api-worker", "dev"},   // DesignWorkerSystem appends "-worker"
		{"tests-worker", "qa"},  // same suffix, different category
		{"tests", "qa"},
		{"ui", "dev"},
		{"devops", "devops"},
		{"dev", "dev"},
		{"qa", "qa"},
		{"", "dev"},
		{"unknown-skill", "dev"},
	}

	for _, tt := range tests {
		got := normalizeAgent(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeAgent(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
