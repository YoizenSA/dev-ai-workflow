package events

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStore implements Store using JSON Lines files (one per session).
// Events are stored in <dir>/<sessionID>.events as newline-delimited JSON.
type FileStore struct {
	dir string
	mu  sync.RWMutex
}

// NewFileStore creates a FileStore rooted at dir. The directory is created
// if it doesn't exist.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create events dir: %w", err)
	}
	return &FileStore{dir: dir}, nil
}

func (fs *FileStore) path(sessionID string) string {
	return filepath.Join(fs.dir, sessionID+".events")
}

// Append writes a single event to the log file for the event's session.
// The file is created if it doesn't exist. Thread-safety is provided by the
// mutex — concurrent appends to different sessions also block each other
// (acceptable given the low throughput of engine events).
func (fs *FileStore) Append(event Event) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	f, err := os.OpenFile(fs.path(event.SessionID), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open event file: %w", err)
	}
	defer func() { _ = f.Close() }()

	line, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	if _, err := f.Write([]byte("\n")); err != nil {
		return fmt.Errorf("write newline: %w", err)
	}
	return nil
}

// Replay returns all events for a session, ordered by sequence (file order).
// Returns an empty slice if the session has no events yet.
func (fs *FileStore) Replay(sessionID string) ([]Event, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	f, err := os.Open(fs.path(sessionID))
	if err != nil {
		if os.IsNotExist(err) {
			return []Event{}, nil
		}
		return nil, fmt.Errorf("open event file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var events []Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(scanner.Text()), &e); err != nil {
			return nil, fmt.Errorf("unmarshal event: %w", err)
		}
		events = append(events, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read event file: %w", err)
	}
	return events, nil
}

// LastSequence returns the highest sequence number for a session (0 if none).
func (fs *FileStore) LastSequence(sessionID string) (int64, error) {
	events, err := fs.Replay(sessionID)
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}
	return events[len(events)-1].Sequence, nil
}

// PendingMissions returns mission IDs that were started but not completed,
// failed, or cancelled. A mission is "pending" if its latest event is one of:
// created, started, paused, or resumed.
func (fs *FileStore) PendingMissions(sessionID string) ([]string, error) {
	events, err := fs.Replay(sessionID)
	if err != nil {
		return nil, err
	}

	// Track latest event type per mission
	type missionState struct {
		lastType EventType
	}
	states := map[string]missionState{}

	for _, e := range events {
		if e.MissionID != "" {
			states[e.MissionID] = missionState{lastType: e.Type}
		}
	}

	var pending []string
	for id, s := range states {
		switch s.lastType {
		case EventMissionCreated, EventMissionStarted, EventMissionPaused, EventMissionResumed:
			pending = append(pending, id)
		}
	}
	return pending, nil
}
