package kanban

import "time"

// Session represents a working session in the Kanban board.
type Session struct {
	ID        string    `json:"id"`
	Goal      string    `json:"goal"`
	Status    string    `json:"status"` // active, closed
	CreatedAt time.Time `json:"created_at"`
}

// Delegation represents a task delegated to an agent on the board.
type Delegation struct {
	ID             string     `json:"id"`
	SessionID      string     `json:"session_id"`
	Agent          string     `json:"agent"` // dev, qa, reviewer, architect, devops
	TaskSummary    string     `json:"task_summary"`
	Status         string     `json:"status"` // pending, running, review, changes, blocked, done
	Column         string     `json:"column"` // backlog, ready, in_progress, review, done
	Dependencies   []string   `json:"dependencies"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	HandoffPreview string     `json:"handoff_preview,omitempty"`
	Blocker        string     `json:"blocker,omitempty"`
}

// BoardUpdate is sent via WebSocket to notify clients of changes.
type BoardUpdate struct {
	Type    string      `json:"type"` // delegation.created, delegation.status_changed, etc.
	Payload interface{} `json:"payload"`
}

// BoardView groups delegations by column for a session.
type BoardView struct {
	Session    *Session              `json:"session"`
	Columns    map[string][]Delegation `json:"columns"`
}
