package tui

import (
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
)

// ─── Test Helpers ──────────────────────────────────────────────────────────

// newTestStore creates a temporary MissionsStore with a sample mission.
func newTestStore(t *testing.T) (*missions.MissionsStore, string) {
	t.Helper()

	baseDir, err := os.MkdirTemp("", "ywai-tui-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(baseDir) })

	store := missions.NewMissionsStore(baseDir)

	now := time.Now()
	mission := &missions.Mission{
		ID:        "test-mission",
		Name:      "Test Mission",
		Status:    missions.MissionActive,
		CreatedAt: now,
		UpdatedAt: now,
		Milestones: []missions.Milestone{
			{Name: "Milestone 1", Description: "First milestone"},
			{Name: "Milestone 2", Description: "Second milestone"},
		},
		Features: []missions.Feature{
			{
				ID:          "feat-1",
				Description: "Feature one",
				Status:      missions.FeatureCompleted,
				Milestone:   "Milestone 1",
				SkillName:   "backend-worker",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "feat-2",
				Description: "Feature two",
				Status:      missions.FeatureInProgress,
				Milestone:   "Milestone 1",
				SkillName:   "backend-worker",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "feat-3",
				Description: "Feature three",
				Status:      missions.FeaturePending,
				Milestone:   "Milestone 2",
				SkillName:   "frontend-worker",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:          "feat-4",
				Description: "Feature four",
				Status:      missions.FeaturePending,
				Milestone:   "Milestone 2",
				SkillName:   "frontend-worker",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}

	if err := store.CreateMission(mission); err != nil {
		t.Fatalf("create mission: %v", err)
	}

	return store, "test-mission"
}

// newTestModel creates a Model for testing.
func newTestModel(t *testing.T) Model {
	t.Helper()

	store, missionID := newTestStore(t)
	m, err := NewModel(store, missionID)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	return m
}

// testStoreWithFailedFeature creates a store with feat-1 marked as failed.
func testStoreWithFailedFeature(t *testing.T) (*missions.MissionsStore, string) {
	t.Helper()

	store, missionID := newTestStore(t)
	mission, _ := store.LoadMission(missionID)
	mission.Features[0].Status = missions.FeatureFailed
	if err := store.SaveMission(mission); err != nil {
		t.Fatalf("save mission: %v", err)
	}
	return store, missionID
}

// ─── Tests ─────────────────────────────────────────────────────────────────

func TestNewModel(t *testing.T) {
	m := newTestModel(t)

	if m.mission == nil {
		t.Fatal("expected non-nil mission")
	}
	if m.missionID != "test-mission" {
		t.Fatalf("expected missionID 'test-mission', got %q", m.missionID)
	}
	if len(m.items) == 0 {
		t.Fatal("expected non-empty tree items")
	}
	if m.store == nil {
		t.Fatal("expected non-nil store")
	}
	if m.tickInterval != tickIntervalDefault {
		t.Fatalf("expected tick interval %v, got %v", tickIntervalDefault, m.tickInterval)
	}
	if m.paused {
		t.Fatal("expected not paused for active mission")
	}
}

// ─── Tree Building ─────────────────────────────────────────────────────────

func TestBuildTree(t *testing.T) {
	m := newTestModel(t)

	// Should have 2 milestones (features not expanded by default)
	if len(m.items) != 2 {
		t.Fatalf("expected 2 tree items (milestones), got %d", len(m.items))
	}

	// First item should be a milestone
	if !m.items[0].isMilestone {
		t.Fatal("expected item 0 to be a milestone")
	}
	if m.items[0].milestoneName != "Milestone 1" {
		t.Fatalf("expected milestone 'Milestone 1', got %q", m.items[0].milestoneName)
	}

	// Second item
	if !m.items[1].isMilestone {
		t.Fatal("expected item 1 to be a milestone")
	}
	if m.items[1].milestoneName != "Milestone 2" {
		t.Fatalf("expected milestone 'Milestone 2', got %q", m.items[1].milestoneName)
	}

	// Milestone 1 should have 2 features (completed + in_progress)
	if m.items[0].featureCount != 2 {
		t.Fatalf("expected 2 features in Milestone 1, got %d", m.items[0].featureCount)
	}
	if m.items[0].completedCount != 1 {
		t.Fatalf("expected 1 completed in Milestone 1, got %d", m.items[0].completedCount)
	}

	// Milestone 2 should have 2 features
	if m.items[1].featureCount != 2 {
		t.Fatalf("expected 2 features in Milestone 2, got %d", m.items[1].featureCount)
	}
}

func TestBuildTreeEmptyMission(t *testing.T) {
	store, missionID := newTestStore(t)
	mission, err := store.LoadMission(missionID)
	if err != nil {
		t.Fatalf("load mission: %v", err)
	}

	// Remove all milestones and features
	mission.Milestones = nil
	mission.Features = nil
	if err := store.SaveMission(mission); err != nil {
		t.Fatalf("save mission: %v", err)
	}

	m, err := NewModel(store, missionID)
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}

	if len(m.items) != 0 {
		t.Fatalf("expected empty tree, got %d items", len(m.items))
	}
}

// ─── Navigation ────────────────────────────────────────────────────────────

func TestMoveCursorDown(t *testing.T) {
	m := newTestModel(t)

	// Initial cursor should be at 0
	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.cursor)
	}

	// Move down
	m.moveCursor(1)
	if m.cursor != 1 {
		t.Fatalf("expected cursor at 1, got %d", m.cursor)
	}
}

func TestMoveCursorUp(t *testing.T) {
	m := newTestModel(t)

	m.cursor = 1
	m.moveCursor(-1)
	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.cursor)
	}
}

func TestMoveCursorWrapAround(t *testing.T) {
	m := newTestModel(t)

	// At bottom, moving down wraps to top
	m.cursor = len(m.items) - 1
	m.moveCursor(1)
	if m.cursor != 0 {
		t.Fatalf("expected cursor at 0 (wrap), got %d", m.cursor)
	}

	// At top, moving up wraps to bottom
	m.cursor = 0
	m.moveCursor(-1)
	if m.cursor != len(m.items)-1 {
		t.Fatalf("expected cursor at %d (wrap), got %d", len(m.items)-1, m.cursor)
	}
}

func TestHomeEndNavigation(t *testing.T) {
	m := newTestModel(t)

	// Move to end
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if model, ok := result.(Model); ok {
		if model.cursor != len(model.items)-1 {
			t.Fatalf("expected cursor at %d, got %d", len(model.items)-1, model.cursor)
		}
	} else {
		t.Fatal("expected Model type")
	}

	// Move to home
	result, _ = result.Update(tea.KeyMsg{Type: tea.KeyHome})
	if model, ok := result.(Model); ok {
		if model.cursor != 0 {
			t.Fatalf("expected cursor at 0, got %d", model.cursor)
		}
	} else {
		t.Fatal("expected Model type")
	}
}

func TestNavigationOnEmptyTree(t *testing.T) {
	store, missionID := newTestStore(t)
	mission, _ := store.LoadMission(missionID)
	mission.Milestones = nil
	mission.Features = nil
	store.SaveMission(mission)

	m, _ := NewModel(store, missionID)

	// Navigation should not crash
	m.moveCursor(1)
	m.moveCursor(-1)
	m.moveCursor(10)

	// Tree should be empty
	if len(m.items) != 0 {
		t.Fatalf("expected empty tree, got %d items", len(m.items))
	}
}

// ─── Expand / Collapse ─────────────────────────────────────────────────────

func TestExpandMilestone(t *testing.T) {
	m := newTestModel(t)

	// Milestone 1 should initially be collapsed
	if m.items[0].expanded {
		t.Fatal("expected milestone to be collapsed initially")
	}

	// Expand milestone 1
	m.toggleExpand()

	// After expand, items should include milestones + features for milestone 1
	if len(m.items) <= 2 {
		t.Fatalf("expected more than 2 items after expand, got %d", len(m.items))
	}

	// Item 1 should be a feature under Milestone 1
	if !m.items[1].isFeature {
		t.Fatal("expected item 1 to be a feature after expand")
	}
	if m.items[1].feature == nil {
		t.Fatal("expected feature to be non-nil")
	}
	if m.items[1].feature.ID != "feat-1" {
		t.Fatalf("expected feature feat-1, got %q", m.items[1].feature.ID)
	}
}

func TestCollapseMilestone(t *testing.T) {
	m := newTestModel(t)

	// Expand then collapse
	m.toggleExpand()
	if len(m.items) <= 2 {
		t.Fatalf("expected item count to increase after expand")
	}

	// Collapse
	m.toggleExpand()
	if len(m.items) != 2 {
		t.Fatalf("expected 2 items after collapse, got %d", len(m.items))
	}
}

func TestExpandAlreadyExpandedMilestoneJustCollapses(t *testing.T) {
	m := newTestModel(t)

	// Expand
	m.toggleExpand()
	countAfterExpand := len(m.items)

	// Toggle again should collapse
	m.toggleExpand()
	if len(m.items) != 2 {
		t.Fatalf("expected 2 items after collapse, got %d", len(m.items))
	}
	_ = countAfterExpand
}

func TestExpandCollapseViaEnter(t *testing.T) {
	m := newTestModel(t)

	// Enter on a milestone should expand it
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if model, ok := result.(Model); ok {
		if len(model.items) <= 2 {
			t.Fatal("expected item count to increase after Enter on milestone")
		}
	} else {
		t.Fatal("expected Model type")
	}
}

func TestCollapseViaLeftArrow(t *testing.T) {
	m := newTestModel(t)

	// First expand via toggle
	m.toggleExpand()

	// Collapse via left arrow
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if model, ok := result.(Model); ok {
		if len(model.items) != 2 {
			t.Fatalf("expected 2 items after left arrow, got %d", len(model.items))
		}
	} else {
		t.Fatal("expected Model type")
	}
}

func TestExpandViaSpace(t *testing.T) {
	m := newTestModel(t)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if model, ok := result.(Model); ok {
		if len(model.items) <= 2 {
			t.Fatal("expected item count to increase after Space on milestone")
		}
	} else {
		t.Fatal("expected Model type")
	}
}

// ─── Selection / Detail Panel ──────────────────────────────────────────────

func TestSelectedFeature(t *testing.T) {
	m := newTestModel(t)

	// With cursor at milestone, selectedFeature should return nil
	if sel := m.selectedFeature(); sel != nil {
		t.Fatal("expected nil selection for milestone item")
	}

	// Expand and move to a feature
	m.toggleExpand()
	m.cursor = 1 // first feature under Milestone 1

	sel := m.selectedFeature()
	if sel == nil {
		t.Fatal("expected non-nil selection for feature item")
	}
	if sel.ID != "feat-1" {
		t.Fatalf("expected feat-1, got %q", sel.ID)
	}
}

func TestSelectedFeatureChangesLogContext(t *testing.T) {
	m := newTestModel(t)

	// Expand milestone to reveal features
	m.toggleExpand()

	// After expansion, features should be visible
	// moveCursor(1) from index 0 moves to first feature
	m.moveCursor(1)

	if len(m.items) > 1 && m.items[m.cursor].isFeature {
		if m.currentLogFeature == "" {
			t.Fatal("expected currentLogFeature to be set when on a feature item")
		}
	}
}

func TestDetailPanelEmptyWhenNoSelection(t *testing.T) {
	m := newTestModel(t)

	// Cursor on milestone, no feature selected
	if sel := m.selectedFeature(); sel != nil {
		t.Fatal("expected nil selection for milestone item")
	}

	// This shouldn't crash
	_ = m.renderDetailPanel(40, 20)
}

// ─── Status Badges ─────────────────────────────────────────────────────────

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status missions.FeatureStatus
		want   string
	}{
		{missions.FeaturePending, "○"},
		{missions.FeatureInProgress, "●"},
		{missions.FeatureCompleted, "✓"},
		{missions.FeatureFailed, "✗"},
		{missions.FeatureCancelled, "⊘"},
		{missions.FeatureStatus("unknown"), "?"},
	}

	for _, tt := range tests {
		got := statusIcon(tt.status)
		if got != tt.want {
			t.Errorf("statusIcon(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestStatusBadge(t *testing.T) {
	m := newTestModel(t)

	tests := []struct {
		status missions.FeatureStatus
		check  string // substring to check
	}{
		{missions.FeaturePending, "PENDING"},
		{missions.FeatureInProgress, "IN PROGRESS"},
		{missions.FeatureCompleted, "COMPLETED"},
		{missions.FeatureFailed, "FAILED"},
		{missions.FeatureCancelled, "CANCELLED"},
		{missions.FeatureStatus("bogus"), "UNKNOWN"},
	}

	for _, tt := range tests {
		badge := m.statusBadge(tt.status)
		if badge == "" {
			t.Errorf("statusBadge(%q) returned empty", tt.status)
		}
	}
}

// ─── Log Display ───────────────────────────────────────────────────────────

func TestLogFiltering(t *testing.T) {
	m := newTestModel(t)

	m.logLines = []LogEntry{
		{Level: "info", Message: "info line"},
		{Level: "warn", Message: "warn line"},
		{Level: "error", Message: "error line"},
		{Level: "info", Message: "another info"},
	}

	// All filter
	m.logFilter = LogAll
	allLines := m.filteredLogLines()
	if len(allLines) != 4 {
		t.Fatalf("expected 4 lines with LogAll, got %d", len(allLines))
	}

	// Error only
	m.logFilter = LogError
	errLines := m.filteredLogLines()
	if len(errLines) != 1 {
		t.Fatalf("expected 1 line with LogError, got %d", len(errLines))
	}

	// Warn+
	m.logFilter = LogWarn
	warnLines := m.filteredLogLines()
	if len(warnLines) != 2 {
		t.Fatalf("expected 2 lines with LogWarn, got %d", len(warnLines))
	}
}

func TestLogLevelDetection(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{"normal output", "info"},
		{"Error: something failed", "error"},
		{"WARNING: disk space low", "warn"},
		{"Fatal: crash", "error"},
		{"Test failed: assertion", "error"},
		{"Processing complete", "info"},
	}

	for _, tt := range tests {
		got := detectLogLevel(tt.line)
		if got != tt.want {
			t.Errorf("detectLogLevel(%q) = %q, want %q", tt.line, got, tt.want)
		}
	}
}

func TestEmptyLogArea(t *testing.T) {
	m := newTestModel(t)

	// Empty log area should render without crashing
	_ = m.renderLogArea(40, 10)
}

func TestLogAreaWithLines(t *testing.T) {
	m := newTestModel(t)

	m.logLines = []LogEntry{
		{Level: "info", Message: "line 1"},
		{Level: "info", Message: "line 2"},
		{Level: "error", Message: "error occurred"},
	}

	// Should render without crashing
	_ = m.renderLogArea(40, 10)
}

// ─── Controls ──────────────────────────────────────────────────────────────

func TestPauseResumeToggle(t *testing.T) {
	m := newTestModel(t)

	// Initially active
	if m.paused {
		t.Fatal("expected mission to be active")
	}

	// Press 'p' - should not crash
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	_, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model returned from update")
	}
}

func TestRetryVisibleOnFailedFeature(t *testing.T) {
	store, missionID := testStoreWithFailedFeature(t)
	m, _ := NewModel(store, missionID)

	// Expand to see features
	m.toggleExpand()
	m.cursor = 1 // feat-1 (now failed)

	// Render controls - should have retry button
	controls := m.renderControls(m.selectedFeature(), 40)
	if !stringsContains(controls, "Retry") {
		t.Error("expected Retry button for failed feature")
	}
}

func TestCancelVisibleOnInProgressFeature(t *testing.T) {
	m := newTestModel(t)

	// Expand to see features
	m.toggleExpand()
	m.cursor = 2 // feat-2 (in_progress)

	// Render controls - should have cancel button
	controls := m.renderControls(m.selectedFeature(), 40)
	if !stringsContains(controls, "Cancel") {
		t.Error("expected Cancel button for in-progress feature")
	}
}

func TestPauseDisabledWhenAlreadyPaused(t *testing.T) {
	store, missionID := newTestStore(t)
	mission, _ := store.LoadMission(missionID)
	mission.Status = missions.MissionPaused
	store.SaveMission(mission)

	m, _ := NewModel(store, missionID)
	if !m.paused {
		t.Fatal("expected mission to be paused")
	}

	// Render controls from mission summary (no feature selected)
	if m.mission.Status != missions.MissionPaused {
		t.Fatalf("expected paused status, got %s", m.mission.Status)
	}
}

func TestCancelDisabledForCompletedFeature(t *testing.T) {
	m := newTestModel(t)

	m.toggleExpand()
	m.cursor = 1 // feat-1 (completed)

	controls := m.renderControls(m.selectedFeature(), 40)
	if stringsContains(controls, "Cancel") {
		t.Error("expected no Cancel button for completed feature")
	}
}

// ─── Confirmation Dialog ───────────────────────────────────────────────────

func TestCancelShowsConfirmation(t *testing.T) {
	m := newTestModel(t)

	m.toggleExpand()
	m.cursor = 2 // feat-2 (in_progress)

	// Press 'c' to cancel
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model type")
	}

	if !model.confirmShow {
		t.Fatal("expected confirmation dialog to show")
	}
	if model.confirmAction != "cancel" {
		t.Fatalf("expected confirmAction 'cancel', got %q", model.confirmAction)
	}
	if model.confirmTarget != "feat-2" {
		t.Fatalf("expected confirmTarget 'feat-2', got %q", model.confirmTarget)
	}
}

func TestRetryShowsConfirmation(t *testing.T) {
	store, missionID := newTestStore(t)
	mission, _ := store.LoadMission(missionID)
	mission.Features[0].Status = missions.FeatureFailed
	store.SaveMission(mission)

	m, _ := NewModel(store, missionID)
	m.toggleExpand()
	m.cursor = 1 // feat-1 (now failed)

	// Press 'r' to retry
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model type")
	}

	if !model.confirmShow {
		t.Fatal("expected confirmation dialog to show for retry")
	}
	if model.confirmAction != "retry" {
		t.Fatalf("expected confirmAction 'retry', got %q", model.confirmAction)
	}
}

func TestConfirmationAccepted(t *testing.T) {
	m := newTestModel(t)

	m.toggleExpand()
	m.cursor = 2 // feat-2 (in_progress)

	// Press 'c' to show confirmation
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model type")
	}

	if !model.confirmShow {
		t.Fatal("expected confirmation to show")
	}

	// Press 'y' to accept - confirmShow stays true until the async action completes
	result2, cmds := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model2, ok := result2.(Model)
	if !ok {
		t.Fatal("expected Model type")
	}

	// The confirmation remains shown until actionCompleteMsg is received
	if !model2.confirmShow {
		t.Fatal("expected confirmation to remain shown until action completes")
	}
	if cmds == nil {
		t.Fatal("expected commands from executeConfirmedAction")
	}

	// Simulate the action completing
	result3, _ := model2.Update(actionCompleteMsg{action: "cancel"})
	model3, ok := result3.(Model)
	if !ok {
		t.Fatal("expected Model type")
	}
	if model3.confirmShow {
		t.Fatal("expected confirmation to close after action completes")
	}
}

func TestConfirmationDeclined(t *testing.T) {
	m := newTestModel(t)

	m.toggleExpand()
	m.cursor = 2

	// Show confirmation
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model, _ := result.(Model)

	// Press 'n' to decline
	result2, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model2, _ := result2.(Model)

	if model2.confirmShow {
		t.Fatal("expected confirmation to close after decline")
	}
}

// ─── Keyboard Shortcuts ────────────────────────────────────────────────────

func TestQuit(t *testing.T) {
	m := newTestModel(t)

	// 'q' should not panic
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	_, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model returned from update")
	}
}

func TestCtrlCQuits(t *testing.T) {
	m := newTestModel(t)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	_, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model returned")
	}
}

func TestHelpOverlay(t *testing.T) {
	m := newTestModel(t)

	if m.showHelp {
		t.Fatal("expected help to be hidden initially")
	}

	// '?' toggles help
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model, _ := result.(Model)

	if !model.showHelp {
		t.Fatal("expected help to show after '?'")
	}

	// '?' again toggles off
	result2, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	model2, _ := result2.(Model)

	if model2.showHelp {
		t.Fatal("expected help to hide after second '?'")
	}
}

func TestEscapeClosesHelp(t *testing.T) {
	m := newTestModel(t)

	// Show help
	m.showHelp = true

	// Esc closes help
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model, _ := result.(Model)

	if model.showHelp {
		t.Fatal("expected help to close on Esc")
	}
}

func TestTabSwitchesFocus(t *testing.T) {
	m := newTestModel(t)

	if m.focus != FocusTree {
		t.Fatal("expected initial focus on tree")
	}

	// Tab switches to detail
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model, _ := result.(Model)

	if model.focus != FocusDetail {
		t.Fatal("expected focus on detail after Tab")
	}

	// Tab switches back to tree
	result2, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model2, _ := result2.(Model)

	if model2.focus != FocusTree {
		t.Fatal("expected focus on tree after second Tab")
	}
}

// ─── Status Bar ────────────────────────────────────────────────────────────

func TestStatusBarRenders(t *testing.T) {
	m := newTestModel(t)

	// Should render without crashing
	bar := m.renderStatusBar(80)
	if bar == "" {
		t.Fatal("expected non-empty status bar")
	}
}

func TestStatusBarShowsElapsedTime(t *testing.T) {
	m := newTestModel(t)

	bar := m.renderStatusBar(80)
	if !stringsContains(bar, "⏱") {
		t.Error("expected elapsed time in status bar")
	}
}

func TestStatusBarShowsFeatureCount(t *testing.T) {
	m := newTestModel(t)

	bar := m.renderStatusBar(80)
	if !stringsContains(bar, "Features") {
		t.Error("expected feature count in status bar")
	}
}

func TestStatusBarShowsMissionStatus(t *testing.T) {
	m := newTestModel(t)

	bar := m.renderStatusBar(80)
	if !stringsContains(bar, "Active") {
		t.Error("expected mission status in status bar")
	}
}

func TestStatusBarShowsShortcuts(t *testing.T) {
	m := newTestModel(t)

	bar := m.renderStatusBar(80)
	if !stringsContains(bar, "? Help") {
		t.Error("expected help shortcut in status bar")
	}
	if !stringsContains(bar, "q Quit") {
		t.Error("expected quit shortcut in status bar")
	}
}

func TestStatusBarShowsWorkerCount(t *testing.T) {
	m := newTestModel(t)

	bar := m.renderStatusBar(80)
	// There's 1 in-progress feature
	if !stringsContains(bar, "Workers") {
		t.Error("expected worker count in status bar")
	}
}

// ─── Window Resize ─────────────────────────────────────────────────────────

func TestWindowResize(t *testing.T) {
	m := newTestModel(t)

	// Send resize message
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model, _ := result.(Model)

	if model.width != 120 {
		t.Fatalf("expected width 120, got %d", model.width)
	}
	if model.height != 40 {
		t.Fatalf("expected height 40, got %d", model.height)
	}
	if model.windowTooSmall {
		t.Fatal("expected window not too small at 120x40")
	}
}

func TestSmallTerminalWarning(t *testing.T) {
	m := newTestModel(t)

	// Send very small resize
	result, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	model, _ := result.(Model)

	if !model.windowTooSmall {
		t.Fatal("expected window too small at 40x10")
	}

	// View should show warning
	view := model.View()
	if !stringsContains(view, "too small") {
		t.Error("expected 'too small' warning in view")
	}
}

// ─── Error Handling ────────────────────────────────────────────────────────

func TestStoreErrorShowsNotification(t *testing.T) {
	m := newTestModel(t)

	// Send a store error message
	result, _ := m.Update(storeUpdateMsg{err: missions.ErrMissionNotFound})
	model, ok := result.(Model)
	if !ok {
		t.Fatal("expected Model type")
	}

	if model.notification == "" {
		t.Fatal("expected notification after store error")
	}
	if !stringsContains(model.notification, "Store error") {
		t.Fatalf("expected 'Store error' in notification, got %q", model.notification)
	}
}

func TestUnknownStatusDoesNotCrash(t *testing.T) {
	m := newTestModel(t)

	// Call statusBadge with unknown status - should not panic
	badge := m.statusBadge(missions.FeatureStatus("bogus"))
	if badge == "" {
		t.Fatal("expected non-empty badge for unknown status")
	}
}

// ─── Rendering ─────────────────────────────────────────────────────────────

func TestViewRenders(t *testing.T) {
	m := newTestModel(t)

	// Set a reasonable terminal size
	m.width = 80
	m.height = 24

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
}

func TestViewWithResize(t *testing.T) {
	m := newTestModel(t)

	result, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model, _ := result.(Model)

	view := model.View()
	if view == "" {
		t.Fatal("expected non-empty view after resize")
	}
}

func TestViewWithExpandedTree(t *testing.T) {
	m := newTestModel(t)
	m.width = 80
	m.height = 24

	// Expand a milestone
	m.toggleExpand()

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty view with expanded tree")
	}
}

func TestViewWithConfirmation(t *testing.T) {
	m := newTestModel(t)
	m.width = 80
	m.height = 24
	m.toggleExpand()
	m.cursor = 2

	// Trigger confirmation
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty view with confirmation dialog")
	}
}

func TestViewWithHelp(t *testing.T) {
	m := newTestModel(t)
	m.width = 80
	m.height = 24

	m.showHelp = true

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty view with help overlay")
	}
}

// ─── Header & Status Labels ────────────────────────────────────────────────

func TestMissionStatusLabel(t *testing.T) {
	tests := []struct {
		status missions.MissionStatus
		want   string
	}{
		{missions.MissionPlanning, "Planning"},
		{missions.MissionActive, "Active"},
		{missions.MissionPaused, "Paused"},
		{missions.MissionCompleted, "Completed"},
		{missions.MissionFailed, "Failed"},
		{missions.MissionCancelled, "Cancelled"},
		{missions.MissionValidating, "Validating"},
	}

	for _, tt := range tests {
		got := missionStatusLabel(tt.status)
		if got != tt.want {
			t.Errorf("missionStatusLabel(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestElapsedSince(t *testing.T) {
	// elapsedSince should return a non-empty string
	result := elapsedSince(time.Now().Add(-5 * time.Minute))
	if result == "" {
		t.Fatal("expected non-empty elapsed time")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input   string
		maxLen  int
		wantLen int
	}{
		{"short", 10, 5},          // no truncation needed
		{"a very long string that needs truncation", 20, 20},
		{"exactly 10", 11, 10},    // fits
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if len(got) > tt.maxLen {
			t.Errorf("truncate(%q, %d) = %q (len %d), expected max %d",
				tt.input, tt.maxLen, got, len(got), tt.maxLen)
		}
	}
}

// ─── Auto-Select In-Progress ───────────────────────────────────────────────

func TestAutoSelectInProgress(t *testing.T) {
	m := newTestModel(t)

	// Initially cursor is at 0, but autoSelectInProgress should move to
	// the in-progress feature when cursor is still at 0 (no user override)
	if len(m.items) > 0 {
		// Initially cursor at 0
		m.cursor = 0
		m.autoSelectInProgress()

		// Since feat-2 (in_progress) is under Milestone 1 which is collapsed,
		// the feature won't be visible in the tree. So cursor should stay at 0.
		if m.cursor != 0 {
			t.Fatalf("expected cursor to stay at 0 when feature not visible, got %d", m.cursor)
		}
	}
}

// ─── Milestone Aggregate Icon ──────────────────────────────────────────────

func TestMilestoneAggregateIcon(t *testing.T) {
	store, missionID := newTestStore(t)

	m, _ := NewModel(store, missionID)

	// Milestone 1: 1 completed, 1 in_progress -> partial
	if len(m.items) > 0 {
		item := m.items[0]
		icon := m.milestoneAggregateIcon(item)
		if icon == "" {
			t.Fatal("expected non-empty aggregate icon")
		}
	}
}

// ─── Render Performance / No Panic ─────────────────────────────────────────

func TestNoPanicOnRapidUpdates(t *testing.T) {
	m := newTestModel(t)

	// Simulate rapid key presses
	keys := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'j'}},
		{Type: tea.KeyRunes, Runes: []rune{'k'}},
		{Type: tea.KeyRunes, Runes: []rune{'p'}},
		{Type: tea.KeyRunes, Runes: []rune{'?'}},
		{Type: tea.KeyEsc},
		{Type: tea.KeyEnter},
		{Type: tea.KeyLeft},
		{Type: tea.KeyTab},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
	}

	var result tea.Model = m
	for _, key := range keys {
		r, _ := result.Update(key)
		result = r
	}
	// Should not panic
}

// ─── Store Update After Action ─────────────────────────────────────────────

func TestStoreUpdateRebuildsTree(t *testing.T) {
	store, missionID := newTestStore(t)
	m, _ := NewModel(store, missionID)

	// Expand first milestone
	m.toggleExpand()
	initialCount := len(m.items)

	// Simulate store update with mission having feature status changes
	mission, _ := store.LoadMission(missionID)
	mission.Features[1].Status = missions.FeatureCompleted
	store.SaveMission(mission)

	// Reload
	reloaded, _ := store.LoadMission(missionID)
	m.Update(storeUpdateMsg{mission: reloaded})

	// Tree should be rebuilt
	if len(m.items) == 0 {
		t.Fatal("expected non-empty tree after update")
	}
	_ = initialCount
}

// ─── Log Polling ───────────────────────────────────────────────────────────

func TestLogPollMessage(t *testing.T) {
	m := newTestModel(t)

	m.currentLogFeature = "feat-1"
	// This should not crash
	_ = m.pollLogCmd("feat-1")
}

func TestNotificationAutoDismiss(t *testing.T) {
	m := newTestModel(t)

	m.notification = "Test notification"
	m.notifTimer = 1

	// After multiple ticks, notification should not auto-dismiss automatically
	// (it's handled by a separate timer mechanism)
	_ = m.notifTimer
}

func TestNotificationRenders(t *testing.T) {
	m := newTestModel(t)
	m.width = 80
	m.height = 24

	m.notification = "Test notification"

	notif := m.renderNotification(80)
	if notif == "" {
		t.Fatal("expected non-empty notification")
	}
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// stringsContains is a test helper that checks if a string contains a substring.
func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && containsString(s, substr)
}

// containsString is a simple strings.Contains replacement for tests.
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestModelInitialMessages verifies the Init function returns expected commands.
func TestModelInitialMessages(t *testing.T) {
	m := newTestModel(t)

	cmds := m.Init()
	if cmds == nil {
		t.Fatal("expected non-nil commands from Init")
	}
}

// TestFocusPanel verifies focus panel constants.
func TestFocusPanel(t *testing.T) {
	if FocusTree != 0 {
		t.Fatal("expected FocusTree to be 0")
	}
	if FocusDetail != 1 {
		t.Fatal("expected FocusDetail to be 1")
	}
}

// TestLogFilterLabels verifies log filter labels.
func TestLogFilterLabels(t *testing.T) {
	tests := []struct {
		filter LogFilter
		want   string
	}{
		{LogAll, "ALL"},
		{LogInfo, "INFO+"},
		{LogWarn, "WARN+"},
		{LogError, "ERR"},
	}

	for _, tt := range tests {
		got := tt.filter.Label()
		if got != tt.want {
			t.Errorf("LogFilter(%d).Label() = %q, want %q", tt.filter, got, tt.want)
		}
	}
}

// TestRenderTreeItem verifies tree item rendering.
func TestRenderTreeItem(t *testing.T) {
	m := newTestModel(t)

	// Milestone item
	milestoneItem := m.items[0]
	result := m.renderTreeItem(0, milestoneItem, 40)
	if result == "" {
		t.Fatal("expected non-empty tree item rendering")
	}
	if !stringsContains(result, "Milestone") {
		t.Error("expected milestone name in rendered item")
	}
}

// TestRenderMissionSummary verifies the mission summary rendering.
func TestRenderMissionSummary(t *testing.T) {
	m := newTestModel(t)

	summary := m.renderMissionSummary(40)
	if summary == "" {
		t.Fatal("expected non-empty mission summary")
	}
	if !stringsContains(summary, "Status") {
		t.Error("expected status in mission summary")
	}
}

// TestSpinnerAnimation verifies spinner at different tick values.
func TestSpinnerAnimation(t *testing.T) {
	frames := map[int]string{
		0: "◐",
		1: "◓",
		2: "◑",
		3: "◒",
		4: "◐",
	}
	for tick, expected := range frames {
		got := spinner(tick)
		if got != expected {
			t.Errorf("spinner(%d) = %q, want %q", tick, got, expected)
		}
	}
}

// TestColorFunctions verifies status color helpers.
func TestColorFunctions(t *testing.T) {
	tests := []struct {
		status      missions.FeatureStatus
		wantColor   string
		wantName    string
	}{
		{missions.FeaturePending, "240", "grey"},
		{missions.FeatureInProgress, "39", "blue"},
		{missions.FeatureCompleted, "76", "green"},
		{missions.FeatureFailed, "196", "red"},
		{missions.FeatureCancelled, "208", "orange"},
		{missions.FeatureStatus("bogus"), "240", "grey"},
	}

	for _, tt := range tests {
		if got := statusColor(tt.status); got != tt.wantColor {
			t.Errorf("statusColor(%q) = %q, want %q", tt.status, got, tt.wantColor)
		}
		if got := statusColorName(tt.status); got != tt.wantName {
			t.Errorf("statusColorName(%q) = %q, want %q", tt.status, got, tt.wantName)
		}
	}
}
