package kanban

import (
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
)

// Session represents a working session in the Kanban board.
type Session struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"` // repo/project name
	Goal      string    `json:"goal"`
	Status    string    `json:"status"` // active, closed
	CreatedAt time.Time `json:"created_at"`
}

// matchQuery checks if the session matches a search query (case-insensitive).
func (s *Session) matchQuery(query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	if strings.Contains(strings.ToLower(s.Goal), q) {
		return true
	}
	if strings.Contains(strings.ToLower(s.Project), q) {
		return true
	}
	if strings.Contains(strings.ToLower(s.ID), q) {
		return true
	}
	return false
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

// DerivedColumn returns the kanban column derived from the delegated FSM status.
func (d *Delegation) DerivedColumn() string {
	return MapFSMToKanbanColumn(missions.MissionStatus(d.Status))
}

// DerivedStatus returns the kanban-equivalent status for the stored FSM status.
// This is useful for display/backward-compat when consuming delegation JSON.
func (d *Delegation) DerivedStatus() string {
	return MapFSMToKanbanStatus(missions.MissionStatus(d.Status))
}

// MapKanbanStatusToFSM converts a kanban delegation status to engine MissionStatus.
func MapKanbanStatusToFSM(status string) missions.MissionStatus {
	switch status {
	case "pending":
		return missions.MissionPending
	case "running":
		return missions.MissionActive
	case "review":
		return missions.MissionValidating
	case "changes":
		return missions.MissionPaused
	case "blocked":
		return missions.MissionPaused
	case "done":
		return missions.MissionCompleted
	default:
		return missions.MissionPending
	}
}

// MapFSMToKanbanColumn derives the kanban column from an engine MissionStatus.
func MapFSMToKanbanColumn(status missions.MissionStatus) string {
	switch status {
	case missions.MissionPending, missions.MissionPlanning:
		return "backlog"
	case missions.MissionActive:
		return "in_progress"
	case missions.MissionPaused:
		return "blocked"
	case missions.MissionValidating:
		return "review"
	case missions.MissionCompleted:
		return "done"
	case missions.MissionFailed:
		return "blocked"
	case missions.MissionCancelled:
		return "backlog"
	default:
		return "backlog"
	}
}

// MapFSMToKanbanStatus converts an engine MissionStatus back to the kanban delegation status.
func MapFSMToKanbanStatus(status missions.MissionStatus) string {
	switch status {
	case missions.MissionPending:
		return "pending"
	case missions.MissionPlanning:
		return "pending"
	case missions.MissionActive:
		return "running"
	case missions.MissionPaused:
		return "blocked"
	case missions.MissionValidating:
		return "review"
	case missions.MissionCompleted:
		return "done"
	case missions.MissionFailed:
		return "blocked"
	case missions.MissionCancelled:
		return "pending"
	default:
		return "pending"
	}
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
	ID             string `json:"id"`
	Agent          string `json:"agent"`
	TaskSummary    string `json:"task_summary"`
	Status         string `json:"status"`
	Column         string `json:"column"`
	HandoffPreview string `json:"handoff_preview,omitempty"`
	PendingAction  bool   `json:"pending_action,omitempty"`
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
