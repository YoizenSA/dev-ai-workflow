package engram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
)

// ErrEngramUnavailable is returned when engram is not reachable.
var ErrEngramUnavailable = errors.New("engram server is not running")

// Client provides access to the engram memory server REST API.
type Client interface {
	Status(ctx context.Context) (Status, error)
	RecentObservations(ctx context.Context, limit int) ([]Observation, error)
	GetObservation(ctx context.Context, id string) (Observation, error)
	UpdateObservation(ctx context.Context, id string, req UpdateRequest) (Observation, error)
	DeleteObservation(ctx context.Context, id string) error
	Save(ctx context.Context, req SaveRequest) (Observation, error)
	Search(ctx context.Context, req SearchRequest) ([]Observation, error)
	GetStats(ctx context.Context) (Stats, error)
	RecentSessions(ctx context.Context, limit int) ([]Session, error)
	DeleteSession(ctx context.Context, id string) error
	RecentPrompts(ctx context.Context, limit int) ([]Prompt, error)
	DeletePrompt(ctx context.Context, id string) error
	Timeline(ctx context.Context, req TimelineRequest) ([]TimelineEvent, error)
	GetContext(ctx context.Context, req ContextRequest) (ContextResult, error)
	UpdateContext(ctx context.Context, text string) (ContextResult, error)
	Export(ctx context.Context) (json.RawMessage, error)
	Import(ctx context.Context, body io.Reader) (ImportResult, error)
	MergeProjects(ctx context.Context, source, target string) (MergeProjectsResult, error)
}
