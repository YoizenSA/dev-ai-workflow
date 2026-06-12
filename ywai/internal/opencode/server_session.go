package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// serverSessionAPI implements SessionAPI via the opencode HTTP server.
type serverSessionAPI struct {
	baseURL    string
	httpClient *http.Client
}

// newServerSessionAPI creates a SessionAPI backed by the opencode HTTP server.
func newServerSessionAPI(baseURL string) *serverSessionAPI {
	return &serverSessionAPI{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ─── Helper ────────────────────────────────────────────────────────────────

func (s *serverSessionAPI) doJSON(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return s.httpClient.Do(req)
}

func decodeResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("opencode session API: %s returned %d: %s", resp.Request.URL.Path, resp.StatusCode, string(body))
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

// ─── Session CRUD ──────────────────────────────────────────────────────────

func (s *serverSessionAPI) Create(ctx context.Context, opts SessionCreateOpts) (*Session, error) {
	// Build request body matching the SDK's Session2.create signature.
	body := map[string]interface{}{
		"title":     opts.Title,
		"agent":     opts.Agent,
		"directory": opts.Directory,
		"workspace": opts.Workspace,
		"parentID":  opts.ParentID,
	}
	if opts.Model != nil {
		body["model"] = map[string]string{
			"id":         opts.Model.ID,
			"providerID": opts.Model.ProviderID,
			"variant":    opts.Model.Variant,
		}
	}

	resp, err := s.doJSON(ctx, http.MethodPost, "/session", body)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	var session Session
	if err := decodeResponse(resp, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *serverSessionAPI) Get(ctx context.Context, sessionID string) (*Session, error) {
	resp, err := s.doJSON(ctx, http.MethodGet, "/session/"+sessionID, nil)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	var session Session
	if err := decodeResponse(resp, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *serverSessionAPI) Status(ctx context.Context) (*SessionStatusResult, error) {
	resp, err := s.doJSON(ctx, http.MethodGet, "/session/status", nil)
	if err != nil {
		return nil, fmt.Errorf("session status: %w", err)
	}

	var result SessionStatusResult
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *serverSessionAPI) Delete(ctx context.Context, sessionID string) error {
	resp, err := s.doJSON(ctx, http.MethodDelete, "/session/"+sessionID, nil)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return decodeResponse(resp, nil)
}

// ─── Prompt / Messages ─────────────────────────────────────────────────────

func (s *serverSessionAPI) Prompt(ctx context.Context, sessionID string, input PromptInput) (*PromptResult, error) {
	// Build the prompt body matching the SDK's Session2.prompt signature.
	// The SDK sends { parts: [{ type: "text", text: "..." }] } with optional delivery.
	parts := []map[string]string{
		{"type": "text", "text": input.Text},
	}
	body := map[string]interface{}{
		"parts":    parts,
		"delivery": input.Delivery,
	}

	path := "/session/" + sessionID
	resp, err := s.doJSON(ctx, http.MethodPost, path, body)
	if err != nil {
		return nil, fmt.Errorf("send prompt: %w", err)
	}

	var result PromptResult
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *serverSessionAPI) Wait(ctx context.Context, sessionID string) error {
	// POST /api/session/{sessionID}/wait blocks until the session finishes.
	// Use a longer timeout for this call since the agent may run for a while.
	// We rely on the context timeout from the caller (WorkerConfig.Timeout).
	waitClient := &http.Client{
		Timeout: 0, // no client-side timeout; context controls cancellation
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/session/"+sessionID+"/wait", nil)
	if err != nil {
		return fmt.Errorf("wait request: %w", err)
	}

	resp, err := waitClient.Do(req)
	if err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	return decodeResponse(resp, nil)
}

func (s *serverSessionAPI) Messages(ctx context.Context, sessionID string) ([]Message, error) {
	resp, err := s.doJSON(ctx, http.MethodGet, "/api/session/"+sessionID+"/message", nil)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}

	var messages []Message
	if err := decodeResponse(resp, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// ─── Questions ─────────────────────────────────────────────────────────────

func (s *serverSessionAPI) ListQuestions(ctx context.Context) ([]Question, error) {
	resp, err := s.doJSON(ctx, http.MethodGet, "/question", nil)
	if err != nil {
		return nil, fmt.Errorf("list questions: %w", err)
	}

	var questions []Question
	if err := decodeResponse(resp, &questions); err != nil {
		return nil, err
	}
	return questions, nil
}

func (s *serverSessionAPI) ReplyQuestion(ctx context.Context, questionID string, answer string) error {
	body := map[string]string{"answer": answer}
	resp, err := s.doJSON(ctx, http.MethodPost, "/question/"+questionID+"/reply", body)
	if err != nil {
		return fmt.Errorf("reply question: %w", err)
	}
	return decodeResponse(resp, nil)
}
