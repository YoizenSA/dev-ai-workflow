package control

import (
	"sync"
)

// TeamStore holds in-memory state for team mode.
type TeamStore struct {
	mu      sync.RWMutex
	members map[string]TeamMember
	tasks   map[string]TeamTask
	steers  map[string][]string // memberID -> steer messages
}

// NewTeamStore creates a new TeamStore.
func NewTeamStore() *TeamStore {
	return &TeamStore{
		members: make(map[string]TeamMember),
		tasks:   make(map[string]TeamTask),
		steers:  make(map[string][]string),
	}
}

func (s *TeamStore) AddMember(m TeamMember) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members[m.ID] = m
}

func (s *TeamStore) UpdateMember(id, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.members[id]; ok {
		m.Status = status
		s.members[id] = m
	}
}

func (s *TeamStore) Members() []TeamMember {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []TeamMember
	for _, m := range s.members {
		list = append(list, m)
	}
	return list
}

func (s *TeamStore) AddTask(t TeamTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[t.ID] = t
}

func (s *TeamStore) UpdateTask(id string, status, result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.tasks[id]; ok {
		t.Status = status
		t.Result = result
		s.tasks[id] = t
	}
}

func (s *TeamStore) Tasks() []TeamTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []TeamTask
	for _, t := range s.tasks {
		list = append(list, t)
	}
	return list
}

func (s *TeamStore) RecordSteer(memberID, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steers[memberID] = append(s.steers[memberID], message)
}
