package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
)

// ─── Style Definitions ─────────────────────────────────────────────────────
//
// Colors are chosen to be readable in both light and dark terminals.
// Foreground colours with explicit backgrounds ensure contrast.

var (
	// ── Base ────────────────────────────────────────────────────────
	baseStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// ── Header ──────────────────────────────────────────────────────
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("33")).
			Padding(0, 1).
			Width(100)

	// ── Tree Panel ──────────────────────────────────────────────────
	treeTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("33")).
			Padding(0, 1)

	treePanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("33"))

	// Selected row
	selectedStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("33")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1)

	// Normal row
	itemStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// ── Detail Panel ────────────────────────────────────────────────
	detailTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("33")).
				Padding(0, 1)

	detailPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("33"))

	infoKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("244"))

	infoValStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))

	// ── Log Area ────────────────────────────────────────────────────
	logInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	logWarnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	logErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	logTimestampStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	// ── Controls ────────────────────────────────────────────────────
	btnActiveStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("33")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1).
			MarginRight(1)

	btnDisabledStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("243")).
				Padding(0, 1).
				MarginRight(1)

	// ── Status Bar ──────────────────────────────────────────────────
	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	statusItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	statusKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	// ── Badges ──────────────────────────────────────────────────────
	pendingBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("240")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1).
			Render(" ○ PENDING ")

	inProgressBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("39")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1).
			Render(" ● IN PROGRESS ")

	completedBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("76")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1).
			Render(" ✓ COMPLETED ")

	failedBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("196")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1).
			Render(" ✗ FAILED ")

	cancelledBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("208")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1).
			Render(" ⊘ CANCELLED ")

	unknownBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("240")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 1).
			Render(" ? UNKNOWN ")

	// ── Help Overlay ────────────────────────────────────────────────
	helpStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("33")).
			Padding(1, 2).
			Width(50)

	helpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("33"))

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	// ── Notification ────────────────────────────────────────────────
	notifStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("33")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 2)

	notifErrorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("196")).
			Foreground(lipgloss.Color("15")).
			Padding(0, 2)

	// ── Confirmation Dialog ─────────────────────────────────────────
	confirmStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("214")).
			Padding(1, 2).
			Width(44)

	confirmTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15"))

	// ── Small Terminal Warning ──────────────────────────────────────
	warningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("196")).
			Padding(1, 2).
			Align(lipgloss.Center)
)

// ─── View ──────────────────────────────────────────────────────────────────

// View renders the complete TUI layout.
func (m Model) View() string {
	// Small terminal warning
	if m.windowTooSmall {
		return m.renderSmallTerminalWarning()
	}

	// If help overlay is shown
	if m.showHelp {
		return m.renderWithOverlay(m.renderHelp())
	}

	// Calculate available height (status bar at bottom, header at top)
	headerHeight := 2 // header + separator line
	statusBarHeight := 1
	availHeight := m.height - headerHeight - statusBarHeight
	availWidth := m.width

	// Header
	header := m.renderHeader()

	// Split into tree (left 40%) and detail (right 60%)
	treeWidth := availWidth * 40 / 100
	detailWidth := availWidth - treeWidth - 2 // -2 for borders/margins
	if treeWidth < 30 {
		treeWidth = 30
	}
	if detailWidth < 40 {
		detailWidth = 40
	}
	if treeWidth+detailWidth > availWidth {
		detailWidth = availWidth - treeWidth
	}
	if detailWidth < 10 {
		detailWidth = 10
	}

	treePanel := m.renderTreePanel(treeWidth, availHeight)
	detailPanel := m.renderDetailPanel(detailWidth, availHeight)

	// Main content
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		treePanel,
		"  ",
		detailPanel,
	)

	// Status bar
	statusBar := m.renderStatusBar(availWidth)

	// Notification overlay (if active)
	if m.notification != "" {
		notif := m.renderNotification(availWidth)
		content = content + "\n" + notif
	}

	// Confirmation dialog overlay
	if m.confirmShow {
		confirm := m.renderConfirmation()
		// Center the dialog over the content
		dialog := m.renderCenteredOverlay(confirm)
		return header + "\n" + dialog + "\n" + statusBar
	}

	return header + "\n" + content + "\n" + statusBar
}

// ─── Header ────────────────────────────────────────────────────────────────

func (m Model) renderHeader() string {
	name := "Mission Control"
	if m.mission != nil {
		name = "Mission Control — " + m.mission.Name
	}
	return headerStyle.Width(m.width - 2).Render(name)
}

// ─── Feature Tree Panel ────────────────────────────────────────────────────

func (m Model) renderTreePanel(width, height int) string {
	title := treeTitleStyle.Width(width - 2).Render(" Features ")

	var body strings.Builder

	if len(m.items) == 0 {
		body.WriteString("  No features defined\n")
	} else {
		for i, item := range m.items {
			line := m.renderTreeItem(i, item, width-2)
			if line == "" {
				continue
			}

			if i == m.cursor && m.focus == FocusTree {
				line = selectedStyle.Width(width - 2).Render(line)
			} else {
				line = itemStyle.Render(line)
			}
			body.WriteString(line + "\n")
		}
	}

	// Fill remaining height
	lines := strings.Split(strings.TrimRight(body.String(), "\n"), "\n")
	contentHeight := len(lines)
	for i := contentHeight; i < height-2; i++ {
		body.WriteString(strings.Repeat(" ", width-2) + "\n")
	}

	panel := treePanelStyle.Width(width).Height(height).Render(title + "\n" + body.String())
	return panel
}

func (m Model) renderTreeItem(index int, item treeItem, maxWidth int) string {
	indent := strings.Repeat("  ", item.indent)
	prefix := "  "

	if item.isMilestone {
		// Aggregate status
		aggIcon := m.milestoneAggregateIcon(item)
		countStr := fmt.Sprintf("(%d/%d)", item.completedCount, item.featureCount)

		if item.hasChildren {
			if item.expanded {
				prefix = "▼ "
			} else {
				prefix = "▶ "
			}
		} else {
			prefix = "  "
		}

		label := fmt.Sprintf("%s%s %s %s", indent, prefix, item.milestoneName, countStr)
		// Add aggregate status
		label = label + " " + aggIcon

		return truncate(label, maxWidth)
	}

	if item.isFeature && item.feature != nil {
		badge := " " + statusIcon(item.feature.Status) + " "
		label := fmt.Sprintf("%s%s %s", indent, badge, item.label)
		return truncate(label, maxWidth)
	}

	return ""
}

func (m Model) milestoneAggregateIcon(item treeItem) string {
	if item.completedCount == 0 && item.featureCount > 0 {
		return "○" // All pending
	}
	if item.completedCount == item.featureCount && item.featureCount > 0 {
		return "✓" // All completed
	}
	// Check if any in-progress
	for _, f := range getFeaturesForMilestone(m.mission, item.milestoneName) {
		if f.Status == missions.FeatureInProgress {
			return "●"
		}
		if f.Status == missions.FeatureFailed {
			return "✗"
		}
	}
	if item.completedCount > 0 {
		return "◔" // Partial
	}
	return "○"
}

// ─── Detail Panel ──────────────────────────────────────────────────────────

func (m Model) renderDetailPanel(width, height int) string {
	title := detailTitleStyle.Width(width - 2).Render(" Detail ")
	var body strings.Builder

	sel := m.selectedFeature()
	if sel == nil {
		// Show mission summary when no feature is selected
		body.WriteString(m.renderMissionSummary(width - 4))
	} else {
		body.WriteString(m.renderFeatureInfo(sel, width-4))
		body.WriteString("\n")
		body.WriteString(m.renderControls(sel, width-4))
		body.WriteString("\n")
		body.WriteString(m.renderLogArea(width-4, height-12))
	}

	// Fill remaining space
	lines := strings.Split(strings.TrimRight(body.String(), "\n"), "\n")
	contentHeight := len(lines)
	for i := contentHeight; i < height-2; i++ {
		body.WriteString("\n")
	}

	panel := detailPanelStyle.Width(width).Height(height).Render(title + "\n" + body.String())
	return panel
}

func (m Model) renderMissionSummary(width int) string {
	if m.mission == nil {
		return "  No mission loaded.\n"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("  Status: %s\n\n", missionStatusLabel(m.mission.Status)))

	// Count features by status
	total := len(m.mission.Features)
	completed := 0
	inProgress := 0
	failed := 0
	pending := 0
	for _, f := range m.mission.Features {
		switch f.Status {
		case missions.FeatureCompleted:
			completed++
		case missions.FeatureInProgress:
			inProgress++
		case missions.FeatureFailed:
			failed++
		case missions.FeaturePending:
			pending++
		}
	}

	b.WriteString(fmt.Sprintf("  Milestones: %d\n", len(m.mission.Milestones)))
	b.WriteString(fmt.Sprintf("  Features:   %d total\n", total))
	b.WriteString(fmt.Sprintf("  ✓ Completed: %d\n", completed))
	b.WriteString(fmt.Sprintf("  ● In Progress: %d\n", inProgress))
	b.WriteString(fmt.Sprintf("  ✗ Failed: %d\n", failed))
	b.WriteString(fmt.Sprintf("  ○ Pending: %d\n", pending))

	// Validation state
	if m.validationState != nil {
		b.WriteString(fmt.Sprintf("\n  ── Validation ──\n"))
		assertions := m.validationState.Assertions
		passed := 0
		failed := 0
		for _, a := range assertions {
			switch a.Status {
			case missions.ValidationPassed:
				passed++
			case missions.ValidationFailed:
				failed++
			}
		}
		b.WriteString(fmt.Sprintf("  ✓ Passed: %d\n", passed))
		b.WriteString(fmt.Sprintf("  ✗ Failed: %d\n", failed))
		b.WriteString(fmt.Sprintf("  📊 Total: %d\n", len(assertions)))
		if failed > 0 {
			b.WriteString(fmt.Sprintf("\n  Failed assertions:\n"))
			for _, a := range assertions {
				if a.Status == missions.ValidationFailed {
					msg := a.Error
					if msg == "" {
						msg = "failed"
					}
					b.WriteString(fmt.Sprintf("    ✗ %s: %s\n", a.ID, msg))
				}
			}
		}
	}

	return b.String()
}

func (m Model) renderFeatureInfo(feature *missions.Feature, width int) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("  %s\n\n", infoKeyStyle.Render("Feature:")) + "   " + infoValStyle.Render(feature.ID) + "\n")

	if feature.Description != "" {
		b.WriteString(fmt.Sprintf("  %s\n   %s\n", infoKeyStyle.Render("Description:"), truncate(feature.Description, width-5)))
	}

	b.WriteString(fmt.Sprintf("  %s\n   %s\n", infoKeyStyle.Render("Status:"), m.statusBadge(feature.Status)))
	b.WriteString(fmt.Sprintf("  %s\n   %s\n", infoKeyStyle.Render("Milestone:"), feature.Milestone))

	if feature.SkillName != "" {
		b.WriteString(fmt.Sprintf("  %s\n   %s\n", infoKeyStyle.Render("Agent:"), feature.SkillName))
	}

	if feature.RetryCount > 0 {
		b.WriteString(fmt.Sprintf("  %s\n   %d\n", infoKeyStyle.Render("Retries:"), feature.RetryCount))
	}

	return b.String()
}

// statusBadge returns a formatted status badge string.
func (m Model) statusBadge(status missions.FeatureStatus) string {
	switch status {
	case missions.FeaturePending:
		return pendingBadge
	case missions.FeatureInProgress:
		return inProgressBadge
	case missions.FeatureCompleted:
		return completedBadge
	case missions.FeatureFailed:
		return failedBadge
	case missions.FeatureCancelled:
		return cancelledBadge
	default:
		return unknownBadge
	}
}

// ─── Controls ──────────────────────────────────────────────────────────────

func (m Model) renderControls(feature *missions.Feature, width int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("  %s\n", infoKeyStyle.Render("Controls:")))

	buttons := "  "
	hasControls := false

	// Pause/Resume button (from mission level)
	if m.mission != nil {
		if m.mission.Status == missions.MissionActive || m.mission.Status == missions.MissionPlanning {
			buttons += btnActiveStyle.Render("[p] Pause")
			hasControls = true
		} else if m.mission.Status == missions.MissionPaused {
			buttons += btnActiveStyle.Render("[p] Resume")
			hasControls = true
		} else {
			buttons += btnDisabledStyle.Render(" Pause ")
			hasControls = true
		}
	}

	// Retry button (visible on failed features)
	if feature.Status == missions.FeatureFailed {
		buttons += btnActiveStyle.Render("[r] Retry")
		hasControls = true
	}

	// Cancel button (visible on in-progress features)
	if feature.Status == missions.FeatureInProgress {
		buttons += btnActiveStyle.Render("[c] Cancel")
		hasControls = true
	}

	if !hasControls {
		b.WriteString("  (no actions available)\n")
	} else {
		b.WriteString(buttons + "\n")
	}

	return b.String()
}

// ─── Log Area ──────────────────────────────────────────────────────────────

func (m Model) renderLogArea(width, height int) string {
	var b strings.Builder

	currentFilter := ""
	switch m.logFilter {
	case LogAll:
		currentFilter = "ALL"
	case LogInfo:
		currentFilter = "INFO+"
	case LogWarn:
		currentFilter = "WARN+"
	case LogError:
		currentFilter = "ERR"
	}

	b.WriteString(fmt.Sprintf("  %s  Filter: %s\n", infoKeyStyle.Render("Logs:"), currentFilter))

	if len(m.logLines) == 0 {
		placeholder := "    No log output yet."
		if sel := m.selectedFeature(); sel != nil {
			if sel.Status == missions.FeaturePending {
				placeholder = "    Feature pending — waiting for execution."
			} else if sel.Status == missions.FeatureCompleted {
				placeholder = "    Feature completed — no active log stream."
			} else if sel.Status == missions.FeatureFailed {
				placeholder = "    Feature failed — check log file for details."
			} else if sel.Status == missions.FeatureCancelled {
				placeholder = "    Feature cancelled."
			}
		}
		b.WriteString("  " + logInfoStyle.Render(placeholder) + "\n")
	} else {
		// Render filtered log lines
		lines := m.filteredLogLines()
		maxLines := height - 3 // header + filter + padding
		if maxLines < 1 {
			maxLines = 1
		}

		start := 0
		totalLines := len(lines)
		if totalLines > maxLines {
			if m.logAutoScroll {
				start = totalLines - maxLines
			} else {
				// Apply scroll offset: logScrollOffset is how many lines above the bottom
				start = totalLines - maxLines - m.logScrollOffset
				if start < 0 {
					start = 0
				}
				// Re-enable auto-scroll if we've scrolled to the bottom
				if start >= totalLines-maxLines {
					m.logAutoScroll = true
				}
			}
		}

		for i := start; i < totalLines && i-start < maxLines; i++ {
			entry := lines[i]
			lineStyle := logInfoStyle
			switch entry.Level {
			case "warn":
				lineStyle = logWarnStyle
			case "error":
				lineStyle = logErrorStyle
			}

			// Format timestamp
			ts := entry.Timestamp.Format("15:04:05")
			tsStr := logTimestampStyle.Render(ts) + " "
			// Truncate message to fit within the remaining width
			msgWidth := width - 2 - len(ts) - 1
			if msgWidth < 10 {
				msgWidth = 10
			}
			msg := truncate(entry.Message, msgWidth)
			b.WriteString("  " + tsStr + lineStyle.Render(msg) + "\n")
		}
	}

	return b.String()
}

// ─── Status Bar ────────────────────────────────────────────────────────────

func (m Model) renderStatusBar(width int) string {
	if width < 10 {
		width = 80
	}

	// Elapsed time
	elapsed := elapsedSince(m.startTime)

	// Feature counts
	var completedCount, totalCount int
	if m.mission != nil {
		totalCount = len(m.mission.Features)
		for _, f := range m.mission.Features {
			if f.Status == missions.FeatureCompleted {
				completedCount++
			}
		}
	}

	// Mission status
	statusLabel := ""
	if m.mission != nil {
		statusLabel = missionStatusLabel(m.mission.Status)
	}

	// Worker count (in-progress features)
	workerCount := 0
	if m.mission != nil {
		for _, f := range m.mission.Features {
			if f.Status == missions.FeatureInProgress {
				workerCount++
			}
		}
	}

	// Build status bar sections
	leftItems := []string{
		statusItemStyle.Render(fmt.Sprintf(" ⏱ %s", elapsed)),
		statusItemStyle.Render(fmt.Sprintf(" Features: %d/%d", completedCount, totalCount)),
		statusItemStyle.Render(fmt.Sprintf(" %s", statusLabel)),
	}

	if workerCount > 0 {
		leftItems = append(leftItems, statusItemStyle.Render(fmt.Sprintf(" Workers: %d", workerCount)))
	}

	left := strings.Join(leftItems, "  │")

	rightItems := []string{
		statusItemStyle.Render("? Help"),
		statusItemStyle.Render("q Quit"),
	}

	// Build with flex-like spacing
	right := strings.Join(rightItems, "  ")

	padding := width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if padding < 1 {
		padding = 1
	}

	bar := left + strings.Repeat(" ", padding) + right
	return statusBarStyle.Width(width).Render(bar)
}

// ─── Help Overlay ──────────────────────────────────────────────────────────

func (m Model) renderHelp() string {
	var b strings.Builder
	b.WriteString(helpStyle.Render(
		"                    " + lipgloss.NewStyle().Bold(true).Render("Keyboard Shortcuts") + "\n\n" +
			helpKeyStyle.Render("↑/↓ or j/k") + "    " + helpDescStyle.Render("Navigate tree") + "\n" +
			helpKeyStyle.Render("→/Enter/Space") + " " + helpDescStyle.Render("Expand milestone") + "\n" +
			helpKeyStyle.Render("←") + "            " + helpDescStyle.Render("Collapse milestone") + "\n" +
			helpKeyStyle.Render("Home/End") + "      " + helpDescStyle.Render("First/Last item") + "\n" +
			helpKeyStyle.Render("Tab") + "           " + helpDescStyle.Render("Switch panel focus") + "\n" +
			helpKeyStyle.Render("p") + "             " + helpDescStyle.Render("Pause/Resume mission") + "\n" +
			helpKeyStyle.Render("r") + "             " + helpDescStyle.Render("Retry failed feature") + "\n" +
			helpKeyStyle.Render("c") + "             " + helpDescStyle.Render("Cancel in-progress") + "\n" +
			helpKeyStyle.Render("1-4") + "           " + helpDescStyle.Render("Log filter level") + "\n" +
			helpKeyStyle.Render("PgUp/PgDn") + "    " + helpDescStyle.Render("Scroll log") + "\n" +
			helpKeyStyle.Render("q/Ctrl+C") + "     " + helpDescStyle.Render("Quit") + "\n" +
			helpKeyStyle.Render("?") + "             " + helpDescStyle.Render("Toggle this help") + "\n" +
			helpKeyStyle.Render("Esc") + "           " + helpDescStyle.Render("Close overlay") + "\n",
	))
	return b.String()
}

// ─── Confirmation Dialog ───────────────────────────────────────────────────

func (m Model) renderConfirmation() string {
	return confirmStyle.Render(
		confirmTextStyle.Render(m.confirmMsg) + "\n\n" +
			btnActiveStyle.Render(" [y] Yes ") + "  " +
			btnActiveStyle.Render(" [n] No "),
	)
}

// ─── Notification ──────────────────────────────────────────────────────────

func (m Model) renderNotification(width int) string {
	style := notifStyle
	if strings.Contains(strings.ToLower(m.notification), "error") {
		style = notifErrorStyle
	}
	return style.Width(width - 2).Render(" " + m.notification + " ")
}

// ─── Small Terminal Warning ────────────────────────────────────────────────

func (m Model) renderSmallTerminalWarning() string {
	msg := fmt.Sprintf(" Terminal too small!\n\n Minimum: %d×%d\n Current: %d×%d\n\n Please enlarge the terminal window.",
		minWidth, minHeight, m.width, m.height)
	return warningStyle.Width(m.width - 2).Render(msg)
}

// ─── Overlay Helpers ───────────────────────────────────────────────────────

func (m Model) renderWithOverlay(overlay string) string {
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		overlay,
	)
}

// renderMainView renders just the main layout without overlay for centering.
func (m Model) renderMainView() string {
	if m.windowTooSmall {
		return m.renderSmallTerminalWarning()
	}

	headerHeight := 2
	statusBarHeight := 1
	availHeight := m.height - headerHeight - statusBarHeight
	availWidth := m.width

	header := m.renderHeader()
	treeWidth := availWidth * 40 / 100
	detailWidth := availWidth - treeWidth - 2
	if treeWidth < 30 {
		treeWidth = 30
	}
	if detailWidth < 40 {
		detailWidth = 40
	}
	if treeWidth+detailWidth > availWidth {
		detailWidth = availWidth - treeWidth
	}
	if detailWidth < 10 {
		detailWidth = 10
	}

	treePanel := m.renderTreePanel(treeWidth, availHeight)
	detailPanel := m.renderDetailPanel(detailWidth, availHeight)

	content := lipgloss.JoinHorizontal(lipgloss.Top, treePanel, "  ", detailPanel)
	statusBar := m.renderStatusBar(availWidth)

	return header + "\n" + content + "\n" + statusBar
}

// renderCenteredOverlay centers a dialog over the main content.
func (m Model) renderCenteredOverlay(dialog string) string {
	// Create an empty content area and place the dialog in the center
	contentWidth := m.width

	// Render the main view content area (without header/status)
	headerHeight := 2
	statusBarHeight := 1
	contentHeight := m.height - headerHeight - statusBarHeight

	treeWidth := contentWidth * 40 / 100
	detailWidth := contentWidth - treeWidth - 2
	if treeWidth < 30 {
		treeWidth = 30
	}
	if detailWidth < 40 {
		detailWidth = 40
	}
	if treeWidth+detailWidth > contentWidth {
		detailWidth = contentWidth - treeWidth
	}
	if detailWidth < 10 {
		detailWidth = 10
	}

	treePanel := m.renderTreePanel(treeWidth, contentHeight)
	detailPanel := m.renderDetailPanel(detailWidth, contentHeight)
	content := lipgloss.JoinHorizontal(lipgloss.Top, treePanel, "  ", detailPanel)

	// Place dialog over the content area
	placed := lipgloss.Place(
		lipgloss.Width(content),
		lipgloss.Height(content),
		lipgloss.Center, lipgloss.Center,
		dialog,
	)

	return placed
}
