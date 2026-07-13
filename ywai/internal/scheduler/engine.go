package scheduler

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"sync/atomic"
	"time"
)

// timeNow is overridden in tests for deterministic time.
var timeNow = time.Now

// Scheduler manages scheduled tasks.
type Scheduler struct {
	store Store
}

// NewScheduler creates a new Scheduler with the given store.
func NewScheduler(store Store) *Scheduler {
	if store == nil {
		panic("scheduler: nil store")
	}
	return &Scheduler{store: store}
}

// AddSchedule registers a new schedule and returns it with an assigned ID.
func (s *Scheduler) AddSchedule(sched Schedule) (Schedule, error) {
	sched.ID = newID()
	d, err := ParseCron(sched.CronExpr)
	if err != nil {
		return Schedule{}, err
	}
	sched.NextRun = timeNow().Add(d)
	if sched.CreatedAt.IsZero() {
		sched.CreatedAt = timeNow()
	}
	return s.store.AddSchedule(sched)
}

// ListSchedules returns all registered schedules.
func (s *Scheduler) ListSchedules() ([]Schedule, error) {
	return s.store.ListSchedules()
}

// RemoveSchedule removes a schedule by ID.
func (s *Scheduler) RemoveSchedule(id string) error {
	if _, err := s.store.GetSchedule(id); err != nil {
		return errNotFound
	}
	return s.store.RemoveSchedule(id)
}

// GetRuns returns all runs for a given schedule ID.
func (s *Scheduler) GetRuns(scheduleID string) ([]ScheduleRun, error) {
	return s.store.GetRuns(scheduleID)
}

// Tick checks for due schedules and executes them.
// Returns the IDs of schedules that were executed.
func (s *Scheduler) Tick() ([]string, error) {
	schedules, err := s.store.ListSchedules()
	if err != nil {
		return nil, err
	}

	var ran []string
	now := timeNow()

	for _, sch := range schedules {
		if !sch.Enabled {
			continue
		}
		if sch.NextRun.IsZero() {
			continue
		}
		if !now.Before(sch.NextRun) {
			// Execute this schedule
			run := ScheduleRun{
				ID:         newID(),
				ScheduleID: sch.ID,
				StartedAt:  now,
				Status:     "running",
			}
			if err := s.store.AddRun(run); err != nil {
				return nil, err
			}

			// Update NextRun
			d, err := ParseCron(sch.CronExpr)
			if err != nil {
				return nil, err
			}
			sch.NextRun = now.Add(d)
			if err := s.store.UpdateSchedule(sch); err != nil {
				return nil, err
			}

			ran = append(ran, sch.ID)
		}
	}

	return ran, nil
}

// ParseCron parses a cron expression and returns the duration between runs.
func ParseCron(expr string) (time.Duration, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, fmt.Errorf("empty cron expression")
	}

	switch {
	case strings.HasPrefix(expr, "@every "):
		return time.ParseDuration(expr[len("@every "):])
	case expr == "@daily":
		return 24 * time.Hour, nil
	case expr == "@hourly":
		return time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported cron expression: %s", expr)
	}
}

// newID generates a unique short ID.
func newID() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		// Fallback: use atomic counter
		v := atomic.AddUintptr(&idCounter, 1)
		return fmt.Sprintf("sched-%d", v)
	}
	return fmt.Sprintf("%x", n.Int64())
}

var idCounter uintptr
