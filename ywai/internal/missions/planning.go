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
	"regexp"
	"strings"
	"time"
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
func GeneratePlanWithOpencode(goal string, clarifications []QAPair, project, model, agent string) *PlanMission {
	opencodePath, err := DetectOpencode()
	if err != nil {
		log.Printf("opencode not available, falling back to local planning: %v", err)
		return GeneratePlan(goal, clarifications)
	}

	// Build the prompt for opencode
	prompt := buildPlanPrompt(goal, clarifications, project)

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
		return GeneratePlan(goal, clarifications)
	}

	// Try to parse the output as JSON plan
	plan, parseErr := parsePlanFromOutput(string(output))
	if parseErr != nil {
		log.Printf("parse opencode output: %v, falling back to local planning", parseErr)
		return GeneratePlan(goal, clarifications)
	}

	// Set project if provided
	if project != "" {
		plan.Project = project
	}

	return plan
}

// buildPlanPrompt creates the prompt for opencode to generate a plan.
func buildPlanPrompt(goal string, clarifications []QAPair, project string) string {
	var sb strings.Builder
	sb.WriteString("You are a technical architect. Generate a development plan for the following goal.\n\n")
	sb.WriteString("## Goal\n")
	sb.WriteString(goal)
	sb.WriteString("\n\n")

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
      "skillName": "implementation",
      "milestone": "milestone-name",
      "preconditions": [],
      "expectedBehavior": ["assertion 1"],
      "fulfills": ["requirement reference"]
    }
  ]
}

IMPORTANT RULES:
- Each feature must have a unique id (feat-1, feat-2, etc.)
- Each feature must reference a milestone that exists
- preconditions is a list of feature IDs that must be done first
- Valid skillName values: implementation, qa, devops, documentation, architecture. Default to 'implementation' for coding tasks.
- expectedBehavior is a list of verifiable assertions
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

	// Set default skillName for features missing it
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

			// Pick a plausible skill name based on description keywords
			skill := detectSkill(desc, hints)

			feat := PlanFeature{
				ID:          featID,
				Description: desc,
				SkillName:   skill,
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

// detectSkill picks a plausible worker skill name based on description.
func detectSkill(desc string, hints generationHint) string {
	lower := strings.ToLower(desc)

	// Check tech hints first
	for _, tech := range hints.Technologies {
		t := strings.ToLower(tech)
		switch {
		case strings.Contains(t, "react") || strings.Contains(t, "frontend"):
			return "frontend-worker"
		case strings.Contains(t, "go") || strings.Contains(t, "golang") || strings.Contains(t, "backend"):
			return "backend-worker"
		case strings.Contains(t, "test") || strings.Contains(t, "qa"):
			return "qa-worker"
		case strings.Contains(t, "infra") || strings.Contains(t, "devops"):
			return "devops-worker"
		}
	}

	// Guess from description keywords
	switch {
	case strings.Contains(lower, "test") || strings.Contains(lower, "qa"):
		return "qa-worker"
	case strings.Contains(lower, "ui") || strings.Contains(lower, "frontend") || strings.Contains(lower, "web"):
		return "frontend-worker"
	case strings.Contains(lower, "api") || strings.Contains(lower, "backend") || strings.Contains(lower, "server"):
		return "backend-worker"
	case strings.Contains(lower, "infra") || strings.Contains(lower, "deploy") || strings.Contains(lower, "ci"):
		return "devops-worker"
	default:
		return "backend-worker"
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
//	1. Prompt for goal
//	2. Validate goal (not empty)
//	3. Generate plan
//	4. Show plan summary
//	5. Ask for approval (y/n)
//	6. On "y": save mission, approve (→ active), return mission
//	7. On "n": ask for feedback, regenerate, go to 4
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

// promptGoal prompts the user for a mission goal, re-prompting on empty input.
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
				newFeat := PlanFeature{
					ID:          fmt.Sprintf("feat-%s-%d", lastMS.Name, len(prevPlan.Features)+1),
					Description: feedback,
					SkillName:   detectSkill(feedback, extractHints(feedback, clarifications)),
					Milestone:   lastMS.Name,
					Expected:    []string{fmt.Sprintf("%s is implemented", feedback)},
				}
				newPlan.Features = append([]PlanFeature{}, prevPlan.Features...)
				newPlan.Features = append(newPlan.Features, newFeat)
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
// state and persists it to the store.
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
			ID:              pf.ID,
			Description:     pf.Description,
			Status:          FeaturePending,
			SkillName:       pf.SkillName,
			Milestone:       pf.Milestone,
			Preconditions:   copyStrings(pf.Preconditions),
			ExpectedBehavior: copyStrings(pf.Expected),
			Fulfills:        copyStrings(pf.Fulfills),
			CreatedAt:       now,
			UpdatedAt:       now,
		}
	}

	mission := &Mission{
		ID:         missionID,
		Name:       plan.Name,
		Project:    plan.Project,
		Status:     MissionPlanning,
		CreatedAt:  now,
		UpdatedAt:  now,
		Features:   features,
		Milestones: milestones,
		Model:      plan.Model,
		Agent:      plan.Agent,
	}

	if err := store.CreateMission(mission); err != nil {
		return nil, fmt.Errorf("save mission: %w", err)
	}

	return mission, nil
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

// generateMissionID creates a short, unique mission ID using crypto/rand.
func generateMissionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("mission-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
