package scheduler

import (
	"errors"
	"sync"
)

// errNotFound is returned when a schedule is not found.
var errNotFound = errors.New("schedule not found")

// Store defines the persistence interface for schedules and runs.
type Store interface {
	AddSchedule(s Schedule) (Schedule, error)
	UpdateSchedule(s Schedule) error
	GetSchedule(id string) (Schedule, error)
	ListSchedules() ([]Schedule, error)
	RemoveSchedule(id string) error
	AddRun(run ScheduleRun) error
	GetRuns(scheduleID string) ([]ScheduleRun, error)
}

// MemoryStore is an in-memory implementation of Store.
type MemoryStore struct {
	mu        sync.Mutex
	schedules map[string]Schedule
	runs      map[string][]ScheduleRun
}

// NewMemoryStore creates a new MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		schedules: make(map[string]Schedule),
		runs:      make(map[string][]ScheduleRun),
	}
}

func (m *MemoryStore) AddSchedule(s Schedule) (Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.schedules[s.ID] = s
	return s, nil
}

func (m *MemoryStore) UpdateSchedule(s Schedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.schedules[s.ID]; !ok {
		return errNotFound
	}
	m.schedules[s.ID] = s
	return nil
}

func (m *MemoryStore) GetSchedule(id string) (Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.schedules[id]
	if !ok {
		return Schedule{}, errNotFound
	}
	return s, nil
}

func (m *MemoryStore) ListSchedules() ([]Schedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []Schedule
	for _, s := range m.schedules {
		result = append(result, s)
	}
	return result, nil
}

func (m *MemoryStore) RemoveSchedule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.schedules[id]; !ok {
		return errNotFound
	}
	delete(m.schedules, id)
	return nil
}

func (m *MemoryStore) AddRun(run ScheduleRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[run.ScheduleID] = append(m.runs[run.ScheduleID], run)
	return nil
}

func (m *MemoryStore) GetRuns(scheduleID string) ([]ScheduleRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs[scheduleID], nil
}
