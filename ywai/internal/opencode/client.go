package opencode

import (
	"context"
	"errors"
)

// ─── Errors ────────────────────────────────────────────────────────────────

var (
	// ErrSessionsUnavailable is returned when session operations are attempted
	// on a client that doesn't support them (local/file-based).
	ErrSessionsUnavailable = errors.New("sessions require the opencode server to be running")
)

// Client provides access to opencode agents and models.
type Client interface {
	// ListAgents returns all available agents.
	ListAgents(ctx context.Context) ([]AgentInfo, error)
	// ListModels returns all available models.
	ListModels(ctx context.Context) ([]ModelInfo, error)
	// Status returns connectivity status of this client.
	Status(ctx context.Context) (ClientStatus, error)
	// Sessions returns the session management API.
	// Returns ErrSessionsUnavailable if the server is not reachable.
	Sessions() SessionAPI
}
