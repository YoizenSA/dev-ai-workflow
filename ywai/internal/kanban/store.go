package kanban

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// Store is a thread-safe in-memory store for sessions and delegations.
type Store struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	delegations map[string]*Delegation
}

// NewStore creates a new empty Store.
func NewStore() *Store {
	return &Store{
		sessions:    make(map[string]*Session),
		delegations: make(map[string]*Delegation),
	}
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// --- Session operations ---

// CreateSession creates a new session with the given goal.
func (s *Store) CreateSession(goal string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := &Session{
		ID:        newID(),
		Goal:      goal,
		Status:    "active",
		CreatedAt: time.Now(),
	}
	s.sessions[session.ID] = session
	return session
}

// GetSession returns a session by ID.
func (s *Store) GetSession(id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[id]
	return session, ok
}

// ListSessions returns all sessions, optionally filtered by status.
func (s *Store) ListSessions(status string) []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Session
	for _, session := range s.sessions {
		if status == "" || session.Status == status {
			result = append(result, session)
		}
	}
	return result
}

// CloseSession marks a session as closed.
func (s *Store) CloseSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		return fmt.Errorf("session %s not found", id)
	}
	session.Status = "closed"
	return nil
}

// --- Delegation operations ---

// CreateDelegation creates a new delegation in the specified session.
func (s *Store) CreateDelegation(sessionID, agent, taskSummary string, deps []string) (*Delegation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[sessionID]; !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	if deps == nil {
		deps = []string{}
	}

	d := &Delegation{
		ID:           newID(),
		SessionID:    sessionID,
		Agent:        agent,
		TaskSummary:  taskSummary,
		Status:       "pending",
		Column:       "backlog",
		Dependencies: deps,
		CreatedAt:    time.Now(),
	}
	s.delegations[d.ID] = d
	return d, nil
}

// GetDelegation returns a delegation by ID.
func (s *Store) GetDelegation(id string) (*Delegation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.delegations[id]
	return d, ok
}

// ListDelegations returns all delegations for a given session.
func (s *Store) ListDelegations(sessionID string) []*Delegation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Delegation
	for _, d := range s.delegations {
		if d.SessionID == sessionID {
			result = append(result, d)
		}
	}
	return result
}

// UpdateDelegation applies the given updates to a delegation.
// Supported fields: status, column, handoff_preview, blocker.
func (s *Store) UpdateDelegation(id string, status, column, handoffPreview, blocker *string) (*Delegation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.delegations[id]
	if !ok {
		return nil, fmt.Errorf("delegation %s not found", id)
	}

	now := time.Now()

	if status != nil {
		d.Status = *status
		if *status == "running" && d.StartedAt == nil {
			d.StartedAt = &now
		}
		if *status == "done" && d.CompletedAt == nil {
			d.CompletedAt = &now
		}
	}
	if column != nil {
		d.Column = *column
	}
	if handoffPreview != nil {
		d.HandoffPreview = *handoffPreview
	}
	if blocker != nil {
		d.Blocker = *blocker
	}

	return d, nil
}

// BoardView returns the board view for a session: delegations grouped by column.
func (s *Store) BoardView(sessionID string) (*BoardView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	columns := map[string][]Delegation{
		"backlog":     {},
		"ready":       {},
		"in_progress": {},
		"review":      {},
		"done":        {},
	}

	for _, d := range s.delegations {
		if d.SessionID == sessionID {
			col := d.Column
			if _, ok := columns[col]; !ok {
				col = "backlog"
			}
			columns[col] = append(columns[col], *d)
		}
	}

	return &BoardView{
		Session: session,
		Columns: columns,
	}, nil
}
