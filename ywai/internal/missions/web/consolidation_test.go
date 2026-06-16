package web

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/engram"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// fakeSessionAPI lets the test script a single consolidation run.
type fakeSessionAPI struct {
	mu        sync.Mutex
	prompted  bool
	msgs      []opencode.Message
	createErr error
}

func (f *fakeSessionAPI) Create(ctx context.Context, opts opencode.SessionCreateOpts) (*opencode.Session, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &opencode.Session{ID: "sess_1", Title: opts.Title, Agent: opts.Agent}, nil
}
func (f *fakeSessionAPI) Get(ctx context.Context, id string) (*opencode.Session, error) {
	return &opencode.Session{ID: id}, nil
}
func (f *fakeSessionAPI) Status(ctx context.Context) (*opencode.SessionStatusResult, error) {
	return &opencode.SessionStatusResult{}, nil
}
func (f *fakeSessionAPI) Prompt(ctx context.Context, id string, in opencode.PromptInput) (*opencode.PromptResult, error) {
	f.mu.Lock()
	f.prompted = true
	f.mu.Unlock()
	return &opencode.PromptResult{MessageID: "msg_1", SessionID: id}, nil
}
func (f *fakeSessionAPI) Wait(ctx context.Context, id string) error { return nil }
func (f *fakeSessionAPI) Messages(ctx context.Context, id string) ([]opencode.Message, error) {
	return f.msgs, nil
}
func (f *fakeSessionAPI) Delete(ctx context.Context, id string) error { return nil }
func (f *fakeSessionAPI) ListQuestions(ctx context.Context) ([]opencode.Question, error) {
	return nil, nil
}
func (f *fakeSessionAPI) ReplyQuestion(ctx context.Context, qid, answer string) error { return nil }

// fakeEngramClient is a minimal engram.Client for manager tests.
type fakeEngramClient struct {
	deletedIDs []string
	saved      []engram.SaveRequest
	updatedIDs []string
	contextErr error
}

func (f *fakeEngramClient) Status(ctx context.Context) (engram.Status, error) {
	return engram.Status{Connected: true}, nil
}
func (f *fakeEngramClient) RecentObservations(ctx context.Context, limit int) ([]engram.Observation, error) {
	return nil, nil
}
func (f *fakeEngramClient) GetObservation(ctx context.Context, id string) (engram.Observation, error) {
	return engram.Observation{ID: 0, SyncID: id}, nil
}
func (f *fakeEngramClient) UpdateObservation(ctx context.Context, id string, req engram.UpdateRequest) (engram.Observation, error) {
	f.updatedIDs = append(f.updatedIDs, id)
	return engram.Observation{ID: 0, SyncID: id}, nil
}
func (f *fakeEngramClient) DeleteObservation(ctx context.Context, id string) error {
	f.deletedIDs = append(f.deletedIDs, id)
	return nil
}
func (f *fakeEngramClient) Save(ctx context.Context, req engram.SaveRequest) (engram.Observation, error) {
	f.saved = append(f.saved, req)
	return engram.Observation{ID: 999, SyncID: "new", Type: req.Type}, nil
}
func (f *fakeEngramClient) Search(ctx context.Context, req engram.SearchRequest) ([]engram.Observation, error) {
	return nil, nil
}
func (f *fakeEngramClient) GetStats(ctx context.Context) (engram.Stats, error) {
	return engram.Stats{}, nil
}
func (f *fakeEngramClient) RecentSessions(ctx context.Context, limit int) ([]engram.Session, error) {
	return nil, nil
}
func (f *fakeEngramClient) DeleteSession(ctx context.Context, id string) error {
	return nil
}
func (f *fakeEngramClient) RecentPrompts(ctx context.Context, limit int) ([]engram.Prompt, error) {
	return nil, nil
}
func (f *fakeEngramClient) DeletePrompt(ctx context.Context, id string) error {
	return nil
}
func (f *fakeEngramClient) Export(ctx context.Context) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}
func (f *fakeEngramClient) Import(ctx context.Context, body io.Reader) (engram.ImportResult, error) {
	return engram.ImportResult{}, nil
}
func (f *fakeEngramClient) MergeProjects(ctx context.Context, source, target string) (engram.MergeProjectsResult, error) {
	return engram.MergeProjectsResult{Source: source, Target: target}, nil
}
func (f *fakeEngramClient) Timeline(ctx context.Context, req engram.TimelineRequest) ([]engram.TimelineEvent, error) {
	return nil, nil
}
func (f *fakeEngramClient) GetContext(ctx context.Context, req engram.ContextRequest) (engram.ContextResult, error) {
	if f.contextErr != nil {
		return engram.ContextResult{}, f.contextErr
	}
	return engram.ContextResult{Context: "o1: dup\no2: dup"}, nil
}
func (f *fakeEngramClient) UpdateContext(ctx context.Context, text string) (engram.ContextResult, error) {
	return engram.ContextResult{Context: text}, nil
}

func TestConsolidationManager_FullRun(t *testing.T) {
	fe := &fakeEngramClient{}
	fs := &fakeSessionAPI{
		msgs: []opencode.Message{
			{Role: "user", Text: "go"},
			{Role: "assistant", Text: "```json\n{\"deletes\":[{\"observation_id\":\"o1\",\"reason\":\"dup\"}],\"new_summaries\":[{\"type\":\"summary\",\"content\":\"one\",\"importance\":7}],\"digest\":\"x\"}\n```"},
		},
	}
	events := []string{}
	mgr := NewConsolidationManager(fe, func() opencode.SessionAPI { return fs },
		func(et string, payload any) { events = append(events, et) })

	id, err := mgr.Start(context.Background(), "anthropic/claude", "memory", ScopeFilter{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for the driver to reach awaiting_review.
	waitForStatus(t, mgr, id, StatusAwaitingReview, 100)

	// Apply a subset.
	err = mgr.Apply(context.Background(), id, ApplySelection{
		Deletes:      []engram.PlanDelete{{ObservationID: "o1", Reason: "dup"}},
		NewSummaries: []engram.PlanSummary{{Type: "summary", Content: "one", Scope: "project"}},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	run, _ := mgr.Get(id)
	if run.Status != StatusApplied {
		t.Fatalf("expected applied, got %s", run.Status)
	}
	if len(fe.deletedIDs) != 1 || fe.deletedIDs[0] != "o1" {
		t.Fatalf("expected delete o1, got %v", fe.deletedIDs)
	}
	if len(fe.saved) != 1 {
		t.Fatalf("expected 1 save, got %v", fe.saved)
	}
}

func TestConsolidationManager_NoSession(t *testing.T) {
	fe := &fakeEngramClient{}
	mgr := NewConsolidationManager(fe, func() opencode.SessionAPI { return nil }, nil)

	id, err := mgr.Start(context.Background(), "m", "memory", ScopeFilter{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForStatus(t, mgr, id, StatusFailed, 100)
	run, _ := mgr.Get(id)
	if run.Error == "" {
		t.Fatal("expected an error message")
	}
}

func TestExtractPlan_BadJSON(t *testing.T) {
	_, err := extractPlan([]opencode.Message{{Role: "assistant", Text: "not json"}})
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestExtractPlan_FencedJSON(t *testing.T) {
	msgs := []opencode.Message{{Role: "assistant", Text: "```json\n{\"digest\":\"ok\"}\n```"}}
	plan, err := extractPlan(msgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Digest != "ok" {
		t.Fatalf("expected digest 'ok', got %q", plan.Digest)
	}
}

func TestParseModelInput(t *testing.T) {
	mi := parseModelInput("anthropic/claude-sonnet-4")
	if mi.ProviderID != "anthropic" || mi.ID != "claude-sonnet-4" {
		t.Fatalf("unexpected: %+v", mi)
	}
	mi2 := parseModelInput("bare-model")
	if mi2.ProviderID != "" || mi2.ID != "bare-model" {
		t.Fatalf("unexpected: %+v", mi2)
	}
	if parseModelInput("") != nil {
		t.Fatal("expected nil for empty input")
	}
}

// waitForStatus polls the manager until the run reaches the wanted status or
// the attempt budget is depleted.
func waitForStatus(t *testing.T, mgr *ConsolidationManager, id, want string, attempts int) {
	t.Helper()
	for i := 0; i < attempts; i++ {
		run, ok := mgr.Get(id)
		if ok && run.Status == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run %s never reached %s", id, want)
}
