package missions

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
)

// ─── Errors ────────────────────────────────────────────────────────────────

var (
	ErrOpencodeNotFound = errors.New("opencode binary not found in PATH")
	ErrWorkerTimeout    = errors.New("worker timed out")
	ErrWorkerCancelled  = errors.New("worker cancelled")
	ErrEmptyHandoff     = errors.New("worker produced empty handoff")
	ErrMaxRetries       = errors.New("max retries reached for feature")
)

// ─── Constants ─────────────────────────────────────────────────────────────

const (
	// DefaultWorkerTimeout is the default maximum duration for a worker session.
	DefaultWorkerTimeout = 30 * time.Minute

	// DefaultMaxRetries is the default number of times a failed feature is retried.
	DefaultMaxRetries = 3

	// gracefulKillTimeout is the time to wait after SIGTERM before SIGKILL.
	gracefulKillTimeout = 5 * time.Second
)

// ─── Types ─────────────────────────────────────────────────────────────────

// WorkerConfig configures how a worker session runs.
type WorkerConfig struct {
	Timeout           time.Duration
	MaxRetries        int
	AgentsDir         string
	Verifier          Verifier // optional: when set, verification runs before completion
	VerifyCleanStreak int      // consecutive clean verify runs required per feature (FASE 5); default 0 = single run
}

// DefaultWorkerConfig returns a WorkerConfig with sensible defaults.
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		Timeout:    DefaultWorkerTimeout,
		MaxRetries: DefaultMaxRetries,
	}
}

// WorkerResult holds the outcome of a worker execution.
type WorkerResult struct {
	Handoff  *WorkerHandoff
	Log      string
	ExitCode int
	Err      error
}

// LogBroadcastFunc is a callback for broadcasting log output to UIs.
type LogBroadcastFunc func(missionID, featureID, line string)

// WorkerManager manages spawning opencode workers for features.
type WorkerManager struct {
	store        *MissionsStore
	config       WorkerConfig
	client       opencode.Client // optional: when set and reachable, uses API instead of CLI
	cmdCreator   func(ctx context.Context, name string, args ...string) *exec.Cmd
	logBroadcast LogBroadcastFunc
}

// NewWorkerManager creates a new WorkerManager.
func NewWorkerManager(store *MissionsStore, config WorkerConfig) *WorkerManager {
	return &WorkerManager{
		store:        store,
		config:       config,
		cmdCreator:   exec.CommandContext,
		logBroadcast: func(string, string, string) {},
	}
}

// SetClient sets the opencode client for API-based execution.
// When set and the server is reachable, ExecuteFeature uses the session API
// instead of spawning CLI subprocesses.
func (wm *WorkerManager) SetClient(client opencode.Client) {
	wm.client = client
}

// SetLogBroadcast sets the log broadcast callback for streaming logs to UIs.
func (wm *WorkerManager) SetLogBroadcast(fn LogBroadcastFunc) {
	if fn != nil {
		wm.logBroadcast = fn
	}
}

// buildStableInstructions reads the agent's AGENT.md for stable instruction caching.
// Returns empty string if the agent type or agents directory is not configured
// (graceful degradation — caller falls back to volatile-only content).
func (wm *WorkerManager) buildStableInstructions(agentType string) string {
	if agentType == "" || wm.config.AgentsDir == "" {
		return ""
	}
	path := filepath.Join(wm.config.AgentsDir, agentType, "AGENT.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// injectedSkill is a resolved skill name + renderable body for injection into
// the worker's feature.md.
type injectedSkill struct {
	Name string
	Body string
}

// resolveSkillsForFeature collects the skills a worker should follow for the
// given feature. The feature's own SkillName is included first (the primary
// skill), followed by the role's configured skills (RoleDefault.Skills), so
// the worker sees both its specific task skill and the role's standard kit.
//
// Each skill name resolves via ResolveSkillContent (per-mission → default →
// global). Skills that don't resolve to any content are skipped silently.
func (wm *WorkerManager) resolveSkillsForFeature(mission *Mission, feature *Feature) []injectedSkill {
	// Build the candidate skill name list, deduped, primary skill first.
	var names []string
	seen := map[string]bool{}
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		names = append(names, s)
	}
	add(feature.SkillName)

	// Role-configured skills from user config (previously dead data).
	role := feature.Role
	if role == "" {
		role = SkillNameToRole(feature.SkillName)
	}
	if role != "" {
		if cfg, _ := config.LoadConfig(); cfg != nil {
			rd := cfg.GetRoleDefault(role)
			for _, s := range rd.Skills {
				add(s)
			}
		}
	}

	missionDir := wm.store.MissionDir(mission.ID)
	var out []injectedSkill
	for _, name := range names {
		if n, body := ResolveSkillContent(missionDir, name); n != "" && body != "" {
			out = append(out, injectedSkill{Name: n, Body: body})
		}
	}
	return out
}

// ─── opencode Detection ────────────────────────────────────────────────────

// DetectOpencode checks if opencode is available in PATH and returns its path.
func DetectOpencode() (string, error) {
	path, err := exec.LookPath("opencode")
	if err != nil {
		return "", ErrOpencodeNotFound
	}
	return path, nil
}

// ─── Context Preparation ───────────────────────────────────────────────────
// maybeRemoveContext removes the context directory only if it was a temp dir.
// For worktree paths, cleanup is handled by WorktreeManager.
func maybeRemoveContext(dir string, isTemp bool) {
	if isTemp {
		os.RemoveAll(dir)
	}
}

// PrepareContext creates the working directory for a worker with all necessary
// context files. Returns the path to the context directory.
//
// The context directory includes:
//   - feature.md: Feature description and expected behaviors
//   - mission.md: Mission overview and context (from mission directory if exists)
//   - AGENTS.md: Worker constraints and handoff format specification (from mission directory if exists)
//   - SKILL.md: The specific skill for this worker type (from mission directory if exists)
//   - services.yaml: Services manifest (from mission directory if exists)
//
// When worktreePath is non-empty, context files are written to <worktreePath>/.ywai/
// and the worktreePath itself is returned as the working directory (for cmd.Dir).
// When worktreePath is empty, a temporary directory is created (legacy behavior).
func (wm *WorkerManager) PrepareContext(mission *Mission, feature *Feature, worktreePath string) (string, error) {
	var contextDir string
	var isTempDir bool
	execAgent := mission.ExecutionAgent
	if execAgent == "" {
		execAgent = "orchestrator"
	}

	if worktreePath != "" {
		contextDir = filepath.Join(worktreePath, ".ywai")
		isTempDir = false
	} else {
		var err error
		contextDir, err = os.MkdirTemp("", fmt.Sprintf("ywai-worker-%s-%s-*", mission.ID, feature.ID))
		if err != nil {
			return "", fmt.Errorf("create worker temp dir: %w", err)
		}
		isTempDir = true
	}

	if err := os.MkdirAll(contextDir, 0755); err != nil {
		if !isTempDir {
			return "", fmt.Errorf("create .ywai dir: %w", err)
		}
		maybeRemoveContext(contextDir, isTempDir)
		return "", fmt.Errorf("create context dir: %w", err)
	}

	// Write feature.md
	var featureBld strings.Builder
	featureBld.WriteString("# You are the technical lead for this feature\n")
	featureBld.WriteString("You OWN this feature end-to-end inside this worktree. Decompose, delegate to\n")
	featureBld.WriteString("@dev / @qa / @reviewer / @devops as needed, and close with a structured handoff.\n")
	featureBld.WriteString("Scope: ONLY this feature and this worktree. Do not create new features or touch other branches.\n\n")
	featureBld.WriteString(fmt.Sprintf("## Feature: %s\n\n", feature.ID))
	featureBld.WriteString(fmt.Sprintf("%s\n\n", feature.Description))
	featureBld.WriteString("## Expected behavior\n")
	for _, b := range feature.ExpectedBehavior {
		featureBld.WriteString(fmt.Sprintf("- %s\n", b))
	}
	if len(feature.Fulfills) > 0 {
		featureBld.WriteString("\n## Fulfills (acceptance criteria)\n")
		for _, f := range feature.Fulfills {
			featureBld.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}
	featureBld.WriteString("\n## Required handoff format\n")
	featureBld.WriteString("Return a JSON WorkerHandoff with:\n")
	featureBld.WriteString("- salientSummary, whatWasImplemented, whatWasLeftUndone, discoveredIssues[], verification.commandsRun[]\n")

	// Inject role-configured skills so the worker sees HOW to work, not just
	// WHAT to build. This connects RoleDefault.Skills (previously dead data)
	// to the worker brief. Resolution order: per-mission → default → global.
	injectedSkills := wm.resolveSkillsForFeature(mission, feature)
	if len(injectedSkills) > 0 {
		featureBld.WriteString("\n## Skills (follow these procedures)\n")
		for _, sk := range injectedSkills {
			featureBld.WriteString(fmt.Sprintf("\n### Skill: %s\n\n", sk.Name))
			featureBld.WriteString(sk.Body)
			if !strings.HasSuffix(sk.Body, "\n") {
				featureBld.WriteString("\n")
			}
		}
	}

	// Upstream handoffs from preconditions
	if len(feature.Preconditions) > 0 {
		var handoffSection strings.Builder
		handoffSection.WriteString("\n## Upstream handoffs (preconditions)\n")
		for _, preID := range feature.Preconditions {
			h := ReadFeatureHandoff(wm.store, mission.ID, preID)
			if h == nil {
				continue
			}
			handoffSection.WriteString(fmt.Sprintf("\n### From %s\n", preID))
			handoffSection.WriteString(fmt.Sprintf("- Summary: %s\n", h.SalientSummary))
			if h.WhatWasImplemented != "" {
				handoffSection.WriteString(fmt.Sprintf("- Implemented: %s\n", h.WhatWasImplemented))
			}
			if h.WhatWasLeftUndone != "" {
				handoffSection.WriteString(fmt.Sprintf("- Left undone: %s\n", h.WhatWasLeftUndone))
			}
			if len(h.DiscoveredIssues) > 0 {
				var descs []string
				for _, iss := range h.DiscoveredIssues {
					descs = append(descs, iss.Description)
				}
				handoffSection.WriteString(fmt.Sprintf("- Issues: %s\n", strings.Join(descs, ", ")))
			}
		}
		featureBld.WriteString(handoffSection.String())
	}

	// Self-correction: include previous failure context (FASE 4)
	if feature.LastError != "" {
		featureBld.WriteString("\n## Previous attempt failed — fix this\n")
		featureBld.WriteString("The previous attempt produced the following error.\n")
		featureBld.WriteString("Ensure your implementation addresses it before marking done.\n\n")
		featureBld.WriteString("```\n")
		featureBld.WriteString(feature.LastError)
		featureBld.WriteString("\n```\n")
	} else if len(feature.VerifyRuns) > 0 {
		// Check if the last verify run failed
		lastRun := feature.VerifyRuns[len(feature.VerifyRuns)-1]
		if !lastRun.Passed {
			featureBld.WriteString("\n## Previous attempt failed — verification output\n")
			featureBld.WriteString("Last verification run failed:\n\n")
			featureBld.WriteString("```\n")
			output := lastRun.Output
			if output == "" {
				output = "No output captured"
			}
			featureBld.WriteString(output)
			featureBld.WriteString("\n```\n")
		}
	}

	if err := os.WriteFile(filepath.Join(contextDir, "feature.md"), []byte(featureBld.String()), 0644); err != nil {
		maybeRemoveContext(contextDir, isTempDir)
		return "", fmt.Errorf("write feature.md: %w", err)
	}

	// Load mission.md from mission directory if it exists, otherwise create basic one
	missionDir := wm.store.MissionDir(mission.ID)
	missionMDPath := filepath.Join(missionDir, "mission.md")
	if missionMDContent, err := os.ReadFile(missionMDPath); err == nil {
		// Use existing mission.md
		if err := os.WriteFile(filepath.Join(contextDir, "mission.md"), missionMDContent, 0644); err != nil {
			maybeRemoveContext(contextDir, isTempDir)
			return "", fmt.Errorf("write mission.md: %w", err)
		}
	} else {
		// Create basic mission.md
		missionMD := fmt.Sprintf("# %s\n\n%s\n\nMission ID: %s\nStatus: %s\n",
			mission.Name, mission.Name, mission.ID, mission.Status)
		for _, ms := range mission.Milestones {
			missionMD += fmt.Sprintf("\n## Milestone: %s\n%s\n", ms.Name, ms.Description)
		}
		if err := os.WriteFile(filepath.Join(contextDir, "mission.md"), []byte(missionMD), 0644); err != nil {
			maybeRemoveContext(contextDir, isTempDir)
			return "", fmt.Errorf("write mission.md: %w", err)
		}
	}

	// Load AGENTS.md from mission directory if it exists, otherwise create basic one
	agentsMDPath := filepath.Join(missionDir, "AGENTS.md")
	if agentsMDContent, err := os.ReadFile(agentsMDPath); err == nil {
		// Use existing AGENTS.md
		if err := os.WriteFile(filepath.Join(contextDir, "AGENTS.md"), agentsMDContent, 0644); err != nil {
			maybeRemoveContext(contextDir, isTempDir)
			return "", fmt.Errorf("write AGENTS.md: %w", err)
		}
	} else {
		// Build AGENTS.md with stable-first structure for LLM caching.
		// STABLE PART FIRST (byte-identical per agent type → cached by LLM provider).
		var agentsMD strings.Builder

		stableInstructions := wm.buildStableInstructions(execAgent)
		if stableInstructions != "" {
			agentsMD.WriteString(stableInstructions)
			agentsMD.WriteString("\n\n---\n\n## Current Task\n\n")
		}

		// VOLATILE PART (changes every invocation)
		agentsMD.WriteString(fmt.Sprintf("Goal: %s\n", mission.Name))
		agentsMD.WriteString("\n## Context\n")
		agentsMD.WriteString("- feature.md: The feature you need to implement\n")
		agentsMD.WriteString("- mission.md: Mission overview\n")
		agentsMD.WriteString("\n## Rules\n")
		agentsMD.WriteString("1. Follow existing codebase patterns\n")
		agentsMD.WriteString("2. Run tests before completing your work\n")
		agentsMD.WriteString("3. Return a structured handoff JSON as the LAST line of stdout\n")
		agentsMD.WriteString("\n## Handoff Format\n")
		agentsMD.WriteString("The final line of stdout MUST be a single JSON object with the following structure:\n")
		agentsMD.WriteString("{\n")
		agentsMD.WriteString("  \"salientSummary\": \"1-4 sentence summary of what was accomplished\",\n")
		agentsMD.WriteString("  \"whatWasImplemented\": \"Detailed description of what was implemented\",\n")
		agentsMD.WriteString("  \"whatWasLeftUndone\": \"What was left incomplete (empty string if complete)\",\n")
		agentsMD.WriteString("  \"verification\": {\n")
		agentsMD.WriteString("    \"commandsRun\": [\n")
		agentsMD.WriteString("      {\"command\": \"command that was run\", \"exitCode\": 0, \"observation\": \"what happened\"}\n")
		agentsMD.WriteString("    ]\n")
		agentsMD.WriteString("  },\n")
		agentsMD.WriteString("  \"tests\": {\n")
		agentsMD.WriteString("    \"added\": [{\"file\": \"path/to/test\", \"cases\": [{\"name\": \"TestName\", \"verifies\": \"what it verifies\"}]}],\n")
		agentsMD.WriteString("    \"coverage\": \"summary of coverage\"\n")
		agentsMD.WriteString("  },\n")
		agentsMD.WriteString("  \"discoveredIssues\": []\n")
		agentsMD.WriteString("}\n")

		if err := os.WriteFile(filepath.Join(contextDir, "AGENTS.md"), []byte(agentsMD.String()), 0644); err != nil {
			maybeRemoveContext(contextDir, isTempDir)
			return "", fmt.Errorf("write AGENTS.md: %w", err)
		}
	}

	// Load SKILL.md for this worker type if it exists
	skillLoader := NewSkillLoader(missionDir)
	if skill, err := skillLoader.LoadSkill(feature.SkillName); err == nil {
		// Write the skill content to SKILL.md in temp dir
		skillPath := filepath.Join(contextDir, "SKILL.md")
		skillContent := fmt.Sprintf(`---
name: %s
description: %s
---

# %s

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

		if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
			maybeRemoveContext(contextDir, isTempDir)
			return "", fmt.Errorf("write SKILL.md: %w", err)
		}
	} else {
		// Use default skill if custom skill not found
		if defaultSkill, err := GetDefaultSkill(feature.SkillName); err == nil {
			skillPath := filepath.Join(contextDir, "SKILL.md")
			skillContent := fmt.Sprintf(`---
name: %s
description: %s
---

# %s

## Required Skills and Tools
`, defaultSkill.Name, defaultSkill.Description, defaultSkill.Name)

			if len(defaultSkill.RequiredSkills) > 0 {
				skillContent += "**Skills:**\n"
				for _, s := range defaultSkill.RequiredSkills {
					skillContent += fmt.Sprintf("- %s\n", s)
				}
			}

			if len(defaultSkill.RequiredTools) > 0 {
				skillContent += "\n**Tools:**\n"
				for _, t := range defaultSkill.RequiredTools {
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
`, defaultSkill.WorkProcedure, defaultSkill.ExampleHandoff, defaultSkill.ReturnConditions)

			if err := os.WriteFile(skillPath, []byte(skillContent), 0644); err != nil {
				maybeRemoveContext(contextDir, isTempDir)
				return "", fmt.Errorf("write default SKILL.md: %w", err)
			}
		}
	}

	// Copy services.yaml if it exists
	servicesPath := filepath.Join(missionDir, "services.yaml")
	if servicesContent, err := os.ReadFile(servicesPath); err == nil {
		if err := os.WriteFile(filepath.Join(contextDir, "services.yaml"), servicesContent, 0644); err != nil {
			maybeRemoveContext(contextDir, isTempDir)
			return "", fmt.Errorf("write services.yaml: %w", err)
		}
	}

	// Create the mission's worker artifacts directory for logs
	workersDir := filepath.Join(wm.store.MissionDir(mission.ID), "workers", feature.ID)
	if err := os.MkdirAll(workersDir, 0755); err != nil {
		maybeRemoveContext(contextDir, isTempDir)
		return "", fmt.Errorf("create workers artifact dir: %w", err)
	}

	return contextDir, nil
}

// ─── Worker Spawning ───────────────────────────────────────────────────────

// SpawnWorker spawns an opencode process to work on the given feature, using
// the supplied model and agent (which the caller resolves via ResolveExecution
// so role defaults and fallback chains apply). Pass empty strings to defer to
// opencode's own defaults.
// Returns a cancel function (to kill the process), a channel for WorkerResult,
// and any error encountered during spawning.
func (wm *WorkerManager) SpawnWorker(mission *Mission, feature *Feature, contextDir, model, agent string) (cancel func(), resultCh <-chan WorkerResult, err error) {
	opencodePath, err := DetectOpencode()
	if err != nil {
		return nil, nil, err
	}

	// Create a cancellable context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), wm.config.Timeout)

	// Build the opencode command
	// Use "opencode run" for non-interactive task execution
	taskDesc := fmt.Sprintf("Implement feature %s: %s", feature.ID, feature.Description)
	if len(feature.ExpectedBehavior) > 0 {
		taskDesc += "\n\nExpected behavior:\n"
		for _, exp := range feature.ExpectedBehavior {
			taskDesc += "- " + exp + "\n"
		}
	}

	if agent == "" {
		agent = "orchestrator"
	}
	args := []string{"run"}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "--agent", agent)
	args = append(args, taskDesc)
	cmd := wm.cmdCreator(ctx, opencodePath, args...)
	cmd.Dir = contextDir

	// Create pipes for stdout and stderr
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("start opencode: %w", err)
	}

	// Channel for the result
	ch := make(chan WorkerResult, 1)

	// Stream output in a goroutine
	go wm.streamOutput(ctx, cmd, stdoutPipe, stderrPipe, mission, feature, ch, cancel)

	return cancel, ch, nil
}

// streamOutput reads stdout and stderr, persists to log file, and delivers
// the result on the provided channel.
func (wm *WorkerManager) streamOutput(
	ctx context.Context,
	cmd *exec.Cmd,
	stdout io.Reader,
	stderr io.Reader,
	mission *Mission,
	feature *Feature,
	resultCh chan<- WorkerResult,
	cancel context.CancelFunc,
) {
	defer close(resultCh)
	defer cancel()

	// Determine log path
	logDir := filepath.Join(wm.store.MissionDir(mission.ID), "workers", feature.ID)
	logPath := filepath.Join(logDir, "output.log")

	// Create or open log file (append mode for crash recovery)
	logFile, err := os.Create(logPath)
	if err != nil {
		resultCh <- WorkerResult{Err: fmt.Errorf("create log file: %w", err)}
		return
	}
	defer logFile.Close()

	// Multi writer: log file + in-memory buffer for handoff parsing
	var buf bytes.Buffer
	multiWriter := io.MultiWriter(logFile, &buf)

	// Read stdout line by line
	var stdoutWg sync.WaitGroup
	stdoutWg.Add(1)
	go func() {
		defer stdoutWg.Done()
		scanner := bufio.NewScanner(stdout)
		// Increase buffer to handle large handoff JSON (>64KB default)
		scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(multiWriter, line)
			wm.logBroadcast(mission.ID, feature.ID, line)
		}
		if err := scanner.Err(); err != nil {
			log.Printf("stdout scanner error for feature %s: %v", feature.ID, err)
		}
	}()

	// Read stderr and write to log with broadcast
	stderrScanner := bufio.NewScanner(stderr)
	stderrScanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for stderrScanner.Scan() {
		line := stderrScanner.Text()
		fmt.Fprintln(multiWriter, line)
		wm.logBroadcast(mission.ID, feature.ID, "[stderr] "+line)
	}
	if err := stderrScanner.Err(); err != nil {
		log.Printf("stderr scanner error for feature %s: %v", feature.ID, err)
	}

	// Wait for stdout to finish
	stdoutWg.Wait()

	// Wait for command to exit
	err = cmd.Wait()
	output := buf.String()
	exitCode := 0

	if err != nil {
		// Check if the context was cancelled or timed out first.
		// When exec.CommandContext kills a process due to context expiry,
		// cmd.Wait() returns an exec.ExitError (signal exit), not the
		// context error directly. So we must check ctx.Err() separately.
		if ctxErr := ctx.Err(); ctxErr != nil {
			resultCh <- WorkerResult{
				ExitCode: -1,
				Log:      output,
				Err:      classifyContextError(ctx, ctxErr),
			}
			return
		}

		// Try to get exit code from ExitError
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			resultCh <- WorkerResult{
				ExitCode: -1,
				Log:      output,
				Err:      fmt.Errorf("worker error: %w", err),
			}
			return
		}
	}

	result := WorkerResult{
		ExitCode: exitCode,
		Log:      output,
	}

	// If non-zero exit, mark as failed
	if exitCode != 0 {
		result.Err = fmt.Errorf("worker exited with code %d", exitCode)
		resultCh <- result
		return
	}

	// Parse handoff from output
	handoff, err := parseHandoff(output)
	if err != nil {
		result.Err = err
		resultCh <- result
		return
	}

	result.Handoff = handoff
	resultCh <- result
}

// ─── Context Error Helpers ─────────────────────────────────────────────────

// classifyContextError maps context errors to user-facing error types.
func classifyContextError(ctx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrWorkerTimeout
	}
	if errors.Is(err, context.Canceled) {
		return ErrWorkerCancelled
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrWorkerTimeout
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return ErrWorkerCancelled
	}
	return err
}

// ─── Handoff Parsing ───────────────────────────────────────────────────────

// parseHandoff extracts a WorkerHandoff from the last JSON line of worker output.
// The worker protocol requires that the final line of stdout is a JSON object
// containing the structured handoff.
func parseHandoff(output string) (*WorkerHandoff, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil, ErrEmptyHandoff
	}

	lines := strings.Split(output, "\n")

	// Find the last non-empty line
	var lastLine string
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			lastLine = trimmed
			break
		}
	}

	if lastLine == "" {
		return nil, ErrEmptyHandoff
	}

	var handoff WorkerHandoff
	if strings.HasPrefix(lastLine, "{") {
		if err := json.Unmarshal([]byte(lastLine), &handoff); err == nil {
			// Validate required fields: at least one of salientSummary or
			// whatWasImplemented must be non-empty.
			if handoff.SalientSummary != "" || handoff.WhatWasImplemented != "" {
				return &handoff, nil
			}
		}
	}

	// Not JSON or missing required fields: create a handoff from the full output.
	// opencode run produces natural language, not JSON, so use the entire output.
	// Log at debug level so developers can detect when structured handoff is missing.
	log.Printf("parseHandoff: worker output is not structured JSON for feature, falling back to raw output (line=%d chars)", len(lastLine))
	return &WorkerHandoff{
		SalientSummary:     output,
		WhatWasImplemented: output,
	}, nil
}

// ─── High-Level Feature Execution ──────────────────────────────────────────

// ExecuteFeature runs the full worker lifecycle for a feature:
//  1. Validates opencode is available
//  2. Checks retry count against max retries
//  3. Starts the feature (transitions to in_progress)
//  4. Prepares context directory
//  5. Spawns opencode worker
//  6. Waits for result
//  7. Handles errors (timeout, cancellation, non-zero exit, invalid handoff)
//  8. Cleans up temp directory
//  9. Completes or fails the feature based on result
//
// Returns the parsed WorkerHandoff on success, or an error describing the failure.
func (wm *WorkerManager) ExecuteViaCLI(mission *Mission, featureID string) (*WorkerHandoff, error) {
	if mission == nil {
		return nil, ErrInvalidMission
	}

	feat, err := GetFeatureByID(mission, featureID)
	if err != nil {
		return nil, err
	}

	// Check if max retries reached
	if feat.RetryCount >= wm.config.MaxRetries {
		// Mark as permanently failed
		FailFeature(wm.store, mission, featureID)
		return nil, fmt.Errorf("%w: feature %q retried %d/%d times",
			ErrMaxRetries, featureID, feat.RetryCount, wm.config.MaxRetries)
	}

	// Validate opencode exists before doing any work
	if _, err := DetectOpencode(); err != nil {
		FailFeature(wm.store, mission, featureID)
		return nil, err
	}

	// Transition feature to in_progress
	if _, err := StartFeature(wm.store, mission, featureID); err != nil {
		return nil, fmt.Errorf("start feature: %w", err)
	}

	// Reload feature pointer after state change
	feat, _ = GetFeatureByID(mission, featureID)

	// Prepare context directory
	contextDir, err := wm.PrepareContext(mission, feat, feat.WorktreePath)
	if err != nil {
		FailFeature(wm.store, mission, featureID, fmt.Sprintf("prepare context: %s", err))
		return nil, fmt.Errorf("prepare context: %w", err)
	}
	if contextDir != feat.WorktreePath {
		defer os.RemoveAll(contextDir)
	}

	// Resolve effective model + agent + fallback chain from role defaults.
	cfg, _ := config.LoadConfig()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	primaryModel, execAgent, chain := ResolveExecution(feat, mission, cfg)
	if len(chain) == 0 {
		chain = []string{primaryModel}
	}

	// Spawn worker — try each model in the fallback chain on retriable errors.
	var (
		result        WorkerResult
		spawnErr      error
		spawnedOK     bool
		attemptCount  = len(chain)
		lastAttempted string
	)
	for idx, attemptModel := range chain {
		lastAttempted = attemptModel
		log.Printf("feature %s attempting model %s (attempt %d/%d)", featureID, attemptModel, idx+1, attemptCount)
		cancel, resultCh, err := wm.SpawnWorker(mission, feat, contextDir, attemptModel, execAgent)
		if err != nil {
			spawnErr = err
			if errors.Is(err, ErrOpencodeNotFound) {
				break
			}
			if isRetriableModelError(err) && idx < attemptCount-1 {
				continue
			}
			break
		}
		result = <-resultCh
		cancel()
		spawnedOK = true
		// Retry next model only on retriable failures, classified either from
		// result.Err directly or from the tail of captured stdout/stderr.
		if result.Err != nil && idx < attemptCount-1 {
			if isRetriableModelError(result.Err) || isRetriableModelError(fmt.Errorf("%s", logTail(result.Log))) {
				log.Printf("feature %s model %s failed retriably: %v — trying next", featureID, attemptModel, result.Err)
				continue
			}
		}
		break
	}

	if !spawnedOK {
		FailFeature(wm.store, mission, featureID, fmt.Sprintf("spawn worker (model=%s): %s", lastAttempted, spawnErr))
		if errors.Is(spawnErr, ErrOpencodeNotFound) {
			return nil, spawnErr
		}
		return nil, fmt.Errorf("spawn worker: %w", spawnErr)
	}

	// Handle worker errors from the final attempt.
	if result.Err != nil {
		// Timeout or cancellation
		if errors.Is(result.Err, ErrWorkerTimeout) {
			FailFeature(wm.store, mission, featureID, "worker timeout")
			return nil, ErrWorkerTimeout
		}
		if errors.Is(result.Err, ErrWorkerCancelled) {
			FailFeature(wm.store, mission, featureID, "worker cancelled")
			return nil, ErrWorkerCancelled
		}

		// Non-zero exit
		if result.ExitCode != 0 {
			FailFeature(wm.store, mission, featureID, fmt.Sprintf("worker exited with code %d (model=%s)", result.ExitCode, lastAttempted))
			return nil, fmt.Errorf("worker exited with code %d", result.ExitCode)
		}

		// Invalid or empty handoff
		if errors.Is(result.Err, ErrEmptyHandoff) {
			FailFeature(wm.store, mission, featureID, "empty handoff")
			return nil, result.Err
		}

		// Generic error
		FailFeature(wm.store, mission, featureID, fmt.Sprintf("worker error (model=%s): %s", lastAttempted, result.Err))
		return nil, result.Err
	}

	// Verify handoff is present
	if result.Handoff == nil {
		FailFeature(wm.store, mission, featureID, "nil handoff")
		return nil, ErrEmptyHandoff
	}

	// Run verification before completing (FASE 3)
	if wm.config.Verifier != nil {
		verifyResult, vErr := RunVerifyOnFeature(wm.store, wm.config.Verifier, mission, feat, wm.config.VerifyCleanStreak)
		if vErr != nil {
			log.Printf("Verification error for %s: %v — completing anyway", featureID, vErr)
		} else if !verifyResult.Passed {
			log.Printf("Verification FAILED for %s — feature will be retried", featureID)
			// FailFeature and LastError are handled inside RunVerifyOnFeature
			return nil, fmt.Errorf("verification failed for %s", featureID)
		}
	}

	// Verification and CompleteFeature are handled by ExecuteFeature.
	return result.Handoff, nil
}

// ─── API-Based Execution ──────────────────────────────────────────────────

// canUseAPI checks whether the opencode server is reachable for session-based execution.
func (wm *WorkerManager) canUseAPI() bool {
	if wm.client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	status, err := wm.client.Status(ctx)
	if err != nil {
		return false
	}
	return status.Connected && status.Source == "server"
}

// ExecuteFeature runs the full worker lifecycle for a feature.
// It first tries the opencode session API (if the server is reachable),
// falling back to CLI-based execution via `opencode run`.
// After the worker completes, verification runs. Only if verification passes
// does the feature transition to completed.
func (wm *WorkerManager) ExecuteFeature(mission *Mission, featureID string) (*WorkerHandoff, error) {
	if mission == nil {
		return nil, ErrInvalidMission
	}

	// Get a fresh feature pointer for the caller (needed by both paths)
	feat, err := GetFeatureByID(mission, featureID)
	if err != nil {
		return nil, err
	}

	var handoff *WorkerHandoff

	if wm.canUseAPI() {
		handoff, err = wm.executeViaAPI(mission, featureID)
		if err != nil {
			if errors.Is(err, opencode.ErrSessionsUnavailable) {
				handoff, err = wm.ExecuteViaCLI(mission, featureID)
			}
		}
	} else {
		handoff, err = wm.ExecuteViaCLI(mission, featureID)
	}

	if err != nil {
		return nil, err
	}

	// Run verification before completing (M3: verify in all execution paths)
	if wm.config.Verifier != nil {
		verifyResult, vErr := RunVerifyOnFeature(wm.store, wm.config.Verifier, mission, feat, wm.config.VerifyCleanStreak)
		if vErr != nil {
			log.Printf("Verification error for %s: %v — completing anyway", featureID, vErr)
		} else if !verifyResult.Passed {
			log.Printf("Verification FAILED for %s — feature will be retried", featureID)
			return nil, fmt.Errorf("verification failed for %s", featureID)
		}
	}

	// Complete the feature
	if _, err := CompleteFeature(wm.store, mission, featureID); err != nil {
		return nil, fmt.Errorf("complete feature: %w", err)
	}

	return handoff, nil
}

// executeViaAPI runs a feature by creating an opencode session, sending the task
// as a prompt, waiting for completion, and extracting the handoff from the response.
func (wm *WorkerManager) executeViaAPI(mission *Mission, featureID string) (*WorkerHandoff, error) {
	feat, err := GetFeatureByID(mission, featureID)
	if err != nil {
		return nil, err
	}

	// Check retries
	if feat.RetryCount >= wm.config.MaxRetries {
		FailFeature(wm.store, mission, featureID)
		return nil, fmt.Errorf("%w: feature %q retried %d/%d times",
			ErrMaxRetries, featureID, feat.RetryCount, wm.config.MaxRetries)
	}

	// Transition feature to in_progress
	if _, err := StartFeature(wm.store, mission, featureID); err != nil {
		return nil, fmt.Errorf("start feature: %w", err)
	}
	feat, _ = GetFeatureByID(mission, featureID)

	// Build the full orchestration brief via PrepareContext
	promptContent, err := wm.PrepareContext(mission, feat, feat.WorktreePath)
	if err != nil {
		// Fall back to simple description on error
		promptContent = fmt.Sprintf("Feature: %s\n%s", feat.ID, feat.Description)
		if len(feat.ExpectedBehavior) > 0 {
			promptContent += "\n\nExpected behavior:\n"
			for _, exp := range feat.ExpectedBehavior {
				promptContent += "- " + exp + "\n"
			}
		}
	}

	// Resolve effective model + agent + fallback chain from role defaults.
	cfg, _ := config.LoadConfig()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	primaryModel, execAgent, chain := ResolveExecution(feat, mission, cfg)
	if len(chain) == 0 {
		chain = []string{primaryModel}
	}

	sessions := wm.client.Sessions()

	// Try each model in the chain. Recreate the session per attempt because
	// opencode binds a session to a single model. Break on success or on a
	// non-retriable error; pass through to the next model on retriable errors.
	var (
		handoff    *WorkerHandoff
		lastErr    error
		attemptCnt = len(chain)
	)
	for idx, attemptModel := range chain {
		log.Printf("feature %s attempting model %s (attempt %d/%d)", featureID, attemptModel, idx+1, attemptCnt)

		ctx, cancel := context.WithTimeout(context.Background(), wm.config.Timeout)
		// scoped block so deferred cleanup runs per-iteration
		err := func() error {
			defer cancel()
			session, err := sessions.Create(ctx, opencode.SessionCreateOpts{
				Title: fmt.Sprintf("%s/%s", mission.ID, feat.ID),
				Agent: execAgent,
				Model: parseModelString(attemptModel),
			})
			if err != nil {
				return fmt.Errorf("create session: %w", err)
			}
			defer func() {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cleanupCancel()
				_ = sessions.Delete(cleanupCtx, session.ID)
			}()

			if _, err := sessions.Prompt(ctx, session.ID, opencode.PromptInput{
				Text:     promptContent,
				Delivery: "immediate",
			}); err != nil {
				return fmt.Errorf("send prompt: %w", err)
			}

			if err := sessions.Wait(ctx, session.ID); err != nil {
				if ctx.Err() == context.DeadlineExceeded {
					return ErrWorkerTimeout
				}
				return fmt.Errorf("wait for session: %w", err)
			}

			messages, err := sessions.Messages(ctx, session.ID)
			if err != nil {
				return fmt.Errorf("get messages: %w", err)
			}
			h := extractHandoffFromMessages(messages)
			if h == nil {
				return ErrEmptyHandoff
			}
			handoff = h
			return nil
		}()

		if err == nil {
			return handoff, nil
		}
		lastErr = err
		if idx >= attemptCnt-1 {
			break
		}
		if isRetriableModelError(err) {
			log.Printf("feature %s model %s failed retriably: %v — trying next", featureID, attemptModel, err)
			continue
		}
		// Non-retriable error — abort the chain.
		break
	}

	FailFeature(wm.store, mission, featureID)
	if errors.Is(lastErr, ErrWorkerTimeout) {
		return nil, ErrWorkerTimeout
	}
	if errors.Is(lastErr, ErrEmptyHandoff) {
		return nil, ErrEmptyHandoff
	}
	if lastErr == nil {
		return nil, ErrEmptyHandoff
	}
	return nil, lastErr
}

// extractHandoffFromMessages searches assistant messages for a WorkerHandoff JSON.
func extractHandoffFromMessages(messages []opencode.Message) *WorkerHandoff {
	// Walk messages in reverse to find the last assistant message with a handoff.
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg.Role != "assistant" {
			continue
		}
		handoff, err := parseHandoff(msg.Text)
		if err == nil && handoff != nil {
			return handoff
		}
	}
	return nil
}

// parseModelString converts a model string like "provider/model" into a ModelInput.
// Returns nil if the string is empty or cannot be parsed.
func parseModelString(modelStr string) *opencode.ModelInput {
	if modelStr == "" {
		return nil
	}
	parts := strings.SplitN(modelStr, "/", 2)
	if len(parts) == 2 {
		return &opencode.ModelInput{
			ProviderID: parts[0],
			ID:         parts[1],
		}
	}
	return &opencode.ModelInput{ID: modelStr}
}
