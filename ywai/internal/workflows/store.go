package workflows

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"
)

// Errors returned by the store.
var (
	ErrWorkflowNotFound = errors.New("workflow not found")
	ErrWorkflowExists   = errors.New("workflow already exists")
	ErrInvalidName      = errors.New("invalid workflow name")
)

// namePattern mirrors cc-wf-studio: lowercase letters, digits, hyphen,
// underscore only, 1–100 chars. The name is used directly as a filename and as
// the opencode command/agent slug, so it must be filesystem-safe.
var namePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// ValidateName checks a workflow name against the naming rules.
func ValidateName(name string) error {
	if len(name) == 0 || len(name) > 100 {
		return fmt.Errorf("%w: must be 1–100 chars", ErrInvalidName)
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("%w: only lowercase letters, digits, '-' and '_' allowed", ErrInvalidName)
	}
	return nil
}

// Store is a thread-safe JSON-on-disk store for workflows. Each workflow lives
// as a single <name>.json file under the base directory. This mirrors the
// missions store's persistence pattern (atomic temp-file + rename).
type Store struct {
	mu      sync.RWMutex
	baseDir string
}

// NewStore creates a store rooted at baseDir. The directory is created lazily
// on the first write.
func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

// BaseDir returns the root directory for workflow JSON files.
func (s *Store) BaseDir() string {
	return s.baseDir
}

// List returns metadata for every persisted workflow, sorted by name. Only the
// top-level fields are read; node graphs are skipped to keep listings light.
func (s *Store) List() ([]Summary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read workflows dir: %w", err)
	}

	var out []Summary
	for _, e := range entries {
		if e.IsDir() || !hasJSONSuffix(e.Name()) {
			continue
		}
		path := filepath.Join(s.baseDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // skip unreadable files rather than failing the whole list
		}
		var wf Workflow
		if err := json.Unmarshal(data, &wf); err != nil {
			continue // skip corrupt files
		}
		out = append(out, Summary{
			Name:        wf.Name,
			Description: wf.Description,
			Version:     wf.Version,
			NodeCount:   len(wf.Nodes),
			UpdatedAt:   wf.UpdatedAt,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Summary is the lightweight metadata returned by List.
type Summary struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	NodeCount   int       `json:"nodeCount"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Load reads a single workflow by name.
func (s *Store) Load(name string) (*Workflow, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.readLocked(name)
}

// Create persists a new workflow. Fails if one with the same name exists.
// ID and timestamps are set if empty.
func (s *Store) Create(wf *Workflow) error {
	if wf == nil {
		return errors.New("nil workflow")
	}
	if err := ValidateName(wf.Name); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Reject if it already exists.
	path := s.pathLocked(wf.Name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%w: %q", ErrWorkflowExists, wf.Name)
	}

	return s.writeLocked(normalize(wf))
}

// Save overwrites an existing workflow. Use this for updates from the editor.
func (s *Store) Save(wf *Workflow) error {
	if wf == nil {
		return errors.New("nil workflow")
	}
	if err := ValidateName(wf.Name); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.writeLocked(normalize(wf))
}

// Delete removes a workflow JSON file. Missing files are not an error.
func (s *Store) Delete(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.pathLocked(name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete workflow %q: %w", name, err)
	}
	return nil
}

// Rename moves a workflow to a new name, failing if the target exists.
func (s *Store) Rename(oldName, newName string) error {
	if err := ValidateName(newName); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	oldPath := s.pathLocked(oldName)
	newPath := s.pathLocked(newName)
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("%w: %q", ErrWorkflowExists, newName)
	}
	if _, err := os.Stat(oldPath); err != nil {
		return fmt.Errorf("%w: %q", ErrWorkflowNotFound, oldName)
	}

	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return fmt.Errorf("create workflows dir: %w", err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("rename workflow: %w", err)
	}

	// Patch the stored name + timestamp to match the new filename.
	wf, err := s.readLocked(newName)
	if err != nil {
		return err
	}
	wf.Name = newName
	wf.UpdatedAt = time.Now().UTC()
	return s.writeLocked(wf)
}

// ─── internals ─────────────────────────────────────────────────────────────

func (s *Store) pathLocked(name string) string {
	return filepath.Join(s.baseDir, name+".json")
}

func (s *Store) readLocked(name string) (*Workflow, error) {
	data, err := os.ReadFile(s.pathLocked(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %q", ErrWorkflowNotFound, name)
		}
		return nil, fmt.Errorf("read workflow %q: %w", name, err)
	}
	var wf Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parse workflow %q: %w", name, err)
	}
	return &wf, nil
}

func (s *Store) writeLocked(wf *Workflow) error {
	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return fmt.Errorf("create workflows dir: %w", err)
	}
	pretty, err := json.MarshalIndent(wf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workflow: %w", err)
	}
	return atomicWrite(s.pathLocked(wf.Name), pretty)
}

// normalize fills in ID and timestamps before persisting.
func normalize(wf *Workflow) *Workflow {
	if wf.ID == "" {
		wf.ID = wf.Name
	}
	now := time.Now().UTC()
	if wf.CreatedAt.IsZero() {
		wf.CreatedAt = now
	}
	wf.UpdatedAt = now
	return wf
}

// atomicWrite writes data to a temp file then renames it atomically (POSIX
// rename guarantee): a crash mid-write leaves the original intact.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmpFile := filepath.Join(dir, "."+filepath.Base(path)+".tmp")
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

func hasJSONSuffix(name string) bool {
	return len(name) > 5 && name[len(name)-5:] == ".json"
}
