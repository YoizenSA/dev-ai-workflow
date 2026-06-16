package missions

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// ─── Errors ────────────────────────────────────────────────────────────────

var (
	ErrEmptyGoal       = errors.New("goal cannot be empty")
	ErrPlanRejected    = errors.New("plan rejected by user")
	ErrInvalidPlanFile = errors.New("invalid plan file")
	ErrPlanNotFound    = errors.New("plan file not found")
	ErrPlanParseError  = errors.New("plan parse error")
)

// ─── Planning Types ────────────────────────────────────────────────────────

// QAPair represents a question and answer collected during interactive planning.
type QAPair struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

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
		return p
	}

	opencodePath, err := DetectOpencode()
	if err != nil {
		log.Printf("opencode not available, falling back to local planning: %v", err)
		return applyModelAgent(GeneratePlan(goal, clarifications))
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
		return applyModelAgent(GeneratePlan(goal, clarifications))
	}

	// Try to parse the output as JSON plan
	plan, parseErr := parsePlanFromOutput(string(output))
	if parseErr != nil {
		log.Printf("parse opencode output: %v, falling back to local planning", parseErr)
		return applyModelAgent(GeneratePlan(goal, clarifications))
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
	cmd.Stderr = os.Stderr

	output, err := cmd.Output()
	if err != nil {
		log.Printf("opencode goal refinement failed: %v, using local refinement", err)
		return localRefineGoal(goal)
	}

	refined := strings.TrimSpace(string(output))
	if refined == "" {
		log.Printf("opencode goal refinement returned empty output, using local refinement")
		return localRefineGoal(goal)
	}
	return refined
}

// planningAgentDefault returns the agent configured for the planning role,
// falling back to "orchestrator" when config is unavailable or unset. This
// keeps goal refinement on the planning agent rather than opencode's default.
func planningAgentDefault() string {
	if cfg, _ := config.LoadConfig(); cfg != nil {
		if a := cfg.GetRoleDefault(config.RolePlanning).Agent; a != "" {
			return a
		}
	}
	return "orchestrator"
}

// buildRefinePrompt constructs the prompt sent to opencode to refine a goal.
func buildRefinePrompt(goal, extraContext string) string {
	prompt := fmt.Sprintf(`Given this goal: %s

Refine it into a structured mission goal. Return markdown with these sections:

## Goal
[Clear, actionable goal statement]

## Scope
- [What is included]

## Out of Scope
- [What is explicitly excluded]

## Acceptance Criteria
- [Measurable criteria for completion]

Keep it concise and practical. Do not include any preamble or explanation — output ONLY the markdown.`, goal)

	if extraContext != "" {
		prompt = fmt.Sprintf("Additional context: %s\n\n%s", extraContext, prompt)
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

// buildPlanPrompt creates the prompt for opencode to generate a plan.
func buildPlanPrompt(goal string, clarifications []QAPair, project string) string {
	return buildPlanPromptWithRepo(goal, clarifications, project, "")
}

// buildPlanPromptWithRepo is the repo-aware variant. When repoPath is non-empty,
// the prompt instructs opencode to read the actual codebase and ground the plan
// in real patterns/conventions (Droid-aligned one-shot investigation).
func buildPlanPromptWithRepo(goal string, clarifications []QAPair, project, repoPath string) string {
	var sb strings.Builder
	sb.WriteString("You are a technical architect following Factory.ai mission planning methodology. Generate a development plan for the following goal.\n\n")
	sb.WriteString("## Planning Phases (Factory.ai Methodology)\n")
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

IMPORTANT RULES (Factory.ai Alignment):
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

// detectSkill picks a plausible worker skill name based on description.
// Kept as a thin wrapper over detectRole + RoleToSkillName for backward
// compatibility with existing callers.
func detectSkill(desc string, hints generationHint) string {
	return RoleToSkillName(detectRole(desc, hints))
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

// ─── Plan Validation ───────────────────────────────────────────────────────

// ValidatePlan checks that a PlanMission has the required fields.
func ValidatePlan(plan *PlanMission) error {
	if plan == nil {
		return fmt.Errorf("%w: plan is nil", ErrInvalidPlan)
	}
	if strings.TrimSpace(plan.Name) == "" {
		return fmt.Errorf("%w: plan name is required", ErrInvalidPlan)
	}
	if strings.TrimSpace(plan.Description) == "" {
		return fmt.Errorf("%w: plan description is required", ErrInvalidPlan)
	}
	if len(plan.Milestones) == 0 {
		return fmt.Errorf("%w: at least one milestone is required", ErrInvalidPlan)
	}
	for i, ms := range plan.Milestones {
		if strings.TrimSpace(ms.Name) == "" {
			return fmt.Errorf("%w: milestone %d has empty name", ErrInvalidPlan, i)
		}
	}
	for i, f := range plan.Features {
		if strings.TrimSpace(f.ID) == "" {
			return fmt.Errorf("%w: feature %d has empty id", ErrInvalidPlan, i)
		}
		if strings.TrimSpace(f.Description) == "" {
			return fmt.Errorf("%w: feature %d has empty description", ErrInvalidPlan, i)
		}
		if strings.TrimSpace(f.SkillName) == "" {
			return fmt.Errorf("%w: feature %d has empty skillName", ErrInvalidPlan, i)
		}
		if strings.TrimSpace(f.Milestone) == "" {
			return fmt.Errorf("%w: feature %d has empty milestone", ErrInvalidPlan, i)
		}
		// Verify feature's milestone exists
		found := false
		for _, ms := range plan.Milestones {
			if f.Milestone == ms.Name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: feature %d references unknown milestone %q", ErrInvalidPlan, i, f.Milestone)
		}
	}
	return nil
}

// ─── Interactive Planning ──────────────────────────────────────────────────

// RunInteractivePlanning runs the interactive planning dialog on the given
// reader/writer (e.g. os.Stdin / os.Stdout) and returns the approved mission
// or an error. An approved mission transitions from "planning" to "active".
//
// The dialog loop:
//
//  1. Prompt for goal
//  2. Validate goal (not empty)
//  3. Generate plan
//  4. Show plan summary
//  5. Ask for approval (y/n)
//  6. On "y": save mission, approve (→ active), return mission
//  7. On "n": ask for feedback, regenerate, go to 4
func RunInteractivePlanning(store *MissionsStore, r io.Reader, w io.Writer, project string) (*Mission, error) {
	scanner := bufio.NewScanner(r)

	// Write welcome banner
	fmt.Fprintf(w, "╔══════════════════════════════════════════════╗\n")
	fmt.Fprintf(w, "║        ywai Missions — Planning Phase       ║\n")
	fmt.Fprintf(w, "╚══════════════════════════════════════════════╝\n\n")

	// Step 1: Collect goal
	goal, err := promptGoal(scanner, w)
	if err != nil {
		return nil, err
	}

	// Step 2: Optional clarifying questions (simplified for MVP)
	var clarifications []QAPair
	clarifications = askClarifyingQuestions(scanner, w)

	// Step 2.5: Optional project name (used if not passed from CLI)
	if project == "" {
		project = promptProject(scanner, w)
	}

	// Step 3-7: Plan → review → approve/reject loop
	plan := GeneratePlanWithOpencode(goal, clarifications, project, "", "")
	if plan == nil {
		return nil, ErrEmptyGoal
	}

	// Create mission in planning state
	mission, err := CreateMissionFromPlan(store, plan)
	if err != nil {
		return nil, fmt.Errorf("create mission from plan: %w", err)
	}

	for {
		// Show plan
		showPlan(w, plan)

		// Ask for approval
		approved, err := promptApproval(scanner, w)
		if err != nil {
			// Clean up mission on SIGINT or read error
			_ = store.DeleteMission(mission.ID)
			return nil, err
		}

		if approved {
			// Approve plan → transition to active
			if err := ApprovePlan(store, mission); err != nil {
				return nil, fmt.Errorf("approve plan: %w", err)
			}
			fmt.Fprintf(w, "\n✓ Mission %q approved and active!\n", mission.Name)
			return mission, nil
		}

		// Rejected: collect feedback and regenerate
		feedback, err := promptFeedback(scanner, w)
		if err != nil {
			_ = store.DeleteMission(mission.ID)
			return nil, err
		}

		// Add feedback as a clarification
		if feedback != "" {
			clarifications = append(clarifications, QAPair{
				Question: "User feedback",
				Answer:   feedback,
			})
		}

		// Regenerate plan with feedback
		newPlan := regeneratePlan(plan, feedback, clarifications)
		plan = newPlan

		// Update mission with new plan
		mission, err = updateMissionFromPlan(store, mission, plan)
		if err != nil {
			return nil, fmt.Errorf("update mission with regenerated plan: %w", err)
		}
	}
}

// RunInteractivePlanningWithClient is the opencode-client-aware variant of
// RunInteractivePlanning. When the client supports the Sessions API (opencode
// server reachable), it drives an iterative Droid-style flow:
//
//  1. Investigate the codebase (PlannerSession.Investigate)
//  2. Surface unknowns to the user, replacing the hardcoded clarifying questions
//  3. Propose architecture (PlannerSession.ProposeArchitecture)
//  4. Generate features (PlannerSession.GenerateFeatures)
//  5. Confirm milestones via the existing showPlan/promptApproval loop
//
// When the client is nil or doesn't support sessions, it falls back to the
// original one-shot GeneratePlanWithOpencode path (delegates to
// RunInteractivePlanning), preserving existing behaviour.
func RunInteractivePlanningWithClient(store *MissionsStore, r io.Reader, w io.Writer, project string, client opencode.Client, repoPath string) (*Mission, error) {
	// Fallback path: no client or no sessions support → original one-shot flow.
	if client == nil {
		return RunInteractivePlanning(store, r, w, project)
	}
	ps := NewPlannerSession(client, "", "")
	if !ps.CanUseSessions() {
		return RunInteractivePlanning(store, r, w, project)
	}
	defer ps.Close()

	scanner := bufio.NewScanner(r)
	fmt.Fprintf(w, "╔══════════════════════════════════════════════╗\n")
	fmt.Fprintf(w, "║   ywai Missions — Iterative Planning (Droid) ║\n")
	fmt.Fprintf(w, "╚══════════════════════════════════════════════╝\n\n")

	goal, err := promptGoal(scanner, w)
	if err != nil {
		return nil, err
	}
	if project == "" {
		project = promptProject(scanner, w)
	}

	// Stage 1: investigate the codebase.
	fmt.Fprintf(w, "\n🔍 Investigating codebase at %s...\n", repoPath)
	unknowns, invErr := ps.Investigate(context.Background(), goal, repoPath)
	if invErr != nil {
		fmt.Fprintf(w, "⚠ Investigation failed (%v) — falling back to one-shot planning.\n", invErr)
		return RunInteractivePlanning(store, r, w, project)
	}
	if strings.TrimSpace(unknowns) != "" {
		fmt.Fprintf(w, "\n%s\n", unknowns)
	}

	// Surface unknowns as clarifying questions so the user can answer them.
	clarifications := askClarifyingQuestions(scanner, w)

	// Stage 2: propose architecture.
	fmt.Fprintf(w, "\n🏗️  Proposing architecture...\n")
	arch, archErr := ps.ProposeArchitecture(context.Background())
	if archErr != nil {
		fmt.Fprintf(w, "⚠ Architecture proposal failed (%v) — falling back to one-shot planning.\n", archErr)
		return RunInteractivePlanning(store, r, w, project)
	}
	if strings.TrimSpace(arch) != "" {
		fmt.Fprintf(w, "\n%s\n", arch)
		if approved, _ := promptApproval(scanner, w); !approved {
			fmt.Fprintf(w, "\nArchitecture rejected. Falling back to one-shot planning.\n")
			return RunInteractivePlanning(store, r, w, project)
		}
	}

	// Stage 3: generate features from confirmed milestones.
	// We don't have explicit milestone confirmation separate from the plan
	// approval loop, so we pass the milestones the planner proposes.
	plan, genErr := ps.GenerateFeatures(context.Background(), nil)
	if genErr != nil || plan == nil {
		fmt.Fprintf(w, "⚠ Feature generation failed (%v) — falling back to one-shot planning.\n", genErr)
		return RunInteractivePlanning(store, r, w, project)
	}

	// Apply role defaults (model/agent/fallbacks) so workers resolve correctly.
	cfg, _ := config.LoadConfig()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	hints := extractHints(goal, clarifications)
	applyRoleDefaults(plan.Features, hints, cfg)
	if project != "" {
		plan.Project = project
	}

	mission, err := CreateMissionFromPlan(store, plan)
	if err != nil {
		return nil, fmt.Errorf("create mission from plan: %w", err)
	}

	// Plan review/approval loop (reuses existing show/prompt/approve machinery).
	for {
		showPlan(w, plan)
		approved, err := promptApproval(scanner, w)
		if err != nil {
			_ = store.DeleteMission(mission.ID)
			return nil, err
		}
		if approved {
			if err := ApprovePlan(store, mission); err != nil {
				return nil, fmt.Errorf("approve plan: %w", err)
			}
			fmt.Fprintf(w, "\n✓ Mission %q approved and active!\n", mission.Name)
			return mission, nil
		}
		// Rejected: collect feedback and regenerate via a follow-up on the same session.
		feedback, err := promptFeedback(scanner, w)
		if err != nil {
			_ = store.DeleteMission(mission.ID)
			return nil, err
		}
		if feedback != "" {
			clarifications = append(clarifications, QAPair{Question: "User feedback", Answer: feedback})
		}
		newPlan, regenErr := ps.GenerateFeatures(context.Background(), milestoneNames(plan))
		if regenErr != nil || newPlan == nil {
			// Regeneration via session failed; fall back to local string manipulation.
			newPlan = regeneratePlan(plan, feedback, clarifications)
		} else {
			applyRoleDefaults(newPlan.Features, hints, cfg)
			if project != "" {
				newPlan.Project = project
			}
		}
		plan = newPlan
		mission, err = updateMissionFromPlan(store, mission, plan)
		if err != nil {
			return nil, fmt.Errorf("update mission with regenerated plan: %w", err)
		}
	}
}

// milestoneNames returns the list of milestone names from a plan.
func milestoneNames(plan *PlanMission) []string {
	if plan == nil {
		return nil
	}
	out := make([]string, 0, len(plan.Milestones))
	for _, m := range plan.Milestones {
		out = append(out, m.Name)
	}
	return out
}
func promptGoal(scanner *bufio.Scanner, w io.Writer) (string, error) {
	for {
		fmt.Fprintf(w, "\nWhat would you like to build?\n")
		fmt.Fprintf(w, "Describe your goal (or Ctrl+C to cancel):\n\n> ")

		if !scanner.Scan() {
			// EOF or read error (includes Ctrl+D)
			if err := scanner.Err(); err != nil {
				return "", fmt.Errorf("read goal: %w", err)
			}
			return "", io.EOF
		}

		goal := strings.TrimSpace(scanner.Text())
		if goal == "" {
			fmt.Fprintf(w, "\n! Goal cannot be empty. Please describe what you want to build.\n")
			continue
		}
		return goal, nil
	}
}

// promptProject asks for an optional project name.
func promptProject(scanner *bufio.Scanner, w io.Writer) string {
	fmt.Fprintf(w, "\nProject (optional, press Enter to skip):\n> ")
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(scanner.Text())
}

// askClarifyingQuestions asks optional clarifying questions.
func askClarifyingQuestions(scanner *bufio.Scanner, w io.Writer) []QAPair {
	var qas []QAPair

	fmt.Fprintf(w, "\n--- Optional: Clarifying Questions ---\n")

	questions := []string{
		"What technologies or stack will you use? (optional, press Enter to skip)",
		"Any specific constraints or requirements? (optional, press Enter to skip)",
	}

	for _, q := range questions {
		fmt.Fprintf(w, "\n%s\n> ", q)
		if !scanner.Scan() {
			return qas
		}
		answer := strings.TrimSpace(scanner.Text())
		if answer != "" {
			qas = append(qas, QAPair{Question: q, Answer: answer})
		}
	}

	return qas
}

// showPlan displays the plan to the user.
func showPlan(w io.Writer, plan *PlanMission) {
	fmt.Fprintf(w, "\n═══════════════════ PLAN ═══════════════════\n")
	fmt.Fprintf(w, "Name:        %s\n", plan.Name)
	fmt.Fprintf(w, "Description: %s\n", plan.Description)
	fmt.Fprintf(w, "\nMilestones (%d):\n", len(plan.Milestones))
	for _, ms := range plan.Milestones {
		fmt.Fprintf(w, "  • %s — %s\n", ms.Name, ms.Description)
	}
	fmt.Fprintf(w, "\nFeatures (%d):\n", len(plan.Features))
	for _, f := range plan.Features {
		fmt.Fprintf(w, "  • [%s] (%s) %s\n", f.ID, f.SkillName, f.Description)
	}
	fmt.Fprintf(w, "══════════════════════════════════════════\n")
}

// promptApproval asks the user to approve or reject the plan.
func promptApproval(scanner *bufio.Scanner, w io.Writer) (bool, error) {
	for {
		fmt.Fprintf(w, "\nApprove this plan? (y/n): ")
		if !scanner.Scan() {
			// EOF or read error (e.g. Ctrl+D which signals EOF)
			return false, io.EOF
		}
		input := strings.TrimSpace(strings.ToLower(scanner.Text()))
		switch input {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintf(w, "Please answer 'y' or 'n'.\n")
		}
	}
}

// promptFeedback asks for feedback when the plan is rejected.
func promptFeedback(scanner *bufio.Scanner, w io.Writer) (string, error) {
	fmt.Fprintf(w, "\nWhat would you like to change?\n")
	fmt.Fprintf(w, "Describe your feedback (or press Enter for minor tweaks):\n> ")

	if !scanner.Scan() {
		return "", io.EOF
	}
	return strings.TrimSpace(scanner.Text()), nil
}

// regeneratePlan creates a new plan incorporating feedback from the rejection.
func regeneratePlan(prevPlan *PlanMission, feedback string, clarifications []QAPair) *PlanMission {
	newPlan := &PlanMission{
		Name:        prevPlan.Name,
		Description: prevPlan.Description,
	}

	// Incorporate feedback into the plan
	if feedback != "" {
		lower := strings.ToLower(feedback)

		// Check for scope modifications
		if strings.Contains(lower, "more") || strings.Contains(lower, "add") || strings.Contains(lower, "additional") {
			// Add a feature or extend existing ones
			newPlan.Milestones = append([]PlanMilestone{}, prevPlan.Milestones...)

			// Add an extra feature to the last milestone
			if len(prevPlan.Features) > 0 {
				lastMS := prevPlan.Milestones[len(prevPlan.Milestones)-1]
				hints := extractHints(feedback, clarifications)
				role := detectRole(feedback, hints)
				newFeat := PlanFeature{
					ID:          fmt.Sprintf("feat-%s-%d", lastMS.Name, len(prevPlan.Features)+1),
					Description: feedback,
					SkillName:   RoleToSkillName(role),
					Role:        role,
					Milestone:   lastMS.Name,
					Expected:    []string{fmt.Sprintf("%s is implemented", feedback)},
				}
				newPlan.Features = append([]PlanFeature{}, prevPlan.Features...)
				newPlan.Features = append(newPlan.Features, newFeat)
				cfg, _ := config.LoadConfig()
				if cfg == nil {
					cfg = config.DefaultConfig()
				}
				applyRoleDefaults(newPlan.Features, hints, cfg)
				return newPlan
			}
		}

		if strings.Contains(lower, "fewer") || strings.Contains(lower, "less") || strings.Contains(lower, "simpl") || strings.Contains(lower, "remove") {
			// Remove last feature or simplify
			if len(prevPlan.Features) > 1 {
				newPlan.Milestones = append([]PlanMilestone{}, prevPlan.Milestones...)
				newPlan.Features = append([]PlanFeature{}, prevPlan.Features[:len(prevPlan.Features)-1]...)
				return newPlan
			}
		}

		// Default: modify the last feature's description to incorporate feedback
		if len(prevPlan.Features) > 0 {
			newPlan.Milestones = append([]PlanMilestone{}, prevPlan.Milestones...)
			newPlan.Features = make([]PlanFeature, len(prevPlan.Features))
			copy(newPlan.Features, prevPlan.Features)
			lastIdx := len(newPlan.Features) - 1
			newPlan.Features[lastIdx].Description = fmt.Sprintf("%s (updated: %s)",
				newPlan.Features[lastIdx].Description, feedback)
			return newPlan
		}
	}

	// No meaningful feedback: return a copy of the previous plan
	newPlan.Milestones = append([]PlanMilestone{}, prevPlan.Milestones...)
	newPlan.Features = make([]PlanFeature, len(prevPlan.Features))
	copy(newPlan.Features, prevPlan.Features)

	return newPlan
}

// ─── Mission Lifecycle ─────────────────────────────────────────────────────

// CreateMissionFromPlan creates a new Mission from a PlanMission in "planning"
// state and persists it to the store. Also creates Factory.ai mission artifacts.
func CreateMissionFromPlan(store *MissionsStore, plan *PlanMission) (*Mission, error) {
	if err := ValidatePlan(plan); err != nil {
		return nil, err
	}

	now := time.Now().Round(time.Millisecond)
	missionID := generateMissionID()

	milestones := make([]Milestone, len(plan.Milestones))
	for i, pm := range plan.Milestones {
		milestones[i] = Milestone{
			Name:        pm.Name,
			Description: pm.Description,
		}
	}

	features := make([]Feature, len(plan.Features))
	for i, pf := range plan.Features {
		features[i] = Feature{
			ID:               pf.ID,
			Description:      pf.Description,
			Status:           FeaturePending,
			SkillName:        pf.SkillName,
			Milestone:        pf.Milestone,
			Preconditions:    copyStrings(pf.Preconditions),
			ExpectedBehavior: copyStrings(pf.Expected),
			Fulfills:         copyStrings(pf.Fulfills),
			Role:             pf.Role,
			Model:            pf.Model,
			Agent:            pf.Agent,
			Fallbacks:        copyStrings(pf.Fallbacks),
			CreatedAt:        now,
			UpdatedAt:        now,
		}
	}

	mission := &Mission{
		ID:             missionID,
		Name:           plan.Name,
		Project:        plan.Project,
		Status:         MissionPlanning,
		CreatedAt:      now,
		UpdatedAt:      now,
		Features:       features,
		Milestones:     milestones,
		Model:          plan.Model,
		Agent:          plan.Agent,
		ExecutionAgent: plan.Agent,
	}

	if err := store.CreateMission(mission); err != nil {
		return nil, fmt.Errorf("save mission: %w", err)
	}

	// Create Factory.ai mission artifacts
	missionDir := store.MissionDir(missionID)
	artifactCreator := NewArtifactCreator(missionDir, store)
	if err := artifactCreator.CreateAllArtifacts(mission); err != nil {
		// Log error but don't fail mission creation - artifacts can be created later
		log.Printf("Warning: failed to create mission artifacts: %v", err)
	}

	// Design worker system based on feature classification
	if err := DesignWorkerSystem(plan, mission, missionDir, plan.Model, plan.Agent); err != nil {
		// Log but don't fail — worker skills can be generated later
		log.Printf("Warning: failed to design worker system: %v", err)
	}

	return mission, nil
}

// workerTypeForFeature classifies a feature into a canonical worker skill
// name by keyword matching. Returns a name that GetDefaultSkill recognizes
// (backend-worker, frontend-worker, qa-worker, devops-worker, implementation).
func workerTypeForFeature(feature PlanFeature) string {
	lower := strings.ToLower(feature.Description) + " " + strings.ToLower(feature.ID)
	// Also check Expected behavior text
	for _, e := range feature.Expected {
		lower += " " + strings.ToLower(e)
	}

	switch {
	case strings.Contains(lower, "architecture") || strings.Contains(lower, "design pattern") || strings.Contains(lower, "system design") || strings.Contains(lower, "design the"):
		return "architect-worker"
	case strings.Contains(lower, "api") || strings.Contains(lower, "handler") || strings.Contains(lower, "endpoint") || strings.Contains(lower, "route"):
		return "backend-worker"
	case strings.Contains(lower, "component") || strings.Contains(lower, "ui") || strings.Contains(lower, "css") || strings.Contains(lower, "style") || strings.Contains(lower, "frontend"):
		return "frontend-worker"
	case strings.Contains(lower, "migration") || strings.Contains(lower, "schema") || strings.Contains(lower, "query") || strings.Contains(lower, "sql") || strings.Contains(lower, "database"):
		return "backend-worker"
	case strings.Contains(lower, "test") || strings.Contains(lower, "spec") || strings.Contains(lower, "coverage"):
		return "qa-worker"
	case strings.Contains(lower, "docker") || strings.Contains(lower, "ci") || strings.Contains(lower, "deploy") || strings.Contains(lower, "infra") || strings.Contains(lower, "kubernetes"):
		return "devops-worker"
	default:
		return "implementation"
	}
}

// DesignWorkerSystem classifies features into worker types, writes Droid-format
// SKILL.md files (Required Skills/Tools, Work Procedure, Example Handoff, When
// to Return), and updates mission.WorkerTypes and mission.Features[].SkillName.
//
// Each generated skill reuses GetDefaultSkill's content (which already matches
// the Droid SKILL.md structure) so workers get a realistic quality bar instead
// of a generic 5-step checklist.
func DesignWorkerSystem(plan *PlanMission, mission *Mission, missionDir, model, agent string) error {
	// Classify features into worker skill names (canonical).
	typeFeatures := make(map[string][]PlanFeature)
	for _, feat := range plan.Features {
		wt := workerTypeForFeature(feat)
		// Honor an explicit role-derived SkillName from the plan when present
		// (applyRoleDefaults already set canonical names); only classify when blank.
		if feat.SkillName != "" {
			wt = feat.SkillName
		}
		typeFeatures[wt] = append(typeFeatures[wt], feat)
	}

	var workerTypes []WorkerType
	for wtName, features := range typeFeatures {
		// Resolve the skill content: prefer GetDefaultSkill (canonical format),
		// fall back to a generated generic skill.
		var skill *Skill
		if def, err := GetDefaultSkill(wtName); err == nil {
			skill = def
		} else {
			skill = genericSkillForWorkerType(wtName)
		}

		// Enrich Required Skills/Tools with role-configured skills so the
		// generated SKILL.md advertises the full kit the worker should load.
		enrichSkillWithRoleSkills(skill, features)

		// Render the SKILL.md body in Droid format.
		skillContent := formatSkillBody(skill)

		// Write skill file.
		skillDir := filepath.Join(missionDir, "skills", wtName)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return fmt.Errorf("create skill dir %s: %w", skillDir, err)
		}
		skillPath := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
			return fmt.Errorf("write skill %s: %w", skillPath, err)
		}

		workerTypes = append(workerTypes, WorkerType{
			Name:        wtName,
			Description: skill.Description,
			SkillPath:   skillPath,
		})

		// Set SkillName for each mission feature mapped to this worker type.
		for _, feat := range features {
			for j := range mission.Features {
				if mission.Features[j].ID == feat.ID {
					mission.Features[j].SkillName = wtName
					break
				}
			}
		}
	}

	mission.WorkerTypes = workerTypes
	log.Printf("Designed %d worker types for %d features", len(workerTypes), len(plan.Features))
	return nil
}

// genericSkillForWorkerType builds a minimal Droid-format skill for a worker
// type that GetDefaultSkill doesn't know. Used as a last-resort fallback.
func genericSkillForWorkerType(wtName string) *Skill {
	return &Skill{
		Name:          wtName,
		Description:   "Implementation worker for " + wtName,
		RequiredTools: []string{"git"},
		WorkProcedure: "1. Read the feature description and expected behavior\n2. Write failing tests first (TDD)\n3. Implement the feature to make tests pass\n4. Run tests and verify they pass\n5. Manually verify the implementation\n6. Return a structured handoff",
		ExampleHandoff: `{
  "salientSummary": "Implemented the feature as described",
  "whatWasImplemented": "Feature implementation completed",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": [
      {"command": "go test ./...", "exitCode": 0, "observation": "All tests passed"}
    ]
  },
  "tests": {
    "added": [],
    "coverage": "N/A"
  },
  "discoveredIssues": []
}`,
		ReturnConditions: "Return to orchestrator if: requirements are ambiguous, existing bugs affect this feature, or you cannot complete within mission boundaries",
	}
}

// enrichSkillWithRoleSkills merges role-configured skills (RoleDefault.Skills)
// from the user config into a skill's RequiredSkills list so the generated
// SKILL.md advertises the full skill kit. Dedupes against existing entries.
func enrichSkillWithRoleSkills(skill *Skill, features []PlanFeature) {
	cfg, _ := config.LoadConfig()
	if cfg == nil || len(features) == 0 {
		return
	}
	// Collect roles referenced by these features.
	roles := map[string]bool{}
	for _, f := range features {
		if f.Role != "" {
			roles[f.Role] = true
		}
	}
	existing := map[string]bool{}
	for _, s := range skill.RequiredSkills {
		existing[s] = true
	}
	for role := range roles {
		for _, s := range cfg.GetRoleDefault(role).Skills {
			if s != "" && !existing[s] {
				existing[s] = true
				skill.RequiredSkills = append(skill.RequiredSkills, s)
			}
		}
	}
}

// workerTypeDescription returns a human-readable description for a worker type key.
func workerTypeDescription(wt string) string {
	switch wt {
	case "api":
		return "API and Backend Developer"
	case "ui":
		return "UI and Frontend Developer"
	case "db":
		return "Database and Data Layer Developer"
	case "tests":
		return "Test Engineer"
	case "infra":
		return "Infrastructure and DevOps Engineer"
	default:
		return "Implementation Developer"
	}
}

// updateMissionFromPlan updates an existing mission with a new plan,
// replacing milestones and features while preserving the mission ID and status.
func updateMissionFromPlan(store *MissionsStore, mission *Mission, plan *PlanMission) (*Mission, error) {
	if err := ValidatePlan(plan); err != nil {
		return nil, err
	}

	now := time.Now().Round(time.Millisecond)

	// Update mission fields
	mission.Name = plan.Name
	mission.Project = plan.Project
	mission.UpdatedAt = now

	// Rebuild milestones
	mission.Milestones = make([]Milestone, len(plan.Milestones))
	for i, pm := range plan.Milestones {
		mission.Milestones[i] = Milestone{
			Name:        pm.Name,
			Description: pm.Description,
		}
	}

	// Rebuild features
	mission.Features = make([]Feature, len(plan.Features))
	for i, pf := range plan.Features {
		mission.Features[i] = Feature{
			ID:               pf.ID,
			Description:      pf.Description,
			Status:           FeaturePending,
			SkillName:        pf.SkillName,
			Milestone:        pf.Milestone,
			Preconditions:    copyStrings(pf.Preconditions),
			ExpectedBehavior: copyStrings(pf.Expected),
			Fulfills:         copyStrings(pf.Fulfills),
			CreatedAt:        now,
			UpdatedAt:        now,
		}
	}

	if err := store.SaveMission(mission); err != nil {
		return nil, fmt.Errorf("save updated mission: %w", err)
	}

	return mission, nil
}

// ApprovePlan transitions a mission from "planning" to "active" state.
func ApprovePlan(store *MissionsStore, mission *Mission) error {
	newStatus, err := TransitionMissionStatus(mission.Status, MissionActive)
	if err != nil {
		return fmt.Errorf("transition mission to active: %w", err)
	}

	mission.Status = newStatus
	mission.UpdatedAt = time.Now().Round(time.Millisecond)

	return store.SaveMission(mission)
}

// ─── File-based Planning ───────────────────────────────────────────────────

// PlanFromFile reads a plan from a JSON file, validates it, creates a mission
// in "planning" state, approves it (transitions to "active"), and returns the
// mission.
func PlanFromFile(store *MissionsStore, filePath string) (*Mission, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrPlanNotFound, filePath)
		}
		return nil, fmt.Errorf("%w: read %s: %v", ErrInvalidPlanFile, filePath, err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty file: %s", ErrInvalidPlanFile, filePath)
	}

	var plan PlanMission
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("%w: parse %s: %v", ErrPlanParseError, filePath, err)
	}

	if err := ValidatePlan(&plan); err != nil {
		return nil, err
	}

	mission, err := CreateMissionFromPlan(store, &plan)
	if err != nil {
		return nil, err
	}

	if err := ApprovePlan(store, mission); err != nil {
		// Clean up on approval failure
		_ = store.DeleteMission(mission.ID)
		return nil, err
	}

	return mission, nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// AutoPlanOpts configures the automatic plan-and-approve flow (FASE 2).
type AutoPlanOpts struct {
	Project     string
	Model       string
	Agent       string
	BaseRef     string
	AutoApprove bool
	// RepoPath is the resolved filesystem path of the project repo. When set,
	// the planner prompt tells opencode to read the repo and ground the plan in
	// the real codebase (Droid-aligned investigation in one-shot for auto mode).
	RepoPath string
}

// PlanAndApprove generates a plan from a goal, creates the mission, and
// optionally approves it — all without interactive prompts.
//
// When AutoPlanOpts.RepoPath is set (or can be resolved from the project name
// via a RepoResolver), the planner prompt is enriched to read the real codebase,
// giving Droid-style grounding in one shot (auto mode skips the interactive
// milestone confirmation the interactive path does).
func PlanAndApprove(store *MissionsStore, goal string, opts AutoPlanOpts) (*Mission, error) {
	repoPath := opts.RepoPath
	plan := GeneratePlanWithRepo(goal, nil, opts.Project, opts.Model, opts.Agent, repoPath)
	if plan == nil {
		return nil, fmt.Errorf("plan generation returned nil")
	}

	mission, err := CreateMissionFromPlan(store, plan)
	if err != nil {
		return nil, fmt.Errorf("create mission from plan: %w", err)
	}

	if opts.AutoApprove {
		if err := ApprovePlan(store, mission); err != nil {
			_ = store.DeleteMission(mission.ID)
			return nil, fmt.Errorf("approve plan: %w", err)
		}
	}

	return mission, nil
}

// generateMissionID creates a short, unique mission ID using crypto/rand.
func generateMissionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("mission-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
