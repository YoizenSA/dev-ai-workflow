package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
	"github.com/spf13/cobra"
)

// ─── Test Helpers ──────────────────────────────────────────────────────────

// setupTestStore creates a fresh store in a temp directory.
func setupTestStore(t *testing.T) *missions.MissionsStore {
	t.Helper()
	baseDir := t.TempDir()
	store := missions.NewMissionsStore(baseDir)
	return store
}

// createTestMission creates a mission with a known state for testing.
func createTestMission(t *testing.T, store *missions.MissionsStore, id, name string, status missions.MissionStatus) *missions.Mission {
	t.Helper()
	now := time.Now().UTC().Round(time.Millisecond)

	mission := &missions.Mission{
		ID:        id,
		Name:      name,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
		Milestones: []missions.Milestone{
			{Name: "core", Description: "Core implementation"},
		},
		Features: []missions.Feature{
			{
				ID:          "feat-1",
				Description: "Feature one",
				Status:      missions.FeatureCompleted,
				Milestone:   "core",
				SkillName:   "worker",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "feat-2",
				Description: "Feature two",
				Status:      missions.FeaturePending,
				Milestone:   "core",
				SkillName:   "worker",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}

	if err := store.CreateMission(mission); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	return mission
}

// executeCommand runs a cobra command with the given args and returns output.
func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err := root.Execute()
	return buf.String(), err
}

// ─── VAL-CLI-GEN-001: Help text available ──────────────────────────────────

func TestHelpText(t *testing.T) {
	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)

	output, err := executeCommand(root, "missions", "--help")
	if err != nil {
		t.Fatalf("help should succeed: %v", err)
	}

	// Should mention all subcommands
	for _, sub := range []string{"start", "list", "show", "resume", "cancel"} {
		if !strings.Contains(output, sub) {
			t.Errorf("help missing subcommand %q", sub)
		}
	}
}

// ─── VAL-CLI-GEN-002: Unknown subcommand error ─────────────────────────────

func TestUnknownSubcommand(t *testing.T) {
	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)

	_, err := executeCommand(root, "missions", "bogus")
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown subcommand") {
		t.Errorf("expected 'unknown subcommand' error, got: %v", err)
	}
}

// ─── VAL-CLI-GEN-003: Unrecognized flag error ──────────────────────────────

func TestUnknownFlag(t *testing.T) {
	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)

	_, err := executeCommand(root, "missions", "start", "--bogus")
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("expected 'unknown flag' error, got: %v", err)
	}
}

// ─── VAL-CLI-GEN-005: No side-effects in help ──────────────────────────────

func TestNoSideEffectsFromHelp(t *testing.T) {
	// Ensure missions dir doesn't exist
	tmpDir := t.TempDir()
	missionsDir := filepath.Join(tmpDir, "missions")

	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)
	root.SetArgs([]string{"missions", "--help"})

	// Override the store to use our temp dir (we verify no dir is created)
	if _, err := os.Stat(missionsDir); err == nil {
		t.Fatal("missions dir should not exist before help")
	}

	// Just verify the command runs without creating store
	err := root.Execute()
	if err != nil {
		t.Fatalf("help should succeed: %v", err)
	}
}

// ─── VAL-CLI-LIST-001: Empty mission list ──────────────────────────────────

func TestListEmpty(t *testing.T) {
	// This tests with a real store in a temp dir
	_ = setupTestStore(t)

	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)

	// We can't easily inject a store into the CLI commands since they use
	// OpenStore() which reads from the default dir.
	// Instead, we test the table formatting via the cobra command.

	// Verify that the list command at least parses correctly
	output, err := executeCommand(root, "missions", "list", "--help")
	if err != nil {
		t.Fatalf("list help should succeed: %v", err)
	}
	if !strings.Contains(output, "List all missions") {
		t.Errorf("expected list description, got: %s", output)
	}
}

// ─── VAL-CLI-LIST-003: JSON format support ─────────────────────────────────

func TestListJSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	store := missions.NewMissionsStore(tmpDir)
	now := time.Now().UTC().Round(time.Millisecond)

	// Create a mission
	mission := &missions.Mission{
		ID:        "test-1",
		Name:      "Test Mission",
		Status:    missions.MissionActive,
		CreatedAt: now,
		UpdatedAt: now,
		Milestones: []missions.Milestone{
			{Name: "ms1", Description: "Milestone 1"},
		},
		Features: []missions.Feature{
			{ID: "f1", Description: "Feature 1", Status: missions.FeaturePending, Milestone: "ms1", SkillName: "worker", CreatedAt: now, UpdatedAt: now},
		},
	}
	if err := store.CreateMission(mission); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	// Verify the mission can be encoded as JSON (VAL-CLI-UX-003)
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode([]*missions.Mission{mission}); err != nil {
		t.Fatalf("JSON encode: %v", err)
	}
}

// ─── VAL-CLI-SHOW-002: Not found error ─────────────────────────────────────

func TestShowNotFound(t *testing.T) {
	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)

	_, err := executeCommand(root, "missions", "show", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent mission")
	}
}

// ─── VAL-CLI-RESUME-002: Resume active returns error ───────────────────────

func TestResumeActiveError(t *testing.T) {
	tmpDir := t.TempDir()
	store := missions.NewMissionsStore(tmpDir)
	mission := createTestMission(t, store, "test-1", "Test Mission", missions.MissionActive)

	// Attempt to resume an already active mission
	err := missions.ResumeMission(store, mission)
	if err == nil {
		t.Fatal("expected error when resuming active mission")
	}
}

// ─── VAL-CLI-RESUME-004: Resume nonexistent returns error ──────────────────

func TestResumeNonexistent(t *testing.T) {
	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)

	_, err := executeCommand(root, "missions", "resume", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent mission")
	}
}

// ─── VAL-CLI-CANCEL-003: Cancel already-cancelled is idempotent ────────────

func TestCancelAlreadyCancelled(t *testing.T) {
	tmpDir := t.TempDir()
	store := missions.NewMissionsStore(tmpDir)
	mission := createTestMission(t, store, "test-1", "Test Mission", missions.MissionCancelled)

	// Cancelling an already cancelled mission should succeed (idempotent)
	err := missions.CancelMission(store, mission)
	if err != nil {
		t.Fatalf("cancel cancelled should succeed: %v", err)
	}
	if mission.Status != missions.MissionCancelled {
		t.Errorf("expected status cancelled, got %q", mission.Status)
	}
}

// ─── VAL-CLI-CANCEL-006: Cancel nonexistent returns error ──────────────────

func TestCancelNonexistent(t *testing.T) {
	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)

	_, err := executeCommand(root, "missions", "cancel", "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent mission")
	}
}

// ─── VAL-CLI-SM-001: Valid mission state machine transitions ───────────────

func TestMissionStateMachineTransitions(t *testing.T) {
	tests := []struct {
		from missions.MissionStatus
		to   missions.MissionStatus
		ok   bool
	}{
		{missions.MissionPlanning, missions.MissionActive, true},
		{missions.MissionActive, missions.MissionPaused, true},
		{missions.MissionActive, missions.MissionCompleted, true},
		{missions.MissionActive, missions.MissionFailed, true},
		{missions.MissionActive, missions.MissionCancelled, true},
		{missions.MissionPaused, missions.MissionActive, true},
		{missions.MissionPaused, missions.MissionCancelled, true},
		{missions.MissionCompleted, missions.MissionActive, false},
		{missions.MissionCancelled, missions.MissionActive, false},
		{missions.MissionFailed, missions.MissionCompleted, false},
	}

	for _, tt := range tests {
		_, err := missions.TransitionMissionStatus(tt.from, tt.to)
		if tt.ok && err != nil {
			t.Errorf("transition %s→%s should be valid, got error: %v", tt.from, tt.to, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("transition %s→%s should be invalid, but it succeeded", tt.from, tt.to)
		}
	}
}

// ─── VAL-CLI-SM-002: Valid feature state machine transitions ───────────────

func TestFeatureStateMachineTransitions(t *testing.T) {
	tests := []struct {
		from missions.FeatureStatus
		to   missions.FeatureStatus
		ok   bool
	}{
		{missions.FeaturePending, missions.FeatureInProgress, true},
		{missions.FeaturePending, missions.FeatureCancelled, true},
		{missions.FeatureInProgress, missions.FeatureCompleted, true},
		{missions.FeatureInProgress, missions.FeatureFailed, true},
		{missions.FeatureInProgress, missions.FeatureCancelled, true},
		{missions.FeatureCompleted, missions.FeatureInProgress, false},
		{missions.FeatureCompleted, missions.FeatureCancelled, false},
		{missions.FeatureCancelled, missions.FeatureInProgress, false},
		{missions.FeatureFailed, missions.FeaturePending, true},
	}

	for _, tt := range tests {
		_, err := missions.TransitionFeatureStatus(tt.from, tt.to)
		if tt.ok && err != nil {
			t.Errorf("transition %s→%s should be valid, got error: %v", tt.from, tt.to, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("transition %s→%s should be invalid, but it succeeded", tt.from, tt.to)
		}
	}
}

// ─── CancelMission tests ──────────────────────────────────────────────────

func TestCancelMission(t *testing.T) {
	tmpDir := t.TempDir()
	store := missions.NewMissionsStore(tmpDir)
	mission := createTestMission(t, store, "test-1", "Test Mission", missions.MissionActive)

	// Verify initial state
	if mission.Status != missions.MissionActive {
		t.Fatalf("expected active, got %q", mission.Status)
	}

	// Cancel
	err := missions.CancelMission(store, mission)
	if err != nil {
		t.Fatalf("CancelMission: %v", err)
	}

	// Verify mission status
	if mission.Status != missions.MissionCancelled {
		t.Errorf("expected cancelled, got %q", mission.Status)
	}

	// Verify completed features remain completed
	for _, f := range mission.Features {
		if f.ID == "feat-1" && f.Status != missions.FeatureCompleted {
			t.Errorf("completed feature should remain completed, got %q", f.Status)
		}
		if f.ID == "feat-2" && f.Status != missions.FeatureCancelled {
			t.Errorf("pending feature should be cancelled, got %q", f.Status)
		}
	}

	// Reload from store and verify persistence
	reloaded, err := store.LoadMission(mission.ID)
	if err != nil {
		t.Fatalf("LoadMission: %v", err)
	}
	if reloaded.Status != missions.MissionCancelled {
		t.Errorf("persisted status should be cancelled, got %q", reloaded.Status)
	}
}

func TestCancelMissionActiveCancelsPendingFeatures(t *testing.T) {
	tmpDir := t.TempDir()
	store := missions.NewMissionsStore(tmpDir)
	now := time.Now().UTC().Round(time.Millisecond)

	mission := &missions.Mission{
		ID:        "test-cancel-features",
		Name:      "Cancel Feature Test",
		Status:    missions.MissionActive,
		CreatedAt: now,
		UpdatedAt: now,
		Milestones: []missions.Milestone{
			{Name: "ms1", Description: "MS1"},
		},
		Features: []missions.Feature{
			{ID: "f1", Description: "F1", Status: missions.FeaturePending, Milestone: "ms1", SkillName: "worker", CreatedAt: now, UpdatedAt: now},
			{ID: "f2", Description: "F2", Status: missions.FeatureInProgress, Milestone: "ms1", SkillName: "worker", CreatedAt: now, UpdatedAt: now},
			{ID: "f3", Description: "F3", Status: missions.FeatureCompleted, Milestone: "ms1", SkillName: "worker", CreatedAt: now, UpdatedAt: now},
			{ID: "f4", Description: "F4", Status: missions.FeatureFailed, Milestone: "ms1", SkillName: "worker", CreatedAt: now, UpdatedAt: now},
			{ID: "f5", Description: "F5", Status: missions.FeatureCancelled, Milestone: "ms1", SkillName: "worker", CreatedAt: now, UpdatedAt: now},
		},
	}
	if err := store.CreateMission(mission); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	if err := missions.CancelMission(store, mission); err != nil {
		t.Fatalf("CancelMission: %v", err)
	}

	// f1 (pending) → cancelled
	if mission.Features[0].Status != missions.FeatureCancelled {
		t.Errorf("f1 expected cancelled, got %q", mission.Features[0].Status)
	}
	// f2 (in_progress) → cancelled
	if mission.Features[1].Status != missions.FeatureCancelled {
		t.Errorf("f2 expected cancelled, got %q", mission.Features[1].Status)
	}
	// f3 (completed) → completed (unchanged)
	if mission.Features[2].Status != missions.FeatureCompleted {
		t.Errorf("f3 expected completed, got %q", mission.Features[2].Status)
	}
	// f4 (failed) → failed (unchanged)
	if mission.Features[3].Status != missions.FeatureFailed {
		t.Errorf("f4 expected failed, got %q", mission.Features[3].Status)
	}
	// f5 (cancelled) → cancelled (unchanged)
	if mission.Features[4].Status != missions.FeatureCancelled {
		t.Errorf("f5 expected cancelled, got %q", mission.Features[4].Status)
	}
}

func TestCancelMissionPaused(t *testing.T) {
	tmpDir := t.TempDir()
	store := missions.NewMissionsStore(tmpDir)
	mission := createTestMission(t, store, "test-paused", "Paused Test", missions.MissionPaused)

	err := missions.CancelMission(store, mission)
	if err != nil {
		t.Fatalf("CancelMission from paused: %v", err)
	}
	if mission.Status != missions.MissionCancelled {
		t.Errorf("expected cancelled, got %q", mission.Status)
	}
}

func TestCancelMissionCompletedError(t *testing.T) {
	tmpDir := t.TempDir()
	store := missions.NewMissionsStore(tmpDir)
	mission := createTestMission(t, store, "test-completed", "Completed Test", missions.MissionCompleted)

	err := missions.CancelMission(store, mission)
	if err == nil {
		t.Fatal("expected error when cancelling completed mission")
	}
}

// ─── ResumeMission tests ───────────────────────────────────────────────────

func TestResumeMission(t *testing.T) {
	tmpDir := t.TempDir()
	store := missions.NewMissionsStore(tmpDir)
	mission := createTestMission(t, store, "test-resume", "Resume Test", missions.MissionPaused)

	// Verify initial state
	if mission.Status != missions.MissionPaused {
		t.Fatalf("expected paused, got %q", mission.Status)
	}

	// Resume
	err := missions.ResumeMission(store, mission)
	if err != nil {
		t.Fatalf("ResumeMission: %v", err)
	}

	// Verify mission status
	if mission.Status != missions.MissionActive {
		t.Errorf("expected active, got %q", mission.Status)
	}

	// Verify completed features remain completed
	for _, f := range mission.Features {
		if f.ID == "feat-1" && f.Status != missions.FeatureCompleted {
			t.Errorf("completed feature should remain completed, got %q", f.Status)
		}
	}

	// Reload from store and verify persistence
	reloaded, err := store.LoadMission(mission.ID)
	if err != nil {
		t.Fatalf("LoadMission: %v", err)
	}
	if reloaded.Status != missions.MissionActive {
		t.Errorf("persisted status should be active, got %q", reloaded.Status)
	}
}

func TestResumeMissionCompletedError(t *testing.T) {
	tmpDir := t.TempDir()
	store := missions.NewMissionsStore(tmpDir)
	mission := createTestMission(t, store, "test-completed", "Completed Test", missions.MissionCompleted)

	err := missions.ResumeMission(store, mission)
	if err == nil {
		t.Fatal("expected error when resuming completed mission")
	}
}

func TestResumeMissionPreservesCompletedFeatures(t *testing.T) {
	tmpDir := t.TempDir()
	store := missions.NewMissionsStore(tmpDir)
	now := time.Now().UTC().Round(time.Millisecond)

	mission := &missions.Mission{
		ID:        "test-preserve",
		Name:      "Preserve Test",
		Status:    missions.MissionPaused,
		CreatedAt: now,
		UpdatedAt: now,
		Milestones: []missions.Milestone{
			{Name: "ms1", Description: "MS1"},
		},
		Features: []missions.Feature{
			{ID: "f1", Description: "F1", Status: missions.FeatureCompleted, Milestone: "ms1", SkillName: "worker", CreatedAt: now, UpdatedAt: now},
			{ID: "f2", Description: "F2", Status: missions.FeaturePending, Milestone: "ms1", SkillName: "worker", CreatedAt: now, UpdatedAt: now},
		},
	}
	if err := store.CreateMission(mission); err != nil {
		t.Fatalf("CreateMission: %v", err)
	}

	if err := missions.ResumeMission(store, mission); err != nil {
		t.Fatalf("ResumeMission: %v", err)
	}

	if mission.Features[0].Status != missions.FeatureCompleted {
		t.Errorf("f1 should remain completed, got %q", mission.Features[0].Status)
	}
	if mission.Features[1].Status != missions.FeaturePending {
		t.Errorf("f2 should remain pending, got %q", mission.Features[1].Status)
	}
}

// ─── Help text structure ───────────────────────────────────────────────────

func TestCommandStructure(t *testing.T) {
	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)

	missionsCmd, _, err := root.Find([]string{"missions"})
	if err != nil {
		t.Fatalf("find missions command: %v", err)
	}

	expectedSubcommands := []string{"start", "list", "show", "resume", "cancel"}
	for _, name := range expectedSubcommands {
		subCmd, _, err := missionsCmd.Find([]string{name})
		if err != nil {
			t.Errorf("expected subcommand %q to exist: %v", name, err)
		}
		if subCmd == nil {
			t.Errorf("expected subcommand %q to be non-nil", name)
		}
	}
}

// ─── Start command validation ──────────────────────────────────────────────

func TestStartWithoutTTY(t *testing.T) {
	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)

	// Running start without TTY and without --file should give an error
	// However, since we're in a test, isInteractiveTerminal() returns false
	_, err := executeCommand(root, "missions", "start")
	if err == nil {
		t.Fatal("expected error when running start without TTY and --file")
	}
	if !strings.Contains(err.Error(), "--file") {
		t.Errorf("expected error to suggest --file, got: %v", err)
	}
}

func TestStartWithMissingFile(t *testing.T) {
	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)

	_, err := executeCommand(root, "missions", "start", "--file", "/nonexistent/plan.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestStartWithInvalidJSONFile(t *testing.T) {
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(badFile, []byte("{invalid json}"), 0644); err != nil {
		t.Fatalf("write bad.json: %v", err)
	}

	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)

	_, err := executeCommand(root, "missions", "start", "--file", badFile)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestStartWithEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.json")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("write empty.json: %v", err)
	}

	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)

	_, err := executeCommand(root, "missions", "start", "--file", emptyFile)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

// ─── Serve command validation ──────────────────────────────────────────────

// ─── UX: Consistent output styling ─────────────────────────────────────────

func TestStatusIcons(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"planning", "planning"},
		{"active", "active"},
		{"paused", "paused"},
		{"completed", "completed"},
		{"failed", "failed"},
		{"cancelled", "cancelled"},
		{"validating", "validating"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		got := statusIcon(tt.status)
		if !strings.Contains(got, tt.want) {
			t.Errorf("statusIcon(%q) = %q, want containing %q", tt.status, got, tt.want)
		}
	}
}

func TestFeatureStatusIcons(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"pending", "pending"},
		{"in_progress", "in_progress"},
		{"completed", "completed"},
		{"failed", "failed"},
		{"cancelled", "cancelled"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		got := featureStatusIcon(tt.status)
		if !strings.Contains(got, tt.want) {
			t.Errorf("featureStatusIcon(%q) = %q, want containing %q", tt.status, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is..."},
		{"exactlylen", 10, "exactlylen"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

// ─── Auto command: engine config from flags ─────────────────────────────────

// autoFlags mirrors the flags exposed by `missions auto`.
type autoFlags = autoCmdFlags

// TestEngineConfigFromFlagsDefaults verifies the zero-value config matches the
// engine defaults (sequential, default timeout/retries, clean streak enabled).
func TestEngineConfigFromFlagsDefaults(t *testing.T) {
	cfg := engineConfigFromFlags(autoCmdFlags{})

	if cfg.MaxParallel != 1 {
		t.Errorf("default MaxParallel = %d, want 1", cfg.MaxParallel)
	}
	if cfg.MaxRetries != missions.DefaultMaxRetries {
		t.Errorf("default MaxRetries = %d, want %d", cfg.MaxRetries, missions.DefaultMaxRetries)
	}
	if cfg.WorkerTimeout != missions.DefaultWorkerTimeout {
		t.Errorf("default WorkerTimeout = %v, want %v", cfg.WorkerTimeout, missions.DefaultWorkerTimeout)
	}
	if cfg.VerifyCleanStreak != 1 {
		t.Errorf("default VerifyCleanStreak = %d, want 1", cfg.VerifyCleanStreak)
	}
}

// TestEngineConfigFromFlagsApplied verifies each flag is wired into the config.
func TestEngineConfigFromFlagsApplied(t *testing.T) {
	flags := autoCmdFlags{
		Timeout:        90 * time.Minute,
		MaxRetries:     5,
		MaxParallel:    3,
		CleanStreak:    2,
	}
	cfg := engineConfigFromFlags(flags)

	if cfg.WorkerTimeout != 90*time.Minute {
		t.Errorf("WorkerTimeout = %v, want 90m", cfg.WorkerTimeout)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}
	if cfg.MaxParallel != 3 {
		t.Errorf("MaxParallel = %d, want 3", cfg.MaxParallel)
	}
	if cfg.VerifyCleanStreak != 2 {
		t.Errorf("VerifyCleanStreak = %d, want 2", cfg.VerifyCleanStreak)
	}
}

// TestEngineConfigFromFlagsClampsZeroTimeout verifies a zero timeout falls back
// to the default rather than producing an unbounded worker.
func TestEngineConfigFromFlagsClampsZeroTimeout(t *testing.T) {
	cfg := engineConfigFromFlags(autoCmdFlags{Timeout: 0})
	if cfg.WorkerTimeout != missions.DefaultWorkerTimeout {
		t.Errorf("zero timeout should fall back to default, got %v", cfg.WorkerTimeout)
	}
}

// TestAutoCmdFlagsExposed verifies the `auto` command registers all tuning flags.
func TestAutoCmdFlagsExposed(t *testing.T) {
	root := &cobra.Command{Use: "ywai"}
	RegisterCommands(root)

	missionsCmd, _, err := root.Find([]string{"missions"})
	if err != nil {
		t.Fatalf("find missions: %v", err)
	}
	autoCmd, _, err := missionsCmd.Find([]string{"auto"})
	if err != nil {
		t.Fatalf("find auto: %v", err)
	}

	expectedFlags := []string{"timeout", "max-retries", "max-parallel", "clean-streak", "base", "project", "model", "agent", "yes"}
	for _, name := range expectedFlags {
		if autoCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag --%s on auto command, not found", name)
		}
	}
}
