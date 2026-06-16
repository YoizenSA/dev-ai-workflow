// Package engram provides a client for the engram memory server REST API
// (default http://127.0.0.1:7437).
package engram

import "time"

// Status reports engram server connectivity.
type Status struct {
	Connected bool   `json:"connected"`
	Source    string `json:"source,omitempty"` // "server"
	Version   string `json:"version,omitempty"`
}

// Observation is a single engram memory record.
type Observation struct {
	ID             int    `json:"id"`
	SyncID         string `json:"sync_id,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	Type           string `json:"type,omitempty"`
	Title          string `json:"title,omitempty"`
	Content        string `json:"content,omitempty"`
	Project        string `json:"project,omitempty"`
	Scope          string `json:"scope,omitempty"`
	TopicKey       string `json:"topic_key,omitempty"`
	RevisionCount  int    `json:"revision_count,omitempty"`
	DuplicateCount int    `json:"duplicate_count,omitempty"`
	LastSeenAt     string `json:"last_seen_at,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

// Session is an engram session summary as returned by GET /sessions/recent.
type Session struct {
	ID               string `json:"id"`
	Project          string `json:"project,omitempty"`
	Directory        string `json:"directory,omitempty"`
	StartedAt        string `json:"started_at,omitempty"`
	ObservationCount int    `json:"observation_count"`
}

// Prompt is a stored user prompt as returned by GET /prompts/recent.
type Prompt struct {
	ID        int    `json:"id"`
	SyncID    string `json:"sync_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content,omitempty"`
	Project   string `json:"project,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
}

// Stats holds aggregate memory counts as returned by GET /stats.
type Stats struct {
	TotalSessions     int      `json:"total_sessions"`
	TotalObservations int      `json:"total_observations"`
	TotalPrompts      int      `json:"total_prompts"`
	Projects          []string `json:"projects,omitempty"`
}

// TimelineEvent is one entry in the memory timeline.
type TimelineEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type,omitempty"`
	Content   string    `json:"content,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

// ContextResult is what engram returns for GET /context.
// Engram returns a markdown string in the "context" field.
type ContextResult struct {
	Context string `json:"context,omitempty"`
}

// ImportResult is what engram returns for POST /import.
type ImportResult struct {
	SessionsImported     int `json:"sessions_imported"`
	ObservationsImported int `json:"observations_imported"`
	PromptsImported      int `json:"prompts_imported"`
}

// MergeProjectsResult reports how many records were re-tagged.
type MergeProjectsResult struct {
	Source              string `json:"source"`
	Target              string `json:"target"`
	ObservationsUpdated int    `json:"observations_updated"`
}

// ─── Request types ──────────────────────────────────────────────────────────

// SaveRequest is the body for POST /save.
type SaveRequest struct {
	Type    string `json:"type"`
	Title   string `json:"title,omitempty"`
	Content string `json:"content"`
	Scope   string `json:"scope,omitempty"`
	Project string `json:"project,omitempty"`
}

// UpdateRequest is the body for PATCH /observations/{id}. All fields optional.
type UpdateRequest struct {
	Content  *string `json:"content,omitempty"`
	Title    *string `json:"title,omitempty"`
	Type     *string `json:"type,omitempty"`
	Scope    *string `json:"scope,omitempty"`
	Project  *string `json:"project,omitempty"`
	TopicKey *string `json:"topic_key,omitempty"`
}

// SearchRequest drives GET /search.
type SearchRequest struct {
	Query string
	Limit int
	Type  string // optional filter
}

// TimelineRequest drives GET /timeline.
type TimelineRequest struct {
	ObservationID string // required by engram
	Limit         int
}

// ContextRequest drives GET /context.
type ContextRequest struct {
	Query string
	Limit int
}
