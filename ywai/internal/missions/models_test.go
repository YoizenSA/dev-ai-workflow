package missions

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMissionStatusConstants(t *testing.T) {
	tests := []struct {
		status MissionStatus
		want   string
	}{
		{MissionPending, "pending"},
		{MissionPlanning, "planning"},
		{MissionActive, "active"},
		{MissionPaused, "paused"},
		{MissionCompleted, "completed"},
		{MissionFailed, "failed"},
		{MissionCancelled, "cancelled"},
		{MissionValidating, "validating"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("expected %q, got %q", tt.want, string(tt.status))
		}
	}
}

func TestFeatureStatusConstants(t *testing.T) {
	tests := []struct {
		status FeatureStatus
		want   string
	}{
		{FeaturePending, "pending"},
		{FeatureInProgress, "in_progress"},
		{FeatureCompleted, "completed"},
		{FeatureFailed, "failed"},
		{FeatureCancelled, "cancelled"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("expected %q, got %q", tt.want, string(tt.status))
		}
	}
}

func TestValidationStatusConstants(t *testing.T) {
	tests := []struct {
		status ValidationStatus
		want   string
	}{
		{ValidationPending, "pending"},
		{ValidationPassed, "passed"},
		{ValidationFailed, "failed"},
		{ValidationInProgress, "in_progress"},
	}
	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("expected %q, got %q", tt.want, string(tt.status))
		}
	}
}

func TestMissionJSONRoundTrip(t *testing.T) {
	now := time.Now().Round(time.Second)
	m := &Mission{
		ID:        "test-1",
		Name:      "Test Mission",
		Status:    MissionPlanning,
		CreatedAt: now,
		UpdatedAt: now,
		Features: []Feature{
			{
				ID:               "feat-1",
				Description:      "First feature",
				Status:           FeaturePending,
				Milestone:        "core-engine",
				Preconditions:    []string{},
				ExpectedBehavior: []string{"works"},
				Fulfills:         []string{"VAL-001"},
				CreatedAt:        now,
				UpdatedAt:        now,
			},
		},
		Milestones: []Milestone{
			{Name: "core-engine", Description: "Core engine milestone"},
		},
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Mission
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != m.ID {
		t.Errorf("ID: got %q, want %q", got.ID, m.ID)
	}
	if got.Name != m.Name {
		t.Errorf("Name: got %q, want %q", got.Name, m.Name)
	}
	if got.Status != m.Status {
		t.Errorf("Status: got %q, want %q", got.Status, m.Status)
	}
	if !got.CreatedAt.Equal(m.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, m.CreatedAt)
	}
	if len(got.Features) != len(m.Features) {
		t.Fatalf("Features length: got %d, want %d", len(got.Features), len(m.Features))
	}
	if got.Features[0].ID != m.Features[0].ID {
		t.Errorf("Feature[0].ID: got %q, want %q", got.Features[0].ID, m.Features[0].ID)
	}
	if len(got.Milestones) != len(m.Milestones) {
		t.Fatalf("Milestones length: got %d, want %d", len(got.Milestones), len(m.Milestones))
	}
}

func TestFeatureClone(t *testing.T) {
	now := time.Now()
	f := &Feature{
		ID:               "feat-1",
		Description:      "Original",
		Status:           FeatureInProgress,
		SkillName:        "backend-worker",
		Milestone:        "core-engine",
		Preconditions:    []string{"pre-1"},
		ExpectedBehavior: []string{"works"},
		Fulfills:         []string{"VAL-001"},
		WorkerSessionIDs: []string{"sess-1"},
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	cp := f.Clone()

	// Modify original
	f.ID = "changed"
	f.Description = "Changed"
	f.Preconditions[0] = "changed"
	f.Fulfills[0] = "CHANGED"
	f.WorkerSessionIDs[0] = "changed"

	if cp.ID != "feat-1" {
		t.Errorf("Clone ID changed: got %q", cp.ID)
	}
	if cp.Description != "Original" {
		t.Errorf("Clone Description changed: got %q", cp.Description)
	}
	if cp.Preconditions[0] != "pre-1" {
		t.Errorf("Clone Preconditions[0] changed: got %q", cp.Preconditions[0])
	}
	if cp.Fulfills[0] != "VAL-001" {
		t.Errorf("Clone Fulfills[0] changed: got %q", cp.Fulfills[0])
	}
	if cp.WorkerSessionIDs[0] != "sess-1" {
		t.Errorf("Clone WorkerSessionIDs[0] changed: got %q", cp.WorkerSessionIDs[0])
	}
}

func TestMissionClone(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(-1 * time.Hour)
	m := &Mission{
		ID:          "mission-1",
		Name:        "Original",
		Status:      MissionCompleted,
		CreatedAt:   now,
		UpdatedAt:   now,
		CompletedAt: &completedAt,
		Features: []Feature{
			{ID: "feat-1", Description: "Feat 1", Status: FeatureCompleted, CreatedAt: now, UpdatedAt: now},
		},
		Milestones: []Milestone{
			{Name: "m1", Description: "Milestone 1"},
		},
	}

	cp := m.Clone()

	// Modify original
	m.ID = "modified"
	m.Name = "Modified"
	m.Features[0].ID = "modified"
	m.Milestones[0].Name = "modified"

	if cp.ID != "mission-1" {
		t.Errorf("Clone ID changed: got %q", cp.ID)
	}
	if cp.Name != "Original" {
		t.Errorf("Clone Name changed: got %q", cp.Name)
	}
	if cp.Features[0].ID != "feat-1" {
		t.Errorf("Clone Feature[0].ID changed: got %q", cp.Features[0].ID)
	}
	if cp.Milestones[0].Name != "m1" {
		t.Errorf("Clone Milestone[0].Name changed: got %q", cp.Milestones[0].Name)
	}
	if cp.CompletedAt == nil || !cp.CompletedAt.Equal(completedAt) {
		t.Errorf("Clone CompletedAt changed or nil")
	}
}

func TestFeatureCompletedAtOmitEmpty(t *testing.T) {
	// A feature without CompletedAt should omit it in JSON
	f := Feature{
		ID:        "feat-1",
		Status:    FeaturePending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "completedAt") {
		t.Errorf("completedAt should be omitted when empty: %s", string(data))
	}
}

func TestValidationStateJSONRoundTrip(t *testing.T) {
	now := time.Now().Round(time.Second)
	vs := &ValidationState{
		UpdatedAt: now,
		Assertions: []ValidationAssertion{
			{
				ID:          "VAL-001",
				Description: "Test assertion",
				Status:      ValidationPassed,
				Surface:     "engine",
				Tool:        "go test",
				RunAt:       now,
			},
			{
				ID:     "VAL-002",
				Status: ValidationPending,
			},
		},
	}

	data, err := json.Marshal(vs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ValidationState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got.Assertions) != 2 {
		t.Fatalf("expected 2 assertions, got %d", len(got.Assertions))
	}
	if got.Assertions[0].ID != "VAL-001" {
		t.Errorf("Assertion[0].ID: got %q", got.Assertions[0].ID)
	}
	if got.Assertions[0].Status != ValidationPassed {
		t.Errorf("Assertion[0].Status: got %v", got.Assertions[0].Status)
	}
	if got.Assertions[1].Status != ValidationPending {
		t.Errorf("Assertion[1].Status: got %v", got.Assertions[1].Status)
	}
}

// VAL-ENG-SYS-005: Date rollover handled correctly
// Test ensures timestamps round-trip correctly across midnight boundaries.
func TestDateRollover(t *testing.T) {
	// Test across midnight boundary
	beforeMidnight := time.Date(2026, 1, 1, 23, 59, 59, 0, time.UTC)
	afterMidnight := time.Date(2026, 1, 2, 0, 0, 1, 0, time.UTC)

	m := &Mission{
		ID:        "rollover-test",
		Name:      "Rollover Test",
		Status:    MissionActive,
		CreatedAt: beforeMidnight,
		UpdatedAt: afterMidnight,
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Mission
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !got.CreatedAt.Equal(beforeMidnight) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, beforeMidnight)
	}
	if !got.UpdatedAt.Equal(afterMidnight) {
		t.Errorf("UpdatedAt: got %v, want %v", got.UpdatedAt, afterMidnight)
	}
}

// TestPlanMissionJSON verifies the PlanMission type marshals/unmarshals.
func TestPlanMissionJSON(t *testing.T) {
	pm := PlanMission{
		Name:        "Test Plan",
		Description: "A test plan",
		Milestones: []PlanMilestone{
			{Name: "m1", Description: "Milestone 1"},
		},
		Features: []PlanFeature{
			{
				ID:          "feat-1",
				Description: "Feature 1",
				SkillName:   "backend-worker",
				Milestone:   "m1",
			},
		},
	}

	data, err := json.Marshal(pm)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got PlanMission
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Name != pm.Name {
		t.Errorf("Name: got %q, want %q", got.Name, pm.Name)
	}
	if len(got.Features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(got.Features))
	}
	if got.Features[0].ID != "feat-1" {
		t.Errorf("Feature[0].ID: got %q", got.Features[0].ID)
	}
}

func TestWorkerHandoffJSON(t *testing.T) {
	handoff := WorkerHandoff{
		SalientSummary:     "Implemented feature X",
		WhatWasImplemented: "Feature X with tests",
		WhatWasLeftUndone:  "Edge case Y",
		Verification: Verification{
			CommandsRun: []CommandRun{
				{Command: "go test", ExitCode: 0, Observation: "All tests passed"},
			},
		},
		Tests: TestInfo{
			Added: []TestFile{
				{
					File: "store_test.go",
					Cases: []TestCase{
						{Name: "TestCreate", Verifies: "Create works"},
					},
				},
			},
			Coverage: "90%",
		},
		DiscoveredIssues: []Issue{
			{Severity: "non_blocking", Description: "Minor issue"},
		},
	}

	data, err := json.Marshal(handoff)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got WorkerHandoff
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.SalientSummary != handoff.SalientSummary {
		t.Errorf("SalientSummary: got %q", got.SalientSummary)
	}
	if len(got.Verification.CommandsRun) != 1 {
		t.Errorf("expected 1 command, got %d", len(got.Verification.CommandsRun))
	}
	if len(got.Tests.Added) != 1 {
		t.Errorf("expected 1 test file, got %d", len(got.Tests.Added))
	}
	if len(got.DiscoveredIssues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(got.DiscoveredIssues))
	}
}
