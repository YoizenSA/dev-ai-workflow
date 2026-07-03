package scheduler

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSessionConfig_Fields(t *testing.T) {
	cfg := SessionConfig{
		Goal:  "Fix login bug",
		Repo:  "github.com/org/repo",
		Agent: "dev",
	}
	if cfg.Goal != "Fix login bug" {
		t.Errorf("Goal = %q, want %q", cfg.Goal, "Fix login bug")
	}
	if cfg.Repo != "github.com/org/repo" {
		t.Errorf("Repo = %q, want %q", cfg.Repo, "github.com/org/repo")
	}
	if cfg.Agent != "dev" {
		t.Errorf("Agent = %q, want %q", cfg.Agent, "dev")
	}
}

func TestSessionConfig_JSONRoundtrip(t *testing.T) {
	orig := SessionConfig{
		Goal:  "Refactor auth",
		Repo:  "github.com/org/auth",
		Agent: "architect",
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var restored SessionConfig
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if restored != orig {
		t.Errorf("Roundtrip mismatch: got %+v, want %+v", restored, orig)
	}
}

func TestSchedule_Defaults(t *testing.T) {
	s := Schedule{}
	if s.ID != "" {
		t.Errorf("default ID should be empty, got %q", s.ID)
	}
	if s.Enabled {
		t.Error("default Enabled should be false")
	}
	if !s.CreatedAt.IsZero() {
		t.Error("default CreatedAt should be zero")
	}
}

func TestSchedule_JSONRoundtrip(t *testing.T) {
	now := time.Now().Truncate(time.Second).UTC()
	orig := Schedule{
		ID: "sched-1",
		Config: SessionConfig{
			Goal:  "Run tests",
			Repo:  "github.com/org/app",
			Agent: "qa",
		},
		CronExpr:  "@every 30m",
		NextRun:   now,
		Enabled:   true,
		CreatedAt: now,
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var restored Schedule
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if restored.ID != orig.ID {
		t.Errorf("ID: got %q, want %q", restored.ID, orig.ID)
	}
	if restored.Config != orig.Config {
		t.Errorf("Config: got %+v, want %+v", restored.Config, orig.Config)
	}
	if restored.CronExpr != orig.CronExpr {
		t.Errorf("CronExpr: got %q, want %q", restored.CronExpr, orig.CronExpr)
	}
	if !restored.NextRun.Equal(orig.NextRun) {
		t.Errorf("NextRun: got %v, want %v", restored.NextRun, orig.NextRun)
	}
	if restored.Enabled != orig.Enabled {
		t.Errorf("Enabled: got %v, want %v", restored.Enabled, orig.Enabled)
	}
	if !restored.CreatedAt.Equal(orig.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", restored.CreatedAt, orig.CreatedAt)
	}
}

func TestSchedule_JSONOmitEmpty(t *testing.T) {
	s := Schedule{ID: "sched-2", Config: SessionConfig{Goal: "test"}}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}
	// Zero-value NextRun and CreatedAt should be omitted if tagged omitempty
	if _, ok := raw["nextRun"]; ok {
		t.Error("zero NextRun should be omitted from JSON due to omitempty tag")
	}
}

func TestScheduleRun_Fields(t *testing.T) {
	now := time.Now().Truncate(time.Second).UTC()
	run := ScheduleRun{
		ID:         "run-1",
		ScheduleID: "sched-1",
		StartedAt:  now,
		Status:     "running",
	}
	if run.ID != "run-1" {
		t.Errorf("ID = %q, want %q", run.ID, "run-1")
	}
	if run.ScheduleID != "sched-1" {
		t.Errorf("ScheduleID = %q, want %q", run.ScheduleID, "sched-1")
	}
	if !run.StartedAt.Equal(now) {
		t.Errorf("StartedAt = %v, want %v", run.StartedAt, now)
	}
	if run.Status != "running" {
		t.Errorf("Status = %q, want %q", run.Status, "running")
	}
}

func TestScheduleRun_JSONRoundtrip(t *testing.T) {
	now := time.Now().Truncate(time.Second).UTC()
	orig := ScheduleRun{
		ID:         "run-2",
		ScheduleID: "sched-1",
		StartedAt:  now,
		FinishedAt: now.Add(30 * time.Minute),
		Status:     "completed",
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var restored ScheduleRun
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if restored.ID != orig.ID {
		t.Errorf("ID: got %q, want %q", restored.ID, orig.ID)
	}
	if restored.ScheduleID != orig.ScheduleID {
		t.Errorf("ScheduleID: got %q, want %q", restored.ScheduleID, orig.ScheduleID)
	}
	if !restored.StartedAt.Equal(orig.StartedAt) {
		t.Errorf("StartedAt: got %v, want %v", restored.StartedAt, orig.StartedAt)
	}
	if !restored.FinishedAt.Equal(orig.FinishedAt) {
		t.Errorf("FinishedAt: got %v, want %v", restored.FinishedAt, orig.FinishedAt)
	}
	if restored.Status != orig.Status {
		t.Errorf("Status: got %q, want %q", restored.Status, orig.Status)
	}
}
