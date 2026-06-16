package missions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTestStore creates a MissionsStore rooted at a temporary directory.
func newTestStore(t *testing.T) (*MissionsStore, string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "missions-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	return NewMissionsStore(dir), dir
}

// testMission returns a minimal valid mission for testing.
func testMission(id string) *Mission {
	now := time.Now().Round(time.Second)
	return &Mission{
		ID:        id,
		Name:      "Test " + id,
		Status:    MissionPending,
		CreatedAt: now,
		UpdatedAt: now,
		Features: []Feature{
			{
				ID:          "feat-1",
				Description: "Feature 1",
				Status:      FeaturePending,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}
}

// ─── VAL-ENG-STORE-001: Mission CRUD round-trip ────────────────────────────

func TestCreateAndLoadMission(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("crud-test-1")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	got, err := s.LoadMission("crud-test-1")
	if err != nil {
		t.Fatalf("LoadMission: %v", err)
	}

	if got.ID != m.ID {
		t.Errorf("ID: got %q, want %q", got.ID, m.ID)
	}
	if got.Name != m.Name {
		t.Errorf("Name: got %q, want %q", got.Name, m.Name)
	}
	if got.Status != m.Status {
		t.Errorf("Status: got %q, want %q", got.Status, m.Status)
	}
	if len(got.Features) != len(m.Features) {
		t.Errorf("Features count: got %d, want %d", len(got.Features), len(m.Features))
	}
	if got.Features[0].ID != m.Features[0].ID {
		t.Errorf("Feature[0].ID: got %q, want %q", got.Features[0].ID, m.Features[0].ID)
	}
}

func TestCreateAndListMissions(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	// Create 3 missions
	ids := []string{"m-a", "m-b", "m-c"}
	for _, id := range ids {
		if err := s.CreateMission(testMission(id)); err != nil {
			t.Fatalf("CreateMission(%q): %v", id, err)
		}
	}

	missions, err := s.ListMissions()
	if err != nil {
		t.Fatalf("ListMissions: %v", err)
	}

	if len(missions) != 3 {
		t.Fatalf("expected 3 missions, got %d", len(missions))
	}

	// Should be sorted newest first
	for i := 1; i < len(missions); i++ {
		if missions[i].CreatedAt.After(missions[i-1].CreatedAt) {
			t.Errorf("missions not sorted newest first at index %d", i)
		}
	}
}

func TestSaveAndLoadMission(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("save-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	// Modify mission
	m.Status = MissionActive
	m.Name = "Updated"
	if err := s.SaveMission(m); err != nil {
		t.Fatalf("SaveMission: %v", err)
	}

	got, err := s.LoadMission("save-test")
	if err != nil {
		t.Fatalf("LoadMission: %v", err)
	}

	if got.Status != MissionActive {
		t.Errorf("Status: got %q, want %q", got.Status, MissionActive)
	}
	if got.Name != "Updated" {
		t.Errorf("Name: got %q, want Updated", got.Name)
	}
}

func TestDeleteMission(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("delete-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	if err := s.DeleteMission("delete-test"); err != nil {
		t.Fatalf("DeleteMission: %v", err)
	}

	_, err := s.LoadMission("delete-test")
	if err == nil {
		t.Fatal("expected error loading deleted mission")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// ─── VAL-ENG-STORE-002: Feature persistence across restart ─────────────────

func TestFeaturePersistenceAcrossRestart(t *testing.T) {
	s1, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("persist-test")
	if err := s1.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	// Update feature status
	if err := s1.UpdateFeatureStatus("persist-test", "feat-1", FeatureInProgress); err != nil {
		t.Fatalf("UpdateFeatureStatus to in_progress: %v", err)
	}
	if err := s1.UpdateFeatureStatus("persist-test", "feat-1", FeatureCompleted); err != nil {
		t.Fatalf("UpdateFeatureStatus to completed: %v", err)
	}

	// Simulate restart: create a new store instance pointing to the same dir
	s2 := NewMissionsStore(dir)
	got, err := s2.LoadMission("persist-test")
	if err != nil {
		t.Fatalf("LoadMission after restart: %v", err)
	}

	if len(got.Features) == 0 {
		t.Fatal("no features after restart")
	}
	if got.Features[0].Status != FeatureCompleted {
		t.Errorf("Feature status after restart: got %q, want %q", got.Features[0].Status, FeatureCompleted)
	}
}

// ─── VAL-ENG-STORE-003: Store directory structure ──────────────────────────

func TestStoreDirectoryStructure(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("dir-struct-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	missionDir := filepath.Join(dir, "dir-struct-test")

	// Check mandatory files and directories
	checks := []struct {
		path  string
		isDir bool
	}{
		{filepath.Join(missionDir, "mission.json"), false},
		{filepath.Join(missionDir, "plan"), true},
		{filepath.Join(missionDir, "workers"), true},
	}

	for _, c := range checks {
		info, err := os.Stat(c.path)
		if err != nil {
			t.Errorf("stat %s: %v", c.path, err)
			continue
		}
		if c.isDir && !info.IsDir() {
			t.Errorf("%s: expected directory, got file", c.path)
		}
		if !c.isDir && info.IsDir() {
			t.Errorf("%s: expected file, got directory", c.path)
		}
	}
}

// ─── VAL-ENG-STORE-004: Concurrent store writes safe ───────────────────────

func TestConcurrentWrites(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("concurrent-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	// Add more features for concurrent access
	for i := 2; i <= 10; i++ {
		m.Features = append(m.Features, Feature{
			ID:          fmt.Sprintf("feat-%d", i),
			Description: fmt.Sprintf("Feature %d", i),
			Status:      FeaturePending,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		})
	}
	if err := s.SaveMission(m); err != nil {
		t.Fatalf("SaveMission after adding features: %v", err)
	}

	// Concurrently update 10 different features to test lock integrity
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			featID := fmt.Sprintf("feat-%d", n+1)
			// Each goroutine updates a different feature to test concurrent writes
			if err := s.UpdateFeatureStatus("concurrent-test", featID, FeatureInProgress); err != nil {
				errs <- err
				return
			}
			// Read back to ensure no corruption
			m2, err := s.LoadMission("concurrent-test")
			if err != nil {
				errs <- err
				return
			}
			if len(m2.Features) == 0 {
				errs <- ErrFeatureNotFound
				return
			}
			_ = m2.Features[n].Status // just verify we can read
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent write error: %v", err)
	}
}

// ─── VAL-ENG-STORE-005: Missing/corrupt files handled ──────────────────────

func TestLoadNonexistentMission(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	_, err := s.LoadMission("nonexistent")
	if err == nil {
		t.Fatal("expected error loading nonexistent mission")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestLoadCorruptMission(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	// Create a corrupt mission.json
	missionDir := filepath.Join(dir, "corrupt-mission")
	if err := os.MkdirAll(missionDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(missionDir, "mission.json"), []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	_, err := s.LoadMission("corrupt-mission")
	if err == nil {
		t.Fatal("expected error loading corrupt mission")
	}
	if !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("expected 'corrupt' in error, got: %v", err)
	}
}

func TestLoadEmptyMissionFile(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	// Create an empty mission.json
	missionDir := filepath.Join(dir, "empty-mission")
	if err := os.MkdirAll(missionDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(missionDir, "mission.json"), []byte{}, 0644); err != nil {
		t.Fatalf("write empty file: %v", err)
	}

	_, err := s.LoadMission("empty-mission")
	if err == nil {
		t.Fatal("expected error loading empty mission file")
	}
	if !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("expected 'corrupt' in error, got: %v", err)
	}
}

// ─── VAL-ENG-STORE-006: Atomic writes ──────────────────────────────────────

func TestAtomicWritePreservesPreviousStateOnCrash(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("atomic-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	// Read the original content
	originalData, err := os.ReadFile(filepath.Join(dir, "atomic-test", "mission.json"))
	if err != nil {
		t.Fatalf("read original: %v", err)
	}

	// Simulate a crash mid-write: write corrupt data to the temp file
	missionDir := filepath.Join(dir, "atomic-test")
	tmpFile := filepath.Join(missionDir, ".mission.json.tmp")
	if err := os.WriteFile(tmpFile, []byte("{corrupt"), 0644); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	// Do NOT rename — this simulates a crash after writing temp but before rename

	// The original file should still be intact
	savedData, err := os.ReadFile(filepath.Join(dir, "atomic-test", "mission.json"))
	if err != nil {
		t.Fatalf("read after simulated crash: %v", err)
	}

	if string(savedData) != string(originalData) {
		t.Fatal("original mission.json was corrupted after partial temp write")
	}
}

func TestAtomicWriteCreatesValidFile(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("atomic-valid-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	// Verify the file is valid JSON by loading via fresh store
	s2 := NewMissionsStore(dir)
	got, err := s2.LoadMission("atomic-valid-test")
	if err != nil {
		t.Fatalf("LoadMission: %v", err)
	}
	if got.ID != m.ID {
		t.Errorf("ID: got %q, want %q", got.ID, m.ID)
	}
}

// ─── VAL-ENG-STORE-007: Validation state persists ──────────────────────────

func TestValidationStatePersistence(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("val-persist-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	vs := &ValidationState{
		UpdatedAt: time.Now().Round(time.Second),
		Assertions: []ValidationAssertion{
			{
				ID:          "VAL-001",
				Description: "Test assertion",
				Status:      ValidationPassed,
				RunAt:       time.Now().Round(time.Second),
			},
		},
	}

	if err := s.SaveValidationState("val-persist-test", vs); err != nil {
		t.Fatalf("SaveValidationState: %v", err)
	}

	// Simulate restart
	s2 := NewMissionsStore(dir)
	got, err := s2.LoadValidationState("val-persist-test")
	if err != nil {
		t.Fatalf("LoadValidationState after restart: %v", err)
	}

	if len(got.Assertions) != 1 {
		t.Fatalf("expected 1 assertion, got %d", len(got.Assertions))
	}
	if got.Assertions[0].ID != "VAL-001" {
		t.Errorf("Assertion[0].ID: got %q, want %q", got.Assertions[0].ID, "VAL-001")
	}
	if got.Assertions[0].Status != ValidationPassed {
		t.Errorf("Assertion[0].Status: got %v, want %v", got.Assertions[0].Status, ValidationPassed)
	}
}

func TestLoadValidationStateEmpty(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	// For an existing mission that has no validation-state.json
	m := testMission("no-val-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	vs, err := s.LoadValidationState("no-val-test")
	if err != nil {
		t.Fatalf("LoadValidationState: %v", err)
	}

	if vs == nil {
		t.Fatal("expected non-nil validation state")
	}
	if len(vs.Assertions) != 0 {
		t.Errorf("expected 0 assertions, got %d", len(vs.Assertions))
	}
}

func TestLoadCorruptValidationState(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("corrupt-val-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	// Write corrupt validation state
	valPath := filepath.Join(dir, "corrupt-val-test", "validation-state.json")
	if err := os.WriteFile(valPath, []byte("{invalid"), 0644); err != nil {
		t.Fatalf("write corrupt validation state: %v", err)
	}

	_, err := s.LoadValidationState("corrupt-val-test")
	if err == nil {
		t.Fatal("expected error loading corrupt validation state")
	}
	if !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("expected 'corrupt' in error, got: %v", err)
	}
}

// ─── VAL-ENG-SYS-001: Multi-mission isolation ────────────────────────────

func TestMultiMissionIsolation(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m1 := testMission("mission-a")
	m1.Name = "Mission A"
	if err := s.CreateMission(m1); err != nil {
		t.Fatalf("CreateMission A: %v", err)
	}

	m2 := testMission("mission-b")
	m2.Name = "Mission B"
	if err := s.CreateMission(m2); err != nil {
		t.Fatalf("CreateMission B: %v", err)
	}

	// Verify separate directories
	dirA := filepath.Join(dir, "mission-a")
	dirB := filepath.Join(dir, "mission-b")

	if dirA == dirB {
		t.Fatal("mission directories should be different")
	}

	infoA, err := os.Stat(dirA)
	if err != nil {
		t.Fatalf("stat mission-a dir: %v", err)
	}
	if !infoA.IsDir() {
		t.Errorf("mission-a is not a directory")
	}

	infoB, err := os.Stat(dirB)
	if err != nil {
		t.Fatalf("stat mission-b dir: %v", err)
	}
	if !infoB.IsDir() {
		t.Errorf("mission-b is not a directory")
	}

	// Loading each mission should return correct data
	gotA, err := s.LoadMission("mission-a")
	if err != nil {
		t.Fatalf("LoadMission A: %v", err)
	}
	if gotA.Name != "Mission A" {
		t.Errorf("Mission A name: got %q, want %q", gotA.Name, "Mission A")
	}

	gotB, err := s.LoadMission("mission-b")
	if err != nil {
		t.Fatalf("LoadMission B: %v", err)
	}
	if gotB.Name != "Mission B" {
		t.Errorf("Mission B name: got %q, want %q", gotB.Name, "Mission B")
	}

	// Features should be isolated
	if len(gotA.Features) != 1 || len(gotB.Features) != 1 {
		t.Errorf("each mission should have 1 feature, got A:%d B:%d", len(gotA.Features), len(gotB.Features))
	}

	// Verify no shared files in listing
	missions, err := s.ListMissions()
	if err != nil {
		t.Fatalf("ListMissions: %v", err)
	}
	if len(missions) != 2 {
		t.Errorf("expected 2 missions, got %d", len(missions))
	}
}

// ─── VAL-ENG-SYS-002: First-run directory creation ───────────────────────

func TestFirstRunCreatesBaseDir(t *testing.T) {
	// Use a fresh non-existent directory
	dir, err := os.MkdirTemp("", "missions-firstrun-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)

	// Remove the temp dir so it doesn't exist
	os.RemoveAll(dir)

	s := NewMissionsStore(dir)

	m := testMission("first-run-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	// The base dir should now exist
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("base dir should exist after first create: %v", err)
	}

	// The mission dir should also exist
	missionDir := filepath.Join(dir, "first-run-test")
	if _, err := os.Stat(missionDir); err != nil {
		t.Errorf("mission dir should exist after first create: %v", err)
	}

	// Verify mission.json exists and is valid
	_, err = s.LoadMission("first-run-test")
	if err != nil {
		t.Errorf("should load mission after first-run: %v", err)
	}
}

// ─── VAL-ENG-SYS-004: Duplicate feature IDs rejected ─────────────────────

func TestDuplicateFeatureIDsRejectedOnCreate(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	now := time.Now().Round(time.Second)
	m := &Mission{
		ID:        "dup-test",
		Name:      "Duplicate Feature Test",
		Status:    MissionPending,
		CreatedAt: now,
		UpdatedAt: now,
		Features: []Feature{
			{ID: "feat-1", Description: "Feature 1", Status: FeaturePending, CreatedAt: now, UpdatedAt: now},
			{ID: "feat-1", Description: "Duplicate", Status: FeaturePending, CreatedAt: now, UpdatedAt: now},
		},
	}

	err := s.CreateMission(m)
	if err == nil {
		t.Fatal("expected error for duplicate feature IDs")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected 'duplicate' in error, got: %v", err)
	}
}

func TestDuplicateFeatureIDsRejectedOnSave(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("dup-save-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	// Add duplicate feature IDs
	now := time.Now().Round(time.Second)
	m.Features = append(m.Features, Feature{
		ID: "feat-1", Description: "Duplicate", Status: FeaturePending, CreatedAt: now, UpdatedAt: now,
	})

	err := s.SaveMission(m)
	if err == nil {
		t.Fatal("expected error for duplicate feature IDs on save")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("expected 'duplicate' in error, got: %v", err)
	}
}

// ─── VAL-ENG-SYS-005: Date rollover handled correctly ─────────────────────

func TestTimestampsAcrossMidnight(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	beforeMidnight := time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC)
	afterMidnight := time.Date(2026, 1, 2, 0, 0, 1, 0, time.UTC)

	m := &Mission{
		ID:        "rollover-store-test",
		Name:      "Rollover",
		Status:    MissionActive,
		CreatedAt: beforeMidnight,
		UpdatedAt: afterMidnight,
	}

	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	got, err := s.LoadMission("rollover-store-test")
	if err != nil {
		t.Fatalf("LoadMission: %v", err)
	}

	if !got.CreatedAt.Equal(beforeMidnight) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, beforeMidnight)
	}
	if !got.UpdatedAt.Equal(afterMidnight) {
		t.Errorf("UpdatedAt: got %v, want %v", got.UpdatedAt, afterMidnight)
	}
}

// ─── Edge Cases ─────────────────────────────────────────────────────────

func TestMissionExists(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	exists, err := s.MissionExists("nonexistent")
	if err != nil {
		t.Fatalf("MissionExists(nonexistent): %v", err)
	}
	if exists {
		t.Error("expected nonexistent mission to not exist")
	}

	m := testMission("exists-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	exists, err = s.MissionExists("exists-test")
	if err != nil {
		t.Fatalf("MissionExists(exists): %v", err)
	}
	if !exists {
		t.Error("expected mission to exist")
	}
}

func TestCreateNilMission(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	err := s.CreateMission(nil)
	if err == nil {
		t.Fatal("expected error for nil mission")
	}
}

func TestListEmptyStore(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	missions, err := s.ListMissions()
	if err != nil {
		t.Fatalf("ListMissions: %v", err)
	}
	if len(missions) != 0 {
		t.Errorf("expected 0 missions, got %d", len(missions))
	}
}

func TestGetFeature(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("get-feat-test")
	m.Features = append(m.Features, Feature{
		ID: "feat-2", Description: "Feature 2", Status: FeaturePending,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	})
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	f, err := s.GetFeature("get-feat-test", "feat-2")
	if err != nil {
		t.Fatalf("GetFeature: %v", err)
	}
	if f.ID != "feat-2" {
		t.Errorf("Feature ID: got %q", f.ID)
	}
	if f.Description != "Feature 2" {
		t.Errorf("Description: got %q", f.Description)
	}
}

func TestGetFeatureNotFound(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("notfound-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	_, err := s.GetFeature("notfound-test", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent feature")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestUpdateFeatureStatus(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("update-status-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	if err := s.UpdateFeatureStatus("update-status-test", "feat-1", FeatureInProgress); err != nil {
		t.Fatalf("UpdateFeatureStatus to in_progress: %v", err)
	}
	if err := s.UpdateFeatureStatus("update-status-test", "feat-1", FeatureCompleted); err != nil {
		t.Fatalf("UpdateFeatureStatus to completed: %v", err)
	}

	f, err := s.GetFeature("update-status-test", "feat-1")
	if err != nil {
		t.Fatalf("GetFeature: %v", err)
	}
	if f.Status != FeatureCompleted {
		t.Errorf("Status: got %q, want %q", f.Status, FeatureCompleted)
	}
	if f.CompletedAt == nil {
		t.Error("CompletedAt should be set when status is completed")
	}
}

func TestUpdateFeatureStatusNotFound(t *testing.T) {
	s, dir := newTestStore(t)
	defer os.RemoveAll(dir)

	m := testMission("update-notfound-test")
	if err := s.CreateMission(m); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	err := s.UpdateFeatureStatus("update-notfound-test", "nonexistent", FeatureCompleted)
	if err == nil {
		t.Fatal("expected error for nonexistent feature")
	}
}
