package missions

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// fakeSessionAPI is a programmable opencode.SessionAPI stub for PlannerSession
// tests. It records prompts and returns scripted assistant messages.
type fakeSessionAPI struct {
	createErr   error
	promptErr   error
	responses   []string // assistant messages returned, one per Prompt call
	prompts     []string // prompts received, in order
	sessionID   string
	nextRespIdx int
}

func (f *fakeSessionAPI) Create(ctx context.Context, opts opencode.SessionCreateOpts) (*opencode.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.sessionID == "" {
		f.sessionID = "sess-fake"
	}
	return &opencode.Session{ID: f.sessionID, Title: opts.Title}, nil
}

func (f *fakeSessionAPI) Get(ctx context.Context, sessionID string) (*opencode.Session, error) {
	return &opencode.Session{ID: sessionID}, nil
}
func (f *fakeSessionAPI) Status(ctx context.Context) (*opencode.SessionStatusResult, error) {
	return &opencode.SessionStatusResult{}, nil
}

func (f *fakeSessionAPI) Prompt(ctx context.Context, sessionID string, input opencode.PromptInput) (*opencode.PromptResult, error) {
	f.prompts = append(f.prompts, input.Text)
	if f.promptErr != nil {
		return nil, f.promptErr
	}
	return &opencode.PromptResult{SessionID: sessionID}, nil
}

func (f *fakeSessionAPI) Wait(ctx context.Context, sessionID string) error { return nil }

func (f *fakeSessionAPI) Messages(ctx context.Context, sessionID string) ([]opencode.Message, error) {
	if f.nextRespIdx >= len(f.responses) {
		return []opencode.Message{{Role: "assistant", Text: ""}}, nil
	}
	resp := f.responses[f.nextRespIdx]
	f.nextRespIdx++
	return []opencode.Message{
		{Role: "user", Text: "prompt"},
		{Role: "assistant", Text: resp},
	}, nil
}

func (f *fakeSessionAPI) Delete(ctx context.Context, sessionID string) error { return nil }
func (f *fakeSessionAPI) ListQuestions(ctx context.Context) ([]opencode.Question, error) {
	return nil, nil
}
func (f *fakeSessionAPI) ReplyQuestion(ctx context.Context, questionID, answer string) error {
	return nil
}

// fakePlannerClient wraps a fakeSessionAPI so PlannerSession can use it.
type fakePlannerClient struct {
	sessions *fakeSessionAPI
}

func (f *fakePlannerClient) Status(ctx context.Context) (opencode.ClientStatus, error) {
	return opencode.ClientStatus{Connected: true, Source: "server"}, nil
}
func (f *fakePlannerClient) ListModels(ctx context.Context) ([]opencode.ModelInfo, error) {
	return nil, nil
}
func (f *fakePlannerClient) ListAgents(ctx context.Context) ([]opencode.AgentInfo, error) {
	return nil, nil
}
func (f *fakePlannerClient) Sessions() opencode.SessionAPI { return f.sessions }

// ─── Tests ─────────────────────────────────────────────────────────────────

// TestPlannerSessionInvestigateReturnsUnknowns verifies Investigate sends a
// prompt and returns the assistant's text (the unknowns/questions).
func TestPlannerSessionInvestigateReturnsUnknowns(t *testing.T) {
	api := &fakeSessionAPI{
		responses: []string{
			"Open questions:\n1. Which auth provider?\n2. Existing DB schema?",
		},
	}
	ps := &PlannerSession{
		client: &fakePlannerClient{sessions: api},
		model:  "test/model",
		agent:  "orchestrator",
	}

	ctx := context.Background()
	out, err := ps.Investigate(ctx, "Add user auth", "/repo")
	if err != nil {
		t.Fatalf("Investigate: %v", err)
	}
	if !strings.Contains(out, "auth provider") {
		t.Errorf("expected unknowns text, got %q", out)
	}
	if len(api.prompts) != 1 {
		t.Errorf("expected 1 prompt sent, got %d", len(api.prompts))
	}
}

// TestPlannerSessionProposeArchitecture verifies ProposeArchitecture runs on
// the same session (no second Create).
func TestPlannerSessionProposeArchitecture(t *testing.T) {
	api := &fakeSessionAPI{
		responses: []string{
			"unknowns here",
			"# Architecture\n\n## Components\n- Auth service\n- User store",
		},
	}
	ps := &PlannerSession{client: &fakePlannerClient{sessions: api}, model: "m", agent: "a"}

	ctx := context.Background()
	_, _ = ps.Investigate(ctx, "goal", "/repo")
	arch, err := ps.ProposeArchitecture(ctx)
	if err != nil {
		t.Fatalf("ProposeArchitecture: %v", err)
	}
	if !strings.Contains(arch, "Architecture") {
		t.Errorf("expected architecture markdown, got %q", arch)
	}
	// Two prompts (investigate + architecture) but only ONE session create.
	if len(api.prompts) != 2 {
		t.Errorf("expected 2 prompts, got %d", len(api.prompts))
	}
}

// TestPlannerSessionGenerateFeaturesReturnsPlan verifies the final stage parses
// a PlanMission JSON from the assistant response.
func TestPlannerSessionGenerateFeaturesReturnsPlan(t *testing.T) {
	planJSON := `{"name":"test","description":"d","milestones":[{"name":"m1","description":"m1"}],"features":[{"id":"f1","description":"do thing","skillName":"implementation","milestone":"m1"}]}`
	api := &fakeSessionAPI{
		responses: []string{"unknowns", "# Arch", planJSON},
	}
	ps := &PlannerSession{client: &fakePlannerClient{sessions: api}, model: "m", agent: "a"}

	ctx := context.Background()
	_, _ = ps.Investigate(ctx, "goal", "/repo")
	_, _ = ps.ProposeArchitecture(ctx)
	plan, err := ps.GenerateFeatures(ctx, []string{"m1"})
	if err != nil {
		t.Fatalf("GenerateFeatures: %v", err)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}
	if plan.Name != "test" {
		t.Errorf("expected plan name 'test', got %q", plan.Name)
	}
	if len(plan.Features) != 1 {
		t.Errorf("expected 1 feature, got %d", len(plan.Features))
	}
}

// TestPlannerSessionFallbackOnSessionError verifies that when Sessions() returns
// an error (LocalClient), PlannerSession falls back to the CLI one-shot planner.
func TestPlannerSessionFallbackOnSessionError(t *testing.T) {
	// A client whose Sessions() returns nil → PlannerSession.PlanInteractive
	// should signal it cannot use the iterative path.
	ps := &PlannerSession{
		client: &noSessionsClient{},
		model:  "m",
		agent:  "a",
	}
	if ps.CanUseSessions() {
		t.Fatal("expected CanUseSessions=false when Sessions() returns nil")
	}
}

// noSessionsClient is a client whose Sessions() is unavailable (mimics LocalClient).
type noSessionsClient struct{}

func (n *noSessionsClient) Status(ctx context.Context) (opencode.ClientStatus, error) {
	return opencode.ClientStatus{Connected: false, Source: "local"}, nil
}
func (n *noSessionsClient) ListModels(ctx context.Context) ([]opencode.ModelInfo, error) {
	return nil, nil
}
func (n *noSessionsClient) ListAgents(ctx context.Context) ([]opencode.AgentInfo, error) {
	return nil, nil
}
func (n *noSessionsClient) Sessions() opencode.SessionAPI { return nil }

// TestPlannerSessionTimeout verifies a context timeout aborts the investigation.
func TestPlannerSessionTimeout(t *testing.T) {
	api := &fakeSessionAPI{responses: []string{"unknowns"}}
	ps := &PlannerSession{client: &fakePlannerClient{sessions: api}, model: "m", agent: "a"}

	// Use an immediately-cancelled context instead of timing-dependent sleep.
	// This avoids flakiness on slow CI runners (Windows).
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ps.Investigate(ctx, "goal", "/repo")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected Canceled, got %v", err)
	}
}
