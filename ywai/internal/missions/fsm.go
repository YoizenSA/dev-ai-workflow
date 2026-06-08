package missions

import (
	"errors"
	"fmt"
)

// ─── Errors ────────────────────────────────────────────────────────────────

var (
	ErrInvalidTransition = errors.New("invalid state transition")
)

// ─── Mission State Machine ─────────────────────────────────────────────────

// missionTransitions defines all valid mission state transitions.
// Key = current status, Value = set of allowed target statuses.
var missionTransitions = map[MissionStatus]map[MissionStatus]bool{
	MissionPlanning: {
		MissionActive: true,
	},
	MissionActive: {
		MissionPaused:    true,
		MissionCompleted: true,
		MissionFailed:    true,
		MissionValidating: true,
	},
	MissionPaused: {
		MissionActive: true,
	},
	MissionCompleted: {},
	MissionFailed: {
		MissionPlanning: true,
	},
	MissionCancelled: {},
	MissionValidating: {
		MissionCompleted: true,
		MissionFailed:    true,
		MissionActive:    true,
	},
}

// TransitionMissionStatus validates and performs a mission state transition.
// Returns the new status on success, or an error if the transition is invalid.
// Idempotent transitions (same→same) are allowed for pause/resume stability.
func TransitionMissionStatus(current, target MissionStatus) (MissionStatus, error) {
	// Allow idempotent transitions for stability (pause→pause, resume→resume)
	if current == target {
		return current, nil
	}

	transitions, ok := missionTransitions[current]
	if !ok {
		return current, fmt.Errorf("%w: unknown source status %q", ErrInvalidTransition, current)
	}

	if !transitions[target] {
		return current, fmt.Errorf("%w: cannot transition from %q to %q", ErrInvalidTransition, current, target)
	}

	return target, nil
}

// IsValidMissionTransition returns true if the transition is allowed.
func IsValidMissionTransition(from, to MissionStatus) bool {
	_, err := TransitionMissionStatus(from, to)
	return err == nil
}

// ValidMissionTransitions returns all valid mission transition pairs.
func ValidMissionTransitions() [][2]MissionStatus {
	var result [][2]MissionStatus
	for from, targets := range missionTransitions {
		for to := range targets {
			result = append(result, [2]MissionStatus{from, to})
		}
	}
	if result == nil {
		return [][2]MissionStatus{}
	}
	return result
}

// ─── Feature State Machine ─────────────────────────────────────────────────

// featureTransitions defines all valid feature state transitions.
// Key = current status, Value = set of allowed target statuses.
var featureTransitions = map[FeatureStatus]map[FeatureStatus]bool{
	FeaturePending: {
		FeatureInProgress: true,
		FeatureCancelled:  true,
	},
	FeatureInProgress: {
		FeatureCompleted: true,
		FeatureFailed:    true,
		FeatureCancelled: true,
	},
	FeatureCompleted: {},
	FeatureFailed: {
		FeaturePending: true, // re-queue for retry
	},
	FeatureCancelled: {},
}

// TransitionFeatureStatus validates and performs a feature state transition.
// Returns the new status on success, or an error if the transition is invalid.
func TransitionFeatureStatus(current, target FeatureStatus) (FeatureStatus, error) {
	transitions, ok := featureTransitions[current]
	if !ok {
		return current, fmt.Errorf("%w: unknown source status %q", ErrInvalidTransition, current)
	}

	if !transitions[target] {
		return current, fmt.Errorf("%w: cannot transition from %q to %q", ErrInvalidTransition, current, target)
	}

	return target, nil
}

// IsValidFeatureTransition returns true if the feature transition is allowed.
func IsValidFeatureTransition(from, to FeatureStatus) bool {
	_, err := TransitionFeatureStatus(from, to)
	return err == nil
}

// ValidFeatureTransitions returns all valid feature transition pairs.
func ValidFeatureTransitions() [][2]FeatureStatus {
	var result [][2]FeatureStatus
	for from, targets := range featureTransitions {
		for to := range targets {
			result = append(result, [2]FeatureStatus{from, to})
		}
	}
	if result == nil {
		return [][2]FeatureStatus{}
	}
	return result
}
