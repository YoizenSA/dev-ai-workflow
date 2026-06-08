package missions

import (
	"time"
)

// ─── Status Constants ──────────────────────────────────────────────────────

type MissionStatus string

const (
	MissionPending    MissionStatus = "pending"
	MissionPlanning  MissionStatus = "planning"
	MissionActive    MissionStatus = "active"
	MissionPaused    MissionStatus = "paused"
	MissionCompleted MissionStatus = "completed"
	MissionFailed    MissionStatus = "failed"
	MissionCancelled MissionStatus = "cancelled"
	MissionValidating MissionStatus = "validating"
)

type FeatureStatus string

const (
	FeaturePending     FeatureStatus = "pending"
	FeatureInProgress  FeatureStatus = "in_progress"
	FeatureCompleted   FeatureStatus = "completed"
	FeatureFailed      FeatureStatus = "failed"
	FeatureCancelled   FeatureStatus = "cancelled"
)

type ValidationStatus string

const (
	ValidationPending    ValidationStatus = "pending"
	ValidationPassed     ValidationStatus = "passed"
	ValidationFailed     ValidationStatus = "failed"
	ValidationInProgress ValidationStatus = "in_progress"
)

// ─── Shared Plan Types ─────────────────────────────────────────────────────

// PlanMission is the user-facing representation used during planning and
// serialised as plan.json for --file input.
type PlanMission struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Project     string          `json:"project,omitempty"`
	Milestones  []PlanMilestone `json:"milestones"`
	Features    []PlanFeature   `json:"features"`
	Model       string          `json:"model,omitempty"`
	Agent       string          `json:"agent,omitempty"`
}

// Clone returns a deep copy of the PlanMission.
func (pm *PlanMission) Clone() *PlanMission {
	if pm == nil {
		return nil
	}
	cp := *pm
	cp.Milestones = make([]PlanMilestone, len(pm.Milestones))
	copy(cp.Milestones, pm.Milestones)
	cp.Features = make([]PlanFeature, len(pm.Features))
	copy(cp.Features, pm.Features)
	return &cp
}

// PlanMilestone describes a milestone in a plan.
type PlanMilestone struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PlanFeature describes a single feature in a plan.
type PlanFeature struct {
	ID              string   `json:"id"`
	Description     string   `json:"description"`
	SkillName       string   `json:"skillName"`
	Milestone       string   `json:"milestone"`
	Preconditions   []string `json:"preconditions,omitempty"`
	Expected        []string `json:"expectedBehavior,omitempty"`
	Fulfills        []string `json:"fulfills,omitempty"`
}

// ─── Core Mission Types ────────────────────────────────────────────────────

// Mission represents a top-level mission created by the user.
type Mission struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Project     string         `json:"project,omitempty"`
	Status      MissionStatus  `json:"status"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	CompletedAt *time.Time     `json:"completedAt,omitempty"`
	Features    []Feature      `json:"features"`
	Milestones  []Milestone    `json:"milestones"`
	Model       string         `json:"model,omitempty"`
	Agent       string         `json:"agent,omitempty"`
}

// Clone returns a deep copy of the Mission.
func (m *Mission) Clone() *Mission {
	if m == nil {
		return nil
	}
	cp := *m
	cp.Features = make([]Feature, len(m.Features))
	for i := range m.Features {
		cp.Features[i] = *m.Features[i].Clone()
	}
	cp.Milestones = make([]Milestone, len(m.Milestones))
	for i := range m.Milestones {
		cp.Milestones[i] = *m.Milestones[i].Clone()
	}
	if m.CompletedAt != nil {
		t := *m.CompletedAt
		cp.CompletedAt = &t
	}
	return &cp
}

// Milestone groups features under a named milestone.
type Milestone struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Clone returns a deep copy of the Milestone.
func (m *Milestone) Clone() *Milestone {
	cp := *m
	return &cp
}

// Feature represents a single feature within a mission.
type Feature struct {
	ID                   string          `json:"id"`
	Description          string          `json:"description"`
	Status               FeatureStatus   `json:"status"`
	SkillName            string          `json:"skillName,omitempty"`
	Milestone            string          `json:"milestone,omitempty"`
	Preconditions        []string        `json:"preconditions,omitempty"`
	ExpectedBehavior     []string        `json:"expectedBehavior,omitempty"`
	Fulfills             []string        `json:"fulfills,omitempty"`
	WorkerSessionIDs     []string        `json:"workerSessionIds,omitempty"`
	CurrentWorkerSessionID *string       `json:"currentWorkerSessionId,omitempty"`
	CompletedWorkerSessionID *string     `json:"completedWorkerSessionId,omitempty"`
	RetryCount           int             `json:"retryCount,omitempty"`
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
	CompletedAt          *time.Time      `json:"completedAt,omitempty"`
}

// Clone returns a deep copy of the Feature.
func (f *Feature) Clone() *Feature {
	cp := *f
	cp.Preconditions = copyStrings(f.Preconditions)
	cp.ExpectedBehavior = copyStrings(f.ExpectedBehavior)
	cp.Fulfills = copyStrings(f.Fulfills)
	cp.WorkerSessionIDs = copyStrings(f.WorkerSessionIDs)
	if f.CurrentWorkerSessionID != nil {
		s := *f.CurrentWorkerSessionID
		cp.CurrentWorkerSessionID = &s
	}
	if f.CompletedWorkerSessionID != nil {
		s := *f.CompletedWorkerSessionID
		cp.CompletedWorkerSessionID = &s
	}
	if f.CompletedAt != nil {
		t := *f.CompletedAt
		cp.CompletedAt = &t
	}
	return &cp
}

// ─── Validation Types ──────────────────────────────────────────────────────

// ValidationState holds the results of validation runs for a mission.
type ValidationState struct {
	Assertions  []ValidationAssertion  `json:"assertions"`
	UpdatedAt   time.Time              `json:"updatedAt"`
}

// ValidationAssertion records the result of a single assertion test.
type ValidationAssertion struct {
	ID          string           `json:"id"`
	Description string           `json:"description,omitempty"`
	Status      ValidationStatus `json:"status"`
	Surface     string           `json:"surface,omitempty"` // cli, tui, web, engine
	Tool        string           `json:"tool,omitempty"`   // bash, tuistory, agent-browser, go test
	Error       string           `json:"error,omitempty"`
	RunAt       time.Time        `json:"runAt,omitempty"`
}

// ─── Worker Handoff Types ──────────────────────────────────────────────────

// WorkerHandoff is the structured output from a worker session.
type WorkerHandoff struct {
	SalientSummary    string        `json:"salientSummary"`
	WhatWasImplemented string       `json:"whatWasImplemented"`
	WhatWasLeftUndone string        `json:"whatWasLeftUndone"`
	Verification      Verification  `json:"verification"`
	Tests             TestInfo      `json:"tests"`
	DiscoveredIssues  []Issue       `json:"discoveredIssues"`
}

// Verification holds commands run and interactive checks performed.
type Verification struct {
	CommandsRun       []CommandRun       `json:"commandsRun"`
	InteractiveChecks []InteractiveCheck `json:"interactiveChecks,omitempty"`
}

// CommandRun records a single shell command execution.
type CommandRun struct {
	Command     string `json:"command"`
	ExitCode    int    `json:"exitCode"`
	Observation string `json:"observation"`
}

// InteractiveCheck records a manual UI/browser check.
type InteractiveCheck struct {
	Action   string `json:"action"`
	Observed string `json:"observed"`
}

// TestInfo describes the tests written or updated.
type TestInfo struct {
	Added    []TestFile   `json:"added"`
	Updated  []string     `json:"updated,omitempty"`
	Coverage string       `json:"coverage"`
}

// TestFile describes a test file and its cases.
type TestFile struct {
	File  string     `json:"file"`
	Cases []TestCase `json:"cases"`
}

// TestCase describes a single test case.
type TestCase struct {
	Name     string `json:"name"`
	Verifies string `json:"verifies"`
}

// Issue represents a discovered issue.
type Issue struct {
	Severity    string `json:"severity"` // blocking, non_blocking, suggestion
	Description string `json:"description"`
	SuggestedFix string `json:"suggestedFix,omitempty"`
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func copyStrings(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}
