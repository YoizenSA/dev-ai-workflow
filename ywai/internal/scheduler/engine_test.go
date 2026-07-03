package scheduler

import (
	"testing"
	"time"
)

// fakeStore implements Store for testing the engine.
type fakeStore struct {
	schedules map[string]Schedule
	runs      map[string][]ScheduleRun
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		schedules: make(map[string]Schedule),
		runs:      make(map[string][]ScheduleRun),
	}
}

func (f *fakeStore) AddSchedule(s Schedule) (Schedule, error) {
	id := "test-" + s.CronExpr // deterministic for tests
	s.ID = id
	f.schedules[id] = s
	return s, nil
}

func (f *fakeStore) UpdateSchedule(s Schedule) error {
	f.schedules[s.ID] = s
	return nil
}

func (f *fakeStore) RemoveSchedule(id string) error {
	delete(f.schedules, id)
	return nil
}

func (f *fakeStore) GetSchedule(id string) (Schedule, error) {
	s, ok := f.schedules[id]
	if !ok {
		return Schedule{}, errNotFound
	}
	return s, nil
}

func (f *fakeStore) ListSchedules() ([]Schedule, error) {
	var result []Schedule
	for _, s := range f.schedules {
		result = append(result, s)
	}
	return result, nil
}

func (f *fakeStore) AddRun(run ScheduleRun) error {
	f.runs[run.ScheduleID] = append(f.runs[run.ScheduleID], run)
	return nil
}

func (f *fakeStore) GetRuns(scheduleID string) ([]ScheduleRun, error) {
	return f.runs[scheduleID], nil
}

func TestNewScheduler_NilStorePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewScheduler(nil) should panic")
		}
	}()
	NewScheduler(nil)
}

func TestNewScheduler_Creates(t *testing.T) {
	s := NewScheduler(newFakeStore())
	if s == nil {
		t.Fatal("NewScheduler returned nil")
	}
}

func TestAddSchedule_ReturnsWithID(t *testing.T) {
	s := NewScheduler(newFakeStore())
	sched := Schedule{
		Config: SessionConfig{
			Goal:  "Test session",
			Repo:  "github.com/org/test",
			Agent: "dev",
		},
		CronExpr: "@every 1h",
		Enabled:  true,
	}
	created, err := s.AddSchedule(sched)
	if err != nil {
		t.Fatalf("AddSchedule failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("AddSchedule returned schedule with empty ID")
	}
	if created.CronExpr != "@every 1h" {
		t.Errorf("CronExpr = %q, want %q", created.CronExpr, "@every 1h")
	}
	if !created.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestAddSchedule_SetsNextRun(t *testing.T) {
	s := NewScheduler(newFakeStore())
	sched := Schedule{
		Config: SessionConfig{
			Goal:  "Daily standup",
			Repo:  "github.com/org/team",
			Agent: "ask",
		},
		CronExpr: "@daily",
		Enabled:  true,
	}
	created, err := s.AddSchedule(sched)
	if err != nil {
		t.Fatalf("AddSchedule failed: %v", err)
	}
	if created.NextRun.IsZero() {
		t.Error("AddSchedule should set NextRun based on CronExpr")
	}
	if created.NextRun.Before(time.Now()) {
		t.Errorf("NextRun (%v) should be in the future", created.NextRun)
	}
}

func TestListSchedules_Empty(t *testing.T) {
	s := NewScheduler(newFakeStore())
	schedules, err := s.ListSchedules()
	if err != nil {
		t.Fatalf("ListSchedules failed: %v", err)
	}
	if len(schedules) != 0 {
		t.Errorf("expected empty list, got %d items", len(schedules))
	}
}

func TestListSchedules_AfterAdd(t *testing.T) {
	store := newFakeStore()
	sch := NewScheduler(store)
	sched := Schedule{
		Config:   SessionConfig{Goal: "One", Repo: "org/a", Agent: "dev"},
		CronExpr: "@every 1h",
		Enabled:  true,
	}
	_, err := sch.AddSchedule(sched)
	if err != nil {
		t.Fatalf("AddSchedule failed: %v", err)
	}

	// Also add via engine
	sched2 := Schedule{
		Config:   SessionConfig{Goal: "Two", Repo: "org/b", Agent: "qa"},
		CronExpr: "@daily",
		Enabled:  true,
	}
	_, err = sch.AddSchedule(sched2)
	if err != nil {
		t.Fatalf("AddSchedule failed: %v", err)
	}

	schedules, err := sch.ListSchedules()
	if err != nil {
		t.Fatalf("ListSchedules failed: %v", err)
	}
	if len(schedules) != 2 {
		t.Errorf("expected 2 schedules, got %d", len(schedules))
	}
}

func TestRemoveSchedule_RemovesExisting(t *testing.T) {
	s := NewScheduler(newFakeStore())
	created, _ := s.AddSchedule(Schedule{
		Config:   SessionConfig{Goal: "Remove me", Repo: "org/x", Agent: "dev"},
		CronExpr: "@every 1h",
		Enabled:  true,
	})
	err := s.RemoveSchedule(created.ID)
	if err != nil {
		t.Fatalf("RemoveSchedule failed: %v", err)
	}
	schedules, _ := s.ListSchedules()
	for _, sch := range schedules {
		if sch.ID == created.ID {
			t.Errorf("schedule %q should have been removed", created.ID)
		}
	}
}

func TestRemoveSchedule_NonExistentReturnsError(t *testing.T) {
	s := NewScheduler(newFakeStore())
	err := s.RemoveSchedule("nonexistent-id")
	if err == nil {
		t.Error("RemoveSchedule with non-existent ID should return error")
	}
}

func TestTick_NoDueReturnsEmpty(t *testing.T) {
	s := NewScheduler(newFakeStore())
	// Add a schedule that's not due yet (NextRun far in the future)
	_, _ = s.AddSchedule(Schedule{
		Config:   SessionConfig{Goal: "Future", Repo: "org/f", Agent: "dev"},
		CronExpr: "@every 24h",
		Enabled:  true,
		// NextRun is zero, Tick should not fire it
	})
	ran, err := s.Tick()
	if err != nil {
		t.Fatalf("Tick failed: %v", err)
	}
	if len(ran) != 0 {
		t.Errorf("expected 0 runs, got %d: %v", len(ran), ran)
	}
}

func TestTick_RunsDueSchedules(t *testing.T) {
	store := newFakeStore()
	sch := NewScheduler(store)
	// Add a schedule with NextRun in the past
	past := time.Now().Add(-5 * time.Minute)
	created, _ := sch.AddSchedule(Schedule{
		Config:   SessionConfig{Goal: "Due session", Repo: "org/d", Agent: "dev"},
		CronExpr: "@every 1h",
		Enabled:  true,
	})
	// Manually set NextRun to the past via store
	stored, _ := store.GetSchedule(created.ID)
	stored.NextRun = past
	_ = store.UpdateSchedule(stored)

	ran, err := sch.Tick()
	if err != nil {
		t.Fatalf("Tick failed: %v", err)
	}
	if len(ran) != 1 {
		t.Fatalf("expected 1 run, got %d: %v", len(ran), ran)
	}
	if ran[0] != created.ID {
		t.Errorf("run schedule ID = %q, want %q", ran[0], created.ID)
	}
}

func TestTick_UpdatesNextRun(t *testing.T) {
	store := newFakeStore()
	sch := NewScheduler(store)
	past := time.Now().Add(-5 * time.Minute)

	created, _ := sch.AddSchedule(Schedule{
		Config:   SessionConfig{Goal: "Recurring", Repo: "org/r", Agent: "qa"},
		CronExpr: "@every 1h",
		Enabled:  true,
	})
	stored, _ := store.GetSchedule(created.ID)
	stored.NextRun = past
	_ = store.UpdateSchedule(stored)

	_, _ = sch.Tick()

	updated, _ := store.GetSchedule(created.ID)
	if updated.NextRun.Before(time.Now()) {
		t.Error("Tick should update NextRun to a future time after processing")
	}
}

func TestTick_SkipsDisabledSchedules(t *testing.T) {
	store := newFakeStore()
	sch := NewScheduler(store)
	past := time.Now().Add(-5 * time.Minute)

	created, _ := sch.AddSchedule(Schedule{
		Config:   SessionConfig{Goal: "Disabled", Repo: "org/d", Agent: "dev"},
		CronExpr: "@every 1h",
		Enabled:  false,
	})
	stored, _ := store.GetSchedule(created.ID)
	stored.NextRun = past
	_ = store.UpdateSchedule(stored)

	ran, _ := sch.Tick()
	if len(ran) != 0 {
		t.Errorf("Tick should skip disabled schedules, got %d runs", len(ran))
	}
}

func TestParseCron_Every1h(t *testing.T) {
	d, err := ParseCron("@every 1h")
	if err != nil {
		t.Fatalf("ParseCron(@every 1h) failed: %v", err)
	}
	if d != time.Hour {
		t.Errorf("duration = %v, want %v", d, time.Hour)
	}
}

func TestParseCron_Every30m(t *testing.T) {
	d, err := ParseCron("@every 30m")
	if err != nil {
		t.Fatalf("ParseCron(@every 30m) failed: %v", err)
	}
	if d != 30*time.Minute {
		t.Errorf("duration = %v, want %v", d, 30*time.Minute)
	}
}

func TestParseCron_Daily(t *testing.T) {
	d, err := ParseCron("@daily")
	if err != nil {
		t.Fatalf("ParseCron(@daily) failed: %v", err)
	}
	if d != 24*time.Hour {
		t.Errorf("duration = %v, want 24h", d)
	}
}

func TestParseCron_Hourly(t *testing.T) {
	d, err := ParseCron("@hourly")
	if err != nil {
		t.Fatalf("ParseCron(@hourly) failed: %v", err)
	}
	if d != time.Hour {
		t.Errorf("duration = %v, want 1h", d)
	}
}

func TestParseCron_InvalidReturnsError(t *testing.T) {
	_, err := ParseCron("not-a-cron")
	if err == nil {
		t.Error("ParseCron with invalid expression should return error")
	}
}

func TestParseCron_EmptyReturnsError(t *testing.T) {
	_, err := ParseCron("")
	if err == nil {
		t.Error("ParseCron with empty string should return error")
	}
}
