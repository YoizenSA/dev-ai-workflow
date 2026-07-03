package scheduler

import "time"

// SessionConfig defines the configuration for a scheduling session.
type SessionConfig struct {
	Goal  string `json:"goal"`
	Repo  string `json:"repo"`
	Agent string `json:"agent"`
}

// Schedule represents a recurring schedule.
type Schedule struct {
	ID        string        `json:"id"`
	Config    SessionConfig `json:"config"`
	CronExpr  string        `json:"cronExpr"`
	NextRun   time.Time     `json:"nextRun,omitzero"`
	Enabled   bool          `json:"enabled"`
	CreatedAt time.Time     `json:"createdAt,omitzero"`
}

// ScheduleRun represents a single execution of a schedule.
type ScheduleRun struct {
	ID         string    `json:"id"`
	ScheduleID string    `json:"scheduleId"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	Status     string    `json:"status"`
}
