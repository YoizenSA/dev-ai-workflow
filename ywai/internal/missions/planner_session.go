package missions

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// PlannerSession drives the iterative (iterative) planning flow over a single
// opencode session: Investigate → ProposeArchitecture → GenerateFeatures.
//
// It keeps the session alive across stages so the model accumulates context
// (codebase findings, confirmed architecture) before generating features — the
// key difference from the one-shot GeneratePlanWithOpencode.
//
// When the opencode server is unavailable (LocalClient returns nil from
// Sessions()), CanUseSessions reports false and callers must fall back to the
// one-shot CLI planner.
type PlannerSession struct {
	client    opencode.Client
	model     string
	agent     string
	sessionID string // set after the first Prompt/Create

	// timeout per stage prompt (investigate/architecture/features).
	timeout time.Duration
}

// NewPlannerSession constructs a PlannerSession over the given client.
func NewPlannerSession(client opencode.Client, model, agent string) *PlannerSession {
	if agent == "" {
		agent = "orchestrator"
	}
	return &PlannerSession{
		client:  client,
		model:   model,
		agent:   agent,
		timeout: 3 * time.Minute,
	}
}

// CanUseSessions reports whether the configured client supports the Sessions
// API (i.e. the opencode server is reachable). When false, callers must use the
// one-shot CLI planner (GeneratePlanWithOpencode) instead of PlanInteractive.
func (ps *PlannerSession) CanUseSessions() bool {
	if ps.client == nil {
		return false
	}
	sessions := ps.client.Sessions()
	if sessions == nil {
		return false
	}
	// Probe with a cheap Status call; LocalClient returns a stub that errors.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	status, err := ps.client.Status(ctx)
	if err != nil {
		return false
	}
	return status.Connected && status.Source == "server"
}

// ensureSession creates the underlying opencode session on first use, then
// reuses it for subsequent stages so context accumulates.
func (ps *PlannerSession) ensureSession(ctx context.Context, title string) error {
	if ps.sessionID != "" {
		return nil // already created
	}
	sessions := ps.client.Sessions()
	if sessions == nil {
		return fmt.Errorf("sessions API unavailable")
	}
	session, err := sessions.Create(ctx, opencode.SessionCreateOpts{
		Title: title,
		Agent: ps.agent,
		Model: parseModelString(ps.model),
	})
	if err != nil {
		return fmt.Errorf("create planner session: %w", err)
	}
	ps.sessionID = session.ID
	return nil
}

// prompt sends a prompt to the session, waits for completion, and returns the
// last assistant message text. Respects ps.timeout unless ctx has a shorter
// deadline.
func (ps *PlannerSession) prompt(ctx context.Context, text string) (string, error) {
	if err := ps.ensureSession(ctx, "ywai-planner"); err != nil {
		return "", err
	}

	stageCtx, cancel := context.WithTimeout(ctx, ps.timeout)
	defer cancel()

	sessions := ps.client.Sessions()
	if _, err := sessions.Prompt(stageCtx, ps.sessionID, opencode.PromptInput{
		Text:     text,
		Delivery: "immediate",
	}); err != nil {
		if stageCtx.Err() == context.DeadlineExceeded {
			return "", context.DeadlineExceeded
		}
		return "", fmt.Errorf("send prompt: %w", err)
	}

	if err := sessions.Wait(stageCtx, ps.sessionID); err != nil {
		if stageCtx.Err() == context.DeadlineExceeded {
			return "", context.DeadlineExceeded
		}
		return "", fmt.Errorf("wait for session: %w", err)
	}

	messages, err := sessions.Messages(stageCtx, ps.sessionID)
	if err != nil {
		return "", fmt.Errorf("get messages: %w", err)
	}
	return lastAssistantText(messages), nil
}

// Investigate is stage 1: ask the model to explore the repo for the given goal
// and enumerate unknowns / open questions. Returns the assistant's response
// (free-form markdown) for the caller to surface to the user.
func (ps *PlannerSession) Investigate(ctx context.Context, goal, repoPath string) (string, error) {
	prompt := buildInvestigatePrompt(goal, repoPath)
	return ps.prompt(ctx, prompt)
}

// ProposeArchitecture is stage 2: given the accumulated investigation context,
// ask the model to produce an architecture proposal. Returns markdown.
func (ps *PlannerSession) ProposeArchitecture(ctx context.Context) (string, error) {
	prompt := `Based on the investigation above, propose the architecture for this mission.
Return markdown with sections: ## Components, ## Data Flow, ## Technology Stack, ## Boundaries.
Be concrete about how the pieces fit together. Output ONLY the markdown.`
	return ps.prompt(ctx, prompt)
}

// GenerateFeatures is stage 3: given confirmed milestones, ask the model to
// decompose into a PlanMission JSON. Every feature must reference one of the
// confirmed milestones.
func (ps *PlannerSession) GenerateFeatures(ctx context.Context, confirmedMilestones []string) (*PlanMission, error) {
	prompt := buildGenerateFeaturesPrompt(confirmedMilestones)
	raw, err := ps.prompt(ctx, prompt)
	if err != nil {
		return nil, err
	}
	plan, err := parsePlanFromOutput(raw)
	if err != nil {
		return nil, fmt.Errorf("parse plan from planner session: %w", err)
	}
	return plan, nil
}

// Close releases the underlying session.
func (ps *PlannerSession) Close() {
	if ps.sessionID == "" || ps.client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if sessions := ps.client.Sessions(); sessions != nil {
		if err := sessions.Delete(ctx, ps.sessionID); err != nil {
			log.Printf("planner session delete: %v", err)
		}
	}
}

// ─── Prompt builders ────────────────────────────────────────────────────────

func buildInvestigatePrompt(goal, repoPath string) string {
	var b strings.Builder
	b.WriteString("You are planning a software mission. Investigate the codebase and enumerate what you need to know.\n\n")
	b.WriteString("## Goal\n")
	b.WriteString(goal)
	b.WriteString("\n\n")
	if repoPath != "" {
		b.WriteString("## Repository\n")
		b.WriteString(fmt.Sprintf("The code is at `%s`. Read it to understand existing patterns, conventions, and how things fit together.\n\n", repoPath))
	}
	b.WriteString("## Your task (investigation ONLY — do NOT write code)\n")
	b.WriteString("1. Read the repo structure, README, configs, and key source files.\n")
	b.WriteString("2. Identify how this goal fits into the existing system.\n")
	b.WriteString("3. Enumerate the OPEN QUESTIONS and UNKNOWNS that need answers before planning can proceed.\n")
	b.WriteString("4. Note any constraints, existing services, ports in use, or off-limits areas.\n\n")
	b.WriteString("Return markdown with:\n")
	b.WriteString("- ## Findings (what you discovered)\n")
	b.WriteString("- ## Open Questions (numbered list of unknowns for the user)\n")
	b.WriteString("Output ONLY the markdown.")
	return b.String()
}

func buildGenerateFeaturesPrompt(milestones []string) string {
	var b strings.Builder
	b.WriteString("Now generate the mission features. Each feature must reference one of these confirmed milestones:\n")
	for _, m := range milestones {
		b.WriteString(fmt.Sprintf("- %s\n", m))
	}
	b.WriteString("\nOutput ONLY a valid JSON object (no markdown, no fences) with this structure:\n")
	b.WriteString(`{
  "name": "short mission name",
  "description": "what this mission accomplishes",
  "milestones": [{"name": "milestone-name", "description": "what it delivers"}],
  "features": [
    {
      "id": "feat-1",
      "description": "concrete description",
      "role": "dev",
      "skillName": "implementation",
      "milestone": "milestone-name",
      "preconditions": [],
      "expectedBehavior": ["assertion 1"],
      "fulfills": ["VAL-AREA-001"]
    }
  ]
}
`)
	b.WriteString("\nRules:\n")
	b.WriteString("- Every feature must reference a milestone from the confirmed list above.\n")
	b.WriteString("- Roles: planning, dev, frontend, backend, qa, reviewer, devops.\n")
	b.WriteString("- Features should be small and focused; order by dependency.\n")
	return b.String()
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// lastAssistantText returns the text of the last assistant message, or empty
// string if there is none. Mirrors the worker's extractHandoffFromMessages.
func lastAssistantText(messages []opencode.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			return messages[i].Text
		}
	}
	return ""
}
