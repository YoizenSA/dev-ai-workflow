package missions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ─── Errors ────────────────────────────────────────────────────────────────

var (
	// ErrValidationTimedOut is returned when a validation step exceeds its timeout.
	ErrValidationTimedOut = errors.New("validation timed out")
	// ErrBlockingIssuesFound indicates blocking issues were found during validation.
	ErrBlockingIssuesFound = errors.New("blocking issues found during validation")
	// ErrReviewerNotFound is returned when the reviewer binary is not available.
	ErrReviewerNotFound = errors.New("reviewer binary not found in PATH")
	// ErrReviewerFailed is returned when the reviewer process exits with an error.
	ErrReviewerFailed = errors.New("reviewer process failed")
	// ErrInvalidReviewOutput is returned when the reviewer produces unparseable output.
	ErrInvalidReviewOutput = errors.New("reviewer produced invalid output")
)

// ─── Constants ─────────────────────────────────────────────────────────────

const (
	// DefaultValidationTimeout is the maximum duration for the full validation pipeline.
	DefaultValidationTimeout = 10 * time.Minute
	// DefaultScrutinyTimeout is the maximum duration for scrutiny code review.
	DefaultScrutinyTimeout = 5 * time.Minute
	// DefaultUserTestingTimeout is the maximum duration for user testing.
	DefaultUserTestingTimeout = 5 * time.Minute
)

// ─── Config ────────────────────────────────────────────────────────────────

// ValidationConfig configures timeout behaviour for the validation pipeline.
type ValidationConfig struct {
	ScrutinyTimeout    time.Duration
	UserTestingTimeout time.Duration
}

// DefaultValidationConfig returns a ValidationConfig with sensible defaults.
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		ScrutinyTimeout:    DefaultScrutinyTimeout,
		UserTestingTimeout: DefaultUserTestingTimeout,
	}
}

// ─── Result Types ──────────────────────────────────────────────────────────

// ScrutinyResult holds the output of a scrutiny code review.
type ScrutinyResult struct {
	Issues  []Issue `json:"issues"`
	Summary string  `json:"summary"`
}

// HasBlockingIssues returns true if any issue has severity "blocking".
func (sr *ScrutinyResult) HasBlockingIssues() bool {
	for _, issue := range sr.Issues {
		if issue.Severity == "blocking" {
			return true
		}
	}
	return false
}

// UserTestingResult holds the results of user testing validation.
type UserTestingResult struct {
	Assertions []ValidationAssertion `json:"assertions"`
}

// ValidationReport aggregates results from both validators.
type ValidationReport struct {
	Milestone      string                `json:"milestone"`
	Scrutiny       ScrutinyResult        `json:"scrutiny"`
	UserTesting    UserTestingResult     `json:"userTesting"`
	Passed         bool                  `json:"passed"`
	BlockingIssues int                   `json:"blockingIssues"`
	VerifyEvidence []FeatureVerifyStatus `json:"verifyEvidence,omitempty"`
	RunAt          time.Time             `json:"runAt"`
	CompletedAt    *time.Time            `json:"completedAt,omitempty"`
}

// ─── Validation Pipeline ───────────────────────────────────────────────────

// ValidationPipeline orchestrates scrutiny (code review) and user testing
// (behavioral assertion) validators. It runs when a milestone completes.
type ValidationPipeline struct {
	store      *MissionsStore
	config     ValidationConfig
	cmdCreator func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewValidationPipeline creates a new ValidationPipeline.
func NewValidationPipeline(store *MissionsStore, config ValidationConfig) *ValidationPipeline {
	return &ValidationPipeline{
		store:      store,
		config:     config,
		cmdCreator: exec.CommandContext,
	}
}

// RunValidation executes the full validation pipeline for a milestone.
//
// Steps:
//  1. Collect features for the milestone
//  2. Run scrutiny (code review) validator with timeout
//  3. Run user testing (behavioral assertions) with timeout
//  4. Compile results and determine pass/fail
//  5. Persist results to validation-state.json (append-only)
//  6. Create fix features for blocking issues
//
// An empty milestone (no features) passes validation automatically.
func (vp *ValidationPipeline) RunValidation(ctx context.Context, mission *Mission, milestoneName string) (*ValidationReport, error) {
	if mission == nil {
		return nil, ErrInvalidMission
	}

	features := GetMilestoneFeatures(mission, milestoneName)

	// Empty milestone always passes (VAL-ENG-VAL-006)
	if len(features) == 0 {
		now := time.Now().UTC()
		report := &ValidationReport{
			Milestone:      milestoneName,
			Passed:         true,
			BlockingIssues: 0,
			RunAt:          now,
			CompletedAt:    &now,
		}
		// Persist empty results to validation-state.json
		if err := vp.persistResults(mission.ID, milestoneName, report); err != nil {
			return nil, fmt.Errorf("persist empty validation: %w", err)
		}
		return report, nil
	}

	// Check context before starting
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// ─── Run Scrutiny Validator ──────────────────────────────────────────
	scrutinyCtx, scrutinyCancel := context.WithTimeout(ctx, vp.config.ScrutinyTimeout)
	defer scrutinyCancel()

	scrutinyResult, err := vp.RunScrutinyValidator(scrutinyCtx, features)
	if err != nil {
		// Allow timeout error to propagate
		if errors.Is(err, ErrValidationTimedOut) {
			return nil, fmt.Errorf("scrutiny validation: %w", err)
		}
		// Non-fatal errors still produce a result with any issues found
		scrutinyResult = &ScrutinyResult{
			Issues:  []Issue{},
			Summary: fmt.Sprintf("Scrutiny encountered an error: %v", err),
		}
	}

	// ─── Run User Testing ────────────────────────────────────────────────
	testingCtx, testingCancel := context.WithTimeout(ctx, vp.config.UserTestingTimeout)
	defer testingCancel()

	userTestingResult, err := vp.RunUserTesting(testingCtx, mission, milestoneName)
	if err != nil {
		if errors.Is(err, ErrValidationTimedOut) {
			return nil, fmt.Errorf("user testing: %w", err)
		}
		userTestingResult = &UserTestingResult{
			Assertions: []ValidationAssertion{},
		}
	}

	// ─── Verify Evidence ──────────────────────────────────────────────────
	var verifyEvidence []FeatureVerifyStatus
	for _, f := range features {
		if n := len(f.VerifyRuns); n > 0 && !f.VerifyRuns[n-1].Passed {
			last := f.VerifyRuns[n-1]
			failedCmd := ""
			if len(last.Commands) > 0 {
				failedCmd = last.Commands[0].Command
			}
			scrutinyResult.Issues = append(scrutinyResult.Issues, Issue{
				Severity:    "blocking",
				Description: fmt.Sprintf("verify failed for %s: %s", f.ID, failedCmd),
			})
			verifyEvidence = append(verifyEvidence, FeatureVerifyStatus{
				FeatureID:     f.ID,
				Passed:        false,
				FailedCommand: failedCmd,
			})
		} else {
			verifyEvidence = append(verifyEvidence, FeatureVerifyStatus{
				FeatureID: f.ID,
				Passed:    true,
			})
		}
	}

	// ─── Compile Report ─────────────────────────────────────────────────
	blockingCount := 0
	for _, issue := range scrutinyResult.Issues {
		if issue.Severity == "blocking" {
			blockingCount++
		}
	}

	passed := blockingCount == 0
	now := time.Now().UTC()

	report := &ValidationReport{
		Milestone:      milestoneName,
		Scrutiny:       *scrutinyResult,
		UserTesting:    *userTestingResult,
		Passed:         passed,
		BlockingIssues: blockingCount,
		VerifyEvidence: verifyEvidence,
		RunAt:          now,
		CompletedAt:    &now,
	}

	// Persist results (VAL-ENG-VAL-004, VAL-CROSS-CONSIST-05)
	if err := vp.persistResults(mission.ID, milestoneName, report); err != nil {
		return nil, fmt.Errorf("persist validation results: %w", err)
	}

	// Create fix features for blocking issues (VAL-CROSS-VAL-06)
	// Note: caller must persist the mission after this returns
	if !passed {
		vp.CreateFixFeatures(mission, milestoneName, scrutinyResult.Issues)
	}

	return report, nil
}

// RunScrutinyValidator runs code review on the given features.
//
// It spawns a reviewer agent and parses its output for issues with severity
// levels (blocking, non_blocking, suggestion). If the reviewer binary is not
// available or the process fails, it falls back to structural validation.
//
// The reviewer is expected to output JSON with the following shape:
//
//	{ "issues": [...], "summary": "..." }
//
// where each issue has:
//
//	{ "severity": "blocking|non_blocking|suggestion", "description": "...", "suggestedFix": "..." }
func (vp *ValidationPipeline) RunScrutinyValidator(ctx context.Context, features []Feature) (*ScrutinyResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, ErrValidationTimedOut
	}

	// Attempt to detect a reviewer binary
	reviewerPath, err := exec.LookPath("opencode")
	if err != nil {
		// No reviewer available — fall back to structural validation
		return vp.structuralValidation(features), nil
	}

	// Build a review prompt as a JSON payload
	prompt := buildReviewPrompt(features)
	promptJSON, err := json.Marshal(prompt)
	if err != nil {
		return vp.structuralValidation(features), nil
	}

	// Spawn the reviewer with the prompt on stdin
	cmd := vp.cmdCreator(ctx, reviewerPath, "--review")
	cmd.Stdin = strings.NewReader(string(promptJSON))

	output, err := cmd.Output()
	if err != nil {
		// Reviewer failed — recover with structural validation
		return vp.structuralValidation(features), nil
	}

	// Try to parse reviewer JSON output
	var scrutinyResult ScrutinyResult
	if err := json.Unmarshal(output, &scrutinyResult); err != nil {
		return vp.structuralValidation(features), nil
	}

	// Validate that issues have proper severity levels
	for _, issue := range scrutinyResult.Issues {
		switch issue.Severity {
		case "blocking", "non_blocking", "suggestion":
			// valid
		default:
			// Invalid severity — fall back to structural
			return vp.structuralValidation(features), nil
		}
	}

	return &scrutinyResult, nil
}

// buildReviewPrompt creates a review prompt for the given features.
func buildReviewPrompt(features []Feature) map[string]interface{} {
	featList := make([]map[string]interface{}, len(features))
	for i, f := range features {
		featList[i] = map[string]interface{}{
			"id":               f.ID,
			"description":      f.Description,
			"expectedBehavior": f.ExpectedBehavior,
			"skillName":        f.SkillName,
			"status":           string(f.Status),
		}
	}
	return map[string]interface{}{
		"action":   "review",
		"features": featList,
	}
}

// structuralValidation performs basic structural checks on features.
// This is the fallback used when no reviewer agent is available.
func (vp *ValidationPipeline) structuralValidation(features []Feature) *ScrutinyResult {
	var issues []Issue

	for _, f := range features {
		// Empty ID — blocking
		if strings.TrimSpace(f.ID) == "" {
			issues = append(issues, Issue{
				Severity:    "blocking",
				Description: "Feature has an empty ID",
			})
		}

		// No expected behaviors — non-blocking
		if len(f.ExpectedBehavior) == 0 {
			issues = append(issues, Issue{
				Severity:    "non_blocking",
				Description: fmt.Sprintf("Feature %q has no expected behaviors defined", f.ID),
			})
		}

		// No skill assigned — non-blocking
		if f.SkillName == "" {
			issues = append(issues, Issue{
				Severity:    "non_blocking",
				Description: fmt.Sprintf("Feature %q has no skill assigned", f.ID),
			})
		}

		// No milestone — blocking
		if f.Milestone == "" {
			issues = append(issues, Issue{
				Severity:    "blocking",
				Description: fmt.Sprintf("Feature %q has no milestone assigned", f.ID),
			})
		}

		// Completed without timestamp — suggestion
		if f.Status == FeatureCompleted && f.CompletedAt == nil {
			issues = append(issues, Issue{
				Severity:    "suggestion",
				Description: fmt.Sprintf("Feature %q is completed but has no completion timestamp", f.ID),
			})
		}

		// Feature was configured but has no fulfills — suggestion
		if f.Status == FeatureCompleted && len(f.Fulfills) == 0 {
			issues = append(issues, Issue{
				Severity:    "suggestion",
				Description: fmt.Sprintf("Feature %q completed without any fulfills assertions", f.ID),
			})
		}
	}

	return &ScrutinyResult{
		Issues:  issues,
		Summary: fmt.Sprintf("Structural validation complete: %d issues found", len(issues)),
	}
}

// RunUserTesting runs behavioral assertion tests for the milestone.
//
// It collects the fulfills assertions from all features in the milestone and
// records their status. Engine-level assertions (VAL-ENG-*, VAL-CROSS-VAL.*)
// can be verified in-process; CLI/TUI/Web assertions are recorded as pending
// since they require external testing tools.
func (vp *ValidationPipeline) RunUserTesting(ctx context.Context, mission *Mission, milestoneName string) (*UserTestingResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, ErrValidationTimedOut
	}

	features := GetMilestoneFeatures(mission, milestoneName)
	var assertions []ValidationAssertion
	now := time.Now().UTC()

	// Load validation contract if it exists to get assertion details
	missionDir := vp.store.MissionDir(mission.ID)
	contractParser := NewContractParser(missionDir)
	contract, _ := contractParser.LoadContract()

	for _, f := range features {
		for _, fulfillsID := range f.Fulfills {
			a := vp.testAssertion(fulfillsID, f, now, contract)
			assertions = append(assertions, a)
		}
	}

	return &UserTestingResult{
		Assertions: assertions,
	}, nil
}

// testAssertion checks a single assertion ID and returns its result.
func (vp *ValidationPipeline) testAssertion(assertionID string, feature Feature, now time.Time, contract *ValidationContract) ValidationAssertion {
	a := ValidationAssertion{
		ID:    assertionID,
		RunAt: now,
	}

	// Try to get assertion details from contract
	if contract != nil {
		if contractAssertion := contract.GetAssertionByID(assertionID); contractAssertion != nil {
			a.Description = contractAssertion.Description
			a.Tool = contractAssertion.Tool
			a.Surface = inferSurfaceFromTool(contractAssertion.Tool)
		}
	}

	// Engine-level VAL-ENG-VAL assertions
	if strings.HasPrefix(assertionID, "VAL-ENG-VAL") {
		return vp.testVALENGVALAssertion(assertionID, feature, now)
	}

	// Cross validation assertions
	if strings.HasPrefix(assertionID, "VAL-CROSS-VAL") || strings.HasPrefix(assertionID, "VAL-CROSS-CONSIST") {
		return vp.testCrossAssertion(assertionID, feature, now)
	}

	// If we have contract details but no specific test, mark as pending
	if a.Description != "" {
		a.Status = ValidationPending
		return a
	}

	// Unknown assertions — mark as pending
	if a.Description == "" {
		a.Description = fmt.Sprintf("Assertion %s requires external tooling", assertionID)
	}
	a.Status = ValidationPending
	if a.Surface == "" {
		a.Surface = "engine"
	}
	if a.Tool == "" {
		a.Tool = "auto-detect"
	}
	return a
}

// inferSurfaceFromTool infers the validation surface from the tool name.
func inferSurfaceFromTool(tool string) string {
	switch tool {
	case "agent-browser":
		return "web"
	case "tuistory":
		return "tui"
	case "bash", "curl":
		return "cli"
	default:
		return "engine"
	}
}

func (vp *ValidationPipeline) testVALENGVALAssertion(assertionID string, feature Feature, now time.Time) ValidationAssertion {
	a := ValidationAssertion{
		ID:      assertionID,
		Surface: "engine",
		Tool:    "manual",
		RunAt:   now,
	}

	// These assertions require external verification (reviewer agent, timing, etc.).
	// Marked pending until actual verification infrastructure runs them.
	switch assertionID {
	case "VAL-ENG-VAL-001":
		a.Description = "Scrutiny validator spawns reviewer agent (requires manual verification)"
	case "VAL-ENG-VAL-002":
		a.Description = "Review output includes severity levels (requires manual verification)"
	case "VAL-ENG-VAL-003":
		a.Description = "Behavioral assertions from contract are tested (requires manual verification)"
	case "VAL-ENG-VAL-004":
		a.Description = "Results written to validation-state.json (requires manual verification)"
	case "VAL-ENG-VAL-005":
		a.Description = "Blocking issues prevent milestone completion (requires manual verification)"
	case "VAL-ENG-VAL-006":
		a.Description = "Empty milestone passes validation (requires manual verification)"
	case "VAL-ENG-VAL-007":
		a.Description = "Validator timeout kills process (requires manual verification)"
	case "VAL-ENG-VAL-008":
		a.Description = "Validation re-run on fix features works (requires manual verification)"
	default:
		a.Description = fmt.Sprintf("Engine assertion: %s", assertionID)
	}
	a.Status = ValidationPending
	return a
}

func (vp *ValidationPipeline) testCrossAssertion(assertionID string, feature Feature, now time.Time) ValidationAssertion {
	a := ValidationAssertion{
		ID:      assertionID,
		Surface: "engine",
		Tool:    "manual",
		RunAt:   now,
	}

	// These assertions require cross-cutting integration testing.
	// Marked pending until actual verification infrastructure runs them.
	switch assertionID {
	case "VAL-CROSS-VAL-01":
		a.Description = "Validation auto-triggers on milestone completion (requires manual verification)"
	case "VAL-CROSS-VAL-02":
		a.Description = "Scrutiny produces review output (requires manual verification)"
	case "VAL-CROSS-VAL-03":
		a.Description = "User testing executes assertions (requires manual verification)"
	case "VAL-CROSS-VAL-06":
		a.Description = "Failed validation creates fix features (requires manual verification)"
	case "VAL-CROSS-VAL-07":
		a.Description = "Validation survives crash (requires manual verification)"
	case "VAL-CROSS-CONSIST-05":
		a.Description = "Validation state append-only (requires manual verification)"
	default:
		a.Description = fmt.Sprintf("Cross assertion: %s", assertionID)
	}
	a.Status = ValidationPending
	return a
}

// CreateFixFeatures creates fix- prefixed features for blocking issues found
// during validation. Each blocking issue produces one fix feature (VAL-CROSS-VAL-06).
func (vp *ValidationPipeline) CreateFixFeatures(mission *Mission, milestoneName string, issues []Issue) []Feature {
	var fixFeatures []Feature
	for _, issue := range issues {
		if issue.Severity != "blocking" {
			continue
		}
		fixID := fmt.Sprintf("fix-%s", generateShortID())
		fixFeature := Feature{
			ID:          fixID,
			Description: fmt.Sprintf("Fix: %s", issue.Description),
			Status:      FeaturePending,
			Milestone:   milestoneName,
			SkillName:   "backend-worker",
			ExpectedBehavior: []string{
				fmt.Sprintf("Resolve blocking issue: %s", issue.Description),
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		mission.Features = append(mission.Features, fixFeature)
		fixFeatures = append(fixFeatures, fixFeature)
	}
	return fixFeatures
}

// persistResults saves validation results to the store in an append-only manner.
// Existing assertions are preserved and new ones are merged in (VAL-CROSS-CONSIST-05).
func (vp *ValidationPipeline) persistResults(missionID, milestoneName string, report *ValidationReport) error {
	// Load existing state for append-only merge
	existingState, err := vp.store.LoadValidationState(missionID)
	if err != nil {
		return fmt.Errorf("load existing validation state: %w", err)
	}

	// Build set of existing assertion IDs
	existingMap := make(map[string]int) // ID → index
	for i, a := range existingState.Assertions {
		existingMap[a.ID] = i
	}

	// Merge new assertions: append new ones, update existing ones
	for _, a := range report.UserTesting.Assertions {
		if idx, ok := existingMap[a.ID]; ok {
			// Update existing assertion status
			existingState.Assertions[idx] = a
		} else {
			// Append new assertion
			existingState.Assertions = append(existingState.Assertions, a)
			existingMap[a.ID] = len(existingState.Assertions) - 1
		}
	}

	existingState.UpdatedAt = time.Now().UTC()
	return vp.store.SaveValidationState(missionID, existingState)
}

// CheckValidationContractCoverage verifies that every assertion in the validation
// contract is claimed by exactly one feature. Returns an error if coverage is incomplete.
func CheckValidationContractCoverage(store *MissionsStore, mission *Mission) error {
	missionDir := store.MissionDir(mission.ID)
	contractParser := NewContractParser(missionDir)

	contract, err := contractParser.LoadContract()
	if err != nil {
		// If contract doesn't exist yet, skip coverage check
		// This is expected during initial planning
		return nil
	}

	return contractParser.CheckCoverage(contract, mission.Features)
}

// ─── Public API ────────────────────────────────────────────────────────────

// ValidateMilestone is a high-level convenience function that runs the full
// validation pipeline for a milestone and handles mission status transitions.
//
// It is designed to be called after ProcessMilestoneAfterFeature transitions
// the mission to "validating" status.
func ValidateMilestone(store *MissionsStore, mission *Mission, milestoneName string) (*ValidationReport, error) {
	if store == nil {
		return nil, ErrInvalidMission
	}
	if mission == nil {
		return nil, ErrInvalidMission
	}

	config := DefaultValidationConfig()
	pipeline := NewValidationPipeline(store, config)

	ctx, cancel := context.WithTimeout(context.Background(), DefaultValidationTimeout)
	defer cancel()

	report, err := pipeline.RunValidation(ctx, mission, milestoneName)
	if err != nil {
		return report, err
	}

	// Update mission status based on validation outcome
	now := time.Now().UTC()
	if report.Passed {
		// Validation passed — transition mission to completed
		newStatus, err := TransitionMissionStatus(mission.Status, MissionCompleted)
		if err != nil {
			// If we can't transition, log but don't fail - results are already persisted
			return report, fmt.Errorf("transition mission to completed after validation: %w", err)
		}
		mission.Status = newStatus
		mission.CompletedAt = &now
	} else {
		// Validation failed — transition back to active for fix features
		newStatus, err := TransitionMissionStatus(mission.Status, MissionActive)
		if err != nil {
			return report, fmt.Errorf("transition mission to active after failed validation: %w", err)
		}
		mission.Status = newStatus
	}

	mission.UpdatedAt = now
	if err := store.SaveMission(mission); err != nil {
		return report, fmt.Errorf("persist mission after validation: %w", err)
	}

	return report, nil
}

// IsBlockingIssue returns true if the issue has "blocking" severity.
func IsBlockingIssue(issue Issue) bool {
	return issue.Severity == "blocking"
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// generateShortID creates a short random hex ID for fix features.
func generateShortID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use timestamp fragment
		return fmt.Sprintf("%x", time.Now().UnixNano())[:8]
	}
	return hex.EncodeToString(b)
}
