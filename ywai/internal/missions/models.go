package missions

import (
	"time"
)

// ─── Status Constants ──────────────────────────────────────────────────────

type MissionStatus string

const (
	MissionPending    MissionStatus = "pending"
	MissionPlanning   MissionStatus = "planning"
	MissionActive     MissionStatus = "active"
	MissionPaused     MissionStatus = "paused"
	MissionCompleted  MissionStatus = "completed"
	MissionFailed     MissionStatus = "failed"
	MissionCancelled  MissionStatus = "cancelled"
	MissionValidating MissionStatus = "validating"
)

type FeatureStatus string

const (
	FeaturePending    FeatureStatus = "pending"
	FeatureInProgress FeatureStatus = "in_progress"
	FeatureCompleted  FeatureStatus = "completed"
	FeatureFailed     FeatureStatus = "failed"
	FeatureCancelled  FeatureStatus = "cancelled"
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

	// LocalFallback is true when the plan was produced by the generic local
	// planner because opencode was unavailable or failed. Such a plan does NOT
	// reflect the specific goal and should be surfaced loudly to the user.
	// Not persisted — it only matters at planning time.
	LocalFallback bool `json:"-"`
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
	for i := range pm.Features {
		cp.Features[i] = pm.Features[i]
		cp.Features[i].Preconditions = copyStrings(pm.Features[i].Preconditions)
		cp.Features[i].Expected = copyStrings(pm.Features[i].Expected)
		cp.Features[i].Fulfills = copyStrings(pm.Features[i].Fulfills)
		cp.Features[i].Fallbacks = copyStrings(pm.Features[i].Fallbacks)
	}
	return &cp
}

// PlanMilestone describes a milestone in a plan.
type PlanMilestone struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PlanFeature describes a single feature in a plan.
type PlanFeature struct {
	ID            string   `json:"id"`
	Description   string   `json:"description"`
	SkillName     string   `json:"skillName"`
	Milestone     string   `json:"milestone"`
	Preconditions []string `json:"preconditions,omitempty"`
	Expected      []string `json:"expectedBehavior,omitempty"`
	Fulfills      []string `json:"fulfills,omitempty"`
	// Role identifies the execution role (planning, dev, frontend, backend, qa, reviewer, devops).
	// Populated by the planner using detectRole or by a planner-LLM contract.
	Role string `json:"role,omitempty"`
	// Model overrides Mission.Model for this feature when non-empty.
	Model string `json:"model,omitempty"`
	// Agent overrides Mission.ExecutionAgent for this feature when non-empty.
	Agent string `json:"agent,omitempty"`
	// Fallbacks are tried in order if the primary Model fails with a retriable error.
	Fallbacks []string `json:"fallbacks,omitempty"`
}

// ─── Core Mission Types ────────────────────────────────────────────────────

// Mission represents a top-level mission created by the user.
type Mission struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Project        string            `json:"project,omitempty"`
	Status         MissionStatus     `json:"status"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
	CompletedAt    *time.Time        `json:"completedAt,omitempty"`
	Features       []Feature         `json:"features"`
	Milestones     []Milestone       `json:"milestones"`
	Model          string            `json:"model,omitempty"`
	Agent          string            `json:"agent,omitempty"`
	WorkerTypes    []WorkerType      `json:"workerTypes,omitempty"`
	ExecutionAgent string            `json:"executionAgent,omitempty"`
	Artifacts      *MissionArtifacts `json:"artifacts,omitempty"`
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
	cp.WorkerTypes = make([]WorkerType, len(m.WorkerTypes))
	copy(cp.WorkerTypes, m.WorkerTypes)
	if m.CompletedAt != nil {
		t := *m.CompletedAt
		cp.CompletedAt = &t
	}
	return &cp
}

// WorkerType describes a specialized worker role for a group of features.
type WorkerType struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SkillPath   string `json:"skillPath"`
}

// Milestone groups features under a named milestone.
type Milestone struct {
	Name              string              `json:"name"`
	Description       string              `json:"description"`
	ValidationReports []*ValidationReport `json:"validationReports,omitempty"`
}

// VerifyRun records a single verification execution for a feature.
type VerifyRun struct {
	Passed   bool         `json:"passed"`
	Commands []CommandRun `json:"commands"` // reuses CommandRun from models.go
	RunAt    time.Time    `json:"runAt"`
	Output   string       `json:"output,omitempty"`
}

// VerifyResult aggregates one or more VerifyRuns for a verification gate.
type VerifyResult struct {
	Passed   bool        `json:"passed"`
	Runs     []VerifyRun `json:"runs"`
	Combined string      `json:"combined"`
}

// Clone returns a deep copy of the Milestone.
func (m *Milestone) Clone() *Milestone {
	cp := *m
	return &cp
}

// Feature represents a single feature within a mission.
type Feature struct {
	ID                       string        `json:"id"`
	Description              string        `json:"description"`
	Status                   FeatureStatus `json:"status"`
	SkillName                string        `json:"skillName,omitempty"`
	Milestone                string        `json:"milestone,omitempty"`
	Preconditions            []string      `json:"preconditions,omitempty"`
	ExpectedBehavior         []string      `json:"expectedBehavior,omitempty"`
	Fulfills                 []string      `json:"fulfills,omitempty"`
	WorkerSessionIDs         []string      `json:"workerSessionIds,omitempty"`
	CurrentWorkerSessionID   *string       `json:"currentWorkerSessionId,omitempty"`
	CompletedWorkerSessionID *string       `json:"completedWorkerSessionId,omitempty"`
	RetryCount               int           `json:"retryCount,omitempty"`
	WorktreePath             string        `json:"worktreePath,omitempty"` // worktree activo de esta feature
	Branch                   string        `json:"branch,omitempty"`       // ywai/<mission>/<feature>
	LastError                string        `json:"lastError,omitempty"`    // para self-correction
	VerifyRuns               []VerifyRun   `json:"verifyRuns,omitempty"`   // historial de verificación
	// Role identifies the execution role for this feature. Inherited from
	// PlanFeature.Role at mission creation; falls back to SkillNameToRole(SkillName)
	// for missions persisted before the field existed.
	Role string `json:"role,omitempty"`
	// Model overrides Mission.Model for this specific feature when non-empty.
	Model string `json:"model,omitempty"`
	// Agent overrides Mission.ExecutionAgent for this feature when non-empty.
	Agent string `json:"agent,omitempty"`
	// Fallbacks are tried in order if the primary Model fails with a retriable error.
	Fallbacks   []string   `json:"fallbacks,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// Clone returns a deep copy of the Feature.
func (f *Feature) Clone() *Feature {
	cp := *f
	cp.Preconditions = copyStrings(f.Preconditions)
	cp.ExpectedBehavior = copyStrings(f.ExpectedBehavior)
	cp.Fulfills = copyStrings(f.Fulfills)
	cp.WorkerSessionIDs = copyStrings(f.WorkerSessionIDs)
	cp.Fallbacks = copyStrings(f.Fallbacks)
	if f.CurrentWorkerSessionID != nil {
		s := *f.CurrentWorkerSessionID
		cp.CurrentWorkerSessionID = &s
	}
	if f.CompletedWorkerSessionID != nil {
		s := *f.CompletedWorkerSessionID
		cp.CompletedWorkerSessionID = &s
	}
	cp.VerifyRuns = make([]VerifyRun, len(f.VerifyRuns))
	copy(cp.VerifyRuns, f.VerifyRuns)
	if f.CompletedAt != nil {
		t := *f.CompletedAt
		cp.CompletedAt = &t
	}
	return &cp
}

// ─── Validation Types ──────────────────────────────────────────────────────

// ValidationState holds the results of validation runs for a mission.
type ValidationState struct {
	Assertions []ValidationAssertion `json:"assertions"`
	UpdatedAt  time.Time             `json:"updatedAt"`
}

// ValidationAssertion records the result of a single assertion test.
type ValidationAssertion struct {
	ID          string           `json:"id"`
	Description string           `json:"description,omitempty"`
	Status      ValidationStatus `json:"status"`
	Surface     string           `json:"surface,omitempty"` // cli, tui, web, engine
	Tool        string           `json:"tool,omitempty"`    // bash, tuistory, agent-browser, go test
	Error       string           `json:"error,omitempty"`
	RunAt       time.Time        `json:"runAt,omitempty"`
}

// ─── Worker Handoff Types ──────────────────────────────────────────────────

// WorkerHandoff is the structured output from a worker session.
type WorkerHandoff struct {
	SalientSummary     string       `json:"salientSummary"`
	WhatWasImplemented string       `json:"whatWasImplemented"`
	WhatWasLeftUndone  string       `json:"whatWasLeftUndone"`
	Verification       Verification `json:"verification"`
	Tests              TestInfo     `json:"tests"`
	DiscoveredIssues   []Issue      `json:"discoveredIssues"`
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
	Added    []TestFile `json:"added"`
	Updated  []string   `json:"updated,omitempty"`
	Coverage string     `json:"coverage"`
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
	Severity     string `json:"severity"` // blocking, non_blocking, suggestion
	Description  string `json:"description"`
	SuggestedFix string `json:"suggestedFix,omitempty"`
}

// FeatureVerifyStatus records whether a feature passed its verify gate.
type FeatureVerifyStatus struct {
	FeatureID     string `json:"featureId"`
	Passed        bool   `json:"passed"`
	FailedCommand string `json:"failedCommand,omitempty"`
}

// ─── Mission Artifact Types ───────────────────────

// ValidationContract represents the validation contract with behavioral assertions.
type ValidationContract struct {
	Assertions []ContractAssertion `json:"assertions"`
}

// ContractAssertion represents a single behavioral assertion in the validation contract.
type ContractAssertion struct {
	ID          string   `json:"id"`          // e.g., VAL-AUTH-001
	Title       string   `json:"title"`       // Short description
	Description string   `json:"description"` // Behavioral description
	Tool        string   `json:"tool"`        // e.g., agent-browser, tuistory, curl
	Evidence    []string `json:"evidence"`    // Required evidence types
	Area        string   `json:"area"`        // e.g., Authentication, Cross-Area
}

// ServicesManifest represents the services.yaml manifest.
type ServicesManifest struct {
	Commands map[string]string     `json:"commands"` // Named command shortcuts
	Services map[string]ServiceDef `json:"services"` // Service definitions
}

// ServiceDef defines a long-running service.
type ServiceDef struct {
	Start       string   `json:"start"`       // Start command
	Stop        string   `json:"stop"`        // Stop command
	Healthcheck string   `json:"healthcheck"` // Health check command
	Port        int      `json:"port"`        // Port this service uses
	DependsOn   []string `json:"depends_on"`  // Services that must run first
}

// Skill represents a worker skill definition from SKILL.md.
type Skill struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	RequiredSkills   []string `json:"requiredSkills,omitempty"`
	RequiredTools    []string `json:"requiredTools,omitempty"`
	WorkProcedure    string   `json:"workProcedure"`
	ExampleHandoff   string   `json:"exampleHandoff"`
	ReturnConditions string   `json:"returnConditions"`
}

// MissionArtifacts represents all mission artifacts.
type MissionArtifacts struct {
	Architecture       string              `json:"architecture"`       // architecture.md content
	ValidationContract *ValidationContract `json:"validationContract"` // Parsed from validation-contract.md
	ServicesManifest   *ServicesManifest   `json:"servicesManifest"`   // Parsed from services.yaml
	AGENTSMD           string              `json:"agentsMD"`           // AGENTS.md content
	Library            map[string]string   `json:"library"`            // library/ files
	Skills             map[string]*Skill   `json:"skills"`             // skills/ files
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
