package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// Skill is a ywai skill that can be bundled or custom.
type Skill struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Body        string    `json:"body"`
	Scope       string    `json:"scope"` // "bundled" or "custom"
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Store manages custom skills persisted as JSON files in ~/.ywai/skills/.
// Bundled skills are read from the skills source directory (SKILL.md files).
type Store struct {
	mu sync.RWMutex
}

// NewStore creates a Store and ensures the custom skills directory exists.
func NewStore() *Store {
	_ = os.MkdirAll(config.DataSkillsDir(), 0o755)
	return &Store{}
}

// customDir returns the directory where custom skills JSON files are stored.
func (s *Store) customDir() string {
	return config.DataSkillsDir()
}

// filePath returns the JSON file path for a custom skill.
func (s *Store) filePath(name string) string {
	return filepath.Join(s.customDir(), name+".json")
}

// List returns all skills — bundled read-only plus custom read/write.
// Bundled skills are detected from SKILL.md files in the skills source dir.
// Custom skills override bundled ones with the same name.
func (s *Store) List() ([]Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Load bundled skills.
	bundled, err := s.loadBundled()
	if err != nil {
		return nil, err
	}

	// Load custom skills.
	custom, err := s.loadCustom()
	if err != nil {
		return nil, err
	}

	// Merge: custom overrides bundled by name.
	byName := make(map[string]Skill, len(bundled)+len(custom))
	for _, sk := range bundled {
		sk.Scope = "bundled"
		byName[sk.Name] = sk
	}
	for _, sk := range custom {
		sk.Scope = "custom"
		byName[sk.Name] = sk
	}

	result := make([]Skill, 0, len(byName))
	for _, sk := range byName {
		result = append(result, sk)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

// Get returns a single skill by name, checking bundled then custom.
func (s *Store) Get(name string) (*Skill, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check custom first (overrides bundled).
	customPath := s.filePath(name)
	if data, err := os.ReadFile(customPath); err == nil {
		var sk Skill
		if err := json.Unmarshal(data, &sk); err == nil {
			sk.Scope = "custom"
			return &sk, nil
		}
	}

	// Fall back to bundled.
	sk, err := s.findBundled(name)
	if err != nil {
		return nil, fmt.Errorf("skill %q not found", name)
	}
	sk.Scope = "bundled"
	return sk, nil
}

// Create adds a new custom skill. Returns error if name already exists as custom.
func (s *Store) Create(skill Skill) error {
	if skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}
	if strings.Contains(skill.Name, "/") || strings.Contains(skill.Name, "..") {
		return fmt.Errorf("invalid skill name: %q", skill.Name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for existing custom skill.
	customPath := s.filePath(skill.Name)
	if _, err := os.Stat(customPath); err == nil {
		return fmt.Errorf("skill %q already exists", skill.Name)
	}

	now := time.Now()
	skill.Scope = "custom"
	skill.CreatedAt = now
	skill.UpdatedAt = now

	data, err := json.MarshalIndent(skill, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal skill: %w", err)
	}
	return os.WriteFile(customPath, data, 0o644)
}

// Update overwrites a custom skill. Returns error if skill doesn't exist or is bundled-only.
func (s *Store) Update(name string, skill Skill) error {
	if name == "" {
		return fmt.Errorf("skill name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	customPath := s.filePath(name)
	var existing Skill
	data, err := os.ReadFile(customPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("custom skill %q not found; cannot update bundled skills", name)
		}
		return fmt.Errorf("read skill %q: %w", name, err)
	}
	if err := json.Unmarshal(data, &existing); err != nil {
		return fmt.Errorf("parse skill %q: %w", name, err)
	}

	existing.Description = skill.Description
	existing.Body = skill.Body
	existing.UpdatedAt = time.Now()

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal skill: %w", err)
	}
	return os.WriteFile(customPath, out, 0o644)
}

// Delete removes a custom skill. Returns error if skill doesn't exist or is bundled-only.
func (s *Store) Delete(name string) error {
	if name == "" {
		return fmt.Errorf("skill name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	customPath := s.filePath(name)
	if _, err := os.Stat(customPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("custom skill %q not found; cannot delete bundled skills", name)
		}
		return err
	}
	return os.Remove(customPath)
}

// loadBundled reads Skill metadata from directories in the skills source dir
// that have a .ywai-extra marker file.
func (s *Store) loadBundled() ([]Skill, error) {
	srcDir := skillsSourceDir()
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		// No source dir means no bundled skills — not an error.
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var skills []Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		skillDir := filepath.Join(srcDir, name)
		if !hasYwaiExtraMarker(skillDir) {
			continue
		}

		sk, err := readSkillMDFile(skillDir)
		if err != nil {
			// Skip skills that don't have a SKILL.md.
			continue
		}
		if sk != nil {
			sk.Name = name
			sk.Scope = "bundled"
			skills = append(skills, *sk)
		}
	}
	return skills, nil
}

// readSkillMDFile reads the SKILL.md from a skill directory and extracts
// name and description from frontmatter if present.
func readSkillMDFile(skillDir string) (*Skill, error) {
	skillPath := filepath.Join(skillDir, "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		return nil, nil // no SKILL.md is fine
	}

	body := string(data)
	desc := extractDescription(body)
	return &Skill{Body: body, Description: desc}, nil
}

// extractDescription grabs the first non-empty, non-frontmatter line as description.
func extractDescription(body string) string {
	lines := strings.SplitN(body, "\n", 10)
	// Skip leading blank lines.
	inFrontmatter := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter {
			continue
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "---") {
			return trimmed
		}
	}
	return ""
}

// loadCustom reads all custom skill JSON files from the data skills dir.
func (s *Store) loadCustom() ([]Skill, error) {
	dir := s.customDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var skills []Skill
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var sk Skill
		if err := json.Unmarshal(data, &sk); err != nil {
			continue
		}
		sk.Scope = "custom"
		skills = append(skills, sk)
	}
	return skills, nil
}

// findBundled finds a single bundled skill by name.
func (s *Store) findBundled(name string) (*Skill, error) {
	srcDir := skillsSourceDir()
	skillDir := filepath.Join(srcDir, name)
	if _, err := os.Stat(skillDir); err != nil {
		return nil, err
	}
	if !hasYwaiExtraMarker(skillDir) {
		return nil, fmt.Errorf("not a ywai skill: %s", name)
	}
	sk, err := readSkillMDFile(skillDir)
	if err != nil || sk == nil {
		return nil, fmt.Errorf("skill %q has no SKILL.md", name)
	}
	sk.Name = name
	return sk, nil
}
