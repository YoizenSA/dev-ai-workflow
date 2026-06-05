package kanban

import "time"

// Session represents a working session in the Kanban board.
type Session struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"` // repo/project name
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
	PendingAction  bool       `json:"pending_action,omitempty"`
	LatestActivity string     `json:"latest_activity,omitempty"`
}

// ActivityType categorizes agent activity events.
type ActivityType string

const (
	ActivityProgress ActivityType = "progress"
	ActivityDecision ActivityType = "decision"
	ActivityQuestion ActivityType = "question"
	ActivityBlocked  ActivityType = "blocked"
)

// ActivityEvent represents a progress update, decision request, or question from an agent.
type ActivityEvent struct {
	ID           string       `json:"id"`
	DelegationID string       `json:"delegation_id"`
	Type         ActivityType `json:"type"`
	Content      string       `json:"content"`
	Options      []string     `json:"options,omitempty"`
	Resolution   string       `json:"resolution,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	ResolvedAt   *time.Time   `json:"resolved_at,omitempty"`
}

// BoardUpdate is sent via WebSocket to notify clients of changes.
type BoardUpdate struct {
	Type    string      `json:"type"` // delegation.created, delegation.status_changed, etc.
	Payload interface{} `json:"payload"`
}

// BoardView groups delegations by column for a session.
type BoardView struct {
	Session *Session                `json:"session"`
	Columns map[string][]Delegation `json:"columns"`
}

// GraphNode represents a delegation node in the dependency graph.
type GraphNode struct {
	ID          string `json:"id"`
	Agent       string `json:"agent"`
	TaskSummary string `json:"task_summary"`
	Status      string `json:"status"`
	Column      string `json:"column"`
}

// GraphEdge represents a dependency edge in the graph.
type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// GraphView is the full dependency graph for a session.
type GraphView struct {
	Session *Session    `json:"session"`
	Nodes   []GraphNode `json:"nodes"`
	Edges   []GraphEdge `json:"edges"`
}
