package opencode

import "context"

// localSessionAPI is a stub that returns errors because the local client
// (file-based config) does not support session management.
// When the opencode server is not running, missions fall back to CLI mode.
type localSessionAPI struct{}

func (l *localSessionAPI) Create(_ context.Context, _ SessionCreateOpts) (*Session, error) {
	return nil, ErrSessionsUnavailable
}

func (l *localSessionAPI) Get(_ context.Context, _ string) (*Session, error) {
	return nil, ErrSessionsUnavailable
}

func (l *localSessionAPI) Status(_ context.Context) (*SessionStatusResult, error) {
	return nil, ErrSessionsUnavailable
}

func (l *localSessionAPI) Prompt(_ context.Context, _ string, _ PromptInput) (*PromptResult, error) {
	return nil, ErrSessionsUnavailable
}

func (l *localSessionAPI) Wait(_ context.Context, _ string) error {
	return ErrSessionsUnavailable
}

func (l *localSessionAPI) Messages(_ context.Context, _ string) ([]Message, error) {
	return nil, ErrSessionsUnavailable
}

func (l *localSessionAPI) Delete(_ context.Context, _ string) error {
	return ErrSessionsUnavailable
}

func (l *localSessionAPI) ListQuestions(_ context.Context) ([]Question, error) {
	return nil, ErrSessionsUnavailable
}

func (l *localSessionAPI) ReplyQuestion(_ context.Context, _ string, _ string) error {
	return ErrSessionsUnavailable
}
