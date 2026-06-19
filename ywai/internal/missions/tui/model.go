package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
)

// ─── Constants ─────────────────────────────────────────────────────────────

const (
	tickIntervalDefault = 1 * time.Second
	logPollInterval     = 500 * time.Millisecond
	notificationTimeout = 8 // ticks before notification auto-dismisses
	minWidth            = 80
	minHeight           = 24
)

// ─── Focus Panel ───────────────────────────────────────────────────────────

// FocusPanel indicates which panel currently has keyboard focus.
type FocusPanel int

const (
	FocusTree   FocusPanel = iota // Left panel: feature tree
	FocusDetail                   // Right panel: detail / logs / controls
)

// ─── Log Filter ────────────────────────────────────────────────────────────

// LogFilter controls which log levels are displayed.
type LogFilter int

const (
	LogAll   LogFilter = iota // Show all log lines
	LogInfo                   // Show info and above
	LogWarn                   // Show warnings and above
	LogError                  // Show errors only
)

// LogFilterLabel returns a short label for the filter.
func (f LogFilter) Label() string {
	switch f {
	case LogAll:
		return "ALL"
	case LogInfo:
		return "INFO+"
	case LogWarn:
		return "WARN+"
	case LogError:
		return "ERR"
	default:
		return "ALL"
	}
}

// ─── Log Entry ─────────────────────────────────────────────────────────────

// LogEntry represents a single line in the log area.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`   // "info", "warn", "error"
	Message   string    `json:"message"` // the log text
}

// ─── Tree Item ─────────────────────────────────────────────────────────────

// treeItem represents a single visible row in the feature tree.
type treeItem struct {
	indent         int               // indentation level (0 = milestone, 1 = feature)
	label          string            // display text
	isMilestone    bool              // true if this is a milestone row
	isFeature      bool              // true if this is a feature row
	feature        *missions.Feature // feature pointer (nil for milestones)
	milestoneIdx   int               // index into mission.Milestones (for milestone rows)
	milestoneName  string            // milestone name
	expanded       bool              // true if milestone is expanded
	hasChildren    bool              // true if milestone has features
	featureCount   int               // total features in this milestone
	completedCount int               // completed features in this milestone
}

// ─── Messages ──────────────────────────────────────────────────────────────

// tickMsg is sent periodically to update elapsed time and reload state.
type tickMsg struct{}

// storeUpdateMsg carries the latest mission data from the store.
type storeUpdateMsg struct {
	mission *missions.Mission
	err     error
}

// validationLoadMsg carries the validation state loaded from the store.
type validationLoadMsg struct {
	state *missions.ValidationState
	err   error
}

// logPollMsg triggers a log file read for the selected feature.
type logPollMsg struct {
	featureID string
}

// logFileContent carries the raw log content read from disk.
type logFileContent struct {
	featureID string
	content   string
	err       error
}

// notificationDismissMsg clears the current notification.
type notificationDismissMsg struct{}

// actionCompleteMsg is sent after a store action (pause, resume, retry, cancel).
type actionCompleteMsg struct {
	action string
	err    error
}

// ─── Model ─────────────────────────────────────────────────────────────────

// Model is the top-level Bubble Tea model for Mission Control TUI.
type Model struct {
	// Dependencies
	store     *missions.MissionsStore
	missionID string

	// Mission state (reloaded on each tick)
	mission *missions.Mission

	// Validation state (loaded alongside mission)
	validationState *missions.ValidationState

	// Tree state
	items  []treeItem // flat list of visible tree rows
	cursor int        // index into items (cursor position)
	focus  FocusPanel // which panel has keyboard focus

	// Log state (per selected feature)
	currentLogFeature string     // feature ID whose log is currently displayed
	logLines          []LogEntry // parsed log lines for the current feature
	logFilter         LogFilter
	logAutoScroll     bool
	logScrollOffset   int // scroll offset for log view (0 = newest)

	// Feature animation state (for in-progress spinners)
	animTick int

	// Status bar state
	startTime time.Time
	paused    bool

	// UI overlays
	showHelp bool

	// Cancel confirmation dialog
	confirmShow   bool
	confirmMsg    string
	confirmAction string // "cancel" or "retry"
	confirmTarget string // feature ID

	// Error notification
	notification string
	notifTimer   int

	// Terminal dimensions
	width          int
	height         int
	windowTooSmall bool

	// Timing
	tickInterval time.Duration
}

// ─── tea.Model Interface ───────────────────────────────────────────────────

// Init returns the initial commands for the Bubble Tea program.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadStoreCmd(),
		m.tickCmd(),
		m.pollLogCmd(""),
	)
}

// Update handles all messages and returns the updated model and commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// ── Terminal Events ────────────────────────────────────────────────

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.windowTooSmall = msg.Width < minWidth || msg.Height < minHeight

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.MouseMsg:
		// Mouse handling not implemented for MVP

	// ── Tick (periodic refresh) ────────────────────────────────────────

	case tickMsg:
		m.animTick++
		// Reload mission state from store
		cmds = append(cmds, m.loadStoreCmd())
		// Load validation state periodically
		cmds = append(cmds, m.loadValidationCmd())
		// Next tick
		cmds = append(cmds, m.tickCmd())

	// ── Store Update ──────────────────────────────────────────────────

	case storeUpdateMsg:
		if msg.err != nil {
			m.notification = fmt.Sprintf("Store error: %v", msg.err)
			m.notifTimer = notificationTimeout
			break
		}
		m.mission = msg.mission
		m.paused = m.mission.Status == missions.MissionPaused
		m.rebuildTree()
		// If we have a selected feature, update the log
		if sel := m.selectedFeature(); sel != nil {
			m.currentLogFeature = sel.ID
		}
		// If an in-progress feature exists and nothing is selected, auto-select it
		if m.cursor >= len(m.items) || m.cursor < 0 {
			m.cursor = 0
		}
		m.autoSelectInProgress()

	// ── Validation Update ─────────────────────────────────────────────

	case validationLoadMsg:
		if msg.err == nil {
			m.validationState = msg.state
		}

	// ── Log Poll ──────────────────────────────────────────────────────

	case logPollMsg:
		if m.currentLogFeature != "" {
			m.loadFeatureLog(m.currentLogFeature)
		}
		cmds = append(cmds, m.pollLogCmd(m.currentLogFeature))

	case logFileContent:
		if msg.err == nil && msg.featureID == m.currentLogFeature {
			m.parseLogContent(msg.content)
		}

	// ── Action Complete ───────────────────────────────────────────────

	case actionCompleteMsg:
		if msg.err != nil {
			m.notification = fmt.Sprintf("%s error: %v", msg.action, msg.err)
			m.notifTimer = notificationTimeout
		} else {
			m.notification = fmt.Sprintf("%s successful", msg.action)
			m.notifTimer = notificationTimeout / 2
		}
		m.confirmShow = false

	// ── Notification Dismiss ──────────────────────────────────────────

	case notificationDismissMsg:
		m.notification = ""
		m.notifTimer = 0
	}

	return m, tea.Batch(cmds...)
}

// ─── Key Handling ──────────────────────────────────────────────────────────

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Global keys (always work)
	switch msg.String() {

	case "q", "ctrl+c":
		if m.confirmShow {
			// Cancel confirmation dialog
			m.confirmShow = false
			return m, nil
		}
		return m, tea.Quit

	case "?":
		if m.confirmShow {
			m.confirmShow = false
			return m, nil
		}
		m.showHelp = !m.showHelp
		return m, nil

	case "esc":
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		if m.confirmShow {
			m.confirmShow = false
			return m, nil
		}
		return m, nil
	}

	// If help overlay is shown, only Esc handled above
	if m.showHelp {
		return m, nil
	}

	// If confirmation dialog is shown
	if m.confirmShow {
		switch msg.String() {
		case "y", "Y":
			cmds = append(cmds, m.executeConfirmedAction())
		case "n", "N", "enter":
			m.confirmShow = false
		}
		return m, tea.Batch(cmds...)
	}

	// Focus-aware keys
	switch msg.String() {

	// ── Navigation (work in both panels) ─────────────────────────────

	case "up", "k":
		m.moveCursor(-1)

	case "down", "j":
		m.moveCursor(1)

	case "home":
		m.cursor = 0

	case "end":
		m.cursor = len(m.items) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}

	// ── Panel switching ──────────────────────────────────────────────

	case "tab":
		if m.focus == FocusTree {
			m.focus = FocusDetail
		} else {
			m.focus = FocusTree
		}

	// ── Tree panel specific ──────────────────────────────────────────

	case "enter", "right":
		if m.focus == FocusTree {
			m.toggleExpand()
		}

	case "left":
		if m.focus == FocusTree {
			m.collapseCurrent()
		}

	case " ":
		if m.focus == FocusTree {
			m.toggleExpand()
		}

	// ── Detail panel: log scrolling ──────────────────────────────────

	case "pgup":
		if m.focus == FocusDetail {
			m.scrollLogUp()
		}

	case "pgdown":
		if m.focus == FocusDetail {
			m.scrollLogDown()
		}

	// ── Controls (work from either panel) ────────────────────────────

	case "p":
		// Pause / Resume toggle
		cmds = append(cmds, m.togglePauseCmd())

	case "r":
		// Retry (on selected feature)
		if sel := m.selectedFeature(); sel != nil && sel.Status == missions.FeatureFailed {
			m.showConfirmation("Retry feature \"" + sel.ID + "\"? (y/n)")
			m.confirmAction = "retry"
			m.confirmTarget = sel.ID
		}

	case "c":
		// Cancel (on selected feature)
		if sel := m.selectedFeature(); sel != nil && sel.Status == missions.FeatureInProgress {
			m.showConfirmation("Cancel feature \"" + sel.ID + "\"? (y/n)")
			m.confirmAction = "cancel"
			m.confirmTarget = sel.ID
		}

	// ── Log filtering (detail panel) ─────────────────────────────────

	case "1":
		if m.focus == FocusDetail {
			m.logFilter = LogAll
		}
	case "2":
		if m.focus == FocusDetail {
			m.logFilter = LogInfo
		}
	case "3":
		if m.focus == FocusDetail {
			m.logFilter = LogWarn
		}
	case "4":
		if m.focus == FocusDetail {
			m.logFilter = LogError
		}
	}

	return m, tea.Batch(cmds...)
}

// ─── Commands ──────────────────────────────────────────────────────────────

// tickCmd returns a command that fires a tickMsg after the tick interval.
func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(m.tickInterval, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// loadStoreCmd returns a command that loads the latest mission state from the store.
func (m Model) loadStoreCmd() tea.Cmd {
	return func() tea.Msg {
		mission, err := m.store.LoadMission(m.missionID)
		if err != nil {
			return storeUpdateMsg{err: fmt.Errorf("load mission: %w", err)}
		}
		return storeUpdateMsg{mission: mission}
	}
}

// pollLogCmd returns a command that reads the worker log file for a feature.
func (m Model) pollLogCmd(featureID string) tea.Cmd {
	return tea.Tick(logPollInterval, func(t time.Time) tea.Msg {
		return logPollMsg{featureID: featureID}
	})
}

// loadFeatureLog reads and parses the worker log file for the given feature.
func (m *Model) loadFeatureLog(featureID string) {
	logPath := fmt.Sprintf("%s/%s/workers/%s/output.log",
		m.store.BaseDir(), m.missionID, featureID)

	data, err := os.ReadFile(logPath)
	if err != nil {
		// File may not exist yet (feature not started)
		return
	}
	m.parseLogContent(string(data))
}

// loadValidationCmd returns a command that loads the validation state from the store.
func (m Model) loadValidationCmd() tea.Cmd {
	return func() tea.Msg {
		vs, err := m.store.LoadValidationState(m.missionID)
		if err != nil {
			return validationLoadMsg{err: err}
		}
		return validationLoadMsg{state: vs}
	}
}

// parseLogContent splits raw log text into LogEntry slice.
func (m *Model) parseLogContent(content string) {
	if content == "" {
		return
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	entries := make([]LogEntry, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		level := detectLogLevel(line)
		entries = append(entries, LogEntry{
			Timestamp: time.Now(),
			Level:     level,
			Message:   line,
		})
	}
	m.logLines = entries
}

// detectLogLevel returns "error", "warn", or "info" based on line content.
func detectLogLevel(line string) string {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") ||
		strings.Contains(lower, "panic") || strings.Contains(lower, "fail") {
		return "error"
	}
	if strings.Contains(lower, "warn") || strings.Contains(lower, "warning") {
		return "warn"
	}
	return "info"
}

// togglePauseCmd pauses or resumes the mission depending on current state.
func (m Model) togglePauseCmd() tea.Cmd {
	return func() tea.Msg {
		if m.mission == nil {
			return actionCompleteMsg{action: "pause", err: fmt.Errorf("no mission loaded")}
		}

		var newStatus missions.MissionStatus
		if m.mission.Status == missions.MissionActive || m.mission.Status == missions.MissionPlanning {
			newStatus = missions.MissionPaused
		} else if m.mission.Status == missions.MissionPaused {
			newStatus = missions.MissionActive
		} else {
			return actionCompleteMsg{
				action: "pause",
				err:    fmt.Errorf("cannot %s mission in %s state", "toggle pause", m.mission.Status),
			}
		}

		// Validate transition
		_, err := missions.TransitionMissionStatus(m.mission.Status, newStatus)
		if err != nil {
			return actionCompleteMsg{action: "pause", err: err}
		}

		// Reload mission to get latest state, then update
		mission, loadErr := m.store.LoadMission(m.missionID)
		if loadErr != nil {
			return actionCompleteMsg{action: "pause", err: fmt.Errorf("load mission: %w", loadErr)}
		}

		mission.Status = newStatus
		mission.UpdatedAt = time.Now()
		if saveErr := m.store.SaveMission(mission); saveErr != nil {
			return actionCompleteMsg{action: "pause", err: fmt.Errorf("save mission: %w", saveErr)}
		}

		return actionCompleteMsg{action: "pause"}
	}
}

// executeConfirmedAction runs the confirmed action (cancel or retry).
func (m Model) executeConfirmedAction() tea.Cmd {
	return func() tea.Msg {
		// Reload mission to get latest state
		mission, err := m.store.LoadMission(m.missionID)
		if err != nil {
			return actionCompleteMsg{action: m.confirmAction, err: fmt.Errorf("load mission: %w", err)}
		}

		switch m.confirmAction {
		case "cancel":
			_, err = missions.CancelFeature(m.store, mission, m.confirmTarget)
		case "retry":
			_, err = missions.RequeueFeature(m.store, mission, m.confirmTarget)
		default:
			err = fmt.Errorf("unknown action: %s", m.confirmAction)
		}

		if err != nil {
			return actionCompleteMsg{action: m.confirmAction, err: err}
		}
		return actionCompleteMsg{action: m.confirmAction}
	}
}

// showConfirmation sets the confirmation dialog state.
func (m *Model) showConfirmation(msg string) {
	m.confirmShow = true
	m.confirmMsg = msg
}

// ─── Navigation ────────────────────────────────────────────────────────────

// moveCursor moves the cursor by delta steps, with wrap-around.
func (m *Model) moveCursor(delta int) {
	if len(m.items) == 0 {
		return
	}
	total := len(m.items)
	m.cursor = (m.cursor + delta) % total
	if m.cursor < 0 {
		m.cursor = total - 1
	}
	// Update currentLogFeature when selection changes
	if sel := m.selectedFeature(); sel != nil {
		if sel.ID != m.currentLogFeature {
			m.currentLogFeature = sel.ID
			m.logScrollOffset = 0
			m.logAutoScroll = true
		}
	} else {
		m.currentLogFeature = ""
		m.logLines = nil
	}
}

// toggleExpand expands or collapses the milestone at the cursor position.
func (m *Model) toggleExpand() {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return
	}
	item := &m.items[m.cursor]
	if !item.isMilestone || !item.hasChildren {
		return
	}
	item.expanded = !item.expanded
	m.rebuildTree()
	// Keep cursor on the same milestone
	m.cursor = m.findItemIndex(item.milestoneName, true)
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// collapseCurrent collapses the milestone at cursor if it's expanded.
func (m *Model) collapseCurrent() {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return
	}
	item := &m.items[m.cursor]
	if item.isMilestone {
		item.expanded = false
		m.rebuildTree()
		m.cursor = m.findItemIndex(item.milestoneName, true)
		if m.cursor < 0 {
			m.cursor = 0
		}
	} else if item.isFeature {
		// Move cursor up to the parent milestone
		for i := m.cursor - 1; i >= 0; i-- {
			if m.items[i].isMilestone {
				m.cursor = i
				break
			}
		}
	}
}

// autoSelectInProgress moves the cursor to the first in-progress feature if found.
func (m *Model) autoSelectInProgress() {
	if len(m.items) == 0 {
		return
	}
	// Don't override explicit user selection
	if m.cursor > 0 {
		return
	}
	for i, item := range m.items {
		if item.isFeature && item.feature != nil && item.feature.Status == missions.FeatureInProgress {
			m.cursor = i
			m.currentLogFeature = item.feature.ID
			return
		}
	}
}

// findItemIndex finds the index of a milestone in the items list.
func (m *Model) findItemIndex(milestoneName string, isMilestone bool) int {
	for i, item := range m.items {
		if item.isMilestone == isMilestone && item.milestoneName == milestoneName {
			return i
		}
	}
	return -1
}

// scrollLogUp scrolls the log view up (older content) by one line.
func (m *Model) scrollLogUp() {
	m.logAutoScroll = false
	m.logScrollOffset++
}

// scrollLogDown scrolls the log view down (newer content) by one line.
func (m *Model) scrollLogDown() {
	if m.logScrollOffset > 0 {
		m.logScrollOffset--
	}
	if m.logScrollOffset == 0 {
		m.logAutoScroll = true
	}
}

// ─── Tree Building ─────────────────────────────────────────────────────────

// buildTree constructs the flat tree item list from the mission data.
func (m *Model) buildTree() {
	if m.mission == nil {
		m.items = []treeItem{}
		return
	}

	var items []treeItem

	for mi, milestone := range m.mission.Milestones {
		features := getFeaturesForMilestone(m.mission, milestone.Name)
		total := len(features)
		completed := countCompleted(features)

		item := treeItem{
			indent:         0,
			isMilestone:    true,
			milestoneIdx:   mi,
			milestoneName:  milestone.Name,
			label:          milestone.Name,
			expanded:       false, // all collapsed at startup
			hasChildren:    total > 0,
			featureCount:   total,
			completedCount: completed,
		}
		items = append(items, item)

		// Features are added when expanded (initially collapsed)
	}

	m.items = items
}

// rebuildTree re-creates the flat item list preserving expand states.
func (m *Model) rebuildTree() {
	if m.mission == nil {
		m.items = []treeItem{}
		return
	}

	// Snapshot current expand states
	expanded := make(map[string]bool)
	for _, item := range m.items {
		if item.isMilestone {
			expanded[item.milestoneName] = item.expanded
		}
	}

	var items []treeItem

	for mi, milestone := range m.mission.Milestones {
		features := getFeaturesForMilestone(m.mission, milestone.Name)
		total := len(features)
		completed := countCompleted(features)
		isExpanded := expanded[milestone.Name]

		item := treeItem{
			indent:         0,
			isMilestone:    true,
			milestoneIdx:   mi,
			milestoneName:  milestone.Name,
			label:          milestone.Name,
			expanded:       isExpanded,
			hasChildren:    total > 0,
			featureCount:   total,
			completedCount: completed,
		}
		items = append(items, item)

		// Add feature children if expanded
		if isExpanded {
			for fi := range features {
				f := &m.mission.Features[getFeatureIndex(m.mission, features[fi].ID)]
				fitem := treeItem{
					indent:        1,
					isFeature:     true,
					feature:       f,
					milestoneName: milestone.Name,
					label:         f.ID,
				}
				items = append(items, fitem)
			}
		}
	}

	m.items = items
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// selectedFeature returns the feature at the cursor position, or nil.
func (m Model) selectedFeature() *missions.Feature {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return nil
	}
	return m.items[m.cursor].feature
}

// getFeaturesForMilestone returns all features belonging to a milestone.
func getFeaturesForMilestone(mission *missions.Mission, milestoneName string) []missions.Feature {
	var result []missions.Feature
	for _, f := range mission.Features {
		if f.Milestone == milestoneName {
			result = append(result, f)
		}
	}
	return result
}

// getFeatureIndex returns the index of a feature in the mission's Features slice.
func getFeatureIndex(mission *missions.Mission, featureID string) int {
	for i, f := range mission.Features {
		if f.ID == featureID {
			return i
		}
	}
	return -1
}

// countCompleted counts how many features in the slice are completed.
func countCompleted(features []missions.Feature) int {
	count := 0
	for _, f := range features {
		if f.Status == missions.FeatureCompleted {
			count++
		}
	}
	return count
}

// statusIcon returns a status badge icon for the given feature status.
func statusIcon(status missions.FeatureStatus) string {
	switch status {
	case missions.FeaturePending:
		return "○"
	case missions.FeatureInProgress:
		return "●"
	case missions.FeatureCompleted:
		return "✓"
	case missions.FeatureFailed:
		return "✗"
	case missions.FeatureCancelled:
		return "⊘"
	default:
		return "?"
	}
}

// statusColor returns the lipgloss color string for a status.
func statusColor(status missions.FeatureStatus) string {
	switch status {
	case missions.FeaturePending:
		return "240" // grey
	case missions.FeatureInProgress:
		return "39" // blue
	case missions.FeatureCompleted:
		return "76" // green
	case missions.FeatureFailed:
		return "196" // red
	case missions.FeatureCancelled:
		return "208" // orange
	default:
		return "240"
	}
}

// statusColorName returns a human-readable color name.
func statusColorName(status missions.FeatureStatus) string {
	switch status {
	case missions.FeaturePending:
		return "grey"
	case missions.FeatureInProgress:
		return "blue"
	case missions.FeatureCompleted:
		return "green"
	case missions.FeatureFailed:
		return "red"
	case missions.FeatureCancelled:
		return "orange"
	default:
		return "grey"
	}
}

// Spinner frames for in-progress animation.
var spinnerFrames = []string{"◐", "◓", "◑", "◒"}

// spinner returns the current spinner frame based on tick count.
func spinner(tick int) string {
	return spinnerFrames[tick%len(spinnerFrames)]
}

// elapsedSince returns a human-readable elapsed time string.
func elapsedSince(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// missionStatusLabel returns a human-readable label for mission status.
func missionStatusLabel(s missions.MissionStatus) string {
	switch s {
	case missions.MissionPlanning:
		return "Planning"
	case missions.MissionActive:
		return "Active"
	case missions.MissionPaused:
		return "Paused"
	case missions.MissionCompleted:
		return "Completed"
	case missions.MissionFailed:
		return "Failed"
	case missions.MissionCancelled:
		return "Cancelled"
	case missions.MissionValidating:
		return "Validating"
	default:
		return string(s)
	}
}

// truncate truncates a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// filteredLogLines returns log entries matching the current filter level.
func (m Model) filteredLogLines() []LogEntry {
	if len(m.logLines) == 0 {
		return nil
	}

	var result []LogEntry
	for _, entry := range m.logLines {
		switch m.logFilter {
		case LogAll:
			result = append(result, entry)
		case LogInfo:
			if entry.Level == "info" || entry.Level == "warn" || entry.Level == "error" {
				result = append(result, entry)
			}
		case LogWarn:
			if entry.Level == "warn" || entry.Level == "error" {
				result = append(result, entry)
			}
		case LogError:
			if entry.Level == "error" {
				result = append(result, entry)
			}
		}
	}
	return result
}
