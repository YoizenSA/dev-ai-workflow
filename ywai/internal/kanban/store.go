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
	activities  map[string][]ActivityEvent // keyed by delegationID
	filePath    string
}

// NewStore creates a new Store backed by a JSON file in the given data directory.
func NewStore(dataDir string) *Store {
	s := &Store{
		sessions:    make(map[string]*Session),
		delegations: make(map[string]*Delegation),
		activities:  make(map[string][]ActivityEvent),
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
		Sessions    map[string]*Session             `json:"sessions"`
		Delegations map[string]*Delegation          `json:"delegations"`
		Activities  map[string][]ActivityEvent      `json:"activities"`
	}{
		Sessions:    s.sessions,
		Delegations: s.delegations,
		Activities:  s.activities,
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
		Sessions    map[string]*Session        `json:"sessions"`
		Delegations map[string]*Delegation     `json:"delegations"`
		Activities  map[string][]ActivityEvent `json:"activities"`
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
	if state.Activities != nil {
		s.activities = state.Activities
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

// DeleteSession removes a session and all its delegations from the store.
func (s *Store) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.autoSave()

	if _, ok := s.sessions[id]; !ok {
		return fmt.Errorf("session %s not found", id)
	}

	// Remove all delegations belonging to this session.
	for dID, d := range s.delegations {
		if d.SessionID == id {
			delete(s.delegations, dID)
			delete(s.activities, dID)
		}
	}

	delete(s.sessions, id)
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

// DelegationExists returns true if a delegation with the given ID exists.
func (s *Store) DelegationExists(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.delegations[id]
	return ok
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

// --- Activity operations ---

// AddActivity appends a new activity event to the given delegation.
func (s *Store) AddActivity(delegationID string, activity *ActivityEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.autoSave()

	// Validate activity type.
	switch activity.Type {
	case ActivityProgress, ActivityDecision, ActivityQuestion, ActivityBlocked:
		// valid
	default:
		return fmt.Errorf("invalid activity type: %s", activity.Type)
	}

	d, ok := s.delegations[delegationID]
	if !ok {
		return fmt.Errorf("delegation %s not found", delegationID)
	}

	activity.ID = newID()
	activity.DelegationID = delegationID
	activity.CreatedAt = time.Now()
	if activity.Options == nil {
		activity.Options = []string{}
	}

	s.activities[delegationID] = append(s.activities[delegationID], *activity)

	// Update delegation's latest activity and pending action flag
	d.LatestActivity = string(activity.Type)
	if activity.Type == ActivityDecision || activity.Type == ActivityQuestion || activity.Type == ActivityBlocked {
		d.PendingAction = true
	}

	return nil
}

// GetActivities returns all activity events for a delegation in chronological order.
func (s *Store) GetActivities(delegationID string) ([]ActivityEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	activities, ok := s.activities[delegationID]
	if !ok {
		return []ActivityEvent{}, nil
	}
	return activities, nil
}

// ResolveActivity sets the resolution on a pending activity event.
func (s *Store) ResolveActivity(delegationID string, activityID string, resolution string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.autoSave()

	activities, ok := s.activities[delegationID]
	if !ok {
		return fmt.Errorf("no activities found for delegation %s", delegationID)
	}

	found := false
	for i := range activities {
		if activities[i].ID == activityID {
			if activities[i].Resolution != "" {
				return fmt.Errorf("activity %s already resolved", activityID)
			}
			now := time.Now()
			activities[i].Resolution = resolution
			activities[i].ResolvedAt = &now
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("activity %s not found in delegation %s", activityID, delegationID)
	}

	s.activities[delegationID] = activities

	// Check if there are still pending activities for this delegation
	if d, ok := s.delegations[delegationID]; ok {
		hasPending := false
		for _, a := range activities {
			if a.ResolvedAt == nil && (a.Type == ActivityDecision || a.Type == ActivityQuestion || a.Type == ActivityBlocked) {
				hasPending = true
				break
			}
		}
		d.PendingAction = hasPending
	}

	return nil
}

// GetPendingDecisions returns unresolved decision, question, or blocked activities for a session.
func (s *Store) GetPendingDecisions(sessionID string) ([]ActivityEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var pending []ActivityEvent

	for _, d := range s.delegations {
		if d.SessionID != sessionID {
			continue
		}
		activities, ok := s.activities[d.ID]
		if !ok {
			continue
		}
		for _, a := range activities {
			if a.ResolvedAt != nil {
				continue
			}
			if a.Type == ActivityDecision || a.Type == ActivityQuestion || a.Type == ActivityBlocked {
				pending = append(pending, a)
			}
		}
	}

	return pending, nil
}
