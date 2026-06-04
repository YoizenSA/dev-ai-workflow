package kanban

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Store is a thread-safe in-memory store for sessions and delegations.
type Store struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	delegations map[string]*Delegation
	filePath    string
}

// NewStore creates a new Store backed by a JSON file in the given data directory.
func NewStore(dataDir string) *Store {
	s := &Store{
		sessions:    make(map[string]*Session),
		delegations: make(map[string]*Delegation),
		filePath:    filepath.Join(dataDir, "kanban-state.json"),
	}
	// Try to load existing state
	if err := s.Load(); err != nil {
		log.Printf("kanban: failed to load state: %v", err)
	}
	return s
}

var idCounter atomic.Uint64

func newID() string {
	return fmt.Sprintf("%x-%x", time.Now().UnixNano(), idCounter.Add(1))
}

// Save persists the current store state to a JSON file using atomic write.
func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.saveLocked()
}

// saveLocked is the inner save implementation; caller must hold at least an RLock.
func (s *Store) saveLocked() error {
	data := struct {
		Sessions    map[string]*Session    `json:"sessions"`
		Delegations map[string]*Delegation `json:"delegations"`
	}{
		Sessions:    s.sessions,
		Delegations: s.delegations,
	}

	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write to temp file first, then rename (atomic)
	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, pretty, 0644); err != nil {
		return err
	}

	return os.Rename(tmpFile, s.filePath)
}

// Load reads store state from the JSON file, if it exists.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // fresh start
		}
		return err
	}

	var state struct {
		Sessions    map[string]*Session    `json:"sessions"`
		Delegations map[string]*Delegation `json:"delegations"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	if state.Sessions != nil {
		s.sessions = state.Sessions
	}
	if state.Delegations != nil {
		s.delegations = state.Delegations
	}
	return nil
}

// autoSave persists state after every write operation.
// Caller must already hold at least a write lock (mu.Lock()).
func (s *Store) autoSave() {
	if s.filePath == "" {
		return
	}
	if err := s.saveLocked(); err != nil {
		log.Printf("kanban: failed to save state: %v", err)
	}
}

// --- Session operations ---

// CreateSession creates a new session with the given project and goal.
func (s *Store) CreateSession(project, goal string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.autoSave()

	session := &Session{
		ID:        newID(),
		Project:   project,
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

	result := make([]*Session, 0)
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
	defer s.autoSave()

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
	defer s.autoSave()

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
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// UpdateDelegation applies the given updates to a delegation.
// Supported fields: status, column, handoff_preview, blocker.
func (s *Store) UpdateDelegation(id string, status, column, handoffPreview, blocker *string) (*Delegation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.autoSave()

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

	for col := range columns {
		sort.Slice(columns[col], func(i, j int) bool {
			return columns[col][i].CreatedAt.After(columns[col][j].CreatedAt)
		})
	}

	return &BoardView{
		Session: session,
		Columns: columns,
	}, nil
}
