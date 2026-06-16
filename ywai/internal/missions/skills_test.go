package missions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillLoader_LoadSkill(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "ywai-skill-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a skill directory
	skillDir := filepath.Join(tmpDir, "skills", "test-worker")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("failed to create skill dir: %v", err)
	}

	// Create a SKILL.md file
	skillContent := `---
name: test-worker
description: Test worker for unit tests
---

# Test Worker

## Required Skills and Tools
- test-skill
- test-tool

## Work Procedure
1. Read the feature
2. Implement it
3. Test it

## Example Handoff
{
  "salientSummary": "Test completed"
}

## When to Return to Orchestrator
When requirements are unclear
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("failed to write SKILL.md: %v", err)
	}

	// Test loading the skill
	loader := NewSkillLoader(tmpDir)
	skill, err := loader.LoadSkill("test-worker")
	if err != nil {
		t.Fatalf("failed to load skill: %v", err)
	}

	// Verify skill content
	if skill.Name != "test-worker" {
		t.Errorf("expected name 'test-worker', got '%s'", skill.Name)
	}
	if skill.Description != "Test worker for unit tests" {
		t.Errorf("expected description 'Test worker for unit tests', got '%s'", skill.Description)
	}
	if len(skill.RequiredSkills) != 1 || skill.RequiredSkills[0] != "test-skill" {
		t.Errorf("expected 1 skill 'test-skill', got %v", skill.RequiredSkills)
	}
	if len(skill.RequiredTools) != 1 || skill.RequiredTools[0] != "test-tool" {
		t.Errorf("expected 1 tool 'test-tool', got %v", skill.RequiredTools)
	}
}

func TestSkillLoader_SkillNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ywai-skill-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	loader := NewSkillLoader(tmpDir)
	_, err = loader.LoadSkill("non-existent")
	if err == nil {
		t.Error("expected error for non-existent skill, got nil")
	}
	// Check if error wraps ErrSkillNotFound
	if err == nil || !strings.Contains(err.Error(), "skill not found") {
		t.Errorf("expected skill not found error, got %v", err)
	}
}

func TestGetDefaultSkill(t *testing.T) {
	// Test default skills for common worker types
	testCases := []string{"backend-worker", "frontend-worker", "qa-worker", "devops-worker"}

	for _, workerType := range testCases {
		skill, err := GetDefaultSkill(workerType)
		if err != nil {
			t.Errorf("failed to get default skill for %s: %v", workerType, err)
			continue
		}
		if skill.Name != workerType {
			t.Errorf("expected name '%s', got '%s'", workerType, skill.Name)
		}
		if skill.WorkProcedure == "" {
			t.Errorf("expected work procedure for %s, got empty", workerType)
		}
	}

	// Test fallback for unknown worker type
	skill, err := GetDefaultSkill("unknown-worker")
	if err != nil {
		t.Errorf("failed to get default skill for unknown worker: %v", err)
	}
	if skill.Name != "unknown-worker" {
		t.Errorf("expected name 'unknown-worker', got '%s'", skill.Name)
	}
}
