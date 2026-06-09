package missions

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ─── Errors ────────────────────────────────────────────────────────────────

var (
	ErrArtifactCreation = fmt.Errorf("failed to create mission artifact")
)

// ─── Artifact Creator ─────────────────────────────────────────────────────

// ArtifactCreator creates Factory.ai mission artifacts.
type ArtifactCreator struct {
	missionDir string
	store      *MissionsStore
}

// NewArtifactCreator creates a new ArtifactCreator for the given mission directory.
func NewArtifactCreator(missionDir string, store *MissionsStore) *ArtifactCreator {
	return &ArtifactCreator{
		missionDir: missionDir,
		store:      store,
	}
}

// CreateAllArtifacts creates all required mission artifacts.
func (ac *ArtifactCreator) CreateAllArtifacts(mission *Mission) error {
	// Create directory structure
	dirs := []string{
		ac.missionDir + "/skills",
		ac.missionDir + "/library",
		ac.missionDir + "/workers",
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("%w: create directory %s: %v", ErrArtifactCreation, dir, err)
		}
	}
	
	// Create each artifact
	if err := ac.CreateArchitectureMD(mission); err != nil {
		return err
	}
	
	if err := ac.CreateValidationContract(mission); err != nil {
		return err
	}
	
	if err := ac.CreateValidationState(mission); err != nil {
		return err
	}
	
	if err := ac.CreateServicesYAML(mission); err != nil {
		return err
	}
	
	if err := ac.CreateAGENTSMD(mission); err != nil {
		return err
	}
	
	if err := ac.CreateMissionMD(mission); err != nil {
		return err
	}
	
	if err := ac.CreateDefaultSkills(mission); err != nil {
		return err
	}
	
	return nil
}

// CreateArchitectureMD creates the architecture.md file.
func (ac *ArtifactCreator) CreateArchitectureMD(mission *Mission) error {
	content := `# Architecture

## Overview

This document describes the architecture for the mission: ` + mission.Name + `

## Mission ID

` + mission.ID + `

## Milestones

`
	for _, ms := range mission.Milestones {
		content += fmt.Sprintf("### %s\n%s\n\n", ms.Name, ms.Description)
	}
	
	content += `## Components

(TODO: Add component descriptions during planning phase)

## Data Flow

(TODO: Add data flow diagrams during planning phase)

## Technology Stack

(TODO: Add technology details during planning phase)
`
	
	path := filepath.Join(ac.missionDir, "architecture.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("%w: write architecture.md: %v", ErrArtifactCreation, err)
	}
	
	return nil
}

// CreateValidationContract creates the validation-contract.md file.
func (ac *ArtifactCreator) CreateValidationContract(mission *Mission) error {
	content := `# Validation Contract

This contract defines the behavioral assertions that must pass for the mission to be considered complete.

## Coverage Note

This is a template validation contract. During the planning phase, the orchestrator will:
1. Identify all user-facing features
2. Enumerate assertions for each feature
3. Add cross-feature flow assertions
4. Run review passes to ensure completeness

## Area: Core Functionality

### VAL-CORE-001: Mission executes successfully
The mission completes without errors and all features are implemented.
Tool: engine
Evidence: mission status, feature completion status

## Cross-Area Flows

### VAL-CROSS-001: Feature integration
All features integrate correctly with each other.
Tool: engine
Evidence: integration tests, feature dependencies
`
	
	if err := os.WriteFile(ac.missionDir+"/validation-contract.md", []byte(content), 0644); err != nil {
		return fmt.Errorf("%w: write validation-contract.md: %v", ErrArtifactCreation, err)
	}
	
	return nil
}

// CreateValidationState creates the validation-state.json file.
func (ac *ArtifactCreator) CreateValidationState(mission *Mission) error {
	// Load the contract to get assertion IDs
	parser := NewContractParser(ac.missionDir)
	contract, err := parser.LoadContract()
	if err != nil {
		// If contract doesn't exist yet, create empty state
		contract = &ValidationContract{}
	}
	
	state := ValidationState{
		Assertions: make([]ValidationAssertion, 0, len(contract.Assertions)),
		UpdatedAt: time.Now().UTC(),
	}
	
	// Initialize all assertions as pending
	for _, assertion := range contract.Assertions {
		state.Assertions = append(state.Assertions, ValidationAssertion{
			ID:     assertion.ID,
			Status: ValidationPending,
		})
	}
	
	if err := ac.store.SaveValidationState(mission.ID, &state); err != nil {
		return fmt.Errorf("%w: save validation-state.json: %v", ErrArtifactCreation, err)
	}
	
	return nil
}

// CreateServicesYAML creates the services.yaml file.
func (ac *ArtifactCreator) CreateServicesYAML(mission *Mission) error {
	content := `# Services Manifest
# This is the single source of truth for all commands and services.

commands:
  # Common commands
  install: echo "Install command not yet defined"
  build: echo "Build command not yet defined"
  test: echo "Test command not yet defined"
  lint: echo "Lint command not yet defined"

services:
  # Define long-running services here
  # Example:
  # api:
  #   start: PORT=3100 npm run dev
  #   stop: lsof -ti :3100 | xargs kill
  #   healthcheck: curl -sf http://localhost:3100/health
  #   port: 3100
  #   depends_on: []
`
	
	if err := os.WriteFile(ac.missionDir+"/services.yaml", []byte(content), 0644); err != nil {
		return fmt.Errorf("%w: write services.yaml: %v", ErrArtifactCreation, err)
	}
	
	return nil
}

// CreateAGENTSMD creates the AGENTS.md file.
func (ac *ArtifactCreator) CreateAGENTSMD(mission *Mission) error {
	content := `# Agent Instructions

This file provides operational guidance for workers executing this mission.

## Mission Boundaries (NEVER VIOLATE)

**Port Range:** 3100-3199. Never start services outside this range.

**External Services:**
- Check what services are already running before starting new ones
- Do not conflict with existing services on the system

**Off-Limits:**
- Do not modify files outside the project directory unless explicitly required
- Do not start services on ports 3000-3010 (common dev server range)

Workers: If you cannot complete your work within these boundaries, return to orchestrator.

## Mission Directives

**Tools:**
- Use appropriate tools for the task (git, package managers, test runners)
- Follow existing project conventions for tool usage

**Skills:**
- Follow the skill procedure specified for your feature type
- Invoke required skills at appropriate times

**Dependencies:**
- Use dependencies specified in the project
- Do not add new dependencies without explicit requirement

**Other:**
- Follow existing code style and conventions
- Write tests for all new code
- Verify your work before returning

## Testing & Validation Guidance

Instructions for validators from the orchestrator/user. Validators must follow these.

(TODO: Add specific testing guidance during planning phase)
`
	
	if err := os.WriteFile(ac.missionDir+"/AGENTS.md", []byte(content), 0644); err != nil {
		return fmt.Errorf("%w: write AGENTS.md: %v", ErrArtifactCreation, err)
	}
	
	return nil
}

// CreateMissionMD creates the mission.md file (mission proposal).
func (ac *ArtifactCreator) CreateMissionMD(mission *Mission) error {
	content := `# Mission: ` + mission.Name + `

## Mission ID

` + mission.ID + `

## Plan Overview

(TODO: Add mission description during planning phase)

## Expected Functionality

### Milestones

`
	for _, ms := range mission.Milestones {
		content += fmt.Sprintf("#### %s\n%s\n\n", ms.Name, ms.Description)
	}
	
	content += `## Environment Setup

(TODO: Add environment setup details during planning phase)

## Infrastructure

**Services:**
(TODO: Add service details during planning phase)

**Off-limits:**
- Ports 3000-3010 (common dev server range)

## Testing Strategy

(TODO: Add testing strategy during planning phase)

## User Testing Strategy

(TODO: Add user testing strategy during planning phase)

## Non-Functional Requirements

(TODO: Add non-functional requirements during planning phase)
`
	
	if err := os.WriteFile(ac.missionDir+"/mission.md", []byte(content), 0644); err != nil {
		return fmt.Errorf("%w: write mission.md: %v", ErrArtifactCreation, err)
	}
	
	return nil
}

// CreateDefaultSkills creates default skill templates for common worker types.
func (ac *ArtifactCreator) CreateDefaultSkills(mission *Mission) error {
	// Collect unique skill names from features
	skillTypes := make(map[string]bool)
	for _, feat := range mission.Features {
		if feat.SkillName != "" {
			skillTypes[feat.SkillName] = true
		}
	}
	
	// Create skill directory and SKILL.md for each type
	for skillType := range skillTypes {
		skillDir := ac.missionDir + "/skills/" + skillType
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return fmt.Errorf("%w: create skill directory %s: %v", ErrArtifactCreation, skillDir, err)
		}
		
		// Get default skill template
		skill, err := GetDefaultSkill(skillType)
		if err != nil {
			// Use generic skill if default not found
			skill, _ = GetDefaultSkill("implementation")
		}
		
		// Write SKILL.md
		skillContent := fmt.Sprintf(`---
name: %s
description: %s
---

# %s

NOTE: Startup and cleanup are handled by mission-worker-base. This skill defines the WORK PROCEDURE.

## Required Skills and Tools

`, skill.Name, skill.Description, skill.Name)
		
		if len(skill.RequiredSkills) > 0 {
			skillContent += "**Skills:**\n"
			for _, s := range skill.RequiredSkills {
				skillContent += fmt.Sprintf("- %s\n", s)
			}
		}
		
		if len(skill.RequiredTools) > 0 {
			skillContent += "\n**Tools:**\n"
			for _, t := range skill.RequiredTools {
				skillContent += fmt.Sprintf("- %s\n", t)
			}
		}
		
		skillContent += fmt.Sprintf(`
## Work Procedure

%s

## Example Handoff

%s

## When to Return to Orchestrator

%s
`, skill.WorkProcedure, skill.ExampleHandoff, skill.ReturnConditions)
		
		skillPath := skillDir + "/SKILL.md"
		if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
			return fmt.Errorf("%w: write SKILL.md for %s: %v", ErrArtifactCreation, skillType, err)
		}
	}
	
	return nil
}
