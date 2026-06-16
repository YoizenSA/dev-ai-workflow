package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// Consolidation status values.
const (
	StatusRunning        = "running"
	StatusAwaitingReview = "awaiting_review"
	StatusApplying       = "applying"
	StatusApplied        = "applied"
	StatusDiscarded      = "discarded"
	StatusFailed         = "failed"
)

// ScopeFilter narrows a consolidation run to a topic or project. Empty values
// mean "no scope" — the run uses the full memory context.
type ScopeFilter struct {
	TopicKey string `json:"topic_key,omitempty"`
	Project  string `json:"project,omitempty"`
}

// ConsolidationRun tracks one consolidation lifecycle.
type ConsolidationRun struct {
	ID        string                    `json:"id"`
	Model     string                    `json:"model"`
	Agent     string                    `json:"agent"`
	Status    string                    `json:"status"`
	Scope     *ScopeFilter              `json:"scope,omitempty"`
	Plan      *engram.ConsolidationPlan `json:"plan,omitempty"`
	Digest    string                    `json:"digest,omitempty"`
	SessionID string                    `json:"sessionID,omitempty"`
	Error     string                    `json:"error,omitempty"`
	StartedAt time.Time                 `json:"startedAt"`
	UpdatedAt time.Time                 `json:"updatedAt"`
}

// ApplySelection is the body of POST /consolidations/{id}/apply: the subsets of
// the plan the user accepted.
type ApplySelection struct {
	Updates      []engram.PlanUpdate  `json:"updates"`
	Deletes      []engram.PlanDelete  `json:"deletes"`
	NewSummaries []engram.PlanSummary `json:"new_summaries"`
}

// ConsolidationManager owns in-memory consolidation runs.
type ConsolidationManager struct {
	mu        sync.RWMutex
	runs      map[string]*ConsolidationRun
	engram    engram.Client
	sessions  func() opencode.SessionAPI // resolved lazily so a late opencode start works
	broadcast func(eventType string, payload any)
}

// NewConsolidationManager creates a manager. sessions may return nil if the
// opencode server isn't up yet; runs will then fail with a clear error.
func NewConsolidationManager(e engram.Client, sessions func() opencode.SessionAPI, broadcast func(string, any)) *ConsolidationManager {
	return &ConsolidationManager{
		runs:      make(map[string]*ConsolidationRun),
		engram:    e,
		sessions:  sessions,
		broadcast: broadcast,
	}
}

// Get returns a snapshot copy of a run.
func (m *ConsolidationManager) Get(id string) (ConsolidationRun, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.runs[id]
	if !ok {
		return ConsolidationRun{}, false
	}
	return *r, true
}

// Start creates a run and kicks off the driver goroutine. Returns the run ID.
func (m *ConsolidationManager) Start(ctx context.Context, model, agent string, scope ScopeFilter) (string, error) {
	if m.engram == nil {
		return "", errors.New("engram client not configured")
	}
	id := newRunID()
	run := &ConsolidationRun{
		ID:        id,
		Model:     model,
		Agent:     agent,
		Status:    StatusRunning,
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if scope.TopicKey != "" || scope.Project != "" {
		run.Scope = &scope
	}
	m.mu.Lock()
	m.runs[id] = run
	m.mu.Unlock()

	m.emit(id, StatusRunning, nil)
	// Detached: use a background context so the HTTP request can return while
	// the opencode session keeps running.
	go m.drive(context.Background(), run)
	return id, nil
}

// Apply applies a user-approved selection and transitions to applied.
func (m *ConsolidationManager) Apply(ctx context.Context, id string, sel ApplySelection) error {
	m.mu.Lock()
	r, ok := m.runs[id]
	if !ok {
		m.mu.Unlock()
		return errors.New("consolidation run not found")
	}
	if r.Status != StatusAwaitingReview {
		m.mu.Unlock()
		return fmt.Errorf("run is %s, not awaiting_review", r.Status)
	}
	r.Status = StatusApplying
	r.UpdatedAt = time.Now()
	m.mu.Unlock()
	m.emit(id, StatusApplying, nil)

	// Apply deletes first (so updates to surviving obs are clean), then updates,
	// then new summaries.
	for _, d := range sel.Deletes {
		if err := m.engram.DeleteObservation(ctx, d.ObservationID); err != nil {
			m.fail(id, fmt.Sprintf("delete %s: %v", d.ObservationID, err))
			return err
		}
	}
	for _, u := range sel.Updates {
		req := engram.UpdateRequest{}
		if u.NewContent != "" {
			req.Content = &u.NewContent
		}
		if u.NewScope != "" {
			req.Scope = &u.NewScope
		}
		if _, err := m.engram.UpdateObservation(ctx, u.ObservationID, req); err != nil {
			m.fail(id, fmt.Sprintf("update %s: %v", u.ObservationID, err))
			return err
		}
	}
	for _, s := range sel.NewSummaries {
		if _, err := m.engram.Save(ctx, engram.SaveRequest{
			Type: s.Type, Content: s.Content, Scope: s.Scope,
		}); err != nil {
			m.fail(id, fmt.Sprintf("save summary: %v", err))
			return err
		}
	}

	m.mu.Lock()
	r.Status = StatusApplied
	r.UpdatedAt = time.Now()
	m.mu.Unlock()
	m.emit(id, StatusApplied, nil)
	return nil
}

// Discard transitions a run to discarded without touching engram.
func (m *ConsolidationManager) Discard(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runs[id]
	if !ok {
		return errors.New("consolidation run not found")
	}
	r.Status = StatusDiscarded
	r.UpdatedAt = time.Now()
	m.emit(id, StatusDiscarded, nil)
	return nil
}

// driveTimeout caps a single consolidation run. The LLM call is the slow part —
// 5 minutes is generous for most models while still ensuring a hung session
// fails loudly instead of leaving the UI spinning forever.
const driveTimeout = 5 * time.Minute

// drive runs the opencode session: fetch context → create session → prompt →
// wait → parse plan from last assistant message.
func (m *ConsolidationManager) drive(parent context.Context, run *ConsolidationRun) {
	ctx, cancel := context.WithTimeout(parent, driveTimeout)
	defer cancel()

	logf := func(format string, args ...any) {
		log.Printf("consolidation[%s] "+format, append([]any{run.ID}, args...)...)
	}
	logf("start  model=%q agent=%q scope=%+v", run.Model, run.Agent, run.Scope)

	sess := m.sessions()
	if sess == nil {
		m.fail(run.ID, "opencode server is not running; start it and retry")
		return
	}

	// 1. Fetch current memory context, narrowed to the run's scope when set.
	req := engram.ContextRequest{Limit: 200}
	if run.Scope != nil {
		// engram's /context accepts a free-text query — feed it the topic key
		// (preferred for precision) or the project name as a fallback.
		if run.Scope.TopicKey != "" {
			req.Query = run.Scope.TopicKey
		} else if run.Scope.Project != "" {
			req.Query = run.Scope.Project
		}
	}
	ctxt, err := m.engram.GetContext(ctx, req)
	if err != nil {
		m.fail(run.ID, fmt.Sprintf("fetch context: %v", err))
		return
	}
	logf("context fetched (%d chars, scope_query=%q)", len(ctxt.Context), req.Query)

	// 2. Build the prompt.
	prompt := buildConsolidationPrompt(ctxt, run.Scope)

	// 3. Create the opencode session. Default agent is "general" — the agent
	// pool varies per environment and "memory" only exists if the user has
	// installed it.
	agent := run.Agent
	if agent == "" {
		agent = "general"
	}
	opts := opencode.SessionCreateOpts{
		Title: "Consolidation " + run.ID,
		Agent: agent,
	}
	if run.Model != "" {
		opts.Model = parseModelInput(run.Model)
	}
	session, err := sess.Create(ctx, opts)
	if err != nil {
		m.fail(run.ID, fmt.Sprintf("create session (agent=%q model=%q): %v", agent, run.Model, err))
		return
	}
	m.setSessionID(run.ID, session.ID)
	logf("session created id=%s", session.ID)
	m.emit(run.ID, StatusRunning, map[string]any{"stage": "session_created"})

	// 4. Prompt + wait.
	if _, err := sess.Prompt(ctx, session.ID, opencode.PromptInput{Text: prompt, Delivery: "immediate"}); err != nil {
		m.fail(run.ID, fmt.Sprintf("prompt: %v", err))
		return
	}
	logf("prompt sent (%d chars), waiting for LLM…", len(prompt))
	m.emit(run.ID, StatusRunning, map[string]any{"stage": "agent_working"})
	if err := sess.Wait(ctx, session.ID); err != nil {
		m.fail(run.ID, fmt.Sprintf("wait: %v", err))
		return
	}
	logf("LLM finished, fetching messages")

	// 5. Parse plan from last assistant message.
	msgs, err := sess.Messages(ctx, session.ID)
	if err != nil {
		m.fail(run.ID, fmt.Sprintf("messages: %v", err))
		return
	}
	plan, err := extractPlan(msgs)
	if err != nil {
		m.fail(run.ID, fmt.Sprintf("parse plan: %v", err))
		return
	}

	m.mu.Lock()
	run.Plan = plan
	run.Digest = plan.Digest
	run.Status = StatusAwaitingReview
	run.UpdatedAt = time.Now()
	m.mu.Unlock()
	logf("plan ready (updates=%d deletes=%d new=%d) → awaiting_review",
		len(plan.Updates), len(plan.Deletes), len(plan.NewSummaries))
	m.emit(run.ID, StatusAwaitingReview, map[string]any{"plan": plan})
}

// fail marks a run failed and emits.
func (m *ConsolidationManager) fail(id, msg string) {
	m.mu.Lock()
	if r, ok := m.runs[id]; ok {
		r.Status = StatusFailed
		r.Error = msg
		r.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
	m.emit(id, StatusFailed, map[string]any{"error": msg})
}

func (m *ConsolidationManager) setSessionID(id, sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.runs[id]; ok {
		r.SessionID = sessionID
	}
}

func (m *ConsolidationManager) emit(id, status string, extra map[string]any) {
	if m.broadcast == nil {
		return
	}
	payload := map[string]any{"run_id": id, "status": status}
	for k, v := range extra {
		payload[k] = v
	}
	m.broadcast("consolidation."+mapStageToEvent(status), payload)
}

// mapStageToEvent normalises internal status strings to event suffixes.
func mapStageToEvent(status string) string {
	switch status {
	case StatusRunning:
		return "progress"
	case StatusAwaitingReview:
		return "completed"
	case StatusApplied:
		return "applied"
	case StatusFailed:
		return "failed"
	case StatusDiscarded:
		return "discarded"
	default:
		return status
	}
}

// buildConsolidationPrompt renders the agent instructions + current memory.
// When scope is non-nil, the run is narrowed to a topic or project; the prompt
// tells the agent to keep changes inside that scope.
func buildConsolidationPrompt(ctxt engram.ContextResult, scope *ScopeFilter) string {
	var b strings.Builder
	b.WriteString("Analyze the following engram memories and produce a single ")
	b.WriteString("consolidation plan as JSON (updates, deletes, new_summaries, digest). ")
	b.WriteString("Output JSON only — no prose, no markdown fences.\n\n")
	if scope != nil {
		b.WriteString("## Scope\n\n")
		if scope.TopicKey != "" {
			b.WriteString("Only consider memories under topic_key: ")
			b.WriteString(scope.TopicKey)
			b.WriteString("\n")
		}
		if scope.Project != "" {
			b.WriteString("Only consider memories under project: ")
			b.WriteString(scope.Project)
			b.WriteString("\n")
		}
		b.WriteString("Do not propose changes to memories outside this scope.\n\n")
	}
	b.WriteString("## Current memory context\n\n")
	if ctxt.Context != "" {
		b.WriteString(ctxt.Context)
		b.WriteString("\n")
	}
	return b.String()
}

// extractPlan finds the last assistant message and parses its JSON plan.
func extractPlan(msgs []opencode.Message) (*engram.ConsolidationPlan, error) {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != "assistant" || strings.TrimSpace(msgs[i].Text) == "" {
			continue
		}
		text := extractJSONBlock(msgs[i].Text)
		var plan engram.ConsolidationPlan
		if err := json.Unmarshal([]byte(text), &plan); err != nil {
			return nil, fmt.Errorf("invalid plan JSON: %w", err)
		}
		return &plan, nil
	}
	return nil, errors.New("no assistant message found")
}

// extractJSONBlock pulls the first {...} JSON object out of text that may be
// wrapped in markdown fences or surrounded by stray prose.
func extractJSONBlock(text string) string {
	text = strings.TrimSpace(text)
	// Strip ```json ... ``` fences if present.
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return text
}

// parseModelInput converts a "provider/model" or "model" string into a
// SessionCreateOpts.Model.
func parseModelInput(s string) *opencode.ModelInput {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	mi := &opencode.ModelInput{ID: s}
	if idx := strings.Index(s, "/"); idx > 0 {
		mi.ProviderID = s[:idx]
		mi.ID = s[idx+1:]
	}
	return mi
}

func newRunID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return "con_" + hex.EncodeToString(b)
}
