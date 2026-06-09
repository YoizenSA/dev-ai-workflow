package opencode

import "context"

// Client provides access to opencode agents and models.
type Client interface {
	// ListAgents returns all available agents.
	ListAgents(ctx context.Context) ([]AgentInfo, error)
	// ListModels returns all available models.
	ListModels(ctx context.Context) ([]ModelInfo, error)
	// Status returns connectivity status of this client.
	Status(ctx context.Context) (ClientStatus, error)
}
