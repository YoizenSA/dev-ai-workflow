package control

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ProfileStore manages named orchestrator configuration profiles.
// Profiles are stored as individual JSON files in ~/.ywai/orchestrator-profiles/.
type ProfileStore struct {
	dir string
	mu  sync.RWMutex
}

// ProfileAgentConfig defines model settings for a single agent role within a profile.
type ProfileAgentConfig struct {
	Model string `json:"model"`
}

// ProfileConfig is the orchestration configuration held by a profile.
type ProfileConfig struct {
	// Agents maps role names to their model overrides.
	Agents map[string]ProfileAgentConfig `json:"agents,omitempty"`
}

// Profile is a named, versioned orchestrator configuration.
type Profile struct {
	Name      string        `json:"name"`
	Config    ProfileConfig `json:"config"`
	IsDefault bool          `json:"is_default"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// NewProfileStore creates a store rooted at ~/.ywai/orchestrator-profiles/.
// It creates the directory if it doesn't exist and bootstraps the "default" profile.
func NewProfileStore() (*ProfileStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home dir: %w", err)
	}
	dir := filepath.Join(home, ".ywai", "orchestrator-profiles")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir profiles: %w", err)
	}
	s := &ProfileStore{dir: dir}

	// Bootstrap default profile on first run.
	if _, err := os.Stat(s.profilePath("default")); os.IsNotExist(err) {
		def := Profile{
			Name:      "default",
			Config:    ProfileConfig{}, // no model overrides
			IsDefault: true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := s.saveLocked(def); err != nil {
			return nil, fmt.Errorf("bootstrap default profile: %w", err)
		}
		// Mark active if no active file exists.
		if _, err := os.Stat(s.activePath()); os.IsNotExist(err) {
			if err := os.WriteFile(s.activePath(), []byte("default"), 0644); err != nil {
				return nil, fmt.Errorf("set default active: %w", err)
			}
		}
	}

	return s, nil
}

func (s *ProfileStore) profilePath(name string) string {
	return filepath.Join(s.dir, name+".json")
}

func (s *ProfileStore) activePath() string {
	return filepath.Join(s.dir, "active.txt")
}

// List returns all profiles sorted by name.
func (s *ProfileStore) List() ([]Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}

	var profiles []Profile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		name := e.Name()[:len(e.Name())-5]
		p, err := s.readLocked(name)
		if err != nil {
			continue // skip corrupt entries
		}
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return profiles, nil
}

// Get retrieves a single profile by name.
func (s *ProfileStore) Get(name string) (Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readLocked(name)
}

func (s *ProfileStore) readLocked(name string) (Profile, error) {
	data, err := os.ReadFile(s.profilePath(name))
	if err != nil {
		if os.IsNotExist(err) {
			return Profile{}, fmt.Errorf("profile %q not found", name)
		}
		return Profile{}, fmt.Errorf("read profile %q: %w", name, err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("parse profile %q: %w", name, err)
	}
	return p, nil
}

// Save creates or overwrites a profile.
func (s *ProfileStore) Save(p Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.UpdatedAt = time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = p.UpdatedAt
	}
	return s.saveLocked(p)
}

func (s *ProfileStore) saveLocked(p Profile) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal profile %q: %w", p.Name, err)
	}
	if err := os.WriteFile(s.profilePath(p.Name), data, 0644); err != nil {
		return fmt.Errorf("write profile %q: %w", p.Name, err)
	}
	return nil
}

// Delete removes a profile by name. The "default" profile cannot be deleted.
func (s *ProfileStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if name == "default" {
		return fmt.Errorf("cannot delete default profile")
	}
	if err := os.Remove(s.profilePath(name)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("profile %q not found", name)
		}
		return fmt.Errorf("delete profile %q: %w", name, err)
	}

	// If the active profile was deleted, fall back to "default".
	active, _ := s.getActiveLocked()
	if active == name {
		_ = os.WriteFile(s.activePath(), []byte("default"), 0644)
	}
	return nil
}

// SetActive marks a profile as the active one.
func (s *ProfileStore) SetActive(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify profile exists.
	if _, err := os.Stat(s.profilePath(name)); err != nil {
		return fmt.Errorf("profile %q not found", name)
	}
	return os.WriteFile(s.activePath(), []byte(name), 0644)
}

// GetActive returns the name of the active profile.
func (s *ProfileStore) GetActive() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getActiveLocked()
}

func (s *ProfileStore) getActiveLocked() (string, error) {
	data, err := os.ReadFile(s.activePath())
	if err != nil {
		if os.IsNotExist(err) {
			return "default", nil
		}
		return "", fmt.Errorf("read active: %w", err)
	}
	name := string(data)
	if name == "" {
		return "default", nil
	}
	// Verify the profile still exists.
	if _, err := os.Stat(s.profilePath(name)); err != nil {
		return "default", nil
	}
	return name, nil
}
