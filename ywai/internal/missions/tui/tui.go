// Package tui implements a Bubble Tea terminal UI for ywai Mission Control.
//
// It provides a real-time dashboard with:
//   - Feature tree with milestone hierarchy, expand/collapse, and status badges
//   - Detail panel showing feature information, controls, and logs
//   - Status bar with elapsed time, feature counts, and keyboard shortcuts
//   - Pause/Resume, Retry, and Cancel controls
//   - Log streaming with color coding and filtering
package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
)

// Run starts the Mission Control TUI for the given mission.
// It loads the mission from the store, renders the dashboard, and
// blocks until the user quits (q or Ctrl+C).
//
// The TUI reads state from the store on a periodic tick and never
// modifies the store directly — actions are dispatched through the
// missions package's exported queue and FSM functions.
func Run(store *missions.MissionsStore, missionID string) error {
	m, err := NewModel(store, missionID)
	if err != nil {
		return fmt.Errorf("initialize TUI: %w", err)
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("run TUI: %w", err)
	}
	return nil
}

// NewModel creates a new Model for the given mission.
// It loads the mission from the store and builds the initial tree.
func NewModel(store *missions.MissionsStore, missionID string) (Model, error) {
	mission, err := store.LoadMission(missionID)
	if err != nil {
		return Model{}, fmt.Errorf("load mission %q: %w", missionID, err)
	}

	m := Model{
		store:          store,
		missionID:      missionID,
		mission:        mission,
		startTime:      mission.CreatedAt,
		paused:         mission.Status == missions.MissionPaused,
		items:          []treeItem{},
		cursor:         0,
		focus:          FocusTree,
		logLines:       []LogEntry{},
		logFilter:      LogAll,
		logAutoScroll:  true,
		showHelp:       false,
		tickInterval:   tickIntervalDefault,
		windowTooSmall: false,
		width:          80,
		height:         24,
	}

	m.buildTree()
	return m, nil
}

// MustRun is like Run but calls os.Exit(1) on error.
func MustRun(store *missions.MissionsStore, missionID string) {
	if err := Run(store, missionID); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
