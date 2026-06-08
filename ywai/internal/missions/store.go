package missions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ─── Errors ────────────────────────────────────────────────────────────────

var (
	ErrMissionNotFound   = errors.New("mission not found")
	ErrFeatureNotFound   = errors.New("feature not found")
	ErrCorruptFile       = errors.New("corrupt mission file")
	ErrInvalidMission    = errors.New("invalid mission data")
	ErrInvalidPlan       = errors.New("invalid plan")
	ErrDuplicateFeature  = errors.New("duplicate feature id")
)

// MissionsStore provides thread-safe JSON persistence for mission data.
// Each mission lives in its own subdirectory under the base data directory.
type MissionsStore struct {
	mu       sync.RWMutex
	baseDir  string
}

// NewMissionsStore creates a new store rooted at baseDir (e.g.
// ~/.local/share/ywai/missions/). The directory is created on first write.
func NewMissionsStore(baseDir string) *MissionsStore {
	return &MissionsStore{
		baseDir: baseDir,
	}
}

// ─── Base Directory ────────────────────────────────────────────────────────

// BaseDir returns the root directory for all mission data.
func (s *MissionsStore) BaseDir() string {
	return s.baseDir
}

// MissionDir returns the per-mission directory.
func (s *MissionsStore) MissionDir(missionID string) string {
	return filepath.Join(s.baseDir, missionID)
}

// ensureDir creates the mission directory if it doesn't exist.
// Caller must NOT hold the lock.
func (s *MissionsStore) ensureDir(missionID string) error {
	dir := s.MissionDir(missionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create mission dir: %w", err)
	}
	// Create subdirectories for plan and workers
	for _, sub := range []string{"plan", "workers"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			return fmt.Errorf("create %s dir: %w", sub, err)
		}
	}
	return nil
}

// ensureBaseDir creates the base missions directory if it doesn't exist.
// Caller must NOT hold the lock.
func (s *MissionsStore) ensureBaseDir() error {
	return os.MkdirAll(s.baseDir, 0755)
}

// ─── Mission CRUD ──────────────────────────────────────────────────────────

// CreateMission persists a new mission. If the mission has features, duplicate
// IDs are rejected.
func (s *MissionsStore) CreateMission(m *Mission) error {
	if m == nil {
		return ErrInvalidMission
	}

	// Validate duplicate feature IDs
	seen := make(map[string]bool, len(m.Features))
	for _, f := range m.Features {
		if f.ID == "" {
			return fmt.Errorf("%w: feature with empty id", ErrInvalidMission)
		}
		if seen[f.ID] {
			return fmt.Errorf("%w: %q", ErrDuplicateFeature, f.ID)
		}
		seen[f.ID] = true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureDir(m.ID); err != nil {
		return err
	}

	return s.writeMissionLocked(m)
}

// LoadMission reads a mission from its directory.
func (s *MissionsStore) LoadMission(missionID string) (*Mission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.readMissionLocked(missionID)
}

// SaveMission persists an updated mission. If the mission has features,
// duplicate IDs are rejected.
func (s *MissionsStore) SaveMission(m *Mission) error {
	if m == nil {
		return ErrInvalidMission
	}

	// Validate duplicate feature IDs
	seen := make(map[string]bool, len(m.Features))
	for _, f := range m.Features {
		if f.ID == "" {
			return fmt.Errorf("%w: feature with empty id", ErrInvalidMission)
		}
		if seen[f.ID] {
			return fmt.Errorf("%w: %q", ErrDuplicateFeature, f.ID)
		}
		seen[f.ID] = true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureDir(m.ID); err != nil {
		return err
	}

	return s.writeMissionLocked(m)
}

// ListMissions returns all missions sorted by creation time (newest first).
func (s *MissionsStore) ListMissions() ([]*Mission, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // fresh state
		}
		return nil, fmt.Errorf("list missions: %w", err)
	}

	var missions []*Mission
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m, err := s.readMissionLocked(entry.Name())
		if err != nil {
			// Skip corrupt missions silently, or log? Let's skip for robustness.
			continue
		}
		missions = append(missions, m)
	}

	// Sort newest first
	sort.Slice(missions, func(i, j int) bool {
		return missions[i].CreatedAt.After(missions[j].CreatedAt)
	})

	return missions, nil
}

// DeleteMission removes a mission and all its data.
func (s *MissionsStore) DeleteMission(missionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.MissionDir(missionID)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("delete mission: %w", err)
	}
	return nil
}

// MissionExists checks if a mission directory exists.
func (s *MissionsStore) MissionExists(missionID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := s.MissionDir(missionID)
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat mission dir: %w", err)
	}
	return info.IsDir(), nil
}

// ─── Feature Operations ────────────────────────────────────────────────────

// UpdateFeatureStatus updates a single feature's status inside a mission.
// It loads the mission, modifies the feature in memory, and saves atomically.
func (s *MissionsStore) UpdateFeatureStatus(missionID, featureID string, status FeatureStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, err := s.readMissionLocked(missionID)
	if err != nil {
		return err
	}

	now := time.Now()
	found := false
	for i := range m.Features {
		if m.Features[i].ID == featureID {
			m.Features[i].Status = status
			m.Features[i].UpdatedAt = now
			if status == FeatureCompleted || status == FeatureFailed || status == FeatureCancelled {
				m.Features[i].CompletedAt = &now
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%w: %q", ErrFeatureNotFound, featureID)
	}

	m.UpdatedAt = now
	return s.writeMissionLocked(m)
}

// GetFeature returns a specific feature from a mission.
func (s *MissionsStore) GetFeature(missionID, featureID string) (*Feature, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, err := s.readMissionLocked(missionID)
	if err != nil {
		return nil, err
	}

	for i := range m.Features {
		if m.Features[i].ID == featureID {
			return m.Features[i].Clone(), nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrFeatureNotFound, featureID)
}

// ─── Validation State ──────────────────────────────────────────────────────

// SaveValidationState persists validation results for a mission.
func (s *MissionsStore) SaveValidationState(missionID string, vs *ValidationState) error {
	if vs == nil {
		return fmt.Errorf("validation state is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureDir(missionID); err != nil {
		return err
	}

	pretty, err := json.MarshalIndent(vs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal validation state: %w", err)
	}

	path := filepath.Join(s.MissionDir(missionID), "validation-state.json")
	return atomicWrite(path, pretty)
}

// LoadValidationState reads validation results for a mission.
// If the file doesn't exist yet, returns an empty validation state (no error).
func (s *MissionsStore) LoadValidationState(missionID string) (*ValidationState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.MissionDir(missionID), "validation-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ValidationState{
				Assertions: []ValidationAssertion{},
				UpdatedAt:  time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("%w: read validation-state.json: %v", ErrCorruptFile, err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty validation-state.json", ErrCorruptFile)
	}

	var vs ValidationState
	if err := json.Unmarshal(data, &vs); err != nil {
		return nil, fmt.Errorf("%w: parse validation-state.json: %v", ErrCorruptFile, err)
	}

	return &vs, nil
}

// ─── Internal I/O ──────────────────────────────────────────────────────────

// missionPath returns the path to mission.json for a given mission ID.
// Caller must ensure the mission directory exists.
func (s *MissionsStore) missionPath(missionID string) string {
	return filepath.Join(s.MissionDir(missionID), "mission.json")
}

// readMissionLocked reads mission.json from disk. Caller must hold at least
// an RLock.
func (s *MissionsStore) readMissionLocked(missionID string) (*Mission, error) {
	path := s.missionPath(missionID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %q", ErrMissionNotFound, missionID)
		}
		return nil, fmt.Errorf("%w: read %s: %v", ErrCorruptFile, path, err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty mission.json in %q", ErrCorruptFile, missionID)
	}

	var m Mission
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%w: parse mission.json in %q: %v", ErrCorruptFile, missionID, err)
	}

	return &m, nil
}

// writeMissionLocked serialises the mission to mission.json using atomic write.
// Caller must hold a write lock.
func (s *MissionsStore) writeMissionLocked(m *Mission) error {
	pretty, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mission: %w", err)
	}

	path := s.missionPath(m.ID)
	return atomicWrite(path, pretty)
}

// ─── Atomic Write ──────────────────────────────────────────────────────────

// atomicWrite writes data to a temp file then renames it atomically,
// ensuring crash-safety: a crash mid-write leaves the original intact.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmpFile := filepath.Join(dir, "."+filepath.Base(path)+".tmp")

	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	// Rename is atomic on the same filesystem (POSIX guarantee).
	if err := os.Rename(tmpFile, path); err != nil {
		// Try to clean up the temp file on error
		os.Remove(tmpFile)
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
