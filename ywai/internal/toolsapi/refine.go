package toolsapi

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// RefineGoalWithOpencode uses the opencode CLI (not the HTTP server, which has
// known issues processing prompts via REST) to refine a user goal into a
// structured mission description. Returns the refined markdown text.
//
// model optionally overrides the opencode default model — important when the
// default has too little context for the refinement prompt.
//
// If opencode is unavailable or fails, it falls back to a locally-built
// refinement so the user still gets something useful.
func RefineGoalWithOpencode(goal, extraContext, model, agentName string) string {
	opencodePath, _ := agent.FindOpenCode()
	if opencodePath == "" {
		log.Printf("opencode not available, using local goal refinement")
		return localRefineGoal(goal)
	}

	// Refinement is a planning task, so default to the planning role's agent
	// instead of letting opencode fall back to its generic "build" agent.
	if agentName == "" {
		agentName = planningAgentDefault()
	}

	prompt := buildRefinePrompt(goal, extraContext)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Build args with optional --model and --agent.
	args := []string{"run"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if agentName != "" {
		args = append(args, "--agent", agentName)
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, opencodePath, args...)
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	output, err := cmd.Output()
	if err != nil {
		log.Printf("opencode goal refinement failed (agent=%s, model=%s): %v\nstderr: %s", agentName, model, err, stderrBuf.String())
		return localRefineGoal(goal)
	}

	refined := strings.TrimSpace(string(output))
	if refined == "" {
		log.Printf("opencode goal refinement returned empty output (agent=%s, model=%s)\nstderr: %s", agentName, model, stderrBuf.String())
		return localRefineGoal(goal)
	}

	// Strip opencode status lines (e.g. "> orchestrator · mimo-v2.5-pro")
	// and ANSI escape codes that get mixed into stdout.
	refined = stripOpencodeNoise(refined)
	if refined == "" {
		log.Printf("opencode goal refinement was empty after stripping noise")
		return localRefineGoal(goal)
	}
	return refined
}

var (
	// ansiCSI matches ANSI CSI escape sequences (colors, cursor moves, …).
	// Single source of truth: control's PTY streaming strips with it too.
	ansiCSI        = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	opencodeLineRE = regexp.MustCompile(`(?m)^>?\s*\w+\s*·\s*\S+\s*$`)
)

// StripANSI removes terminal ANSI escape sequences from command output.
func StripANSI(s string) string {
	return ansiCSI.ReplaceAllString(s, "")
}

// stripOpencodeNoise removes ANSI escape codes and opencode status lines
// (e.g. "> orchestrator · mimo-v2.5-pro") from command output.
func stripOpencodeNoise(s string) string {
	s = StripANSI(s)
	s = opencodeLineRE.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// planningAgentDefault returns the agent configured for the planning role in
// the role-defaults — the single source of truth for agent selection. The value
// comes from user config or the embedded seed; it is never a hardcoded literal.
// This keeps goal refinement on the configured planning agent.
func planningAgentDefault() string {
	cfg, _ := config.LoadConfig()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return cfg.GetRoleDefault(config.RolePlanning).Agent
}

// buildRefinePrompt constructs the prompt sent to opencode to refine a goal.
func buildRefinePrompt(goal, extraContext string) string {
	prompt := fmt.Sprintf(`You are a senior product engineer. Analyze this mission goal and produce a concrete, actionable refinement.

User's goal: %s

Think step by step BEFORE writing the output:
1. What is the user actually trying to build or achieve?
2. What technologies, frameworks, or tools are likely involved?
3. What are the key deliverables?
4. What are realistic acceptance criteria that prove "done"?
5. What should be explicitly OUT of scope to keep the mission focused?

Then return ONLY this markdown (no preamble, no explanation):

## Goal
[One clear sentence - what specifically will be built or changed]

## Scope
- [Concrete deliverable 1 - name the feature/component/system]
- [Concrete deliverable 2]
- [Concrete deliverable 3 if applicable]

## Out of Scope
- [Specific thing that's related but deferred - be explicit, not generic]
- [Another explicit exclusion]

## Acceptance Criteria
- [Specific, testable criterion — e.g. "user can do X and sees Y"]
- [Another specific criterion]
- [Another if applicable]`, goal)

	if extraContext != "" {
		prompt = fmt.Sprintf("Additional context about the project or codebase:\n%s\n\n%s", extraContext, prompt)
	}
	return prompt
}

// localRefineGoal produces a structured refinement without calling opencode,
// used as a fallback when the CLI is unavailable or fails.
func localRefineGoal(goal string) string {
	return fmt.Sprintf(`## Goal
%s

## Scope
- Core implementation of the described feature
- Basic tests covering the main behavior

## Out of Scope
- Advanced edge cases (can be added in follow-up missions)
- Performance optimization

## Acceptance Criteria
- The feature works as described in the goal
- Tests pass for the implemented behavior`, goal)
}
