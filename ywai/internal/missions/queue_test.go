package missions

import (
	"os"
	"testing"
	"time"
)

// ─── Test Helpers ──────────────────────────────────────────────────────────

func testQueueMission(id string) *Mission {
	now := time.Now().UTC().Truncate(time.Second)
	return &Mission{
		ID:        id,
		Name:      "Test Mission " + id,
		Status:    MissionActive,
		CreatedAt: now,
		UpdatedAt: now,
		Milestones: []Milestone{
			{Name: "milestone-1", Description: "First milestone"},
			{Name: "milestone-2", Description: "Second milestone"},
		},
		Features: []Feature{
			{
				ID:          "feat-1",
				Description: "Feature 1",
				Status:      FeaturePending,
				Milestone:   "milestone-1",
				SkillName:   "backend-worker",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "feat-2",
				Description: "Feature 2",
				Status:      FeaturePending,
				Milestone:   "milestone-1",
				SkillName:   "backend-worker",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "feat-3",
				Description: "Feature 3",
				Status:      FeaturePending,
				Milestone:   "milestone-2",
				SkillName:   "backend-worker",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}
}

// ─── NextPendingFeature ────────────────────────────────────────────────────

// VAL-ENG-QUEUE-001: Sequential execution order
func TestNextPendingFeatureReturnsFirstPending(t *testing.T) {
	m := testQueueMission("next-pending-1")
	f := NextPendingFeature(m)
	if f == nil {
		t.Fatal("expected a pending feature, got nil")
	}
	if f.ID != "feat-1" {
		t.Fatalf("expected feat-1 (first in array), got %s", f.ID)
	}
}

func TestNextPendingFeatureSkipsCompleted(t *testing.T) {
	m := testQueueMission("next-pending-2")
	m.Features[0].Status = FeatureCompleted
	m.Features[1].Status = FeatureInProgress
	m.Features[2].Status = FeaturePending
	f := NextPendingFeature(m)
	if f == nil {
		t.Fatal("expected feat-3 (next pending after completed/in_progress), got nil")
	}
	if f.ID != "feat-3" {
		t.Fatalf("expected feat-3 (next pending), got %s", f.ID)
	}
}

// VAL-ENG-QUEUE-008: Empty feature queue returns error
func TestNextPendingFeatureNoPendingFeatures(t *testing.T) {
	m := testQueueMission("next-pending-3")
	for i := range m.Features {
		m.Features[i].Status = FeatureCompleted
	}
	f := NextPendingFeature(m)
	if f != nil {
		t.Fatalf("expected nil for no pending features, got %s", f.ID)
	}
}

func TestNextPendingFeatureEmptyMission(t *testing.T) {
	m := testQueueMission("next-pending-4")
	m.Features = nil
	f := NextPendingFeature(m)
	if f != nil {
		t.Fatalf("expected nil for empty mission, got %s", f.ID)
	}
}

// VAL-ENG-QUEUE-001: Features execute in sequential order
func TestNextPendingFeatureSequentialOrder(t *testing.T) {
	m := testQueueMission("next-pending-5")
	// Mark first feature as completed, should return second
	m.Features[0].Status = FeatureCompleted
	f := NextPendingFeature(m)
	if f == nil {
		t.Fatal("expected a pending feature")
	}
	if f.ID != "feat-2" {
		t.Fatalf("expected feat-2 (second in array), got %s", f.ID)
	}
}

func TestNextPendingFeatureAfterFailed(t *testing.T) {
	m := testQueueMission("next-pending-6")
	// feat-1 is failed, feat-2 is completed, feat-3 is still pending
	m.Features[0].Status = FeatureFailed
	m.Features[1].Status = FeatureCompleted
	m.Features[2].Status = FeaturePending
	f := NextPendingFeature(m)
	if f == nil {
		t.Fatal("expected feat-3 (next pending after failed/completed), got nil")
	}
	if f.ID != "feat-3" {
		t.Fatalf("expected feat-3, got %s", f.ID)
	}
}

func TestNextPendingFeatureWithMixedStatuses(t *testing.T) {
	m := testQueueMission("next-pending-7")
	m.Features[0].Status = FeatureCancelled
	m.Features[1].Status = FeatureInProgress
	m.Features[2].Status = FeaturePending // still pending

	// in_progress is the current running one, feat-2 is still pending
	f := NextPendingFeature(m)
	if f == nil {
		t.Fatal("expected a pending feature")
	}
	if f.ID != "feat-3" {
		t.Fatalf("expected feat-3 (the pending one), got %s", f.ID)
	}
}

// ─── StartFeature ──────────────────────────────────────────────────────────

func TestStartFeature(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("start-feat-1")
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	feat, err := StartFeature(store, m, "feat-1")
	if err != nil {
		t.Fatalf("start feature: %v", err)
	}
	if feat.Status != FeatureInProgress {
		t.Fatalf("expected in_progress, got %s", feat.Status)
	}

	// Verify it's persisted
	loaded, err := store.LoadMission(m.ID)
	if err != nil {
		t.Fatalf("load mission: %v", err)
	}
	found := false
	for _, f := range loaded.Features {
		if f.ID == "feat-1" {
			if f.Status != FeatureInProgress {
				t.Fatalf("persisted status expected in_progress, got %s", f.Status)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("feature not found in persisted mission")
	}
}

func TestStartFeatureNotFound(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("start-feat-2")
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	_, err := StartFeature(store, m, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent feature")
	}
}

func TestStartFeatureOnCompleted(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("start-feat-3")
	m.Features[0].Status = FeatureCompleted
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	_, err := StartFeature(store, m, "feat-1")
	if err == nil {
		t.Fatal("expected error starting completed feature")
	}
}

// ─── CompleteFeature ───────────────────────────────────────────────────────

func TestCompleteFeature(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("complete-feat-1")
	m.Features[0].Status = FeatureInProgress
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	feat, err := CompleteFeature(store, m, "feat-1")
	if err != nil {
		t.Fatalf("complete feature: %v", err)
	}
	if feat.Status != FeatureCompleted {
		t.Fatalf("expected completed, got %s", feat.Status)
	}
	if feat.CompletedAt == nil {
		t.Fatal("expected CompletedAt to be set")
	}

	// Verify persistence
	loaded, err := store.LoadMission(m.ID)
	if err != nil {
		t.Fatalf("load mission: %v", err)
	}
	for _, f := range loaded.Features {
		if f.ID == "feat-1" && f.Status != FeatureCompleted {
			t.Fatalf("expected persisted completed, got %s", f.Status)
		}
	}
}

// ─── FailFeature ───────────────────────────────────────────────────────────

// VAL-ENG-QUEUE-003: Worker failure transitions to failed state
func TestFailFeature(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("fail-feat-1")
	m.Features[0].Status = FeatureInProgress
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	feat, err := FailFeature(store, m, "feat-1")
	if err != nil {
		t.Fatalf("fail feature: %v", err)
	}
	if feat.Status != FeatureFailed {
		t.Fatalf("expected failed, got %s", feat.Status)
	}
	// Retry count should be incremented
	if feat.RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", feat.RetryCount)
	}
}

// VAL-ENG-QUEUE-005: Failed features re-queued with retry count
func TestFailFeatureIncrementsRetryCount(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("fail-feat-2")
	m.Features[0].Status = FeatureInProgress
	m.Features[0].RetryCount = 2
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	feat, err := FailFeature(store, m, "feat-1")
	if err != nil {
		t.Fatalf("fail feature: %v", err)
	}
	if feat.RetryCount != 3 {
		t.Fatalf("expected retry count 3, got %d", feat.RetryCount)
	}
}

// ─── CancelFeature ─────────────────────────────────────────────────────────

// VAL-ENG-QUEUE-004: Cancellation transitions to cancelled
func TestCancelPendingFeature(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("cancel-feat-1")
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	feat, err := CancelFeature(store, m, "feat-1")
	if err != nil {
		t.Fatalf("cancel feature: %v", err)
	}
	if feat.Status != FeatureCancelled {
		t.Fatalf("expected cancelled, got %s", feat.Status)
	}
}

func TestCancelInProgressFeature(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("cancel-feat-2")
	m.Features[0].Status = FeatureInProgress
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	feat, err := CancelFeature(store, m, "feat-1")
	if err != nil {
		t.Fatalf("cancel feature: %v", err)
	}
	if feat.Status != FeatureCancelled {
		t.Fatalf("expected cancelled, got %s", feat.Status)
	}
}

func TestCancelCompletedFeatureError(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("cancel-feat-3")
	m.Features[0].Status = FeatureCompleted
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	_, err := CancelFeature(store, m, "feat-1")
	if err == nil {
		t.Fatal("expected error cancelling completed feature")
	}
}

// ─── RequeueFeature ────────────────────────────────────────────────────────

// VAL-ENG-QUEUE-005: Failed features re-queued with retry count
func TestRequeueFeature(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("requeue-1")
	m.Features[0].Status = FeatureFailed
	m.Features[0].RetryCount = 1
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	feat, err := RequeueFeature(store, m, "feat-1")
	if err != nil {
		t.Fatalf("requeue feature: %v", err)
	}
	if feat.Status != FeaturePending {
		t.Fatalf("expected pending after requeue, got %s", feat.Status)
	}
	// Retry count should be preserved
	if feat.RetryCount != 1 {
		t.Fatalf("expected retry count 1, got %d", feat.RetryCount)
	}
}

func TestRequeueNonFailedFeatureError(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("requeue-2")
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	_, err := RequeueFeature(store, m, "feat-1")
	if err == nil {
		t.Fatal("expected error re-queueing non-failed feature")
	}
}

// ─── Milestone Completion Detection ────────────────────────────────────────

// VAL-ENG-QUEUE-006: Milestone completion auto-detected
func TestCheckMilestoneCompletion(t *testing.T) {
	m := testQueueMission("milestone-check-1")

	// Mark all milestone-1 features as completed
	m.Features[0].Status = FeatureCompleted
	m.Features[1].Status = FeatureCompleted

	completed, err := CheckMilestoneCompletion(m, "milestone-1")
	if err != nil {
		t.Fatalf("check milestone: %v", err)
	}
	if !completed {
		t.Fatal("expected milestone-1 to be completed")
	}
}

func TestCheckMilestoneNotComplete(t *testing.T) {
	m := testQueueMission("milestone-check-2")

	// Only one feature completed
	m.Features[0].Status = FeatureCompleted

	completed, err := CheckMilestoneCompletion(m, "milestone-1")
	if err != nil {
		t.Fatalf("check milestone: %v", err)
	}
	if completed {
		t.Fatal("expected milestone-1 to NOT be completed")
	}
}

func TestCheckMilestoneEmpty(t *testing.T) {
	m := testQueueMission("milestone-check-3")
	m.Features = nil

	completed, err := CheckMilestoneCompletion(m, "milestone-1")
	if err != nil {
		t.Fatalf("check milestone: %v", err)
	}
	if !completed {
		t.Fatal("expected empty milestone to be completed (no features = done)")
	}
}

func TestCheckMilestoneWithFailedAndCancelled(t *testing.T) {
	m := testQueueMission("milestone-check-4")

	m.Features[0].Status = FeatureFailed
	m.Features[1].Status = FeatureCancelled

	completed, err := CheckMilestoneCompletion(m, "milestone-1")
	if err != nil {
		t.Fatalf("check milestone: %v", err)
	}
	if completed {
		t.Fatal("expected milestone-1 to NOT be completed (failed/cancelled != completed)")
	}
}

// ─── All Milestones Complete ───────────────────────────────────────────────

// VAL-ENG-FSM-003: active→completed on all milestones sealed
func TestAllMilestonesComplete(t *testing.T) {
	m := testQueueMission("all-milestones-1")

	// Complete all features
	for i := range m.Features {
		m.Features[i].Status = FeatureCompleted
	}

	allDone, err := AllMilestonesComplete(m)
	if err != nil {
		t.Fatalf("check all milestones: %v", err)
	}
	if !allDone {
		t.Fatal("expected all milestones to be complete")
	}
}

func TestAllMilestonesNotComplete(t *testing.T) {
	m := testQueueMission("all-milestones-2")

	// Only milestone-1 features complete
	m.Features[0].Status = FeatureCompleted
	m.Features[1].Status = FeatureCompleted
	// feat-3 (milestone-2) is still pending

	allDone, err := AllMilestonesComplete(m)
	if err != nil {
		t.Fatalf("check all milestones: %v", err)
	}
	if allDone {
		t.Fatal("expected NOT all milestones complete")
	}
}

func TestAllMilestonesCompleteWithNoMilestones(t *testing.T) {
	m := testQueueMission("all-milestones-3")
	m.Milestones = nil
	m.Features = nil

	allDone, err := AllMilestonesComplete(m)
	if err != nil {
		t.Fatalf("check all milestones: %v", err)
	}
	if !allDone {
		t.Fatal("expected all milestones to be complete when none defined")
	}
}

// ─── Milestone Features Status ─────────────────────────────────────────────

func TestGetMilestoneFeatures(t *testing.T) {
	m := testQueueMission("milestone-feat-1")

	feats := GetMilestoneFeatures(m, "milestone-1")
	if len(feats) != 2 {
		t.Fatalf("expected 2 features in milestone-1, got %d", len(feats))
	}
	if feats[0].ID != "feat-1" || feats[1].ID != "feat-2" {
		t.Fatal("unexpected features returned")
	}
}

func TestGetMilestoneFeaturesNonexistent(t *testing.T) {
	m := testQueueMission("milestone-feat-2")

	feats := GetMilestoneFeatures(m, "nonexistent")
	if len(feats) != 0 {
		t.Fatalf("expected 0 features for nonexistent milestone, got %d", len(feats))
	}
}

// ─── Milestone Status Summary ──────────────────────────────────────────────

func TestGetMilestoneStatusCompleted(t *testing.T) {
	m := testQueueMission("milestone-status-1")
	m.Features[0].Status = FeatureCompleted
	m.Features[1].Status = FeatureCompleted

	summary := GetMilestoneStatus(m, "milestone-1")
	if summary.Completed != 2 {
		t.Fatalf("expected 2 completed, got %d", summary.Completed)
	}
	if summary.Total != 2 {
		t.Fatalf("expected 2 total, got %d", summary.Total)
	}
	if !summary.AllDone {
		t.Fatal("expected AllDone to be true")
	}
}

func TestGetMilestoneStatusPartial(t *testing.T) {
	m := testQueueMission("milestone-status-2")
	m.Features[0].Status = FeatureCompleted
	// feat-2 is still pending

	summary := GetMilestoneStatus(m, "milestone-1")
	if summary.Completed != 1 {
		t.Fatalf("expected 1 completed, got %d", summary.Completed)
	}
	if summary.Total != 2 {
		t.Fatalf("expected 2 total, got %d", summary.Total)
	}
	if summary.AllDone {
		t.Fatal("expected AllDone to be false")
	}
}

func TestGetMilestoneStatusWithFailed(t *testing.T) {
	m := testQueueMission("milestone-status-3")
	m.Features[0].Status = FeatureFailed
	m.Features[1].Status = FeatureCompleted

	summary := GetMilestoneStatus(m, "milestone-1")
	if summary.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", summary.Failed)
	}
	if summary.Completed != 1 {
		t.Fatalf("expected 1 completed, got %d", summary.Completed)
	}
}

// ─── ProcessMilestoneAfterFeature ──────────────────────────────────────────

func TestProcessMilestoneAfterFeatureNoTransition(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("process-milestone-1")
	m.Features[0].Status = FeatureCompleted
	m.Features[1].Status = FeatureInProgress
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	transition, err := ProcessMilestoneAfterFeature(store, m, "feat-1")
	if err != nil {
		t.Fatalf("process milestone: %v", err)
	}
	if transition != "" {
		t.Fatalf("expected no transition (milestone not complete), got %s", transition)
	}
}

// VAL-ENG-QUEUE-006 + VAL-ENG-QUEUE-007: Milestone completion + validation injection
func TestProcessMilestoneAfterFeatureTransitionsToValidating(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("process-milestone-2")
	m.Features[0].Status = FeatureCompleted
	m.Features[1].Status = FeatureCompleted // last feature in milestone-1
	m.Features[2].Status = FeaturePending   // milestone-2 still pending
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	// Complete feat-2 (last in milestone-1)
	transition, err := ProcessMilestoneAfterFeature(store, m, "feat-2")
	if err != nil {
		t.Fatalf("process milestone: %v", err)
	}
	if transition != string(MissionValidating) {
		t.Fatalf("expected transition to validating, got %s", transition)
	}
	if m.Status != MissionValidating {
		t.Fatalf("expected mission status to be validating, got %s", m.Status)
	}
}

// VAL-CROSS-E2E-03: Milestone auto-transitions to validating
func TestProcessMilestoneLastFeatureTransitionsToValidating(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("process-milestone-3")
	// All features completed except last one
	for i := range m.Features {
		m.Features[i].Status = FeatureCompleted
	}
	m.Features[2].Status = FeatureInProgress // feat-3 in progress
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	// Complete feat-3 (last feature in last milestone) first
	_, err := CompleteFeature(store, m, "feat-3")
	if err != nil {
		t.Fatalf("complete feature: %v", err)
	}

	transition, err := ProcessMilestoneAfterFeature(store, m, "feat-3")
	if err != nil {
		t.Fatalf("process milestone: %v", err)
	}
	if transition != string(MissionValidating) {
		t.Fatalf("expected transition to validating, got %s", transition)
	}
}

func TestProcessMilestoneStorePersistence(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("process-milestone-4")
	m.Features[0].Status = FeatureCompleted
	m.Features[1].Status = FeatureCompleted
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	_, err := ProcessMilestoneAfterFeature(store, m, "feat-2")
	if err != nil {
		t.Fatalf("process milestone: %v", err)
	}

	// Verify persistence
	loaded, err := store.LoadMission(m.ID)
	if err != nil {
		t.Fatalf("load mission: %v", err)
	}
	if loaded.Status != MissionValidating {
		t.Fatalf("expected persisted status to be validating, got %s", loaded.Status)
	}
}

// ─── CancelMission Tests ───────────────────────────────────────────────────

func TestCancelMissionActive(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("cancel-active")
	m.Status = MissionActive
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	if err := CancelMission(store, m); err != nil {
		t.Fatalf("CancelMission: %v", err)
	}

	if m.Status != MissionCancelled {
		t.Errorf("expected cancelled, got %q", m.Status)
	}

	// Verify persistence
	loaded, err := store.LoadMission(m.ID)
	if err != nil {
		t.Fatalf("load mission: %v", err)
	}
	if loaded.Status != MissionCancelled {
		t.Errorf("persisted status should be cancelled, got %q", loaded.Status)
	}
}

func TestCancelMissionAlreadyCancelled(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("cancel-already")
	m.Status = MissionCancelled
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	if err := CancelMission(store, m); err != nil {
		t.Fatalf("cancel cancelled should succeed: %v", err)
	}
	if m.Status != MissionCancelled {
		t.Errorf("expected cancelled, got %q", m.Status)
	}
}

func TestCancelMissionCompletedError(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("cancel-completed")
	m.Status = MissionCompleted
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	if err := CancelMission(store, m); err == nil {
		t.Fatal("expected error cancelling completed mission")
	}
}

func TestCancelMissionCancelsPendingFeatures(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("cancel-features")
	m.Status = MissionActive
	m.Features[0].Status = FeaturePending
	m.Features[1].Status = FeatureInProgress
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	if err := CancelMission(store, m); err != nil {
		t.Fatalf("CancelMission: %v", err)
	}

	if m.Features[0].Status != FeatureCancelled {
		t.Errorf("pending feature should be cancelled, got %q", m.Features[0].Status)
	}
	if m.Features[1].Status != FeatureCancelled {
		t.Errorf("in_progress feature should be cancelled, got %q", m.Features[1].Status)
	}
}

// ─── ResumeMission Tests ───────────────────────────────────────────────────

func TestResumeMissionPaused(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("resume-paused")
	m.Status = MissionPaused
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	if err := ResumeMission(store, m); err != nil {
		t.Fatalf("ResumeMission: %v", err)
	}

	if m.Status != MissionActive {
		t.Errorf("expected active, got %q", m.Status)
	}

	// Verify persistence
	loaded, err := store.LoadMission(m.ID)
	if err != nil {
		t.Fatalf("load mission: %v", err)
	}
	if loaded.Status != MissionActive {
		t.Errorf("persisted status should be active, got %q", loaded.Status)
	}
}

func TestResumeMissionActiveError(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("resume-active")
	m.Status = MissionActive
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	if err := ResumeMission(store, m); err == nil {
		t.Fatal("expected error resuming active mission")
	}
}

func TestResumeMissionCompletedError(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("resume-completed")
	m.Status = MissionCompleted
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	if err := ResumeMission(store, m); err == nil {
		t.Fatal("expected error resuming completed mission")
	}
}

func TestResumeMissionPreservesCompletedFeatures(t *testing.T) {
	store, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testQueueMission("resume-preserve")
	m.Status = MissionPaused
	m.Features[0].Status = FeatureCompleted
	m.Features[1].Status = FeaturePending
	if err := store.CreateMission(m); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	if err := ResumeMission(store, m); err != nil {
		t.Fatalf("ResumeMission: %v", err)
	}

	if m.Features[0].Status != FeatureCompleted {
		t.Errorf("completed feature should remain completed, got %q", m.Features[0].Status)
	}
	if m.Features[1].Status != FeaturePending {
		t.Errorf("pending feature should remain pending, got %q", m.Features[1].Status)
	}
}
