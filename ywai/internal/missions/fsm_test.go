package missions

import (
	"testing"
)

// ─── Mission State Transitions ─────────────────────────────────────────────

func TestMissionTransitionPlanningToActive(t *testing.T) {
	newStatus, err := TransitionMissionStatus(MissionPlanning, MissionActive)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if newStatus != MissionActive {
		t.Fatalf("expected %s, got %s", MissionActive, newStatus)
	}
}

func TestMissionTransitionActiveToPaused(t *testing.T) {
	newStatus, err := TransitionMissionStatus(MissionActive, MissionPaused)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if newStatus != MissionPaused {
		t.Fatalf("expected %s, got %s", MissionPaused, newStatus)
	}
}

func TestMissionTransitionPausedToActive(t *testing.T) {
	newStatus, err := TransitionMissionStatus(MissionPaused, MissionActive)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if newStatus != MissionActive {
		t.Fatalf("expected %s, got %s", MissionActive, newStatus)
	}
}

func TestMissionTransitionActiveToCompleted(t *testing.T) {
	newStatus, err := TransitionMissionStatus(MissionActive, MissionCompleted)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if newStatus != MissionCompleted {
		t.Fatalf("expected %s, got %s", MissionCompleted, newStatus)
	}
}

func TestMissionTransitionActiveToFailed(t *testing.T) {
	newStatus, err := TransitionMissionStatus(MissionActive, MissionFailed)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if newStatus != MissionFailed {
		t.Fatalf("expected %s, got %s", MissionFailed, newStatus)
	}
}

// VAL-ENG-FSM-009: Reset from failed
func TestMissionTransitionFailedToPlanning(t *testing.T) {
	newStatus, err := TransitionMissionStatus(MissionFailed, MissionPlanning)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if newStatus != MissionPlanning {
		t.Fatalf("expected %s, got %s", MissionPlanning, newStatus)
	}
}

// VAL-ENG-FSM-005: Invalid transition rejected (planning→completed)
func TestMissionTransitionInvalidPlanningToCompleted(t *testing.T) {
	_, err := TransitionMissionStatus(MissionPlanning, MissionCompleted)
	if err == nil {
		t.Fatal("expected error for invalid transition planning→completed")
	}
}

// VAL-ENG-FSM-006: Invalid transition rejected (paused→completed)
func TestMissionTransitionInvalidPausedToCompleted(t *testing.T) {
	_, err := TransitionMissionStatus(MissionPaused, MissionCompleted)
	if err == nil {
		t.Fatal("expected error for invalid transition paused→completed")
	}
}

func TestMissionTransitionInvalidPlanningToFailed(t *testing.T) {
	_, err := TransitionMissionStatus(MissionPlanning, MissionFailed)
	if err == nil {
		t.Fatal("expected error for invalid transition planning→failed")
	}
}

func TestMissionTransitionInvalidPausedToFailed(t *testing.T) {
	_, err := TransitionMissionStatus(MissionPaused, MissionFailed)
	if err == nil {
		t.Fatal("expected error for invalid transition paused→failed")
	}
}

func TestMissionTransitionInvalidCompletedToActive(t *testing.T) {
	_, err := TransitionMissionStatus(MissionCompleted, MissionActive)
	if err == nil {
		t.Fatal("expected error for invalid transition completed→active")
	}
}

func TestMissionTransitionInvalidCancelledToActive(t *testing.T) {
	_, err := TransitionMissionStatus(MissionCancelled, MissionActive)
	if err == nil {
		t.Fatal("expected error for invalid transition cancelled→active")
	}
}

func TestMissionTransitionInvalidFailedToCompleted(t *testing.T) {
	_, err := TransitionMissionStatus(MissionFailed, MissionCompleted)
	if err == nil {
		t.Fatal("expected error for invalid transition failed→completed")
	}
}

// VAL-ENG-FSM-007: Repeat pause idempotent (pausing paused is no-op)
func TestMissionTransitionPauseIdempotent(t *testing.T) {
	newStatus, err := TransitionMissionStatus(MissionPaused, MissionPaused)
	if err != nil {
		t.Fatalf("expected no error for pause→pause (idempotent), got: %v", err)
	}
	if newStatus != MissionPaused {
		t.Fatalf("expected %s, got %s", MissionPaused, newStatus)
	}
}

// VAL-ENG-FSM-008: Repeat resume idempotent (resuming active is no-op)
func TestMissionTransitionResumeIdempotent(t *testing.T) {
	newStatus, err := TransitionMissionStatus(MissionActive, MissionActive)
	if err != nil {
		t.Fatalf("expected no error for active→active (idempotent), got: %v", err)
	}
	if newStatus != MissionActive {
		t.Fatalf("expected %s, got %s", MissionActive, newStatus)
	}
}

// VAL-ENG-SYS-003: Rapid pause/resume stability
func TestRapidPauseResumeCycle(t *testing.T) {
	status := MissionStatus(MissionActive)
	var err error

	// Rapid cycle: active→paused→active→paused→active
	for i := 0; i < 10; i++ {
		status, err = TransitionMissionStatus(status, MissionPaused)
		if err != nil {
			t.Fatalf("cycle %d: pause failed: %v", i, err)
		}
		status, err = TransitionMissionStatus(status, MissionActive)
		if err != nil {
			t.Fatalf("cycle %d: resume failed: %v", i, err)
		}
	}
	if status != MissionActive {
		t.Fatalf("expected active after cycles, got %s", status)
	}
}

// ─── Feature State Transitions ─────────────────────────────────────────────

// VAL-ENG-QUEUE-002: pending→in_progress→completed
func TestFeatureTransitionPendingToInProgress(t *testing.T) {
	newStatus, err := TransitionFeatureStatus(FeaturePending, FeatureInProgress)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if newStatus != FeatureInProgress {
		t.Fatalf("expected %s, got %s", FeatureInProgress, newStatus)
	}
}

func TestFeatureTransitionInProgressToCompleted(t *testing.T) {
	newStatus, err := TransitionFeatureStatus(FeatureInProgress, FeatureCompleted)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if newStatus != FeatureCompleted {
		t.Fatalf("expected %s, got %s", FeatureCompleted, newStatus)
	}
}

// VAL-ENG-QUEUE-003: pending→in_progress→failed
func TestFeatureTransitionInProgressToFailed(t *testing.T) {
	newStatus, err := TransitionFeatureStatus(FeatureInProgress, FeatureFailed)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if newStatus != FeatureFailed {
		t.Fatalf("expected %s, got %s", FeatureFailed, newStatus)
	}
}

// VAL-ENG-QUEUE-004: pending→cancelled
func TestFeatureTransitionPendingToCancelled(t *testing.T) {
	newStatus, err := TransitionFeatureStatus(FeaturePending, FeatureCancelled)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if newStatus != FeatureCancelled {
		t.Fatalf("expected %s, got %s", FeatureCancelled, newStatus)
	}
}

func TestFeatureTransitionInProgressToCancelled(t *testing.T) {
	newStatus, err := TransitionFeatureStatus(FeatureInProgress, FeatureCancelled)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if newStatus != FeatureCancelled {
		t.Fatalf("expected %s, got %s", FeatureCancelled, newStatus)
	}
}

// VAL-ENG-QUEUE-005: Failed feature re-queuing (failed→pending)
func TestFeatureTransitionFailedToPending(t *testing.T) {
	newStatus, err := TransitionFeatureStatus(FeatureFailed, FeaturePending)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if newStatus != FeaturePending {
		t.Fatalf("expected %s, got %s", FeaturePending, newStatus)
	}
}

// VAL-ENG-QUEUE-009: Invalid feature status handling
func TestFeatureTransitionCompletedToInProgress(t *testing.T) {
	_, err := TransitionFeatureStatus(FeatureCompleted, FeatureInProgress)
	if err == nil {
		t.Fatal("expected error for invalid transition completed→in_progress")
	}
}

func TestFeatureTransitionCancelledToInProgress(t *testing.T) {
	_, err := TransitionFeatureStatus(FeatureCancelled, FeatureInProgress)
	if err == nil {
		t.Fatal("expected error for invalid transition cancelled→in_progress")
	}
}

func TestFeatureTransitionPendingToCompleted(t *testing.T) {
	_, err := TransitionFeatureStatus(FeaturePending, FeatureCompleted)
	if err == nil {
		t.Fatal("expected error for invalid transition pending→completed")
	}
}

func TestFeatureTransitionPendingToFailed(t *testing.T) {
	// pending→failed is valid e.g. max retries reached before starting
	_, err := TransitionFeatureStatus(FeaturePending, FeatureFailed)
	if err != nil {
		t.Fatalf("pending→failed should be valid: %v", err)
	}
}

func TestFeatureTransitionCompletedToFailed(t *testing.T) {
	_, err := TransitionFeatureStatus(FeatureCompleted, FeatureFailed)
	if err == nil {
		t.Fatal("expected error for invalid transition completed→failed")
	}
}

func TestFeatureTransitionCompletedToCancelled(t *testing.T) {
	_, err := TransitionFeatureStatus(FeatureCompleted, FeatureCancelled)
	if err == nil {
		t.Fatal("expected error for invalid transition completed→cancelled")
	}
}

// ─── Helper / Edge Cases ───────────────────────────────────────────────────

// VAL-ENG-QUEUE-009: Unknown status handled gracefully
func TestFeatureTransitionUnknownStatus(t *testing.T) {
	_, err := TransitionFeatureStatus("unknown", FeatureCompleted)
	if err == nil {
		t.Fatal("expected error for unknown source status")
	}
}

func TestMissionTransitionUnknownStatus(t *testing.T) {
	_, err := TransitionMissionStatus("unknown", MissionActive)
	if err == nil {
		t.Fatal("expected error for unknown source status")
	}
}

func TestValidMissionTransitionsCount(t *testing.T) {
	transitions := ValidMissionTransitions()
	if len(transitions) == 0 {
		t.Fatal("expected non-empty valid mission transitions")
	}
}

func TestValidFeatureTransitionsCount(t *testing.T) {
	transitions := ValidFeatureTransitions()
	if len(transitions) == 0 {
		t.Fatal("expected non-empty valid feature transitions")
	}
}

func TestIsValidMissionTransition(t *testing.T) {
	if !IsValidMissionTransition(MissionPlanning, MissionActive) {
		t.Error("expected planning→active to be valid")
	}
	if IsValidMissionTransition(MissionPlanning, MissionCompleted) {
		t.Error("expected planning→completed to be invalid")
	}
}

func TestIsValidFeatureTransition(t *testing.T) {
	if !IsValidFeatureTransition(FeaturePending, FeatureInProgress) {
		t.Error("expected pending→in_progress to be valid")
	}
	if IsValidFeatureTransition(FeatureCompleted, FeatureInProgress) {
		t.Error("expected completed→in_progress to be invalid")
	}
}
