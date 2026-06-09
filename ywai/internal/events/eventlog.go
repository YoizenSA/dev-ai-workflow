// Package events provides an append-only event log for mission state changes.
// This is the event sourcing layer for crash recovery and audit trails.
package events

import "time"

// EventType categorizes engine events for the append-only log.
type EventType string

const (
	// Mission lifecycle events
	EventMissionCreated   EventType = "mission.created"
	EventMissionStarted   EventType = "mission.started"
	EventMissionCompleted EventType = "mission.completed"
	EventMissionFailed    EventType = "mission.failed"
	EventMissionCancelled EventType = "mission.cancelled"
	EventMissionPaused    EventType = "mission.paused"
	EventMissionResumed   EventType = "mission.resumed"
	EventMissionValidated EventType = "mission.validated"
)

// Event is a single entry in the append-only event log.
type Event struct {
	ID        string    `json:"id"`                 // unique identifier
	SessionID string    `json:"session_id"`          // which session this belongs to
	Type      EventType `json:"type"`                // event category
	MissionID string    `json:"mission_id"`          // affected mission
	Data      []byte    `json:"data,omitempty"`      // optional payload (JSON)
	Sequence  int64     `json:"sequence"`            // monotonically increasing per session
	Timestamp time.Time `json:"timestamp"`           // when the event occurred
}

// Store is the interface for the append-only event log.
type Store interface {
	// Append writes a single event to the log. Must be thread-safe.
	Append(event Event) error
	// Replay returns all events for a session, ordered by sequence.
	Replay(sessionID string) ([]Event, error)
	// LastSequence returns the highest sequence for a session (0 if none).
	LastSequence(sessionID string) (int64, error)
	// PendingMissions returns mission IDs that were started but not completed/failed/cancelled.
	PendingMissions(sessionID string) ([]string, error)
}
