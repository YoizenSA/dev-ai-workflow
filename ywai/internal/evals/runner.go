package evals

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// ServerMode is the OpenCode HTTP API dialect the Runner talks to.
type ServerMode string

const (
	// ModeV1 is the classic `opencode serve` API: POST /session and
	// POST /session/{id}/message with parts[] in both directions. No auth.
	ModeV1 ServerMode = "v1"
	// ModeV2 is the opencode2 (beta) API: POST /api/session (sync create) and
	// POST /api/session/{id}/prompt (async: accepted, then /wait blocks until
	// idle and the final answer is read back from GET /api/session/{id}/message).
	// Every endpoint requires HTTP Basic Auth.
	ModeV2 ServerMode = "v2"
)

// Attempt is one model running the task once.
type Attempt struct {
	Model     string    `json:"model"`
	Round     int       `json:"round"`
	SessionID string    `json:"sessionId"`
	Seconds   float64   `json:"seconds"`
	Score     Score     `json:"score"`
	Metrics   Metrics   `json:"metrics"`
	Response  string    `json:"response,omitempty"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"startedAt"`
}

// Metrics is what the session actually cost, read back from OpenCode's own database
// rather than counted here — the transcript is the source of truth.
type Metrics struct {
	Turns     int   `json:"turns"`
	Calls     int   `json:"calls"`
	Reads     int   `json:"reads"`
	CodeGraph int   `json:"codegraph"`
	Invalid   int   `json:"invalid"`
	WorstFile int   `json:"worstFileReads"` // most reads of any single file
	TokensIn  int64 `json:"tokensInput"`
	TokensOut int64 `json:"tokensOutput"`
}

// RunRequest configures a benchmark.
type RunRequest struct {
	TaskID   string   `json:"taskId"`
	Models   []string `json:"models"`
	Provider string   `json:"provider"`
	Rounds   int      `json:"rounds"`
}

// Run is a completed or in-flight benchmark.
type Run struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"taskId"`
	TaskName  string    `json:"taskName"`
	Agent     string    `json:"agent"`
	Provider  string    `json:"provider"`
	Rounds    int       `json:"rounds"`
	Models    []string  `json:"models"`
	Attempts  []Attempt `json:"attempts"`
	Status    string    `json:"status"` // running | done | failed
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"startedAt"`
	EndedAt   time.Time `json:"endedAt,omitempty"`
}

// Runner drives an OpenCode server and reads results back out of its database.
type Runner struct {
	// BaseURL must point at a server whose working directory is the project under
	// test. Sessions inherit that directory, and a mismatch silently guts the run:
	// CodeGraph resolves no index, and the agent's permissions are never applied.
	BaseURL string
	DB      *sql.DB
	Client  *http.Client
	// Mode pins the API dialect. Empty = detected lazily by ProbeMode on the
	// first call (v2 wins when the server answers /api/session; v1 otherwise).
	Mode ServerMode
}

// ProbeMode detects which OpenCode HTTP API the server behind BaseURL speaks.
// opencode2 (v2) answers GET /api/session with Basic Auth; the classic server
// answers POST /session without auth. The probe is safe: sessions are never
// created, only a health-shaped read. The answer is cached on the Runner.
func (r *Runner) ProbeMode(ctx context.Context) (ServerMode, error) {
	if r.Mode != "" {
		return r.Mode, nil
	}
	base := strings.TrimRight(r.BaseURL, "/")

	// v2 probe: GET /api/session?limit=1 — 200 (or 401 without auth) means v2.
	// The classic v1 server serves its SPA shell for ANY path (200 + HTML), so
	// a bare status check is not enough: the body must be JSON, not the page.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/session?limit=1", nil)
	if err == nil {
		req.Header.Set("content-type", "application/json")
		opencode.ApplyServerAuth(req)
		resp, err := r.doRaw(req)
		if err == nil {
			buf := make([]byte, 256)
			n, _ := resp.Body.Read(buf)
			_ = resp.Body.Close()
			body := strings.ToLower(strings.TrimSpace(string(buf[:n])))
			isJSON := strings.HasPrefix(body, "{") || strings.HasPrefix(body, "[")
			if (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized) && (isJSON || resp.StatusCode == http.StatusUnauthorized) {
				r.Mode = ModeV2
				return r.Mode, nil
			}
		}
	}

	// v1 probe: POST /session with a minimal body — the v1 server answers 200
	// with a session object (or an error), never 404 for this path.
	body, _ := json.Marshal(map[string]any{"title": "mode-probe"})
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, base+"/session", bytes.NewReader(body))
	if err == nil {
		req.Header.Set("content-type", "application/json")
		resp, err := r.doRaw(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				r.Mode = ModeV1
				return r.Mode, nil
			}
		}
	}

	return "", fmt.Errorf("no OpenCode server detected at %s (tried v2 GET /api/session and v1 POST /session)", r.BaseURL)
}

// doRaw performs one request with the runner's client (no body decoding).
func (r *Runner) doRaw(req *http.Request) (*http.Response, error) {
	cl := r.Client
	if cl == nil {
		cl = http.DefaultClient
	}
	return cl.Do(req)
}

// Preflight proves the server is usable before a long run starts. It asks the agent
// for a denied tool and for its directory; a correctly bound agent reports the
// denial. Without this check a whole benchmark can complete while measuring the
// default agent in the wrong folder — which is exactly how a first attempt at this
// harness produced a full table of meaningless numbers.
func (r *Runner) Preflight(ctx context.Context, agent, model, provider string) error {
	sid, err := r.createSession(ctx, "preflight-"+agent, agent)
	if err != nil {
		return fmt.Errorf("create preflight session: %w", err)
	}
	if _, err := r.prompt(ctx, sid, agent, model, provider,
		"Run the bash command: echo PREFLIGHT. Then stop."); err != nil {
		return fmt.Errorf("preflight prompt: %w", err)
	}

	mode, err := r.ProbeMode(ctx)
	if err != nil {
		return err
	}
	if mode == ModeV2 {
		return r.preflightReadbackV2(ctx, sid, agent)
	}

	var dir string
	var bashCalls, invalidCalls int
	row := r.DB.QueryRowContext(ctx, `
		SELECT COALESCE(s.directory,''),
		       COALESCE(SUM(json_extract(p.data,'$.tool')='bash'),0),
		       COALESCE(SUM(json_extract(p.data,'$.tool')='invalid'),0)
		FROM session s LEFT JOIN part p ON p.session_id = s.id
		WHERE s.id = ?`, sid)
	if err := row.Scan(&dir, &bashCalls, &invalidCalls); err != nil {
		return fmt.Errorf("preflight readback: %w", err)
	}
	if bashCalls > 0 {
		return fmt.Errorf("agent %q is not being applied: bash ran despite being denied "+
			"(the server on %s may predate the agent install — restart it from the project)", agent, r.BaseURL)
	}
	if dir == "" {
		return fmt.Errorf("preflight session has no directory; start the OpenCode server from the project root")
	}
	return nil
}

// preflightReadbackV2 is the opencode2 counterpart: directory comes from
// session_v2 and bash/invalid tool calls from session_message JSON (the tool
// name in content[i].name via json_each expansion).
func (r *Runner) preflightReadbackV2(ctx context.Context, sid, agent string) error {
	var dir string
	var bashCalls, invalidCalls int
	row := r.DB.QueryRowContext(ctx, `
		SELECT COALESCE(s.directory,''),
		       COALESCE(SUM(je.value->>'name'='bash'),0),
		       COALESCE(SUM(je.value->>'name'='invalid'),0)
		FROM session_v2 s
		LEFT JOIN session_message m ON m.session_id = s.id
		LEFT JOIN json_each(m.data, '$.content') je ON je.value->>'type' = 'tool'
		WHERE s.id = ?`, sid)
	if err := row.Scan(&dir, &bashCalls, &invalidCalls); err != nil {
		return fmt.Errorf("preflight readback: %w", err)
	}
	if bashCalls > 0 {
		return fmt.Errorf("agent %q is not being applied: bash ran despite being denied "+
			"(the server on %s may predate the agent install — restart it from the project)", agent, r.BaseURL)
	}
	if dir == "" {
		return fmt.Errorf("preflight session has no directory; start the OpenCode server from the project root")
	}
	return nil
}

// Execute runs every model for every round, sequentially. Concurrency would have the
// runs contend for the same CodeGraph index and the same provider, inflating exactly
// the timings the benchmark exists to compare.
func (r *Runner) Execute(ctx context.Context, task Task, req RunRequest, onAttempt func(Attempt)) ([]Attempt, error) {
	rounds := req.Rounds
	if rounds < 1 {
		rounds = 1
	}
	var attempts []Attempt
	for round := 1; round <= rounds; round++ {
		for _, model := range req.Models {
			if ctx.Err() != nil {
				return attempts, ctx.Err()
			}
			a := r.runOne(ctx, task, model, req.Provider, round)
			attempts = append(attempts, a)
			if onAttempt != nil {
				onAttempt(a)
			}
		}
	}
	return attempts, nil
}

func (r *Runner) runOne(ctx context.Context, task Task, model, provider string, round int) Attempt {
	a := Attempt{Model: model, Round: round, StartedAt: time.Now().UTC()}
	title := fmt.Sprintf("eval-%s-r%d-%s", task.ID, round, model)

	sid, err := r.createSession(ctx, title, task.Agent)
	if err != nil {
		a.Error = err.Error()
		return a
	}
	a.SessionID = sid

	start := time.Now()
	resp, err := r.prompt(ctx, sid, task.Agent, model, provider, task.Brief)
	a.Seconds = time.Since(start).Seconds()
	if err != nil {
		a.Error = err.Error()
	}
	a.Response = resp
	a.Score = task.Score(resp)
	a.Metrics = r.metrics(ctx, sid)
	return a
}

func (r *Runner) metrics(ctx context.Context, sessionID string) Metrics {
	mode, err := r.ProbeMode(ctx)
	if err != nil {
		return Metrics{}
	}
	if mode == ModeV2 {
		return r.metricsV2(ctx, sessionID)
	}
	var m Metrics
	_ = r.DB.QueryRowContext(ctx, `
		SELECT COALESCE(COUNT(DISTINCT message_id),0), COALESCE(COUNT(*),0),
		       COALESCE(SUM(json_extract(data,'$.tool')='read'),0),
		       COALESCE(SUM(json_extract(data,'$.tool') LIKE 'codegraph%'),0),
		       COALESCE(SUM(json_extract(data,'$.tool')='invalid'),0)
		FROM part WHERE session_id=? AND json_extract(data,'$.type')='tool'`,
		sessionID).Scan(&m.Turns, &m.Calls, &m.Reads, &m.CodeGraph, &m.Invalid)

	// Repeatedly re-reading one file is the signature of a model losing track of what
	// it already saw, and it tracks turn count closely enough to be worth surfacing.
	_ = r.DB.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(v),0) FROM (
		  SELECT COUNT(*) v FROM part
		  WHERE session_id=? AND json_extract(data,'$.tool')='read'
		  GROUP BY json_extract(data,'$.state.input.filePath'))`,
		sessionID).Scan(&m.WorstFile)

	_ = r.DB.QueryRowContext(ctx,
		`SELECT COALESCE(tokens_input,0), COALESCE(tokens_output,0) FROM session WHERE id=?`,
		sessionID).Scan(&m.TokensIn, &m.TokensOut)
	return m
}

// metricsV2 mirrors metrics() against the opencode2 storage schema: sessions in
// session_v2, messages in session_message. Tool calls live inside the message
// JSON under content[] (any index), so the query expands with json_each — the
// same technique the analytics provider uses.
func (r *Runner) metricsV2(ctx context.Context, sessionID string) Metrics {
	var m Metrics
	_ = r.DB.QueryRowContext(ctx, `
		SELECT COALESCE(COUNT(DISTINCT sm.id),0),
		       COALESCE(COUNT(*),0),
		       COALESCE(SUM(je.value->>'name' = 'read'),0),
		       COALESCE(SUM(je.value->>'name' LIKE 'codegraph%'),0),
		       COALESCE(SUM(je.value->>'name' = 'invalid'),0)
		FROM session_message sm, json_each(sm.data, '$.content') je
		WHERE sm.session_id=? AND je.value->>'type'='tool'`,
		sessionID).Scan(&m.Turns, &m.Calls, &m.Reads, &m.CodeGraph, &m.Invalid)

	// In v2 the tool name is in content[i].name and the input in content[i].state.input.
	_ = r.DB.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(v),0) FROM (
		  SELECT COUNT(*) v FROM session_message sm, json_each(sm.data, '$.content') je
		  WHERE sm.session_id=? AND je.value->>'name'='read'
		  GROUP BY json_extract(je.value, '$.state.input.filePath'))`,
		sessionID).Scan(&m.WorstFile)

	_ = r.DB.QueryRowContext(ctx,
		`SELECT COALESCE(tokens_input,0), COALESCE(tokens_output,0) FROM session_v2 WHERE id=?`,
		sessionID).Scan(&m.TokensIn, &m.TokensOut)
	return m
}

func (r *Runner) createSession(ctx context.Context, title, agent string) (string, error) {
	mode, err := r.ProbeMode(ctx)
	if err != nil {
		return "", err
	}
	if mode == ModeV2 {
		return r.createSessionV2(ctx, title, agent)
	}
	body, _ := json.Marshal(map[string]any{"title": title, "agent": agent})
	var out struct {
		ID string `json:"id"`
	}
	if err := r.do(ctx, http.MethodPost, "/session", body, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("server returned no session id")
	}
	return out.ID, nil
}

// createSessionV2 is the opencode2 dialect: POST /api/session, sync response
// with the session id in data.id. Location is the reviewer working directory
// (the control server starts the server from the project root, so cwd is
// already correct).
func (r *Runner) createSessionV2(ctx context.Context, title, agent string) (string, error) {
	body, _ := json.Marshal(map[string]any{"title": title, "agent": agent})
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := r.do(ctx, http.MethodPost, "/api/session", body, &out); err != nil {
		return "", err
	}
	if out.Data.ID == "" {
		return "", fmt.Errorf("v2 server returned no session id")
	}
	return out.Data.ID, nil
}

func (r *Runner) prompt(ctx context.Context, sessionID, agent, model, provider, text string) (string, error) {
	mode, err := r.ProbeMode(ctx)
	if err != nil {
		return "", err
	}
	if mode == ModeV2 {
		return r.promptV2(ctx, sessionID, model, provider, text)
	}
	body, _ := json.Marshal(map[string]any{
		"agent": agent,
		"model": map[string]string{"providerID": provider, "modelID": model},
		"parts": []map[string]string{{"type": "text", "text": text}},
	})
	var out struct {
		Parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"parts"`
	}
	if err := r.do(ctx, http.MethodPost, "/session/"+sessionID+"/message", body, &out); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, p := range out.Parts {
		if p.Type == "text" {
			b.WriteString(p.Text)
		}
	}
	return b.String(), nil
}

// promptV2 is the opencode2 dialect. Prompting is asynchronous: POST the prompt
// (data-admitted), then POST /wait to block until the session is idle, then
// read the final assistant text back from GET /api/session/{id}/message.
//
// NOTE: the live v2 server (opencode2 beta-18684) accepts the prompt at the
// TOP LEVEL of the payload ({text, files?, agents?}); an older schema wrote
// it under {"prompt": {...}}. Both are tried; the first that admits wins.
func (r *Runner) promptV2(ctx context.Context, sessionID, model, provider, text string) (string, error) {
	// The prompt payload is `{ "prompt": { "text": ... } }` plus optional
	// model delivery; the model is set on the session (POST /api/session) at
	// create time and per-prompt via /model when provided.
	body, _ := json.Marshal(map[string]any{
		"prompt": map[string]any{"text": text},
	})
	var admitted struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	err := r.do(ctx, http.MethodPost, "/api/session/"+sessionID+"/prompt", body, &admitted)
	if err != nil {
		// Top-level shape (beta-18684): {"text": ...}
		flat, _ := json.Marshal(map[string]any{"text": text})
		admitted.Data.ID = ""
		err = r.do(ctx, http.MethodPost, "/api/session/"+sessionID+"/prompt", flat, &admitted)
		if err != nil {
			return "", err
		}
	}

	// Switch model before the run if the caller asked for one — the v2 session
	// is created without a model and the /model endpoint is sync.
	if model != "" {
		mbody, _ := json.Marshal(map[string]any{
			"model": map[string]string{"providerID": provider, "id": model},
		})
		_ = r.do(ctx, http.MethodPost, "/api/session/"+sessionID+"/model", mbody, nil) // best-effort
	}

	// Block until idle: the agent loop drains the prompt's work. Some servers
	// (hybrid 1.18.x) answer "wait is not available yet" — tolerate that: the
	// message read below polls until the assistant's text (or an error finish)
	// appears, so a busy session is still waited out by retrying.
	haveWait := r.do(ctx, http.MethodPost, "/api/session/"+sessionID+"/wait", nil, nil) == nil

	// Read the final assistant message(s). Retry while the session is still
	// draining — the caller's deadline bounds the total wait.
	var b strings.Builder
	for attempt := 0; attempt < 30; attempt++ {
		var msgs struct {
			Data []struct {
				ID     string `json:"id"`
				Type   string `json:"type"`
				Finish string `json:"finish"`
				Error  *struct {
					Message string `json:"message"`
				} `json:"error"`
				// beta-18684 assistant messages: content[] of text/reasoning.
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
				// newer schema: assistant text lives in payload.text.
				Payload *struct {
					Text string `json:"text"`
				} `json:"payload"`
			} `json:"data"`
		}
		if err := r.do(ctx, http.MethodGet, "/api/session/"+sessionID+"/message?order=desc&limit=5", nil, &msgs); err != nil {
			return "", fmt.Errorf("v2 messages: %w", err)
		}
		// Messages come newest-first; the last assistant text block is the answer.
		b.Reset()
		finished := false
		for i := len(msgs.Data) - 1; i >= 0; i-- {
			m := msgs.Data[i]
			if m.Type != "assistant" {
				continue
			}
			if m.Finish == "error" && m.Error != nil {
				return "", fmt.Errorf("v2 assistant error: %s", m.Error.Message)
			}
			if m.Finish != "" {
				finished = true
			}
			if m.Payload != nil && m.Payload.Text != "" {
				b.WriteString(m.Payload.Text)
			}
			for _, c := range m.Content {
				if c.Type == "text" {
					b.WriteString(c.Text)
				}
			}
			if b.Len() > 0 {
				break
			}
		}
		if b.Len() > 0 || finished || haveWait {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return b.String(), nil
}

func (r *Runner) do(ctx context.Context, method, path string, body []byte, out any) error {
	// OpenCode v2 (opencode2) protects every endpoint with Basic Auth; v1 has
	// no auth and ignores the header. Applying it when the env password exists
	// is safe for both dialects.
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(r.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json")
	opencode.ApplyServerAuth(req)
	cl := r.Client
	if cl == nil {
		cl = http.DefaultClient
	}
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s", method, path, resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
