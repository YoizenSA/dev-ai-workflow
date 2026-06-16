package engram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultEngramTimeout = 5 * time.Second

// HTTPClient implements Client against the engram REST API.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPClient creates a client targeting baseURL (e.g. "http://127.0.0.1:7437").
func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultEngramTimeout,
		},
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

func (c *HTTPClient) getJSON(ctx context.Context, path string, query url.Values, target interface{}) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("engram: create request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("engram: %s: %w", path, err)
	}
	return decodeJSON(resp, target)
}

func (c *HTTPClient) sendJSON(ctx context.Context, method, path string, body, target interface{}) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("engram: marshal body: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("engram: create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("engram: %s: %w", path, err)
	}
	if method == http.MethodDelete && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		resp.Body.Close()
		return nil
	}
	return decodeJSON(resp, target)
}

func decodeJSON(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("engram: %s returned %d: %s", resp.Request.URL.Path, resp.StatusCode, string(body))
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

// unwrapList handles engram responses that may be a bare array or wrapped in
// {"data": [...]}, {"observations": [...]}, etc. It returns the JSON array
// slice (or the original raw message if no known wrapper is found).
func unwrapList(raw json.RawMessage) json.RawMessage {
	trim := bytes.TrimSpace(raw)
	if len(trim) > 0 && trim[0] == '[' {
		return raw
	}
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapper); err == nil {
		for _, key := range []string{"data", "observations", "sessions", "events", "timeline"} {
			if v, ok := wrapper[key]; ok {
				return v
			}
		}
	}
	return raw
}

// ─── Client interface implementation ────────────────────────────────────────

func (c *HTTPClient) Status(ctx context.Context) (Status, error) {
	var st Status
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return st, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return st, ErrEngramUnavailable
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return st, ErrEngramUnavailable
	}
	return Status{Connected: true, Source: "server"}, nil
}

func (c *HTTPClient) RecentObservations(ctx context.Context, limit int) ([]Observation, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw json.RawMessage
	if err := c.getJSON(ctx, "/observations/recent", q, &raw); err != nil {
		return nil, err
	}
	var obs []Observation
	if err := json.Unmarshal(unwrapList(raw), &obs); err != nil {
		return nil, fmt.Errorf("engram: decode observations: %w", err)
	}
	return obs, nil
}

func (c *HTTPClient) GetObservation(ctx context.Context, id string) (Observation, error) {
	var obs Observation
	if err := c.getJSON(ctx, "/observations/"+url.PathEscape(id), nil, &obs); err != nil {
		return Observation{}, err
	}
	return obs, nil
}

func (c *HTTPClient) UpdateObservation(ctx context.Context, id string, req UpdateRequest) (Observation, error) {
	var obs Observation
	if err := c.sendJSON(ctx, http.MethodPatch, "/observations/"+url.PathEscape(id), req, &obs); err != nil {
		return Observation{}, err
	}
	return obs, nil
}

func (c *HTTPClient) DeleteObservation(ctx context.Context, id string) error {
	return c.sendJSON(ctx, http.MethodDelete, "/observations/"+url.PathEscape(id), nil, nil)
}

func (c *HTTPClient) Save(ctx context.Context, req SaveRequest) (Observation, error) {
	var obs Observation
	if err := c.sendJSON(ctx, http.MethodPost, "/save", req, &obs); err != nil {
		return Observation{}, err
	}
	return obs, nil
}

func (c *HTTPClient) Search(ctx context.Context, req SearchRequest) ([]Observation, error) {
	q := url.Values{}
	q.Set("q", req.Query)
	if req.Limit > 0 {
		q.Set("limit", strconv.Itoa(req.Limit))
	}
	if req.Type != "" {
		q.Set("type", req.Type)
	}
	var raw json.RawMessage
	if err := c.getJSON(ctx, "/search", q, &raw); err != nil {
		return nil, err
	}
	// engram returns null when no results match.
	trim := bytes.TrimSpace(raw)
	if len(trim) == 0 || string(trim) == "null" {
		return []Observation{}, nil
	}
	var obs []Observation
	if err := json.Unmarshal(unwrapList(raw), &obs); err != nil {
		return nil, fmt.Errorf("engram: decode search: %w", err)
	}
	return obs, nil
}

func (c *HTTPClient) GetStats(ctx context.Context) (Stats, error) {
	var stats Stats
	if err := c.getJSON(ctx, "/stats", nil, &stats); err != nil {
		return Stats{}, err
	}
	return stats, nil
}

func (c *HTTPClient) RecentSessions(ctx context.Context, limit int) ([]Session, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw json.RawMessage
	if err := c.getJSON(ctx, "/sessions/recent", q, &raw); err != nil {
		return nil, err
	}
	var sessions []Session
	if err := json.Unmarshal(unwrapList(raw), &sessions); err != nil {
		return nil, fmt.Errorf("engram: decode sessions: %w", err)
	}
	return sessions, nil
}

func (c *HTTPClient) DeleteSession(ctx context.Context, id string) error {
	return c.sendJSON(ctx, http.MethodDelete, "/sessions/"+url.PathEscape(id), nil, nil)
}

func (c *HTTPClient) RecentPrompts(ctx context.Context, limit int) ([]Prompt, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var raw json.RawMessage
	if err := c.getJSON(ctx, "/prompts/recent", q, &raw); err != nil {
		return nil, err
	}
	var prompts []Prompt
	if err := json.Unmarshal(unwrapList(raw), &prompts); err != nil {
		return nil, fmt.Errorf("engram: decode prompts: %w", err)
	}
	return prompts, nil
}

func (c *HTTPClient) DeletePrompt(ctx context.Context, id string) error {
	return c.sendJSON(ctx, http.MethodDelete, "/prompts/"+url.PathEscape(id), nil, nil)
}

func (c *HTTPClient) Export(ctx context.Context) (json.RawMessage, error) {
	// Export can be large; use a longer timeout than default.
	cli := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/export", nil)
	if err != nil {
		return nil, fmt.Errorf("engram: create request: %w", err)
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("engram: /export: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("engram: /export returned %d: %s", resp.StatusCode, string(body))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("engram: read export: %w", err)
	}
	return json.RawMessage(data), nil
}

func (c *HTTPClient) Import(ctx context.Context, body io.Reader) (ImportResult, error) {
	cli := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/import", body)
	if err != nil {
		return ImportResult{}, fmt.Errorf("engram: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := cli.Do(req)
	if err != nil {
		return ImportResult{}, fmt.Errorf("engram: /import: %w", err)
	}
	var result ImportResult
	if err := decodeJSON(resp, &result); err != nil {
		return ImportResult{}, err
	}
	return result, nil
}

// MergeProjects re-tags every observation matching source.project so it lives
// under target.project. Engram has no native merge endpoint, so we batch-PATCH
// via the search API.
func (c *HTTPClient) MergeProjects(ctx context.Context, source, target string) (MergeProjectsResult, error) {
	result := MergeProjectsResult{Source: source, Target: target}
	if source == "" || target == "" || source == target {
		return result, fmt.Errorf("engram: merge requires distinct source and target")
	}
	// Pull every observation under the source project. /search supports a
	// project filter via q="" + we fall back to listing observations and
	// filtering client-side (more robust against query syntax changes).
	q := url.Values{}
	q.Set("limit", "1000")
	var raw json.RawMessage
	if err := c.getJSON(ctx, "/observations", q, &raw); err != nil {
		return result, err
	}
	var all []Observation
	if err := json.Unmarshal(unwrapList(raw), &all); err != nil {
		return result, fmt.Errorf("engram: decode observations: %w", err)
	}
	for _, o := range all {
		if o.Project != source {
			continue
		}
		tgt := target
		if _, err := c.UpdateObservation(ctx, strconv.Itoa(o.ID), UpdateRequest{Project: &tgt}); err != nil {
			return result, fmt.Errorf("engram: update obs %d: %w", o.ID, err)
		}
		result.ObservationsUpdated++
	}
	return result, nil
}

func (c *HTTPClient) Timeline(ctx context.Context, req TimelineRequest) ([]TimelineEvent, error) {
	q := url.Values{}
	if req.ObservationID != "" {
		q.Set("observation_id", req.ObservationID)
	}
	if req.Limit > 0 {
		q.Set("limit", strconv.Itoa(req.Limit))
	}
	var raw json.RawMessage
	if err := c.getJSON(ctx, "/timeline", q, &raw); err != nil {
		return nil, err
	}
	var events []TimelineEvent
	if err := json.Unmarshal(unwrapList(raw), &events); err != nil {
		return nil, fmt.Errorf("engram: decode timeline: %w", err)
	}
	return events, nil
}

func (c *HTTPClient) GetContext(ctx context.Context, req ContextRequest) (ContextResult, error) {
	q := url.Values{}
	if req.Query != "" {
		q.Set("q", req.Query)
	}
	if req.Limit > 0 {
		q.Set("limit", strconv.Itoa(req.Limit))
	}
	var result ContextResult
	if err := c.getJSON(ctx, "/context", q, &result); err != nil {
		return ContextResult{}, err
	}
	return result, nil
}

func (c *HTTPClient) UpdateContext(ctx context.Context, text string) (ContextResult, error) {
	var result ContextResult
	if err := c.sendJSON(ctx, http.MethodPut, "/context", map[string]string{"context": text}, &result); err != nil {
		return ContextResult{}, err
	}
	return result, nil
}
