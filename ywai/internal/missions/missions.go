package missions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/events"
)

// DefaultBaseDir is the default directory for all mission data.
const DefaultBaseDir = "~/.local/share/ywai/missions"

// ─── Cross-Surface Integration ─────────────────────────────────────────────

// BroadcastFunc is a callback for broadcasting state changes to UIs (TUI, Web).
type BroadcastFunc func(eventType string, payload interface{})

// EngineConfig configures the mission engine.
// RepoResolver resolves a project name to a filesystem path.
type RepoResolver interface {
	Resolve(projectName string) (repoPath string, err error)
}

// ProjectRepoResolver resolves using a ProjectStore.
type ProjectRepoResolver struct {
	store *ProjectStore
}

// NewProjectRepoResolver creates a ProjectRepoResolver.
func NewProjectRepoResolver(store *ProjectStore) *ProjectRepoResolver {
	return &ProjectRepoResolver{store: store}
}

// Resolve looks up the project by name and returns its path.
func (r *ProjectRepoResolver) Resolve(projectName string) (string, error) {
	proj, err := r.store.Get(projectName)
	if err != nil {
		return "", fmt.Errorf("resolve project %q: %w", projectName, err)
	}
	return proj.Path, nil
}

type EngineConfig struct {
	WorkerTimeout     time.Duration
	MaxRetries        int
	MaxParallel       int // default 1; >1 enables concurrent feature execution (FASE 6)
	VerifyCleanStreak int // default 1; number of clean verify runs required per feature (FASE 5)
	RepoResolver      RepoResolver
	Validation        ValidationConfig
	EventStore        events.Store // optional event sourcing; nil = no events emitted
	AgentsDir         string       // path to agents/core directory for stable instructions caching
	BaseRef           string       // git ref to branch feature worktrees from (default: repo HEAD)
}

// DefaultEngineConfig returns sensible defaults.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		WorkerTimeout:     DefaultWorkerTimeout,
		MaxRetries:        DefaultMaxRetries,
		MaxParallel:       1,
		VerifyCleanStreak: 1,
		Validation:        DefaultValidationConfig(),
	}
}

// Engine is the high-level mission orchestrator that ties together the store,
// worker manager, validation pipeline, and broadcast to UIs.
type Engine struct {
	store        *MissionsStore
	config       EngineConfig
	workers      *WorkerManager
	val          *ValidationPipeline
	broadcast    BroadcastFunc
	workspaceMgr *WorkspaceManager // set per mission run when RepoResolver is configured

	// Active tracking
	mu              sync.Mutex
	activeMissionID string
	cancelFn        context.CancelFunc
}

// ValidateTransition checks if a mission state transition is valid per the FSM.
func (e *Engine) ValidateTransition(current, next MissionStatus) error {
	return IsValidTransition(current, next)
}

// IsValidTransition is a standalone validation function that checks
// if a mission state transition is valid per the FSM rules.
// Unlike ValidateTransition, it does not require an Engine instance.
// Uses TransitionMissionStatus from fsm.go as the single source of truth.
func IsValidTransition(current, next MissionStatus) error {
	if IsValidMissionTransition(current, next) {
		return nil
	}
	return fmt.Errorf("invalid mission transition: %s → %s", current, next)
}

// emitEvent writes an event to the configured event store (if any).
// It is a no-op when EventStore is nil.
// cleanupFeatureWorktree removes the worktree for a feature if workspace manager is active.
func (e *Engine) cleanupFeatureWorktree(feature *Feature) {
	if e.workspaceMgr != nil && feature.WorktreePath != "" {
		if err := e.workspaceMgr.RemoveWorktree(feature.WorktreePath, feature.Branch); err != nil {
			log.Printf("Warning: could not remove worktree for %s: %v", feature.ID, err)
		}
	}
}

// runFeaturesSequentially processes features one at a time in order (FASE 6 fallback).
func (e *Engine) runFeaturesSequentially(ctx context.Context, mission *Mission) error {
	for {
		select {
		case <-ctx.Done():
			return ErrWorkerCancelled
		default:
		}

		// Reload mission for latest state
		mission, err := e.store.LoadMission(mission.ID)
		if err != nil {
			return err
		}

		// Check if mission was paused or cancelled
		if mission.Status == MissionPaused || mission.Status == MissionCancelled {
			return nil
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

		// Set up worktree if workspace manager is configured
		if e.workspaceMgr != nil {
			feature.WorktreePath = e.workspaceMgr.GetWorktreePath(mission.ID, feature.ID)
			feature.Branch = e.workspaceMgr.BranchName(mission.ID, feature.ID)
			if err := e.workspaceMgr.CreateWorktree(feature.WorktreePath, feature.Branch); err != nil {
				log.Printf("Warning: could not create worktree for %s: %v — running in tempdir mode", feature.ID, err)
				feature.WorktreePath = ""
				feature.Branch = ""
			}
			_ = e.store.SaveMission(mission)
		}

		// Run the feature
		log.Printf("Running feature: %s (%s)", feature.ID, feature.Description)
		handoff, err := e.workers.ExecuteFeature(mission, feature.ID)
		if err != nil {
			log.Printf("Feature %s failed: %v", feature.ID, err)

			e.cleanupFeatureWorktree(feature)

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

		// Merge to integration branch if workspace manager is active
		if e.workspaceMgr != nil {
			if err := e.workspaceMgr.MergeToIntegration(mission.ID, feature.ID); err != nil {
				log.Printf("Warning: could not merge %s to integration: %v", feature.ID, err)
			}
		}

		e.cleanupFeatureWorktree(feature)

		e.broadcast("feature_status_changed", map[string]interface{}{
			"missionId": mission.ID,
			"featureId": feature.ID,
			"status":    FeatureCompleted,
			"action":    "completed",
		})
	}
	return nil
}

func (e *Engine) emitEvent(sessionID, missionID string, eventType events.EventType, data interface{}) {
	if e.config.EventStore == nil {
		return
	}

	seq, err := e.config.EventStore.LastSequence(sessionID)
	if err != nil {
		log.Printf("event sourcing: failed to get last sequence: %v", err)
		return
	}

	var dataBytes []byte
	if data != nil {
		dataBytes, err = json.Marshal(data)
		if err != nil {
			log.Printf("event sourcing: failed to marshal event data: %v", err)
			return
		}
	}

	event := events.Event{
		ID:        fmt.Sprintf("%s-%d", missionID, seq+1),
		SessionID: sessionID,
		MissionID: missionID,
		Type:      eventType,
		Data:      dataBytes,
		Sequence:  seq + 1,
		Timestamp: time.Now(),
	}
	if err := e.config.EventStore.Append(event); err != nil {
		log.Printf("event sourcing: failed to append event: %v", err)
	}
}

// NewEngine creates a new mission orchestration engine.
func NewEngine(store *MissionsStore, config EngineConfig, broadcast BroadcastFunc) *Engine {
	if broadcast == nil {
		broadcast = func(string, interface{}) {}
	}
	e := &Engine{
		store:     store,
		config:    config,
		val:       NewValidationPipeline(store, config.Validation),
		broadcast: broadcast,
	}
	workerCfg := WorkerConfig{
		Timeout:           config.WorkerTimeout,
		MaxRetries:        config.MaxRetries,
		AgentsDir:         config.AgentsDir,
		VerifyCleanStreak: config.VerifyCleanStreak,
	}
	if config.VerifyCleanStreak > 0 {
		// Enable verification when clean streak is configured
		workerCfg.Verifier = NewCommandVerifier()
	}
	e.workers = NewWorkerManager(store, workerCfg)
	e.workers.SetLogBroadcast(func(missionID, featureID, line string) {
		e.broadcast("log_update", map[string]interface{}{
			"missionId": missionID,
			"featureId": featureID,
			"line":      line,
			"timestamp": time.Now().Unix(),
		})
	})
	return e
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

	// Load mission
	mission, err := e.store.LoadMission(missionID)
	if err != nil {
		return fmt.Errorf("load mission: %w", err)
	}

	e.emitEvent(mission.ID, mission.ID, events.EventMissionCreated, nil)

	// Set up workspace manager if a RepoResolver is configured
	if e.config.RepoResolver != nil && mission.Project != "" {
		repoPath, err := e.config.RepoResolver.Resolve(mission.Project)
		if err != nil {
			log.Printf("Warning: could not resolve project %q: %v — running in tempdir mode", mission.Project, err)
		} else {
			wm := NewWorkspaceManager(repoPath)
			wm.BaseRef = e.config.BaseRef
			e.workspaceMgr = wm
		}
	}

	// Check validation contract coverage before running
	// Log warning but don't fail execution for incomplete contracts
	if err := CheckValidationContractCoverage(e.store, mission); err != nil {
		log.Printf("validation contract coverage check warning: %v", err)
	}

	defer func() {
		e.mu.Lock()
		e.activeMissionID = ""
		e.cancelFn = nil
		e.mu.Unlock()
	}()

	// Transition to active if still in planning
	if mission.Status == MissionPlanning {
		newStatus, err := TransitionMissionStatus(mission.Status, MissionActive)
		if err != nil {
			return fmt.Errorf("transition to active: %w", err)
		}
		if err := e.ValidateTransition(mission.Status, newStatus); err != nil {
			return fmt.Errorf("mission %s: %w", missionID, err)
		}
		mission.Status = newStatus
		mission.UpdatedAt = time.Now()
		if err := e.store.SaveMission(mission); err != nil {
			return fmt.Errorf("save mission: %w", err)
		}
		e.emitEvent(mission.ID, mission.ID, events.EventMissionStarted, nil)
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

	// Process features — use concurrent scheduler if parallel, else sequential
	if e.config.MaxParallel > 1 {
		if err := e.RunFeaturesConcurrently(ctx, mission); err != nil {
			return err
		}
	} else {
		// Sequential processing (backward compatible)
		if err := e.runFeaturesSequentially(ctx, mission); err != nil {
			return err
		}
	}

	// Process milestone completion and validation
	mission, _ = e.store.LoadMission(missionID)
	for mi := range mission.Milestones {
		ms := &mission.Milestones[mi]
		completed, _ := CheckMilestoneCompletion(mission, ms.Name)
		if completed {
			log.Printf("Milestone %q complete – running validation", ms.Name)

			newStatus, tErr := TransitionMissionStatus(mission.Status, MissionValidating)
			if tErr == nil {
				if err := e.ValidateTransition(mission.Status, newStatus); err != nil {
					return fmt.Errorf("mission %s: %w", missionID, err)
				}
				mission.Status = newStatus
				mission.UpdatedAt = time.Now()
				_ = e.store.SaveMission(mission)
				e.emitEvent(mission.ID, mission.ID, events.EventMissionValidated, map[string]string{
					"milestone": ms.Name,
				})
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

			// Store the report on the milestone for report generation
			ms.ValidationReports = append(ms.ValidationReports, report)

			// Save mission to persist fix features and validation reports
			// before the reload below discards in-memory changes
			_ = e.store.SaveMission(mission)

			e.broadcast("validation_complete", map[string]interface{}{
				"missionId": mission.ID,
				"milestone": ms.Name,
				"passed":    report.Passed,
				"report":    report,
			})
		}
	}

	// Final mission status
	mission, _ = e.store.LoadMission(missionID)
	allDone, _ := AllMilestonesComplete(mission)
	if allDone {
		newStatus, tErr := TransitionMissionStatus(mission.Status, MissionCompleted)
		if tErr == nil {
			if err := e.ValidateTransition(mission.Status, newStatus); err != nil {
				return fmt.Errorf("mission %s: %w", missionID, err)
			}
			mission.Status = newStatus
			now := time.Now()
			mission.CompletedAt = &now
			mission.UpdatedAt = now
			_ = e.store.SaveMission(mission)
			// Generate mission report (FASE 7)
			if err := GenerateMissionReport(e.store, mission); err != nil {
				log.Printf("Warning: could not generate mission report: %v", err)
			}
			e.emitEvent(mission.ID, mission.ID, events.EventMissionCompleted, nil)
		}
	} else {
		newStatus, tErr := TransitionMissionStatus(mission.Status, MissionFailed)
		if tErr == nil {
			if err := e.ValidateTransition(mission.Status, newStatus); err != nil {
				return fmt.Errorf("mission %s: %w", missionID, err)
			}
			mission.Status = newStatus
			mission.UpdatedAt = time.Now()
			_ = e.store.SaveMission(mission)
			e.emitEvent(mission.ID, mission.ID, events.EventMissionFailed, nil)
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
// If a broadcast function is provided, it emits a feature_status_changed event
// so the kanban projector and other UI surfaces pick up the state change.
func RetryFeatureFromSurface(store *MissionsStore, missionID, featureID string, broadcast ...BroadcastFunc) error {
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
	if err != nil {
		return err
	}

	// Emit event so UI surfaces pick up the state change
	if len(broadcast) > 0 && broadcast[0] != nil {
		broadcast[0]("feature_status_changed", map[string]interface{}{
			"missionId": mission.ID,
			"featureId": featureID,
			"status":    FeaturePending,
			"action":    "retry",
		})
	}

	return nil
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
	return RunInteractivePlanning(store, os.Stdin, os.Stdout, "")
}

// StartInteractivePlanningWithProject begins an interactive planning session with a project name.
func StartInteractivePlanningWithProject(store *MissionsStore, project string) (*Mission, error) {
	return RunInteractivePlanning(store, os.Stdin, os.Stdout, project)
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
