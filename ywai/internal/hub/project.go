package hub

import "time"

// Project represents a multi-repo project in the hub registry.
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	AgentType   string    `json:"agent_type"`
	SyncEnabled bool      `json:"sync_enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
