package missions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ─── Worker Lifecycle Tracking ─────────────────────────────────────────────
//
// These methods record worker session lifecycle data on the MissionsStore.
// They live next to the store's other persistence methods rather than in
// recovery.go so the store's record surface stays in one place.

// RecordWorkerStart records the start of a worker session in the feature's
// store. It generates a session ID, records the PID, and updates the feature
// in the mission store.
func (s *MissionsStore) RecordWorkerStart(mission *Mission, featureID string, sessionID string, pid int) error {
	feat, err := GetFeatureByID(mission, featureID)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if feat.CurrentWorkerSessionID != nil {
		// Add previous session to history
		feat.WorkerSessionIDs = append(feat.WorkerSessionIDs, *feat.CurrentWorkerSessionID)
	}
	feat.CurrentWorkerSessionID = &sessionID
	feat.UpdatedAt = now
	// NOTE: CompletedWorkerSessionID is set separately when the worker finishes,
	// not at start (previous code had a copy-paste bug setting it here).
	mission.UpdatedAt = now

	return s.SaveMission(mission)
}

// RecordWorkerHandoff records the handoff result for a completed worker
// session and persists the handoff JSON to the workers artifact directory.
func (s *MissionsStore) RecordWorkerHandoff(missionID, featureID string, handoff *WorkerHandoff) error {
	if err := ValidateMissionID(missionID); err != nil {
		return err
	}
	workersDir := filepath.Join(s.MissionDir(missionID), "workers", featureID)
	if err := os.MkdirAll(workersDir, 0755); err != nil {
		return fmt.Errorf("create workers artifact dir for handoff: %w", err)
	}

	handoffPath := filepath.Join(workersDir, "handoff.json")
	data, err := json.MarshalIndent(handoff, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal handoff: %w", err)
	}

	if err := atomicWrite(handoffPath, data); err != nil {
		return fmt.Errorf("write handoff file: %w", err)
	}

	return nil
}

// RecordWorkerLog saves the worker's log output to the workers artifact
// directory for persistence across crashes.
func (s *MissionsStore) RecordWorkerLog(missionID, featureID, logContent string) error {
	if err := ValidateMissionID(missionID); err != nil {
		return err
	}
	workersDir := filepath.Join(s.MissionDir(missionID), "workers", featureID)
	if err := os.MkdirAll(workersDir, 0755); err != nil {
		return fmt.Errorf("create workers artifact dir for log: %w", err)
	}

	logPath := filepath.Join(workersDir, "output.log")
	if err := atomicWrite(logPath, []byte(logContent)); err != nil {
		return fmt.Errorf("write worker log: %w", err)
	}

	return nil
}

// ReadWorkerLog reads the worker's log output from the workers artifact directory.
// Returns empty string if no log file exists.
func (s *MissionsStore) ReadWorkerLog(missionID, featureID string) (string, error) {
	if err := ValidateMissionID(missionID); err != nil {
		return "", err
	}
	logPath := filepath.Join(s.MissionDir(missionID), "workers", featureID, "output.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("log not found for feature %q in mission %q", featureID, missionID)
		}
		return "", fmt.Errorf("read worker log: %w", err)
	}
	return string(data), nil
}

// WorkerLogPath returns the path to a worker's log file.
func (s *MissionsStore) WorkerLogPath(missionID, featureID string) string {
	// Note: WorkerLogPath returns a path for external use. Validation is expected
	// to happen before the path is used for actual file operations, but we validate
	// here as a defense-in-depth measure.
	if err := ValidateMissionID(missionID); err != nil {
		return ""
	}
	return filepath.Join(s.MissionDir(missionID), "workers", featureID, "output.log")
}
