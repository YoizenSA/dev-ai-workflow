package missions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// FindRunnableFeatures returns features that are pending and whose preconditions are met.
// Preconditions reference other feature IDs that must be completed first.
func FindRunnableFeatures(mission *Mission) []*Feature {
	if mission == nil {
		return nil
	}

	// Build set of completed feature IDs
	completed := make(map[string]bool)
	for i := range mission.Features {
		if mission.Features[i].Status == FeatureCompleted {
			completed[mission.Features[i].ID] = true
		}
	}

	var runnable []*Feature
	for i := range mission.Features {
		f := &mission.Features[i]
		if f.Status != FeaturePending {
			continue
		}
		// Check preconditions
		met := true
		for _, pre := range f.Preconditions {
			if !completed[pre] {
				met = false
				break
			}
		}
		if met {
			runnable = append(runnable, f)
		}
	}
	return runnable
}

// featureStatusMap returns a map of feature ID → status for the mission.
func featureStatusMap(mission *Mission) map[string]FeatureStatus {
	statuses := make(map[string]FeatureStatus)
	for i := range mission.Features {
		statuses[mission.Features[i].ID] = mission.Features[i].Status
	}
	return statuses
}

// isPermanentlyBlocked checks if a pending feature has a precondition in a
// terminal failed state, meaning it will never become runnable (C3 fix).
func isPermanentlyBlocked(feature *Feature, statuses map[string]FeatureStatus) bool {
	for _, pre := range feature.Preconditions {
		s, ok := statuses[pre]
		if !ok {
			continue // precondition ID not found — skip
		}
		// If precondition failed or was cancelled, the dependent can never run
		if s == FeatureFailed || s == FeatureCancelled {
			return true
		}
	}
	return false
}

// featureResult holds the outcome of a single feature execution for the scheduler.
type featureResult struct {
	FeatureID string
	Handoff   *WorkerHandoff
	Err       error
}

// RunFeaturesConcurrently runs up to maxParallel features concurrently.
// It respects preconditions: only features whose preconditions are met will start.
func (e *Engine) RunFeaturesConcurrently(ctx context.Context, mission *Mission) error {
	missionID := mission.ID
	maxParallel := e.config.MaxParallel
	if maxParallel < 1 {
		maxParallel = 1
	}

	for {
		select {
		case <-ctx.Done():
			return ErrWorkerCancelled
		default:
		}

		// Reload mission for latest state
		var err error
		mission, err = e.store.LoadMission(missionID)
		if err != nil {
			return err
		}

		// Check mission status
		if mission.Status == MissionPaused || mission.Status == MissionCancelled {
			return nil
		}

		// Find runnable features
		runnable := FindRunnableFeatures(mission)
		if len(runnable) == 0 {
			// No more features can run — check if all are done
			allDone := true
			statuses := featureStatusMap(mission)
			for i := range mission.Features {
				f := &mission.Features[i]
				if f.Status == FeaturePending {
					allDone = false
					// C3 fix: check if this pending feature has a failed precondition
					if isPermanentlyBlocked(f, statuses) {
						log.Printf("Feature %s is blocked by a failed precondition — cancelling", f.ID)
						f.Status = FeatureCancelled
						f.UpdatedAt = time.Now()
						_ = e.store.SaveMission(mission)
					}
				} else if f.Status == FeatureInProgress {
					allDone = false
				}
			}
			if allDone {
				return nil // all features processed
			}
			// Some are pending (blocked by unmet preconditions) or in-progress
			// Yield to avoid busy-loop
			select {
			case <-ctx.Done():
				return ErrWorkerCancelled
			default:
				continue
			}
		}

		// Limit to maxParallel
		if len(runnable) > maxParallel {
			runnable = runnable[:maxParallel]
		}

		// Launch features concurrently (C2: reload mission per goroutine, serialize saves).
		var wg sync.WaitGroup
		var saveMu sync.Mutex
		results := make(chan featureResult, len(runnable))

		for _, feature := range runnable {
			wg.Add(1)
			go func(f *Feature) {
				defer wg.Done()

				e.broadcast("feature_status_changed", map[string]interface{}{
					"missionId": mission.ID,
					"featureId": f.ID,
					"status":    FeatureInProgress,
					"action":    "start",
				})

				// C2a fix: track the freshest mission snapshot for this goroutine.
				// Default to the shared outer mission; replace with the reloaded one if we
				// set up a worktree — so ExecuteFeature sees the correct WorktreePath.
				currentMission := mission

				if e.workspaceMgr != nil {
					saveMu.Lock()
					rel, rErr := e.store.LoadMission(mission.ID)
					var reloadedFeat *Feature
					if rErr == nil && rel != nil {
						for fi := range rel.Features {
							if rel.Features[fi].ID == f.ID {
								reloadedFeat = &rel.Features[fi]
								break
							}
						}
					}
					if reloadedFeat != nil {
						// Branch the feature worktree from the integration branch so it
						// inherits all previously completed features (e.g. a test feature
						// sees the implementation that merged before it).
						wtPath, wtBranch, wErr := e.workspaceMgr.CreateFeatureWorktree(mission.ID, reloadedFeat.ID)
						if wErr != nil {
							log.Printf("Warning: worktree for %s: %v — using tempdir", f.ID, wErr)
							reloadedFeat.WorktreePath = ""
							reloadedFeat.Branch = ""
						} else {
							reloadedFeat.WorktreePath = wtPath
							reloadedFeat.Branch = wtBranch
						}
						_ = e.store.SaveMission(rel)
						currentMission = rel // C2a fix: fresh mission with WorktreePath populated
					}
					saveMu.Unlock()
				}

				// C2a fix: pass currentMission so the worker sees the correct WorktreePath.
				handoff, execErr := e.workers.ExecuteFeature(currentMission, f.ID)

				if execErr != nil {
					log.Printf("Feature %s failed: %v", f.ID, execErr)
					e.cleanupFeatureWorktree(f)
					results <- featureResult{FeatureID: f.ID, Err: execErr}
					return
				}

				if handoff != nil {
					_ = e.store.RecordWorkerHandoff(mission.ID, f.ID, handoff)
				}

				// Commit the worker's edits to the feature branch, then merge to
				// integration. Without the commit the branch has no new commits and
				// the merge brings nothing — the produced code would never land.
				if e.workspaceMgr != nil {
					wt := e.workspaceMgr.GetWorktreePath(mission.ID, f.ID)
					if cErr := e.workspaceMgr.CommitWorktree(wt, fmt.Sprintf("ywai(%s): %s", f.ID, f.Description)); cErr != nil {
						log.Printf("Warning: commit %s: %v", f.ID, cErr)
					}
					// C2b fix: MergeToIntegration serializes internally via WorkspaceManager.mergeMu.
					if mErr := e.workspaceMgr.MergeToIntegration(mission.ID, f.ID); mErr != nil {
						log.Printf("Warning: merge %s: %v", f.ID, mErr)
					}
				}
				e.cleanupFeatureWorktree(f)

				e.broadcast("feature_status_changed", map[string]interface{}{
					"missionId": mission.ID,
					"featureId": f.ID,
					"status":    FeatureCompleted,
					"action":    "completed",
				})

				results <- featureResult{FeatureID: f.ID, Handoff: handoff}
			}(feature)
		}

		wg.Wait()
		close(results)

		// Check results for critical errors
		for res := range results {
			if res.Err != nil {
				if errors.Is(res.Err, ErrWorkerCancelled) || errors.Is(res.Err, ErrWorkerTimeout) {
					return res.Err
				}
				if errors.Is(res.Err, ErrMaxRetries) {
					return res.Err
				}
				// Other errors: continue with other features
			}
		}
	}
}
