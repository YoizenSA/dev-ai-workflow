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

func (r *Runner) createSession(ctx context.Context, title, agent string) (string, error) {
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

func (r *Runner) prompt(ctx context.Context, sessionID, agent, model, provider, text string) (string, error) {
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

func (r *Runner) do(ctx context.Context, method, path string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(r.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("content-type", "application/json")
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
