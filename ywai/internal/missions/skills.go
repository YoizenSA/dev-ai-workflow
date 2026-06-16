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
	// 1. Repo checkout: walk up from the store base to find ywai/skills.
	//    The missions base dir lives under ~/.local/share/ywai/missions, so
	//    look relative to the executable / known locations instead.
	for _, candidate := range []string{
		os.Getenv("YWAI_SKILLS_DIR"),
		filepath.Join(os.Getenv("HOME"), ".ywai", "skills"),
	} {
		if candidate != "" {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

// ResolveSkillContent resolves a skill name to its renderable markdown body,
// in priority order:
//  1. per-mission skill: {missionDir}/skills/{name}/SKILL.md (raw file)
//  2. default skill: GetDefaultSkill(name) → formatted body
//  3. global skill: {globalSkillsSourceDir}/{name}/SKILL.md (raw file)
//
// Returns the skill name and its markdown body, or empty strings when nothing
// resolves. The raw-file tiers return the file verbatim so custom/global skills
// keep their authored formatting.
func ResolveSkillContent(missionDir, name string) (resolvedName, body string) {
	if name == "" {
		return "", ""
	}

	// Tier 1: per-mission SKILL.md.
	missionSkillPath := filepath.Join(missionDir, "skills", name, "SKILL.md")
	if data, err := os.ReadFile(missionSkillPath); err == nil {
		return name, string(data)
	}

	// Tier 2: default skill template.
	if def, err := GetDefaultSkill(name); err == nil && def != nil {
		return name, formatSkillBody(def)
	}

	// Tier 3: global skill.
	if gdir := globalSkillsSourceDir(); gdir != "" {
		if data, err := os.ReadFile(filepath.Join(gdir, name, "SKILL.md")); err == nil {
			return name, string(data)
		}
	}

	return "", ""
}

// formatSkillBody renders a parsed Skill struct as markdown matching the Droid
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

// GetDefaultSkill returns a default skill template for a worker type.
// Used when no custom skill exists in the mission directory.
func GetDefaultSkill(workerType string) (*Skill, error) {
	// Default skill templates for common worker types
	switch workerType {
	case "backend-worker", "implementation":
		return &Skill{
			Name:          "backend-worker",
			Description:   "Backend implementation worker",
			RequiredTools: []string{"go test", "git"},
			WorkProcedure: `1. Read the feature description and expected behavior
2. Write failing tests first (TDD)
3. Implement the feature to make tests pass
4. Run tests and verify they pass
5. Manually verify the implementation
6. Return a structured handoff`,
			ExampleHandoff: `{
  "salientSummary": "Implemented GET /api/users endpoint with pagination",
  "whatWasImplemented": "Created users controller with list endpoint supporting cursor-based pagination and filtering",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": [
      {"command": "go test ./internal/users/...", "exitCode": 0, "observation": "All tests passed"}
    ]
  },
  "tests": {
    "added": [{"file": "internal/users/users_test.go", "cases": [{"name": "TestListUsers", "verifies": "Returns paginated user list"}]}],
    "coverage": "85%"
  },
  "discoveredIssues": []
}`,
			ReturnConditions: "Return to orchestrator if: requirements are ambiguous, existing bugs affect this feature, or you cannot complete within mission boundaries",
		}, nil
	case "frontend-worker":
		return &Skill{
			Name:          "frontend-worker",
			Description:   "Frontend implementation worker",
			RequiredTools: []string{"npm test", "git"},
			WorkProcedure: `1. Read the feature description and expected behavior
2. Write failing tests first (TDD)
3. Implement the feature to make tests pass
4. Run tests and verify they pass
5. Manually verify the implementation in browser
6. Return a structured handoff`,
			ExampleHandoff: `{
  "salientSummary": "Implemented user profile page with edit form",
  "whatWasImplemented": "Created UserProfile component with form validation and API integration",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": [
      {"command": "npm test -- UserProfile.test.tsx", "exitCode": 0, "observation": "All tests passed"}
    ]
  },
  "tests": {
    "added": [{"file": "src/components/UserProfile.test.tsx", "cases": [{"name": "testRendersProfile", "verifies": "Component renders user data"}]}],
    "coverage": "80%"
  },
  "discoveredIssues": []
}`,
			ReturnConditions: "Return to orchestrator if: requirements are ambiguous, existing bugs affect this feature, or you cannot complete within mission boundaries",
		}, nil
	case "qa-worker":
		return &Skill{
			Name:          "qa-worker",
			Description:   "QA and testing worker",
			RequiredTools: []string{"go test", "npm test"},
			WorkProcedure: `1. Read the feature description and expected behavior
2. Review existing test coverage
3. Write comprehensive tests for the feature
4. Run tests and verify they pass
5. Check for edge cases and error conditions
6. Return a structured handoff`,
			ExampleHandoff: `{
  "salientSummary": "Added comprehensive test coverage for authentication module",
  "whatWasImplemented": "Added unit tests for login, logout, and token refresh flows",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": [
      {"command": "go test ./internal/auth/... -cover", "exitCode": 0, "observation": "Coverage increased from 60% to 90%"}
    ]
  },
  "tests": {
    "added": [{"file": "internal/auth/auth_test.go", "cases": [{"name": "TestLoginSuccess", "verifies": "Valid credentials return token"}]}],
    "coverage": "90%"
  },
  "discoveredIssues": []
}`,
			ReturnConditions: "Return to orchestrator if: test infrastructure is missing, or you cannot write meaningful tests",
		}, nil
	case "reviewer-worker":
		return &Skill{
			Name:          "reviewer-worker",
			Description:   "Code review worker — audits diffs and reports findings without editing code",
			RequiredTools: []string{"git", "gh"},
			WorkProcedure: `1. Read the feature description and the diff produced by upstream workers
2. Audit for correctness, security, performance, readability, and project conventions
3. Cross-check tests cover the behavior described
4. Do NOT modify source code; report findings in the handoff
5. Return a structured handoff with severity-labelled issues`,
			ExampleHandoff: `{
  "salientSummary": "Reviewed auth refactor — found 2 issues, recommended changes",
  "whatWasImplemented": "Code review only; no source changes",
  "whatWasLeftUndone": "",
  "verification": {"commandsRun": [{"command": "git diff main...HEAD", "exitCode": 0, "observation": "Inspected diff"}]},
  "tests": {"added": [], "coverage": "N/A"},
  "discoveredIssues": [
    {"severity": "blocking", "description": "JWT secret read from env without validation", "suggestedFix": "Add startup check that secret is non-empty"},
    {"severity": "suggestion", "description": "Missing test for token expiry path"}
  ]
}`,
			ReturnConditions: "Return to orchestrator if: the diff is too large to audit in one pass, or upstream context is missing",
		}, nil
	case "planner":
		return &Skill{
			Name:          "planner",
			Description:   "Mission planner — breaks goals into milestones, features, and validation contracts",
			RequiredTools: []string{"opencode"},
			WorkProcedure: `1. Read the mission goal and any clarifications
2. Decompose into milestones and features with explicit preconditions and expected behavior
3. Assign a role (planning, dev, frontend, backend, qa, reviewer, devops) per feature
4. Emit the plan JSON matching the documented contract
5. Return a structured handoff`,
			ExampleHandoff: `{
  "salientSummary": "Drafted plan with 3 milestones and 7 features",
  "whatWasImplemented": "Plan JSON written to disk",
  "whatWasLeftUndone": "",
  "verification": {"commandsRun": []},
  "tests": {"added": [], "coverage": "N/A"},
  "discoveredIssues": []
}`,
			ReturnConditions: "Return to orchestrator if: the goal is too ambiguous to plan or required context is missing",
		}, nil
	case "devops-worker":
		return &Skill{
			Name:          "devops-worker",
			Description:   "DevOps and infrastructure worker",
			RequiredTools: []string{"docker", "kubectl", "helm"},
			WorkProcedure: `1. Read the feature description and expected behavior
2. Implement infrastructure changes (Docker, K8s, CI/CD)
3. Test the infrastructure locally
4. Verify services start and healthcheck passes
5. Return a structured handoff`,
			ExampleHandoff: `{
  "salientSummary": "Added Docker configuration for API service",
  "whatWasImplemented": "Created Dockerfile and docker-compose configuration for API service",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": [
      {"command": "docker compose up -d api", "exitCode": 0, "observation": "Service started successfully"},
      {"command": "curl -sf http://localhost:3100/health", "exitCode": 0, "observation": "Healthcheck passed"}
    ]
  },
  "tests": {
    "added": [],
    "coverage": "N/A"
  },
  "discoveredIssues": []
}`,
			ReturnConditions: "Return to orchestrator if: infrastructure requirements are unclear, or you cannot complete within mission boundaries",
		}, nil
	default:
		// Return a generic implementation skill as fallback
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
		}, nil
	}
}
