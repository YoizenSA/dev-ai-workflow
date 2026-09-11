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

// Attempt is one model running the task once.
type Attempt struct {
	Model     string  `json:"model"`
	Round     int     `json:"round"`
	SessionID string  `json:"sessionId"`
	Seconds   float64 `json:"seconds"`
	Score     Score   `json:"score"`
	Metrics   Metrics `json:"metrics"`
	Response  string  `json:"response,omitempty"`
	Error     string  `json:"error,omitempty"`
	// CostUSD prices the attempt from the model's table rates; CostKnown is
	// false when the model is missing from the pricing table (CostUSD is then
	// meaningless and callers render an unknown cost).
	CostUSD   float64   `json:"costUsd,omitempty"`
	CostKnown bool      `json:"costKnown,omitempty"`
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

	// healthy caches a passed ProbeMode so later calls skip the request.
	healthy bool
}

// ProbeMode is the v2 health check that used to be a dialect probe. It confirms
// the OpenCode 2 server behind BaseURL answers GET /api/session: 200 with a
// JSON body, or 401 (present but unauthenticated). The classic v1 server — a
// withdrawn dialect — serves its SPA shell for ANY path (200 + HTML), so the
// body must be JSON, not the page. The probe is safe: sessions are never
// created, only a health-shaped read. The answer is cached on the Runner.
func (r *Runner) ProbeMode(ctx context.Context) error {
	if r.healthy {
		return nil
	}
	base := strings.TrimRight(r.BaseURL, "/")

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
				r.healthy = true
				return nil
			}
		}
	}

	return fmt.Errorf("no OpenCode 2 server detected at %s (GET /api/session failed)", r.BaseURL)
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
	if err := r.ProbeMode(ctx); err != nil {
		return err
	}
	return r.preflightReadback(ctx, sid, agent)
}

// preflightReadback verifies the run from local storage: directory comes from
// session_v2 and bash/invalid tool calls from session_message JSON (the tool
// name in content[i].name via json_each expansion).
func (r *Runner) preflightReadback(ctx context.Context, sid, agent string) error {
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
	a.CostUSD, a.CostKnown = LookupCost(model, a.Metrics.TokensIn, a.Metrics.TokensOut)
	return a
}

func (r *Runner) metrics(ctx context.Context, sessionID string) Metrics {
	if err := r.ProbeMode(ctx); err != nil {
		return Metrics{}
	}
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

	// The tool name is in content[i].name and the input in content[i].state.input.
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
	if err := r.ProbeMode(ctx); err != nil {
		return "", err
	}
	// POST /api/session, sync response with the session id in data.id.
	// Location is the reviewer working directory (the control server starts
	// the server from the project root, so cwd is already correct).
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
		return "", fmt.Errorf("server returned no session id")
	}
	return out.Data.ID, nil
}

func (r *Runner) prompt(ctx context.Context, sessionID, agent, model, provider, text string) (string, error) {
	if err := r.ProbeMode(ctx); err != nil {
		return "", err
	}
	// Prompting is asynchronous: POST the prompt (data-admitted), then POST
	// /wait to block until the session is idle, then read the final assistant
	// text back from GET /api/session/{id}/message.
	//
	// NOTE: the live server (beta-18684) accepts the prompt at the TOP LEVEL
	// of the payload ({text, files?, agents?}); an older schema wrote it under
	// {"prompt": {...}}. Both are tried; the first that admits wins.
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

	// Switch model before the run if the caller asked for one — the session
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
			return "", fmt.Errorf("messages: %w", err)
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
				return "", fmt.Errorf("assistant error: %s", m.Error.Message)
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
