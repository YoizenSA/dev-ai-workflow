package opencode

import (
	"context"
	"encoding/json"
)

// ─── Session API Interface ─────────────────────────────────────────────────

// SessionAPI provides programmatic control over opencode sessions via the HTTP API.
// This is used by missions to create sessions, send prompts, and collect results
// without spawning CLI subprocesses.
type SessionAPI interface {
	// Create creates a new opencode session.
	Create(ctx context.Context, opts SessionCreateOpts) (*Session, error)

	// Get retrieves a session by ID.
	Get(ctx context.Context, sessionID string) (*Session, error)

	// Status returns the status of all sessions.
	Status(ctx context.Context) (*SessionStatusResult, error)

	// Prompt sends a message to a session and returns the result.
	Prompt(ctx context.Context, sessionID string, input PromptInput) (*PromptResult, error)

	// Wait blocks until the session finishes processing.
	Wait(ctx context.Context, sessionID string) error

	// Messages retrieves all messages for a session.
	Messages(ctx context.Context, sessionID string) ([]Message, error)

	// Delete removes a session.
	Delete(ctx context.Context, sessionID string) error

	// ListQuestions returns pending questions from the assistant.
	ListQuestions(ctx context.Context) ([]Question, error)

	// ReplyQuestion answers a pending question.
	ReplyQuestion(ctx context.Context, questionID string, answer string) error
}

// ─── Session Types ─────────────────────────────────────────────────────────

// SessionCreateOpts configures session creation.
type SessionCreateOpts struct {
	Title     string      `json:"title,omitempty"`
	Agent     string      `json:"agent,omitempty"`
	Model     *ModelInput `json:"model,omitempty"`
	ParentID  string      `json:"parentID,omitempty"`
	Directory string      `json:"directory,omitempty"`
	Workspace string      `json:"workspace,omitempty"`
}

// ModelInput specifies which model a session should use.
type ModelInput struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
	Variant    string `json:"variant,omitempty"`
}

// Session represents an opencode session.
//
// Model mirrors the session-creation request shape: a {id, providerID, variant}
// object. Older opencode servers (and some list endpoints) emit it as a bare
// string or omit it; json.RawMessage lets us decode leniently without failing
// the whole response. Use SessionModel() to read a normalised string.
type Session struct {
	ID        string          `json:"id"`
	Title     string          `json:"title,omitempty"`
	Agent     string          `json:"agent,omitempty"`
	Model     json.RawMessage `json:"model,omitempty"`
	ParentID  string          `json:"parentID,omitempty"`
	CreatedAt int64           `json:"createdAt,omitempty"`
	UpdatedAt int64           `json:"updatedAt,omitempty"`
}

// SessionModel returns the session's model as a "providerID/id" style string,
// tolerating the three shapes opencode emits: omitted/null, a bare string, or
// a {id, providerID} object. Returns "" when there is no model.
func (s Session) SessionModel() string {
	if len(s.Model) == 0 {
		return ""
	}
	// Object form: {"id": "...", "providerID": "..."}
	var obj struct {
		ID         string `json:"id"`
		ProviderID string `json:"providerID"`
	}
	if err := json.Unmarshal(s.Model, &obj); err == nil && obj.ID != "" {
		if obj.ProviderID != "" {
			return obj.ProviderID + "/" + obj.ID
		}
		return obj.ID
	}
	// Bare-string form.
	var s2 string
	if err := json.Unmarshal(s.Model, &s2); err == nil {
		return s2
	}
	return ""
}

// SessionStatusResult holds the result of GET /session/status.
type SessionStatusResult struct {
	Sessions []SessionStatusEntry `json:"sessions,omitempty"`
}

// SessionStatusEntry is a single session's status summary.
type SessionStatusEntry struct {
	ID     string `json:"id"`
	Status string `json:"status,omitempty"`
	Title  string `json:"title,omitempty"`
}

// ─── Prompt Types ──────────────────────────────────────────────────────────

// PromptInput is the request body for sending a message to a session.
type PromptInput struct {
	Text     string `json:"text"`
	Delivery string `json:"delivery,omitempty"` // "steer" (default) | "queue"
}

// PromptResult is returned after sending a prompt.
type PromptResult struct {
	MessageID string `json:"id,omitempty"`
	SessionID string `json:"sessionID,omitempty"`
}

// ─── Message Types ─────────────────────────────────────────────────────────

// Message represents a message in a session (user or assistant).
type Message struct {
	ID        string `json:"id"`
	Role      string `json:"role,omitempty"` // "user" | "assistant"
	Text      string `json:"text,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

// ─── Question Types ────────────────────────────────────────────────────────

// Question represents a pending question from the assistant (ask_user_question).
type Question struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionID,omitempty"`
	Text      string `json:"text,omitempty"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}
