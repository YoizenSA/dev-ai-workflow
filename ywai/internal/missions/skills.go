package missions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ─── Errors ────────────────────────────────────────────────────────────────

var (
	ErrSkillNotFound    = fmt.Errorf("skill not found")
	ErrInvalidSkillFile = fmt.Errorf("invalid skill file")
)

// ─── Skill Loader ─────────────────────────────────────────────────────────

// SkillLoader loads worker skills from missionDir/skills/{worker-type}/SKILL.md
type SkillLoader struct {
	missionDir string
}

// NewSkillLoader creates a new SkillLoader for the given mission directory.
func NewSkillLoader(missionDir string) *SkillLoader {
	return &SkillLoader{missionDir: missionDir}
}

// LoadSkill loads a skill by worker type name.
// It reads {missionDir}/skills/{workerType}/SKILL.md and parses it.
func (sl *SkillLoader) LoadSkill(workerType string) (*Skill, error) {
	skillPath := filepath.Join(sl.missionDir, "skills", workerType, "SKILL.md")

	content, err := os.ReadFile(skillPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrSkillNotFound, skillPath)
		}
		return nil, fmt.Errorf("read skill file: %w", err)
	}

	return sl.parseSkill(content)
}

// parseSkill parses SKILL.md content into a Skill struct.
func (sl *SkillLoader) parseSkill(content []byte) (*Skill, error) {
	lines := strings.Split(string(content), "\n")

	skill := &Skill{}
	var section string
	var procedureBuilder strings.Builder
	var handoffBuilder strings.Builder
	var returnBuilder strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Parse YAML frontmatter
		if strings.HasPrefix(trimmed, "---") {
			continue
		}
		if strings.HasPrefix(trimmed, "name:") {
			skill.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			continue
		}
		if strings.HasPrefix(trimmed, "description:") {
			skill.Description = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			continue
		}

		// Parse sections
		if strings.HasPrefix(trimmed, "## Required Skills and Tools") {
			section = "requirements"
			continue
		}
		if strings.HasPrefix(trimmed, "## Work Procedure") {
			section = "procedure"
			continue
		}
		if strings.HasPrefix(trimmed, "## Example Handoff") {
			section = "handoff"
			continue
		}
		if strings.HasPrefix(trimmed, "## When to Return to Orchestrator") {
			section = "return"
			continue
		}

		// Collect content based on section
		switch section {
		case "requirements":
			// Parse skills and tools from this section
			if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") {
				item := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, "-"), "*"))
				if item != "" {
					// Simple heuristic: if it contains "skill" it's a skill, otherwise tool
					if strings.Contains(strings.ToLower(item), "skill") {
						skill.RequiredSkills = append(skill.RequiredSkills, item)
					} else {
						skill.RequiredTools = append(skill.RequiredTools, item)
					}
				}
			}
		case "procedure":
			if trimmed != "" && !strings.HasPrefix(trimmed, "##") {
				procedureBuilder.WriteString(line + "\n")
			}
		case "handoff":
			if trimmed != "" && !strings.HasPrefix(trimmed, "##") {
				handoffBuilder.WriteString(line + "\n")
			}
		case "return":
			if trimmed != "" && !strings.HasPrefix(trimmed, "##") {
				returnBuilder.WriteString(line + "\n")
			}
		}
	}

	skill.WorkProcedure = procedureBuilder.String()
	skill.ExampleHandoff = handoffBuilder.String()
	skill.ReturnConditions = returnBuilder.String()

	if skill.Name == "" {
		return nil, fmt.Errorf("%w: missing name in skill file", ErrInvalidSkillFile)
	}

	return skill, nil
}

// LoadAllSkills loads all available skills from the mission directory.
func (sl *SkillLoader) LoadAllSkills() (map[string]*Skill, error) {
	skillsDir := filepath.Join(sl.missionDir, "skills")

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*Skill{}, nil // No skills directory yet
		}
		return nil, fmt.Errorf("read skills directory: %w", err)
	}

	skills := make(map[string]*Skill)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skill, err := sl.LoadSkill(entry.Name())
		if err != nil {
			// Skip invalid skills but log
			continue
		}
		skills[entry.Name()] = skill
	}

	return skills, nil
}

// globalSkillsSourceDir returns the directory where ywai ships its bundled
// skills (ywai/skills/ in the repo checkout or ~/.ywai/skills for an installed
// binary). It is the third resolution tier after per-mission and default
// skills. Returns empty string when the directory cannot be located.
func globalSkillsSourceDir() string {
	// 1. Explicit override.
	if dir := os.Getenv("YWAI_SKILLS_DIR"); dir != "" {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	// 2. Repo checkout: walk up from the working directory to the ywai module
	//    root (the dir holding go.mod) and use its skills/ directory. This wins
	//    over the seeded home copy so dev runs and tests pick up edited skills
	//    without re-seeding ~/.ywai/skills.
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; {
			if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
				skillsDir := filepath.Join(dir, "skills")
				if di, err := os.Stat(skillsDir); err == nil && di.IsDir() {
					return skillsDir
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	// 3. Installed binary: skills seeded into the user data dir.
	if home := filepath.Join(os.Getenv("HOME"), ".ywai", "skills"); home != "" {
		if info, err := os.Stat(home); err == nil && info.IsDir() {
			return home
		}
	}
	return ""
}

// ResolveSkillContent resolves a skill name to its renderable markdown body,
// in priority order:
//  1. per-mission skill: {missionDir}/skills/{name}/SKILL.md (raw file)
//  2. global skill: {globalSkillsSourceDir}/{name}/SKILL.md (raw file)
//  3. default skill: GetDefaultSkill(name) → formatted body
//
// The raw-file tiers win so the authored content (full tech-skill guides like
// docker/angular and worker skills alike) is injected verbatim. GetDefaultSkill
// is only a last-resort generic fallback for names that have no file, since it
// re-renders through formatSkillBody and would otherwise strip authored content.
//
// Returns the skill name and its markdown body, or empty strings when nothing
// resolves.
func ResolveSkillContent(missionDir, name string) (resolvedName, body string) {
	if name == "" {
		return "", ""
	}

	// Tier 1: per-mission SKILL.md.
	missionSkillPath := filepath.Join(missionDir, "skills", name, "SKILL.md")
	if data, err := os.ReadFile(missionSkillPath); err == nil {
		return name, string(data)
	}

	// Tier 2: global SKILL.md (raw, full authored content).
	if gdir := globalSkillsSourceDir(); gdir != "" {
		if data, err := os.ReadFile(filepath.Join(gdir, name, "SKILL.md")); err == nil {
			return name, string(data)
		}
	}

	// Tier 3: generic default template for names without a file.
	if def, err := GetDefaultSkill(name); err == nil && def != nil {
		return name, formatSkillBody(def)
	}

	return "", ""
}

// formatSkillBody renders a parsed Skill struct as markdown matching the the worker
// SKILL.md format (frontmatter + the four required body sections).
func formatSkillBody(s *Skill) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("name: %s\n", s.Name))
	b.WriteString(fmt.Sprintf("description: %s\n", s.Description))
	b.WriteString("---\n\n")
	b.WriteString(fmt.Sprintf("# %s\n\n", s.Name))

	b.WriteString("## Required Skills and Tools\n")
	if len(s.RequiredSkills) == 0 && len(s.RequiredTools) == 0 {
		b.WriteString("None\n")
	} else {
		for _, sk := range s.RequiredSkills {
			b.WriteString(fmt.Sprintf("- %s\n", sk))
		}
		for _, t := range s.RequiredTools {
			b.WriteString(fmt.Sprintf("- %s\n", t))
		}
	}
	b.WriteString("\n")

	if s.WorkProcedure != "" {
		b.WriteString("## Work Procedure\n\n")
		b.WriteString(s.WorkProcedure)
		if !strings.HasSuffix(s.WorkProcedure, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if s.ExampleHandoff != "" {
		b.WriteString("## Example Handoff\n\n")
		b.WriteString(s.ExampleHandoff)
		if !strings.HasSuffix(s.ExampleHandoff, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if s.ReturnConditions != "" {
		b.WriteString("## When to Return to Orchestrator\n\n")
		b.WriteString(s.ReturnConditions)
		if !strings.HasSuffix(s.ReturnConditions, "\n") {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// GetDefaultSkill returns the skill for a worker type. The canonical worker
// skills live as editable SKILL.md files under the bundled skills directory
// (see globalSkillsSourceDir) — the same place tech skills like yz-ui live.
// When no file resolves, a generic implementation skill is returned so worker
// execution always has a usable procedure.
func GetDefaultSkill(workerType string) (*Skill, error) {
	if gdir := globalSkillsSourceDir(); gdir != "" {
		path := filepath.Join(gdir, workerType, "SKILL.md")
		if data, err := os.ReadFile(path); err == nil {
			if skill, perr := (&SkillLoader{}).parseSkill(data); perr == nil && skill.Name != "" {
				return skill, nil
			}
		}
	}
	return genericWorkerSkill(workerType), nil
}

// genericWorkerSkill is the last-resort skill used when no SKILL.md file exists
// for a worker type. It keeps Name set to the requested type so callers that
// classify by name keep working.
func genericWorkerSkill(workerType string) *Skill {
	return &Skill{
		Name:          workerType,
		Description:   "Generic implementation worker",
		RequiredTools: []string{"git"},
		WorkProcedure: `1. Read the feature description and expected behavior
2. Implement the feature
3. Test the implementation
4. Return a structured handoff`,
		ExampleHandoff: `{
  "salientSummary": "Implemented feature as described",
  "whatWasImplemented": "Feature implementation completed",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": []
  },
  "tests": {
    "added": [],
    "coverage": "N/A"
  },
  "discoveredIssues": []
}`,
		ReturnConditions: "Return to orchestrator if: requirements are ambiguous or you cannot complete the work",
	}
}
