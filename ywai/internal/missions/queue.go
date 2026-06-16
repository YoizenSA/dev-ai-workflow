package missions

import (
	"errors"
	"fmt"
	"time"
)

// ─── Errors ────────────────────────────────────────────────────────────────

var (
	ErrNoPendingFeatures = errors.New("no pending features")
	ErrQueueEmpty        = errors.New("feature queue is empty")
)

// ─── MilestoneStatusSummary ────────────────────────────────────────────────

// MilestoneStatusSummary provides a summary of a milestone's feature statuses.
type MilestoneStatusSummary struct {
	Milestone string `json:"milestone"`
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
	Cancelled int    `json:"cancelled"`
	Pending   int    `json:"pending"`
	AllDone   bool   `json:"allDone"`
}

// ─── Feature Queue Operations ──────────────────────────────────────────────

// NextPendingFeature returns the first feature with pending status,
// processing features in array order (sequential order).
// Returns nil if no pending features exist.
func NextPendingFeature(mission *Mission) *Feature {
	if mission == nil {
		return nil
	}
	for i := range mission.Features {
		if mission.Features[i].Status == FeaturePending {
			return &mission.Features[i]
		}
	}
	return nil
}

// GetFeatureByID finds a feature in the mission by its ID.
func GetFeatureByID(mission *Mission, featureID string) (*Feature, error) {
	if mission == nil {
		return nil, ErrInvalidMission
	}
	for i := range mission.Features {
		if mission.Features[i].ID == featureID {
			return &mission.Features[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrFeatureNotFound, featureID)
}

// StartFeature transitions a feature from pending to in_progress.
// It validates the transition, updates timestamps, and persists the change.
func StartFeature(store *MissionsStore, mission *Mission, featureID string) (*Feature, error) {
	if mission == nil {
		return nil, ErrInvalidMission
	}

	feat, err := GetFeatureByID(mission, featureID)
	if err != nil {
		return nil, err
	}

	newStatus, err := TransitionFeatureStatus(feat.Status, FeatureInProgress)
	if err != nil {
		return nil, err
	}

	// Use granular update to avoid lost-update with stale mission snapshots
	if err := store.UpdateFeatureStatus(mission.ID, featureID, newStatus); err != nil {
		return nil, fmt.Errorf("persist start feature: %w", err)
	}

	// Update local snapshot for caller use
	feat.Status = newStatus
	feat.UpdatedAt = time.Now().UTC()
	mission.UpdatedAt = feat.UpdatedAt

	return feat, nil
}

// CompleteFeature transitions a feature from in_progress to completed.
// Sets CompletedAt timestamp and persists the change.
func CompleteFeature(store *MissionsStore, mission *Mission, featureID string) (*Feature, error) {
	if mission == nil {
		return nil, ErrInvalidMission
	}

	feat, err := GetFeatureByID(mission, featureID)
	if err != nil {
		return nil, err
	}

	newStatus, err := TransitionFeatureStatus(feat.Status, FeatureCompleted)
	if err != nil {
		return nil, err
	}

	// Use granular update to avoid lost-update with stale mission snapshots
	if err := store.UpdateFeatureStatus(mission.ID, featureID, newStatus); err != nil {
		return nil, fmt.Errorf("persist complete feature: %w", err)
	}

	// Update local snapshot for caller use
	now := time.Now().UTC()
	feat.Status = newStatus
	feat.CompletedAt = &now
	feat.UpdatedAt = now
	mission.UpdatedAt = now

	return feat, nil
}

// FailFeature transitions a feature from in_progress to failed.
// Increments the RetryCount and persists the change.
// If lastError is provided, it is persisted on the feature for self-correction feedback.
func FailFeature(store *MissionsStore, mission *Mission, featureID string, lastError ...string) (*Feature, error) {
	if mission == nil {
		return nil, ErrInvalidMission
	}

	feat, err := GetFeatureByID(mission, featureID)
	if err != nil {
		return nil, err
	}

	newStatus, err := TransitionFeatureStatus(feat.Status, FeatureFailed)
	if err != nil {
		return nil, err
	}

	// Use granular update to avoid lost-update with stale mission snapshots.
	// UpdateFeatureStatus handles RetryCount increment atomically inside the lock.
	if err := store.UpdateFeatureStatus(mission.ID, featureID, newStatus, lastError...); err != nil {
		return nil, fmt.Errorf("persist fail feature: %w", err)
	}

	// Update local snapshot status (RetryCount is managed atomically in the store).
	// The snapshot may lag on RetryCount, but callers of FailFeature don't consume it.
	now := time.Now().UTC()
	feat.Status = newStatus
	if len(lastError) > 0 {
		feat.LastError = lastError[0]
	}
	feat.UpdatedAt = now
	mission.UpdatedAt = now

	return feat, nil
}

// CancelFeature transitions a feature to cancelled.
// Works from pending or in_progress states. Persists the change.
func CancelFeature(store *MissionsStore, mission *Mission, featureID string) (*Feature, error) {
	if mission == nil {
		return nil, ErrInvalidMission
	}

	feat, err := GetFeatureByID(mission, featureID)
	if err != nil {
		return nil, err
	}

	newStatus, err := TransitionFeatureStatus(feat.Status, FeatureCancelled)
	if err != nil {
		return nil, err
	}

	// Use granular update to avoid lost-update with stale mission snapshots
	if err := store.UpdateFeatureStatus(mission.ID, featureID, newStatus); err != nil {
		return nil, fmt.Errorf("persist cancel feature: %w", err)
	}

	// Update local snapshot for caller use
	now := time.Now().UTC()
	feat.Status = newStatus
	feat.UpdatedAt = now
	mission.UpdatedAt = now

	return feat, nil
}

// CancelMission cancels an entire mission: transitions the mission status
// to cancelled and cancels all pending/in-progress features.
// Idempotent: if already cancelled, returns nil error.
// Returns error if mission is in a state that cannot be cancelled (e.g., completed).
func CancelMission(store *MissionsStore, mission *Mission) error {
	if mission == nil {
		return ErrInvalidMission
	}

	// Idempotent: already cancelled is not an error
	if mission.Status == MissionCancelled {
		return nil
	}

	// Explicitly validate that cancellation is allowed from current state.
	// We do this because TransitionMissionStatus allows idempotent transitions,
	// but we need to reject transitions that don't make sense.
	switch mission.Status {
	case MissionActive, MissionPaused:
		// Valid cancellation sources
	default:
		return fmt.Errorf("cannot cancel mission in state %q: only active or paused missions can be cancelled", mission.Status)
	}

	newStatus, err := TransitionMissionStatus(mission.Status, MissionCancelled)
	if err != nil {
		return fmt.Errorf("cannot cancel mission in state %q: %w", mission.Status, err)
	}

	now := time.Now().UTC().Round(time.Millisecond)
	mission.Status = newStatus
	mission.UpdatedAt = now

	// Cancel all pending/in-progress features
	for i := range mission.Features {
		f := &mission.Features[i]
		if f.Status == FeaturePending || f.Status == FeatureInProgress {
			f.Status = FeatureCancelled
			f.UpdatedAt = now
		}
	}

	if err := store.SaveMission(mission); err != nil {
		return fmt.Errorf("persist cancel mission: %w", err)
	}

	return nil
}

// ResumeMission resumes a paused mission: transitions the mission status
// to active. Preserves all feature states (completed features remain completed).
// Returns error if mission is not in paused state.
func ResumeMission(store *MissionsStore, mission *Mission) error {
	if mission == nil {
		return ErrInvalidMission
	}

	// Explicitly require paused state
	if mission.Status != MissionPaused {
		return fmt.Errorf("cannot resume mission in state %q: only paused missions can be resumed", mission.Status)
	}

	newStatus, err := TransitionMissionStatus(mission.Status, MissionActive)
	if err != nil {
		return fmt.Errorf("cannot resume mission in state %q: %w", mission.Status, err)
	}

	now := time.Now().UTC().Round(time.Millisecond)
	mission.Status = newStatus
	mission.UpdatedAt = now

	if err := store.SaveMission(mission); err != nil {
		return fmt.Errorf("persist resume mission: %w", err)
	}

	return nil
}

// RequeueFeature transitions a failed feature back to pending for retry.
// Preserves the current RetryCount and persists the change.
func RequeueFeature(store *MissionsStore, mission *Mission, featureID string) (*Feature, error) {
	if mission == nil {
		return nil, ErrInvalidMission
	}

	feat, err := GetFeatureByID(mission, featureID)
	if err != nil {
		return nil, err
	}

	if feat.Status != FeatureFailed {
		return nil, fmt.Errorf("cannot requeue feature %q: current status is %s, expected %s",
			featureID, feat.Status, FeatureFailed)
	}

	newStatus, err := TransitionFeatureStatus(feat.Status, FeaturePending)
	if err != nil {
		return nil, err
	}

	// Use granular update to avoid lost-update with stale mission snapshots
	if err := store.UpdateFeatureStatus(mission.ID, featureID, newStatus); err != nil {
		return nil, fmt.Errorf("persist requeue feature: %w", err)
	}

	// Update local snapshot for caller use
	now := time.Now().UTC()
	feat.Status = newStatus
	feat.UpdatedAt = now
	mission.UpdatedAt = now

	return feat, nil
}

// ─── Milestone Operations ──────────────────────────────────────────────────

// GetMilestoneFeatures returns all features belonging to the given milestone.
func GetMilestoneFeatures(mission *Mission, milestoneName string) []Feature {
	if mission == nil {
		return nil
	}
	var result []Feature
	for _, f := range mission.Features {
		if f.Milestone == milestoneName {
			result = append(result, f)
		}
	}
	return result
}

// CheckMilestoneCompletion checks if all features in a milestone are completed.
// A milestone with no features is considered complete.
// Does NOT consider failed or cancelled features as complete.
func CheckMilestoneCompletion(mission *Mission, milestoneName string) (bool, error) {
	if mission == nil {
		return false, ErrInvalidMission
	}

	features := GetMilestoneFeatures(mission, milestoneName)
	if len(features) == 0 {
		// No features assigned to this milestone — it's complete by default
		return true, nil
	}

	for _, f := range features {
		if f.Status != FeatureCompleted {
			return false, nil
		}
	}
	return true, nil
}

// AllMilestonesComplete checks if every milestone in the mission has all
// its features completed. A mission with no milestones is considered complete.
func AllMilestonesComplete(mission *Mission) (bool, error) {
	if mission == nil {
		return false, ErrInvalidMission
	}

	if len(mission.Milestones) == 0 {
		// No milestones to complete
		return true, nil
	}

	for _, ms := range mission.Milestones {
		done, err := CheckMilestoneCompletion(mission, ms.Name)
		if err != nil {
			return false, err
		}
		if !done {
			return false, nil
		}
	}
	return true, nil
}

// GetMilestoneStatus returns a summary of feature statuses for a milestone.
func GetMilestoneStatus(mission *Mission, milestoneName string) *MilestoneStatusSummary {
	summary := &MilestoneStatusSummary{
		Milestone: milestoneName,
	}

	features := GetMilestoneFeatures(mission, milestoneName)
	summary.Total = len(features)
	summary.AllDone = true

	for _, f := range features {
		switch f.Status {
		case FeatureCompleted:
			summary.Completed++
		case FeatureFailed:
			summary.Failed++
			summary.AllDone = false
		case FeatureCancelled:
			summary.Cancelled++
			summary.AllDone = false
		case FeaturePending, FeatureInProgress:
			summary.Pending++
			summary.AllDone = false
		}
	}

	if summary.Total == 0 {
		summary.AllDone = true
	}

	return summary
}

// ProcessMilestoneAfterFeature checks whether completing a given feature
// causes its milestone to be fully completed. If so, it transitions the
// mission to MissionValidating and persists the change.
//
// Returns the new mission status (or empty string if no transition occurred)
// and any error encountered.
func ProcessMilestoneAfterFeature(store *MissionsStore, mission *Mission, featureID string) (string, error) {
	if mission == nil {
		return "", ErrInvalidMission
	}

	feat, err := GetFeatureByID(mission, featureID)
	if err != nil {
		return "", err
	}

	// Check if this feature's milestone is now complete
	done, err := CheckMilestoneCompletion(mission, feat.Milestone)
	if err != nil {
		return "", err
	}

	if !done {
		return "", nil // milestone not yet complete
	}

	// Transition mission to validating state
	newStatus, err := TransitionMissionStatus(mission.Status, MissionValidating)
	if err != nil {
		return "", fmt.Errorf("transition mission to validating: %w", err)
	}

	now := time.Now().UTC()
	mission.Status = newStatus
	mission.UpdatedAt = now

	if err := store.SaveMission(mission); err != nil {
		return "", fmt.Errorf("persist milestone completion: %w", err)
	}

	return string(newStatus), nil
}
