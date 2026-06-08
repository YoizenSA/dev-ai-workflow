package missions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// DefaultBaseDir is the default directory for all mission data.
const DefaultBaseDir = "~/.local/share/ywai/missions"

// ─── Cross-Surface Integration ─────────────────────────────────────────────

// BroadcastFunc is a callback for broadcasting state changes to UIs (TUI, Web).
type BroadcastFunc func(eventType string, payload interface{})

// EngineConfig configures the mission engine.
type EngineConfig struct {
	WorkerTimeout   time.Duration
	MaxRetries      int
	Validation      ValidationConfig
}

// DefaultEngineConfig returns sensible defaults.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		WorkerTimeout: DefaultWorkerTimeout,
		MaxRetries:    DefaultMaxRetries,
		Validation:    DefaultValidationConfig(),
	}
}

// Engine is the high-level mission orchestrator that ties together the store,
// worker manager, validation pipeline, and broadcast to UIs.
type Engine struct {
	store    *MissionsStore
	config   EngineConfig
	workers  *WorkerManager
	val      *ValidationPipeline
	broadcast BroadcastFunc

	// Active tracking
	mu           sync.Mutex
	activeMissionID string
	cancelFn     context.CancelFunc
}

// NewEngine creates a new mission orchestration engine.
func NewEngine(store *MissionsStore, config EngineConfig, broadcast BroadcastFunc) *Engine {
	if broadcast == nil {
		broadcast = func(string, interface{}) {}
	}
	return &Engine{
		store:     store,
		config:    config,
		workers:   NewWorkerManager(store, WorkerConfig{Timeout: config.WorkerTimeout, MaxRetries: config.MaxRetries}),
		val:       NewValidationPipeline(store, config.Validation),
		broadcast: broadcast,
	}
}

// RunMission runs a mission from active through all features and milestones.
// It processes features sequentially, runs validation at milestone boundaries,
// and broadcasts state changes to connected UIs.
func (e *Engine) RunMission(missionID string) error {
	e.mu.Lock()
	if e.activeMissionID != "" {
		e.mu.Unlock()
		return fmt.Errorf("engine is already running mission %q", e.activeMissionID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.activeMissionID = missionID
	e.cancelFn = cancel
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.activeMissionID = ""
		e.cancelFn = nil
		e.mu.Unlock()
	}()

	mission, err := e.store.LoadMission(missionID)
	if err != nil {
		return fmt.Errorf("load mission: %w", err)
	}

	// Transition to active if still in planning
	if mission.Status == MissionPlanning {
		newStatus, err := TransitionMissionStatus(mission.Status, MissionActive)
		if err != nil {
			return fmt.Errorf("transition to active: %w", err)
		}
		mission.Status = newStatus
		mission.UpdatedAt = time.Now()
		if err := e.store.SaveMission(mission); err != nil {
			return fmt.Errorf("save mission: %w", err)
		}
	}

	// Recover any in-progress features from a previous crash
	recovered, err := e.store.RecoverInProgressFeatures(mission)
	if err != nil {
		return fmt.Errorf("recover in-progress features: %w", err)
	}
	for _, featureID := range recovered {
		log.Printf("Recovered feature %s – resetting to pending", featureID)
		feat, fErr := GetFeatureByID(mission, featureID)
		if fErr != nil {
			continue
		}
		feat.Status = FeaturePending
	}

	e.broadcast("mission_status_changed", map[string]interface{}{
		"id":     mission.ID,
		"status": mission.Status,
		"action": "start",
	})

	// Process features
	for {
		select {
		case <-ctx.Done():
			return ErrWorkerCancelled
		default:
		}

		// Reload mission for latest state
		mission, err = e.store.LoadMission(missionID)
		if err != nil {
			return err
		}

		// Check if mission was paused or cancelled
		if mission.Status == MissionPaused {
			e.broadcast("mission_status_changed", map[string]interface{}{
				"id":     mission.ID,
				"status": mission.Status,
				"action": "paused",
			})
			return nil // caller should check status and wait
		}
		if mission.Status == MissionCancelled {
			e.broadcast("mission_status_changed", map[string]interface{}{
				"id":     mission.ID,
				"status": mission.Status,
				"action": "cancelled",
			})
			return ErrWorkerCancelled
		}

		// Get next pending feature
		feature := NextPendingFeature(mission)
		if feature == nil {
			break // all features processed
		}

		e.broadcast("feature_status_changed", map[string]interface{}{
			"missionId": mission.ID,
			"featureId": feature.ID,
			"status":    FeatureInProgress,
			"action":    "start",
		})

		// Run the feature
		log.Printf("Running feature: %s (%s)", feature.ID, feature.Description)
		handoff, err := e.workers.ExecuteFeature(mission, feature.ID)
		if err != nil {
			log.Printf("Feature %s failed: %v", feature.ID, err)

			// Check for cancellation/timeout
			if errors.Is(err, ErrWorkerCancelled) || errors.Is(err, ErrWorkerTimeout) {
				e.broadcast("feature_status_changed", map[string]interface{}{
					"missionId": mission.ID,
					"featureId": feature.ID,
					"status":    FeatureFailed,
					"action":    "failed",
					"error":     err.Error(),
				})
				return err
			}

			// Check if max retries reached
			if errors.Is(err, ErrMaxRetries) {
				e.broadcast("feature_status_changed", map[string]interface{}{
					"missionId": mission.ID,
					"featureId": feature.ID,
					"status":    FeatureFailed,
					"action":    "max_retries",
					"error":     err.Error(),
				})
				return err
			}

			e.broadcast("feature_status_changed", map[string]interface{}{
				"missionId": mission.ID,
				"featureId": feature.ID,
				"status":    FeatureFailed,
				"action":    "failed",
			})

			// Don't fail the whole mission for a single feature failure
			continue
		}

		if handoff != nil {
			_ = e.store.RecordWorkerHandoff(mission.ID, feature.ID, handoff)
		}

		e.broadcast("feature_status_changed", map[string]interface{}{
			"missionId": mission.ID,
			"featureId": feature.ID,
			"status":    FeatureCompleted,
			"action":    "completed",
		})
	}

	// Process milestone completion and validation
	mission, _ = e.store.LoadMission(missionID)
	for _, ms := range mission.Milestones {
		completed, _ := CheckMilestoneCompletion(mission, ms.Name)
		if completed {
			log.Printf("Milestone %q complete – running validation", ms.Name)

			newStatus, tErr := TransitionMissionStatus(mission.Status, MissionValidating)
			if tErr == nil {
				mission.Status = newStatus
				mission.UpdatedAt = time.Now()
				_ = e.store.SaveMission(mission)
			}

			e.broadcast("mission_status_changed", map[string]interface{}{
				"id":        mission.ID,
				"status":    MissionValidating,
				"milestone": ms.Name,
				"action":    "validating",
			})

			report, vErr := e.val.RunValidation(context.Background(), mission, ms.Name)
			if vErr != nil {
				log.Printf("Validation failed for milestone %q: %v", ms.Name, vErr)

				report = &ValidationReport{
					Milestone: ms.Name,
					Passed:    false,
					RunAt:     time.Now(),
				}
			}

			// Persist validation results
			_ = e.val.PersistReport(mission.ID, ms.Name, report)

			e.broadcast("validation_complete", map[string]interface{}{
				"missionId":  mission.ID,
				"milestone":  ms.Name,
				"passed":     report.Passed,
				"report":     report,
			})
		}
	}

	// Final mission status
	mission, _ = e.store.LoadMission(missionID)
	allDone, _ := AllMilestonesComplete(mission)
	if allDone {
		newStatus, tErr := TransitionMissionStatus(mission.Status, MissionCompleted)
		if tErr == nil {
			mission.Status = newStatus
			now := time.Now()
			mission.CompletedAt = &now
			mission.UpdatedAt = now
			_ = e.store.SaveMission(mission)
		}
	} else {
		newStatus, tErr := TransitionMissionStatus(mission.Status, MissionFailed)
		if tErr == nil {
			mission.Status = newStatus
			mission.UpdatedAt = time.Now()
			_ = e.store.SaveMission(mission)
		}
	}

	e.broadcast("mission_status_changed", map[string]interface{}{
		"id":     mission.ID,
		"status": mission.Status,
		"action": string(mission.Status),
	})

	return nil
}

// PauseMission pauses an active mission. Returns an error if the mission
// is not in the active state.
func PauseMission(store *MissionsStore, missionID string) error {
	mission, err := store.LoadMission(missionID)
	if err != nil {
		return err
	}

	if mission.Status != MissionActive && mission.Status != MissionPlanning {
		return fmt.Errorf("%w: cannot pause mission in state %q", ErrInvalidTransition, mission.Status)
	}

	newStatus, err := TransitionMissionStatus(mission.Status, MissionPaused)
	if err != nil {
		return err
	}
	mission.Status = newStatus
	mission.UpdatedAt = time.Now()

	return store.SaveMission(mission)
}

// CancelMissionFromSurface cancels a mission from any surface (CLI, TUI, Web UI).
func CancelMissionFromSurface(store *MissionsStore, missionID string) error {
	mission, err := store.LoadMission(missionID)
	if err != nil {
		return err
	}

	if mission.Status != MissionActive && mission.Status != MissionPaused {
		return fmt.Errorf("%w: cannot cancel mission in state %q", ErrInvalidTransition, mission.Status)
	}

	return CancelMission(store, mission)
}

// RetryFeatureFromSurface re-queues a failed feature from any surface.
func RetryFeatureFromSurface(store *MissionsStore, missionID, featureID string) error {
	mission, err := store.LoadMission(missionID)
	if err != nil {
		return err
	}

	feat, err := GetFeatureByID(mission, featureID)
	if err != nil {
		return err
	}

	if feat.Status != FeatureFailed {
		return fmt.Errorf("cannot retry feature in state %q", feat.Status)
	}

	_, err = RequeueFeature(store, mission, featureID)
	return err
}

// ─── Store helpers for integration ─────────────────────────────────────────

// PersistReport is an alias for validation pipeline's persistReport that is
// callable from the engine.
func (vp *ValidationPipeline) PersistReport(missionID, milestoneName string, report *ValidationReport) error {
	return vp.persistResults(missionID, milestoneName, report)
}

// ─── Original Public API ───────────────────────────────────────────────────

// StartInteractivePlanning begins an interactive planning session.
func StartInteractivePlanning(store *MissionsStore) (*Mission, error) {
	return RunInteractivePlanning(store, os.Stdin, os.Stdout)
}

// OpenStore opens (or creates) the missions store at the default location.
func OpenStore() (*MissionsStore, error) {
	baseDir, err := expandPath(DefaultBaseDir)
	if err != nil {
		return nil, fmt.Errorf("expand missions dir: %w", err)
	}

	store := NewMissionsStore(baseDir)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create missions dir: %w", err)
	}

	return store, nil
}

// expandPath expands the leading ~ to the user's home directory.
func expandPath(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	return home + path[1:], nil
}
