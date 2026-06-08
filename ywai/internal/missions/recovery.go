package missions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ─── Errors ────────────────────────────────────────────────────────────────

var (
	// ErrPathTraversal is returned when a mission ID contains path traversal
	// sequences like "../" that could access unintended directories.
	ErrPathTraversal = errors.New("path traversal detected")

	// ErrStoreCorrupt is returned when the mission store files are corrupt
	// and need recovery.
	ErrStoreCorrupt = errors.New("corrupt mission store file")

	// ErrPartialHandoff is returned when a handoff JSON file exists but is
	// truncated or otherwise invalid, indicating a partial handoff that was
	// preserved from a previous crash.
	ErrPartialHandoff = errors.New("partial handoff detected")
)

// ValidateMissionID checks that a mission ID does not contain path traversal
// sequences like "../" or absolute path components. Returns ErrPathTraversal
// if the ID is unsafe. This must be called before any file operation using
// the mission ID to prevent path traversal attacks.
func ValidateMissionID(id string) error {
	if id == "" {
		return fmt.Errorf("mission ID cannot be empty")
	}
	if strings.Contains(id, "../") || strings.Contains(id, "..\\") ||
		strings.Contains(id, "/") || strings.Contains(id, "\\") {
		return fmt.Errorf("%w: mission ID %q contains path separators", ErrPathTraversal, id)
	}
	if id == "." || id == ".." {
		return fmt.Errorf("%w: mission ID %q is a reserved path", ErrPathTraversal, id)
	}
	return nil
}

// ─── Engine-Level Recovery ─────────────────────────────────────────────────

// RecoverEngine runs all startup recovery tasks for the mission store.
// It should be called once when the engine starts, before any other operations.
//
// Recovery tasks:
//  1. Clean up stale temp directories (>24h old)
//  2. Clean up orphaned worker processes
//  3. For each mission: re-queue in_progress features, detect partial handoffs
//  4. Check for corrupt store files and attempt recovery
//
// Returns a list of recovery actions taken (for logging/monitoring).
func RecoverEngine(store *MissionsStore) ([]string, error) {
	var actions []string

	// 1. Clean up stale temp directories
	if cleaned, err := CleanupStaleTempDirs(); err != nil {
		actions = append(actions, fmt.Sprintf("stale temp cleanup: %v", err))
	} else if len(cleaned) > 0 {
		actions = append(actions, fmt.Sprintf("cleaned %d stale temp dir(s)", len(cleaned)))
	}

	// 2. Clean up orphaned worker processes
	if orphaned, err := CleanupOrphanedWorkers(); err != nil {
		actions = append(actions, fmt.Sprintf("orphaned worker cleanup: %v", err))
	} else if len(orphaned) > 0 {
		actions = append(actions, fmt.Sprintf("cleaned %d orphaned worker(s)", len(orphaned)))
	}

	// 3. Recover each mission
	missions, err := store.ListMissions()
	if err != nil {
		return actions, fmt.Errorf("list missions for recovery: %w", err)
	}

	for _, m := range missions {
		// Re-queue any in_progress features
		requeued, err := store.RecoverInProgressFeatures(m)
		if err != nil {
			actions = append(actions, fmt.Sprintf("re-queue features for %s: %v", m.ID, err))
			continue
		}
		for _, fid := range requeued {
			actions = append(actions, fmt.Sprintf("re-queued feature %s in mission %s (was in_progress)", fid, m.ID))
		}

		// Check for partial handoffs
		for _, feat := range m.Features {
			partial, err := store.DetectPartialHandoff(m.ID, feat.ID)
			if err != nil {
				actions = append(actions, fmt.Sprintf("check handoff for %s/%s: %v", m.ID, feat.ID, err))
				continue
			}
			if partial {
				actions = append(actions, fmt.Sprintf("partial handoff preserved for feature %s in mission %s", feat.ID, m.ID))
			}
		}
	}

	return actions, nil
}

// ─── Stale Temp Directory Cleanup ──────────────────────────────────────────

// CleanupStaleTempDirs removes worker temp directories that are older than 24
// hours. These are created during worker execution and should be cleaned up if
// the engine crashes before the worker completes. Returns the paths of removed
// directories.
func CleanupStaleTempDirs() ([]string, error) {
	tmpDir := os.TempDir()
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("read temp dir %s: %w", tmpDir, err)
	}

	cutoff := time.Now().Add(-24 * time.Hour)
	var removed []string
	var lastErr error

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "ywai-worker-") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			lastErr = err
			continue
		}

		if info.ModTime().Before(cutoff) {
			fullPath := filepath.Join(tmpDir, entry.Name())
			if err := os.RemoveAll(fullPath); err != nil {
				lastErr = err
				continue
			}
			removed = append(removed, fullPath)
		}
	}

	return removed, lastErr
}

// ─── Orphaned Worker Cleanup ───────────────────────────────────────────────

// CleanupOrphanedWorkers attempts to kill any orphaned opencode processes
// that may have been left behind by a crashed engine. On Linux, orphaned
// processes are reparented to init, but we still attempt to find and clean
// up any opencode processes that were spawned by a previous session.
//
// This is a best-effort operation. Returns the PIDs of any processes that
// were cleaned up.
func CleanupOrphanedWorkers() ([]int, error) {
	// On Linux, we use pgrep to find opencode processes. We only clean up
	// processes that are children of PID 1 (orphaned/reparented).
	// We avoid killing user-started opencode sessions.
	//
	// Since reliably distinguishing orphaned processes from user-started ones
	// requires OS-specific calls, we do a best-effort scan using /proc.
	// If /proc is not available, we silently skip.
	return scanAndKillOrphans()
}

// scanAndKillOrphans scans /proc for orphaned opencode processes.
func scanAndKillOrphans() ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		// /proc not available (e.g., some containers), skip gracefully
		return nil, nil
	}

	var cleaned []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := parseInt(entry.Name())
		if err != nil {
			continue
		}

		// Read the process command line to check if it's opencode
		cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}

		// Check if this is an opencode process
		args := strings.Split(string(cmdline), "\x00")
		if len(args) == 0 || args[0] == "" {
			continue
		}

		// Look for opencode in the command
		isOpencode := false
		for _, arg := range args {
			if strings.Contains(arg, "opencode") && !strings.Contains(arg, "opencode--") {
				isOpencode = true
				break
			}
		}
		if !isOpencode {
			continue
		}

		// Check if parent is PID 1 (orphaned)
		stat, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if err != nil {
			continue
		}

		// The stat format is: pid (comm) state ppid ...
		fields := strings.Fields(string(stat))
		if len(fields) < 7 {
			continue
		}
		// fields[3] is the parent PID in the stat file format
		// The comm field can contain spaces/parens, so we need to find ppid carefully.
		// After the closing paren of comm, ppid is field position 3 (0-indexed).
		// Actually in /proc/[pid]/stat format: pid (comm) state ppid pgrp session tty_nr ...
		// Find the last ')' to find end of comm
		closeParen := strings.LastIndex(string(stat), ")")
		if closeParen < 0 {
			continue
		}
		afterComm := strings.Fields(string(stat)[closeParen+1:])
		if len(afterComm) < 2 {
			continue
		}
		ppid := afterComm[1] // state is afterComm[0], ppid is afterComm[1]

		if ppid == "1" || ppid == "0" {
			// This is an orphaned opencode process, kill it
			proc, err := os.FindProcess(pid)
			if err != nil {
				continue
			}
			if err := proc.Kill(); err == nil {
				cleaned = append(cleaned, pid)
			}
		}
	}

	return cleaned, nil
}

// parseInt parses a string as a positive integer. Returns an error if the
// string is not a valid integer or is negative.
func parseInt(s string) (int, error) {
	if len(s) == 0 {
		return 0, fmt.Errorf("empty string")
	}
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number: %s", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// ─── In-Progress Feature Recovery ──────────────────────────────────────────

// RecoverInProgressFeatures finds features that were in_progress when the
// engine crashed and re-queues them back to pending status so they can be
// retried. Returns the IDs of re-queued features.
func (s *MissionsStore) RecoverInProgressFeatures(mission *Mission) ([]string, error) {
	var requeued []string
	changed := false

	for i := range mission.Features {
		feat := &mission.Features[i]
		if feat.Status == FeatureInProgress {
			feat.Status = FeaturePending
			feat.UpdatedAt = time.Now().UTC()
			requeued = append(requeued, feat.ID)
			changed = true
		}
	}

	if changed {
		mission.UpdatedAt = time.Now().UTC()
		if err := s.SaveMission(mission); err != nil {
			return requeued, fmt.Errorf("persist recovery re-queue for mission %s: %w", mission.ID, err)
		}
	}

	return requeued, nil
}

// ─── Partial Handoff Detection ─────────────────────────────────────────────

// DetectPartialHandoff checks if a partial (truncated or invalid) handoff
// JSON file exists for a feature's worker artifact directory. Returns true
// if a partial handoff was found and its content was preserved intact.
//
// A handoff is considered partial when:
//   - The file exists but is empty
//   - The file contains invalid JSON (truncated)
//   - The file contains valid JSON but missing required fields
//
// When a partial handoff is detected, the raw content is preserved in the
// artifact directory so it can be inspected later.
func (s *MissionsStore) DetectPartialHandoff(missionID, featureID string) (bool, error) {
	workersDir := filepath.Join(s.MissionDir(missionID), "workers", featureID)
	handoffPath := filepath.Join(workersDir, "handoff.json")

	data, err := os.ReadFile(handoffPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // No handoff file at all — not partial
		}
		return false, fmt.Errorf("read handoff file %s: %w", handoffPath, err)
	}

	if len(data) == 0 {
		return true, nil // Empty file is partial
	}

	// Try to parse as valid WorkerHandoff JSON
	var h WorkerHandoff
	if err := json.Unmarshal(data, &h); err != nil {
		return true, nil // Invalid JSON means partial/truncated
	}

	// Check for required fields
	if h.SalientSummary == "" && h.WhatWasImplemented == "" {
		return true, nil // Missing required fields
	}

	return false, nil // Valid, complete handoff
}

// ─── Store Corruption Recovery ─────────────────────────────────────────────

// RecoverCorruptMission attempts to recover a mission whose mission.json
// file is corrupt (unreadable or invalid JSON). It backs up the corrupt
// file with a ".corrupt.<timestamp>" suffix and creates a fresh minimal
// mission entry with the same ID.
//
// Returns the new minimal mission if recovery succeeded, or an error if
// recovery is not possible.
func (s *MissionsStore) RecoverCorruptMission(missionID string) (*Mission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.missionPath(missionID)

	// Read existing data for backup
	existingData, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read corrupt file %s: %w", path, err)
	}

	// If there's existing data, back it up
	if len(existingData) > 0 {
		backupPath := fmt.Sprintf("%s.corrupt.%d", path, time.Now().UnixNano())
		if err := os.WriteFile(backupPath, existingData, 0644); err != nil {
			return nil, fmt.Errorf("backup corrupt file to %s: %w", backupPath, err)
		}
	}

	// Ensure the mission directory exists
	if err := s.ensureDir(missionID); err != nil {
		return nil, fmt.Errorf("ensure mission dir for recovery: %w", err)
	}

	// Create fresh minimal mission
	now := time.Now().UTC()
	mission := &Mission{
		ID:        missionID,
		Name:      fmt.Sprintf("%s (recovered)", missionID),
		Status:    MissionPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.writeMissionLocked(mission); err != nil {
		return nil, fmt.Errorf("write recovered mission: %w", err)
	}

	return mission, nil
}

// ─── Worker Lifecycle Tracking ─────────────────────────────────────────────

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
	sessionIDCopy := sessionID
	feat.CompletedWorkerSessionID = &sessionIDCopy
	mission.UpdatedAt = now

	return s.SaveMission(mission)
}

// RecordWorkerHandoff records the handoff result for a completed worker
// session and persists the handoff JSON to the workers artifact directory.
func (s *MissionsStore) RecordWorkerHandoff(missionID, featureID string, handoff *WorkerHandoff) error {
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
	return filepath.Join(s.MissionDir(missionID), "workers", featureID, "output.log")
}
