package missions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// generationHint is used internally to shape plan generation based on
// user clarifications.
type generationHint struct {
	Technologies []string
	Scope        string // "small", "medium", "large"
	Constraints  []string
}

// ─── Plan Generation ───────────────────────────────────────────────────────

// GeneratePlan produces a structured PlanMission from a user goal and
// optional clarifications. The result is deterministic for the same inputs.
func GeneratePlan(goal string, clarifications []QAPair) *PlanMission {
	if goal == "" {
		return nil
	}

	hints := extractHints(goal, clarifications)
	name := extractName(goal)
	description := goal

	now := time.Now()

	// Determine milestones and features based on scope and goal analysis
	var milestones []PlanMilestone
	var features []PlanFeature

	switch hints.Scope {
	case "large":
		milestones = []PlanMilestone{
			{Name: "foundation", Description: "Core setup and infrastructure"},
			{Name: "core-features", Description: "Primary feature implementation"},
			{Name: "polish", Description: "Testing, refinement, and documentation"},
		}
	case "medium":
		milestones = []PlanMilestone{
			{Name: "core-implementation", Description: "Main feature development"},
			{Name: "refinement", Description: "Testing, fixes, and documentation"},
		}
	default: // "small"
		milestones = []PlanMilestone{
			{Name: "core-engine", Description: "Core implementation milestone"},
		}
	}

	// Generate features based on goal analysis
	features = generateFeatures(goal, milestones, hints, now)

	// Assign assertion IDs for validation
	for i := range features {
		features[i].Fulfills = []string{
			fmt.Sprintf("VAL-ENG-PLAN-%03d", i+1),
		}
	}

	return &PlanMission{
		Name:        name,
		Description: description,
		Milestones:  milestones,
		Features:    features,
	}
}

// GeneratePlanWithOpencode spawns opencode to generate a plan from a goal.
// Falls back to GeneratePlan if opencode is unavailable or fails.
//
// User-configured role defaults are loaded and applied per feature, so each
// feature carries the right Role/Model/Agent/Fallbacks for the worker stage.
func GeneratePlanWithOpencode(goal string, clarifications []QAPair, project, model, agent string) *PlanMission {
	return GeneratePlanWithRepo(goal, clarifications, project, model, agent, "")
}

// GeneratePlanWithRepo is the repo-aware variant of GeneratePlanWithOpencode.
// When repoPath is non-empty, the planner prompt instructs opencode to read the
// actual codebase and ground the plan in real patterns. Used by the auto path
// (PlanAndApprove) where there's no interactive investigation step.
func GeneratePlanWithRepo(goal string, clarifications []QAPair, project, model, agent, repoPath string) *PlanMission {
	cfg, _ := config.LoadConfig()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	hints := extractHints(goal, clarifications)

	// applyModelAgent ensures the selected model/agent are persisted on the plan
	// so they flow through approval into the mission and reach the workers.
	// Role defaults are then applied per feature so individual workers can use
	// role-specific models even when the mission has no global override.
	applyModelAgent := func(p *PlanMission) *PlanMission {
		if p == nil {
			return nil
		}
		if model != "" {
			p.Model = model
		}
		if agent != "" {
			p.Agent = agent
		}
		if project != "" {
			p.Project = project
		}
		applyRoleDefaults(p.Features, hints, cfg)
		// An explicit --model / --agent must reach the workers: applyRoleDefaults
		// fills per-feature Model/Agent from role-default config, which otherwise
		// silently overrides the user's choice (the worker uses feature.Model, not
		// the mission-level Model). Force the override per feature when set.
		if model != "" {
			for i := range p.Features {
				p.Features[i].Model = model
			}
		}
		if agent != "" {
			for i := range p.Features {
				p.Features[i].Agent = agent
			}
		}
		return p
	}

	// localFallbackPlan builds a generic plan with the local planner and flags
	// it as a fallback so callers can warn that it doesn't reflect the goal.
	localFallbackPlan := func() *PlanMission {
		p := applyModelAgent(GeneratePlan(goal, clarifications))
		if p != nil {
			p.LocalFallback = true
		}
		return p
	}

	opencodePath, err := DetectOpencode()
	if err != nil {
		log.Printf("opencode not available, falling back to local planning: %v", err)
		return localFallbackPlan()
	}

	// Build the prompt for opencode
	prompt := buildPlanPromptWithRepo(goal, clarifications, project, repoPath)

	// Spawn opencode with the prompt as a task argument
	// opencode run "..." processes a task non-interactively and exits
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Build args with optional --model and --agent
	args := []string{"run"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if agent != "" {
		args = append(args, "--agent", agent)
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, opencodePath, args...)
	cmd.Stderr = os.Stderr // let stderr show through for debugging

	output, err := cmd.Output()
	if err != nil {
		log.Printf("opencode plan generation failed: %v, falling back to local planning", err)
		return localFallbackPlan()
	}

	// Try to parse the output as JSON plan
	plan, parseErr := parsePlanFromOutput(string(output))
	if parseErr != nil {
		log.Printf("parse opencode output: %v, falling back to local planning", parseErr)
		return localFallbackPlan()
	}

	return applyModelAgent(plan)
}

// RefineGoalWithOpencode uses the opencode CLI (not the HTTP server, which has
// known issues processing prompts via REST) to refine a user goal into a
// structured mission description. Returns the refined markdown text.
//
// model optionally overrides the opencode default model — important when the
// default has too little context for the refinement prompt.
//
// If opencode is unavailable or fails, it falls back to a locally-built
// refinement so the user still gets something useful.
func RefineGoalWithOpencode(goal, extraContext, model, agent string) string {
	opencodePath, err := DetectOpencode()
	if err != nil {
		log.Printf("opencode not available, using local goal refinement: %v", err)
		return localRefineGoal(goal)
	}

	// Refinement is a planning task, so default to the planning role's agent
	// instead of letting opencode fall back to its generic "build" agent.
	if agent == "" {
		agent = planningAgentDefault()
	}

	prompt := buildRefinePrompt(goal, extraContext)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Build args with optional --model and --agent (mirrors GeneratePlanWithOpencode).
	args := []string{"run"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if agent != "" {
		args = append(args, "--agent", agent)
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, opencodePath, args...)
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	output, err := cmd.Output()
	if err != nil {
		log.Printf("opencode goal refinement failed (agent=%s, model=%s): %v\nstderr: %s", agent, model, err, stderrBuf.String())
		return localRefineGoal(goal)
	}

	refined := strings.TrimSpace(string(output))
	if refined == "" {
		log.Printf("opencode goal refinement returned empty output (agent=%s, model=%s)\nstderr: %s", agent, model, stderrBuf.String())
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
	ansiRE         = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	opencodeLineRE = regexp.MustCompile(`(?m)^>?\s*\w+\s*·\s*\S+\s*$`)
)

// stripOpencodeNoise removes ANSI escape codes and opencode status lines
// (e.g. "> orchestrator · mimo-v2.5-pro") from command output.
func stripOpencodeNoise(s string) string {
	s = ansiRE.ReplaceAllString(s, "")
	s = opencodeLineRE.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// planningAgentDefault returns the agent configured for the planning role in
// the role-defaults — the single source of truth for agent selection. The value
// comes from user config or the embedded seed; it is never a hardcoded literal.
// This keeps goal refinement and orchestration on the configured planning agent.
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
[One clear sentence — what specifically will be built or changed]

## Scope
- [Concrete deliverable 1 — name the feature/component/system]
- [Concrete deliverable 2]
- [Concrete deliverable 3 if applicable]

## Out of Scope
- [Specific thing that's related but deferred — be explicit, not generic]
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

// buildPlanPromptWithRepo is the repo-aware variant. When repoPath is non-empty,
// the prompt instructs opencode to read the actual codebase and ground the plan
// in real patterns/conventions (codebase-aligned one-shot investigation).
func buildPlanPromptWithRepo(goal string, clarifications []QAPair, project, repoPath string) string {
	var sb strings.Builder
	sb.WriteString("You are a technical architect following a structured mission planning methodology. Generate a development plan for the following goal.\n\n")
	sb.WriteString("## Planning Phases\n")
	sb.WriteString("1. Understand & Plan - Deeply understand requirements, investigate codebase, identify unknowns\n")
	sb.WriteString("2. Architectural Design & Decomposition - Define system components, responsibilities, interactions\n")
	sb.WriteString("3. Infrastructure & Boundaries - Check existing services, define port ranges, identify off-limits resources\n")
	sb.WriteString("4. Testing & Validation Strategy - Determine testing infrastructure, user testing surface\n")
	sb.WriteString("5. Identify & Confirm Milestones - Get explicit user agreement on milestone boundaries\n")
	sb.WriteString("6. Create Mission Proposal - Generate the plan with features and assertions\n\n")

	sb.WriteString("## Goal\n")
	sb.WriteString(goal)
	sb.WriteString("\n\n")

	if repoPath != "" {
		sb.WriteString("## Repository (READ THIS FIRST)\n")
		sb.WriteString(fmt.Sprintf("The code is at `%s`. Read the README, directory structure, configs, and key source files before planning. Ground every feature in real existing patterns, conventions, and dependencies you find there. Do not invent components that already exist.\n\n", repoPath))
	}

	if len(clarifications) > 0 {
		sb.WriteString("## Clarifications\n")
		for _, qa := range clarifications {
			sb.WriteString(fmt.Sprintf("- Q: %s\n  A: %s\n", qa.Question, qa.Answer))
		}
		sb.WriteString("\n")
	}

	if project != "" {
		sb.WriteString(fmt.Sprintf("## Project\n%s\n\n", project))
	}

	sb.WriteString(`Output ONLY a valid JSON object with this exact structure (no markdown, no code fences, no explanations):
{
  "name": "short mission name",
  "description": "brief description of what this mission accomplishes",
  "project": "` + project + `",
  "milestones": [
    {"name": "milestone-name", "description": "what this milestone delivers"}
  ],
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

IMPORTANT RULES:
- Each feature must have a unique id (feat-1, feat-2, etc.)
- Each feature must reference a milestone that exists
- preconditions is a list of feature IDs that must be done first
- Valid role values: planning, architect, dev, frontend, backend, qa, reviewer, devops. Pick the one that best matches the work the feature implies. Use "architect" for upfront design work — choosing patterns, system structure, interfaces, and trade-offs — that should land before implementation features.
- skillName is optional; if omitted it is derived from role (architect → architect-worker, frontend → frontend-worker, backend → backend-worker, qa → qa-worker, devops → devops-worker, reviewer → reviewer-worker, planning → planner, dev → implementation).
- expectedBehavior is a list of verifiable assertions
- fulfills MUST reference validation contract assertion IDs with format VAL-AREA-XXX (e.g., VAL-AUTH-001, VAL-CROSS-002)
- Each assertion ID should be claimed by exactly ONE feature across the entire plan
- Milestones should represent testable, coherent states that leave the product in a working condition
- Consider infrastructure needs (services, ports, dependencies) when designing features
- Features should be small and focused (one feature = one logical change)
- Order features by dependency (prerequisites first)
`)
	return sb.String()
}

// parsePlanFromOutput extracts a PlanMission from opencode's output.
// Handles both raw JSON and markdown code-fenced JSON.
func parsePlanFromOutput(output string) (*PlanMission, error) {
	// Try to find JSON within markdown code fences first
	re := regexp.MustCompile("```(?:json)?\\s*\\n([\\s\\S]*?)```")
	matches := re.FindStringSubmatch(output)

	var jsonStr string
	if len(matches) >= 2 {
		jsonStr = strings.TrimSpace(matches[1])
	} else {
		// Try the whole output as JSON
		jsonStr = strings.TrimSpace(output)
	}

	// Normalize opencode output: some fields may be strings instead of arrays
	reStr := regexp.MustCompile(`"(fulfills|expectedBehavior|preconditions)":\s*"([^"]+)"`)
	jsonStr = reStr.ReplaceAllString(jsonStr, `"$1":["$2"]`)

	var plan PlanMission
	if err := json.Unmarshal([]byte(jsonStr), &plan); err != nil {
		return nil, fmt.Errorf("unmarshal plan: %w", err)
	}

	if plan.Name == "" {
		return nil, errors.New("plan missing name")
	}
	if len(plan.Milestones) == 0 {
		return nil, errors.New("plan missing milestones")
	}
	if len(plan.Features) == 0 {
		return nil, errors.New("plan missing features")
	}

	// Set default skillName for features missing it. Role/Model/Agent/Fallbacks
	// are populated downstream by applyRoleDefaults so user-configured role
	// defaults always win over the planner's bare output.
	for i := range plan.Features {
		if plan.Features[i].SkillName == "" {
			plan.Features[i].SkillName = "implementation"
		}
	}

	return &plan, nil
}

// generateFeatures creates features based on the goal and milestone structure.
func generateFeatures(goal string, milestones []PlanMilestone, hints generationHint, now time.Time) []PlanFeature {
	var features []PlanFeature
	words := strings.Fields(goal)

	// For each milestone, create appropriate features
	for mi, ms := range milestones {
		featCount := featuresPerMilestone(len(milestones), mi, len(words))

		for fi := 0; fi < featCount; fi++ {
			featID := fmt.Sprintf("feat-%s-%d", ms.Name, fi+1)
			desc := deriveFeatureDescription(goal, ms, fi, featCount, hints)

			// Pick a role first, then derive the canonical worker skill name.
			role := detectRole(desc, hints)
			skill := RoleToSkillName(role)

			feat := PlanFeature{
				ID:          featID,
				Description: desc,
				SkillName:   skill,
				Role:        role,
				Milestone:   ms.Name,
				Expected:    deriveExpectedBehaviors(desc),
			}

			// Set preconditions: second+ features in a milestone depend on the first
			if fi > 0 {
				feat.Preconditions = []string{
					fmt.Sprintf("feat-%s-%d", ms.Name, fi),
				}
			}

			features = append(features, feat)
		}
	}

	return features
}

// featuresPerMilestone determines how many features to create for a given
// milestone based on scope and position.
func featuresPerMilestone(milestoneCount, milestoneIndex, wordCount int) int {
	if wordCount < 5 {
		return 1 // very simple goal → 1 feature per milestone
	}
	if milestoneCount <= 1 {
		if wordCount < 10 {
			return 1
		}
		return 2
	}
	if milestoneIndex == 0 {
		return 2 // first milestone gets minimum 2 features
	}
	return 2
}

// deriveFeatureDescription creates a human-readable description for a feature
// based on the milestone context and goal.
func deriveFeatureDescription(goal string, ms PlanMilestone, featureIndex, totalFeatures int, hints generationHint) string {
	if totalFeatures == 1 {
		return fmt.Sprintf("Implement %s", strings.ToLower(goal))
	}

	templates := []string{
		fmt.Sprintf("Implement core %s functionality", ms.Name),
		fmt.Sprintf("Add %s integration and tests", ms.Name),
		fmt.Sprintf("Polish and document %s", ms.Name),
	}

	idx := featureIndex
	if idx >= len(templates) {
		idx = len(templates) - 1
	}
	return templates[idx]
}

// deriveExpectedBehaviors returns default expected behaviors for a feature.
func deriveExpectedBehaviors(description string) []string {
	return []string{
		fmt.Sprintf("%s is implemented correctly", description),
		fmt.Sprintf("%s has tests", description),
		fmt.Sprintf("%s integrates with existing system", description),
	}
}

// detectRole picks a plausible execution role based on description keywords
// and the parsed technology hints. Returned roles match config.Role* constants.
func detectRole(desc string, hints generationHint) string {
	lower := strings.ToLower(desc)

	for _, tech := range hints.Technologies {
		t := strings.ToLower(tech)
		switch {
		case strings.Contains(t, "react") || strings.Contains(t, "frontend") || strings.Contains(t, "ui"):
			return config.RoleFrontend
		case strings.Contains(t, "go") || strings.Contains(t, "golang") || strings.Contains(t, "backend") || strings.Contains(t, "api"):
			return config.RoleBackend
		case strings.Contains(t, "test") || strings.Contains(t, "qa"):
			return config.RoleQA
		case strings.Contains(t, "infra") || strings.Contains(t, "devops") || strings.Contains(t, "deploy"):
			return config.RoleDevops
		case strings.Contains(t, "review") || strings.Contains(t, "audit"):
			return config.RoleReviewer
		}
	}

	switch {
	case strings.Contains(lower, "architecture") || strings.Contains(lower, "design pattern") || strings.Contains(lower, "system design") || strings.Contains(lower, "design the"):
		return config.RoleArchitect
	case strings.Contains(lower, "review") || strings.Contains(lower, "audit"):
		return config.RoleReviewer
	case strings.Contains(lower, "test") || strings.Contains(lower, "qa") || strings.Contains(lower, "coverage"):
		return config.RoleQA
	case strings.Contains(lower, "ui") || strings.Contains(lower, "frontend") || strings.Contains(lower, "web") || strings.Contains(lower, "component"):
		return config.RoleFrontend
	case strings.Contains(lower, "api") || strings.Contains(lower, "backend") || strings.Contains(lower, "server") || strings.Contains(lower, "endpoint"):
		return config.RoleBackend
	case strings.Contains(lower, "infra") || strings.Contains(lower, "deploy") || strings.Contains(lower, "ci") || strings.Contains(lower, "docker") || strings.Contains(lower, "kubernetes"):
		return config.RoleDevops
	default:
		return config.RoleDev
	}
}

// applyRoleDefaults populates Role/Model/Agent/Fallbacks on each plan feature
// from the user's role defaults. Pre-existing values on the feature are kept
// (so an LLM-emitted plan can pin specific models).
func applyRoleDefaults(features []PlanFeature, hints generationHint, cfg *config.UserConfig) {
	for i := range features {
		if features[i].Role == "" {
			features[i].Role = detectRole(features[i].Description, hints)
		}
		if features[i].SkillName == "" {
			features[i].SkillName = RoleToSkillName(features[i].Role)
		}
		rd := cfg.GetRoleDefault(features[i].Role)
		if features[i].Model == "" {
			features[i].Model = rd.Model
		}
		if features[i].Agent == "" {
			features[i].Agent = rd.Agent
		}
		if len(features[i].Fallbacks) == 0 && len(rd.Fallbacks) > 0 {
			features[i].Fallbacks = append([]string{}, rd.Fallbacks...)
		}
	}
}

// extractHints parses the goal and clarifications to shape plan generation.
func extractHints(goal string, clarifications []QAPair) generationHint {
	hints := generationHint{
		Scope: "small",
	}

	// Detect scope from goal verbs and length
	wordCount := len(strings.Fields(goal))
	switch {
	case wordCount > 20:
		hints.Scope = "large"
	case wordCount > 8:
		hints.Scope = "medium"
	}

	// Extract technology hints from clarifications
	for _, qa := range clarifications {
		answer := strings.ToLower(qa.Answer)
		techs := []string{"react", "angular", "vue", "go", "golang", "python",
			"typescript", "javascript", "node", "rust", "postgres", "sql",
			"docker", "kubernetes", "aws", "frontend", "backend", "fullstack"}
		for _, t := range techs {
			if strings.Contains(answer, t) {
				hints.Technologies = append(hints.Technologies, t)
			}
		}

		// Scope hints from answer
		if strings.Contains(answer, "small") || strings.Contains(answer, "simple") {
			hints.Scope = "small"
		} else if strings.Contains(answer, "large") || strings.Contains(answer, "complex") {
			hints.Scope = "large"
		}
	}

	return hints
}

// extractName produces a short mission name from the goal.
func extractName(goal string) string {
	goal = strings.TrimSpace(goal)
	if len(goal) <= 50 {
		return goal
	}
	// Try to break at sentence or comma boundary
	if idx := strings.IndexAny(goal, ".,;"); idx > 0 && idx <= 50 {
		return strings.TrimSpace(goal[:idx])
	}
	return strings.TrimSpace(goal[:50])
}
