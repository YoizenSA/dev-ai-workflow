package missions

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ─── Errors ────────────────────────────────────────────────────────────────

var (
	ErrOpencodeNotFound = errors.New("opencode binary not found in PATH")
	ErrWorkerTimeout    = errors.New("worker timed out")
	ErrWorkerCancelled  = errors.New("worker cancelled")
	ErrEmptyHandoff     = errors.New("worker produced empty handoff")
	ErrInvalidHandoff   = errors.New("worker produced invalid handoff")
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
	Timeout    time.Duration
	MaxRetries int
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

// WorkerManager manages spawning opencode workers for features.
type WorkerManager struct {
	store       *MissionsStore
	config      WorkerConfig
	cmdCreator  func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewWorkerManager creates a new WorkerManager.
func NewWorkerManager(store *MissionsStore, config WorkerConfig) *WorkerManager {
	return &WorkerManager{
		store:      store,
		config:     config,
		cmdCreator: exec.CommandContext,
	}
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

// PrepareContext creates the working directory for a worker with all necessary
// context files. Returns the path to the context directory.
//
// The context directory includes:
//   - feature.md: Feature description and expected behaviors
//   - mission.md: Mission overview and context
//   - AGENTS.md: Worker constraints and handoff format specification
func (wm *WorkerManager) PrepareContext(mission *Mission, feature *Feature) (string, error) {
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("ywai-worker-%s-%s-*", mission.ID, feature.ID))
	if err != nil {
		return "", fmt.Errorf("create worker temp dir: %w", err)
	}

	// Write feature.md
	var featureBld strings.Builder
	featureBld.WriteString(fmt.Sprintf("# Feature: %s\n\n", feature.ID))
	featureBld.WriteString(fmt.Sprintf("%s\n\n", feature.Description))
	featureBld.WriteString("## Expected Behavior\n")
	for _, b := range feature.ExpectedBehavior {
		featureBld.WriteString(fmt.Sprintf("- %s\n", b))
	}
	featureBld.WriteString(fmt.Sprintf("\n## Skill\n%s\n", feature.SkillName))
	featureBld.WriteString(fmt.Sprintf("\n## Milestone\n%s\n", feature.Milestone))
	if len(feature.Fulfills) > 0 {
		featureBld.WriteString("\n## Fulfills\n")
		for _, f := range feature.Fulfills {
			featureBld.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "feature.md"), []byte(featureBld.String()), 0644); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("write feature.md: %w", err)
	}

	// Write mission.md
	missionMD := fmt.Sprintf("# %s\n\n%s\n\nMission ID: %s\nStatus: %s\n",
		mission.Name, mission.Name, mission.ID, mission.Status)
	// Add milestone descriptions
	for _, ms := range mission.Milestones {
		missionMD += fmt.Sprintf("\n## Milestone: %s\n%s\n", ms.Name, ms.Description)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "mission.md"), []byte(missionMD), 0644); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("write mission.md: %w", err)
	}

	// Write AGENTS.md with worker instructions and handoff format
	agentsMD := `# Worker Instructions

You are a worker in the ywai Missions system. Your task is to implement the feature described in feature.md.

## Context
- feature.md: The feature you need to implement
- mission.md: Mission overview

## Rules
1. Follow existing codebase patterns
2. Run tests before completing your work
3. Return a structured handoff JSON as the LAST line of stdout

## Handoff Format
The final line of stdout MUST be a single JSON object with the following structure:
{
  "salientSummary": "1-4 sentence summary of what was accomplished",
  "whatWasImplemented": "Detailed description of what was implemented",
  "whatWasLeftUndone": "What was left incomplete (empty string if complete)",
  "verification": {
    "commandsRun": [
      {"command": "command that was run", "exitCode": 0, "observation": "what happened"}
    ]
  },
  "tests": {
    "added": [{"file": "path/to/test", "cases": [{"name": "TestName", "verifies": "what it verifies"}]}],
    "coverage": "summary of coverage"
  },
  "discoveredIssues": []
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte(agentsMD), 0644); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("write AGENTS.md: %w", err)
	}

	// Create the mission's worker artifacts directory for logs
	workersDir := filepath.Join(wm.store.MissionDir(mission.ID), "workers", feature.ID)
	if err := os.MkdirAll(workersDir, 0755); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("create workers artifact dir: %w", err)
	}

	return tmpDir, nil
}

// ─── Worker Spawning ───────────────────────────────────────────────────────

// SpawnWorker spawns an opencode process to work on the given feature.
// Returns a cancel function (to kill the process), a channel for WorkerResult,
// and any error encountered during spawning.
func (wm *WorkerManager) SpawnWorker(mission *Mission, feature *Feature, contextDir string) (cancel func(), resultCh <-chan WorkerResult, err error) {
	opencodePath, err := DetectOpencode()
	if err != nil {
		return nil, nil, err
	}

	// Create a cancellable context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), wm.config.Timeout)

	// Build the opencode command
	args := []string{"--skill", "backend-worker"}
	if feature.SkillName != "" && feature.SkillName != "backend-worker" {
		args = []string{"--skill", feature.SkillName}
	}
	args = append(args, "implement", feature.ID)

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
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(multiWriter, line)
		}
	}()

	// Read stderr and write to log
	io.Copy(multiWriter, stderr)

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

	// Verify it looks like a JSON object
	if !strings.HasPrefix(lastLine, "{") {
		return nil, fmt.Errorf("%w: last output line is not JSON", ErrInvalidHandoff)
	}

	var handoff WorkerHandoff
	if err := json.Unmarshal([]byte(lastLine), &handoff); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidHandoff, err)
	}

	// Validate required fields: at least one of salientSummary or
	// whatWasImplemented must be non-empty.
	if handoff.SalientSummary == "" && handoff.WhatWasImplemented == "" {
		return nil, fmt.Errorf("%w: missing required fields (salientSummary, whatWasImplemented)", ErrInvalidHandoff)
	}

	return &handoff, nil
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
func (wm *WorkerManager) ExecuteFeature(mission *Mission, featureID string) (*WorkerHandoff, error) {
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
	contextDir, err := wm.PrepareContext(mission, feat)
	if err != nil {
		FailFeature(wm.store, mission, featureID)
		return nil, fmt.Errorf("prepare context: %w", err)
	}
	defer os.RemoveAll(contextDir)

	// Spawn worker
	cancel, resultCh, err := wm.SpawnWorker(mission, feat, contextDir)
	if err != nil {
		FailFeature(wm.store, mission, featureID)
		// Special case: if opencode was just found but now isn't
		if errors.Is(err, ErrOpencodeNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("spawn worker: %w", err)
	}
	defer cancel()

	// Wait for result
	result := <-resultCh

	// Handle worker errors
	if result.Err != nil {
		// Timeout or cancellation
		if errors.Is(result.Err, ErrWorkerTimeout) {
			FailFeature(wm.store, mission, featureID)
			return nil, ErrWorkerTimeout
		}
		if errors.Is(result.Err, ErrWorkerCancelled) {
			FailFeature(wm.store, mission, featureID)
			return nil, ErrWorkerCancelled
		}

		// Non-zero exit
		if result.ExitCode != 0 {
			FailFeature(wm.store, mission, featureID)
			return nil, fmt.Errorf("worker exited with code %d", result.ExitCode)
		}

		// Invalid or empty handoff
		if errors.Is(result.Err, ErrEmptyHandoff) || errors.Is(result.Err, ErrInvalidHandoff) {
			FailFeature(wm.store, mission, featureID)
			return nil, result.Err
		}

		// Generic error
		FailFeature(wm.store, mission, featureID)
		return nil, result.Err
	}

	// Verify handoff is present
	if result.Handoff == nil {
		FailFeature(wm.store, mission, featureID)
		return nil, ErrEmptyHandoff
	}

	// Complete the feature
	if _, err := CompleteFeature(wm.store, mission, featureID); err != nil {
		return nil, fmt.Errorf("complete feature: %w", err)
	}

	return result.Handoff, nil
}
