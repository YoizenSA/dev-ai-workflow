package control

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// PushSubscription represents a Web Push subscription as received from the browser.
type PushSubscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256DH string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// PushStore persists push subscriptions to a JSON file.
type PushStore struct {
	mu   sync.RWMutex
	path string
	subs []PushSubscription
}

// NewPushStore loads subscriptions from ~/.ywai/push-subscriptions.json.
func NewPushStore() (*PushStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".ywai")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "push-subscriptions.json")
	ps := &PushStore{path: path}
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &ps.subs)
	}
	if ps.subs == nil {
		ps.subs = []PushSubscription{}
	}
	return ps, nil
}

// Subscribe adds or updates a subscription.
func (ps *PushStore) Subscribe(s PushSubscription) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for i := range ps.subs {
		if ps.subs[i].Endpoint == s.Endpoint {
			ps.subs[i] = s
			return ps.save()
		}
	}
	ps.subs = append(ps.subs, s)
	return ps.save()
}

// Unsubscribe removes a subscription by endpoint URL.
func (ps *PushStore) Unsubscribe(endpoint string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	filtered := make([]PushSubscription, 0, len(ps.subs))
	for _, s := range ps.subs {
		if s.Endpoint != endpoint {
			filtered = append(filtered, s)
		}
	}
	ps.subs = filtered
	return ps.save()
}

// List returns all stored subscriptions.
func (ps *PushStore) List() []PushSubscription {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	cp := make([]PushSubscription, len(ps.subs))
	copy(cp, ps.subs)
	return cp
}

func (ps *PushStore) save() error {
	data, err := json.MarshalIndent(ps.subs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ps.path, data, 0644)
}
