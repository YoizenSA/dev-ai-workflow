package opencode

import "context"

// ─── Errors ────────────────────────────────────────────────────────────────

// Client provides access to opencode agents and models.
type Client interface {
	// ListAgents returns all available agents.
	ListAgents(ctx context.Context) ([]AgentInfo, error)
	// ListModels returns all available models.
	ListModels(ctx context.Context) ([]ModelInfo, error)
	// Status returns connectivity status of this client.
	Status(ctx context.Context) (ClientStatus, error)
	// Sessions returns the session management API, or nil when this client
	// cannot manage sessions (local/file-based client, no server running).
	Sessions() SessionAPI
}
