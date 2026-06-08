package missions

import (
	"fmt"
	"os"
)

// DefaultBaseDir is the default directory for all mission data.
const DefaultBaseDir = "~/.local/share/ywai/missions"

// ─── Public API ────────────────────────────────────────────────────────────

// StartInteractivePlanning begins an interactive planning session.
// It creates a new MissionsStore, runs the interactive dialog, and returns
// the approved mission or an error.
//
// The planning dialog:
//  1. Prompts the user for a goal
//  2. Asks optional clarifying questions
//  3. Generates a structured plan (milestones + features)
//  4. Presents the plan for approval
//  5. On approval: saves the mission and transitions to "active"
//  6. On rejection: collects feedback and regenerates the plan
func StartInteractivePlanning(store *MissionsStore) (*Mission, error) {
	return RunInteractivePlanning(store, os.Stdin, os.Stdout)
}

// ─── Store Creation Helper ─────────────────────────────────────────────────

// OpenStore opens (or creates) the missions store at the default location.
func OpenStore() (*MissionsStore, error) {
	baseDir, err := expandPath(DefaultBaseDir)
	if err != nil {
		return nil, fmt.Errorf("expand missions dir: %w", err)
	}

	store := NewMissionsStore(baseDir)
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create missions dir: %w", err)
	}

	return store, nil
}

// expandPath expands the leading ~ to the user's home directory.
func expandPath(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}

	return home + path[1:], nil
}
