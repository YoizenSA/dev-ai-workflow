package missions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Project represents a registered project in Mission Control.
type Project struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ProjectStore persists projects to a JSON file.
type ProjectStore struct {
	mu       sync.RWMutex
	baseDir  string
	projects map[string]*Project // keyed by name
}

// NewProjectStore creates a new ProjectStore rooted at baseDir.
func NewProjectStore(baseDir string) (*ProjectStore, error) {
	store := &ProjectStore{
		baseDir:  baseDir,
		projects: make(map[string]*Project),
	}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *ProjectStore) path() string {
	return filepath.Join(s.baseDir, "projects.json")
}

func (s *ProjectStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &s.projects)
}

func (s *ProjectStore) save() error {
	data, err := json.MarshalIndent(s.projects, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(), data, 0644)
}

// List returns all registered projects.
func (s *ProjectStore) List() []*Project {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Project, 0, len(s.projects))
	for _, p := range s.projects {
		result = append(result, p)
	}
	return result
}

// Get returns a project by name.
func (s *ProjectStore) Get(name string) (*Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.projects[name]
	if !ok {
		return nil, fmt.Errorf("project %q not found", name)
	}
	return p, nil
}

// Create registers a new project.
func (s *ProjectStore) Create(name, path, description string) (*Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[name]; ok {
		return nil, fmt.Errorf("project %q already exists", name)
	}

	now := time.Now()
	p := &Project{
		Name:        name,
		Path:        path,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.projects[name] = p

	if err := s.save(); err != nil {
		delete(s.projects, name)
		return nil, err
	}

	return p, nil
}

// Delete removes a project by name.
func (s *ProjectStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.projects[name]; !ok {
		return fmt.Errorf("project %q not found", name)
	}

	delete(s.projects, name)
	return s.save()
}
