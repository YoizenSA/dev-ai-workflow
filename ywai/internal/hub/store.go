package hub

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ProjectStore is an in-memory store for projects.
type ProjectStore struct {
	mu       sync.RWMutex
	projects map[string]*Project
	paths    map[string]string // path -> id
}

// NewProjectStore creates a new empty ProjectStore.
func NewProjectStore() *ProjectStore {
	return &ProjectStore{
		projects: make(map[string]*Project),
		paths:    make(map[string]string),
	}
}

// AddProject adds a project to the store.
func (s *ProjectStore) AddProject(ctx context.Context, p Project) error {
	if p.ID == "" {
		return errors.New("project ID is required")
	}
	if p.Path == "" {
		return errors.New("project path is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.projects[p.ID]; exists {
		return fmt.Errorf("project with ID %q already exists", p.ID)
	}
	if existingID, exists := s.paths[p.Path]; exists {
		return fmt.Errorf("project with path %q already exists as %q", p.Path, existingID)
	}

	proj := new(Project)
	*proj = p
	s.projects[p.ID] = proj
	s.paths[p.Path] = p.ID
	return nil
}

// GetProject retrieves a project by ID.
func (s *ProjectStore) GetProject(ctx context.Context, id string) (*Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	p, ok := s.projects[id]
	if !ok {
		return nil, fmt.Errorf("project %q not found", id)
	}
	return p, nil
}

// ListProjects returns all projects.
func (s *ProjectStore) ListProjects(ctx context.Context) ([]Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.projects) == 0 {
		return []Project{}, nil
	}

	projects := make([]Project, 0, len(s.projects))
	for _, p := range s.projects {
		projects = append(projects, *p)
	}
	return projects, nil
}

// UpdateProject updates an existing project's mutable fields.
func (s *ProjectStore) UpdateProject(ctx context.Context, id string, p Project) (*Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.projects[id]
	if !ok {
		return nil, fmt.Errorf("project %q not found", id)
	}

	if p.Path != existing.Path {
		if existingID, exists := s.paths[p.Path]; exists {
			return nil, fmt.Errorf("project with path %q already exists as %q", p.Path, existingID)
		}
		delete(s.paths, existing.Path)
		s.paths[p.Path] = id
	}

	existing.Name = p.Name
	existing.Path = p.Path
	existing.UpdatedAt = time.Now()
	return existing, nil
}

// RemoveProject removes a project by ID. Idempotent — no error if not found.
func (s *ProjectStore) RemoveProject(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.projects[id]
	if !ok {
		return nil
	}

	delete(s.paths, p.Path)
	delete(s.projects, id)
	return nil
}
