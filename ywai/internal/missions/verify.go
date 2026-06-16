package missions

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Verifier is the interface for verifying a feature's implementation.
// Verification runs after the worker completes a feature and before it's marked as complete.
type Verifier interface {
	// Verify runs verification in the given worktree path and returns the result.
	Verify(ctx context.Context, worktreePath string, mission *Mission, feature *Feature) (VerifyResult, error)
}

// CommandVerifier runs build and test commands to verify a feature.
type CommandVerifier struct {
	cmdCreator func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// NewCommandVerifier creates a CommandVerifier.
func NewCommandVerifier() *CommandVerifier {
	return &CommandVerifier{
		cmdCreator: exec.CommandContext,
	}
}

// Verify runs the detected build and test commands in the worktree path.
func (cv *CommandVerifier) Verify(ctx context.Context, worktreePath string, mission *Mission, feature *Feature) (VerifyResult, error) {
	if worktreePath == "" {
		// No worktree — skip verification
		return VerifyResult{Passed: true, Runs: nil, Combined: "no worktree — verification skipped"}, nil
	}

	var manifest *ServicesManifest
	if mission.Artifacts != nil {
		manifest = mission.Artifacts.ServicesManifest
	}
	commands := DetectVerifyCommands(worktreePath, manifest)
	if len(commands) == 0 {
		// No detectable build system — skip
		return VerifyResult{Passed: true, Runs: nil, Combined: "no detectable build system — verification skipped"}, nil
	}

	var runs []VerifyRun
	var outputs []string
	allPassed := true

	for _, cmdStr := range commands {
		parts := strings.Fields(cmdStr)
		if len(parts) == 0 {
			continue
		}
		runAt := time.Now().UTC()
		cmd := cv.cmdCreator(ctx, parts[0], parts[1:]...)
		cmd.Dir = worktreePath

		output, err := cmd.CombinedOutput()
		outputStr := string(output)
		passed := err == nil

		run := VerifyRun{
			Passed:   passed,
			RunAt:    runAt,
			Commands: []CommandRun{{Command: cmdStr, ExitCode: exitCodeFromError(err)}},
			Output:   truncateOutput(outputStr, 4096),
		}
		runs = append(runs, run)
		outputs = append(outputs, outputStr)
		if !passed {
			allPassed = false
		}
	}

	combined := strings.Join(outputs, "\n---\n")
	return VerifyResult{
		Passed:   allPassed,
		Runs:     runs,
		Combined: truncateOutput(combined, 65536),
	}, nil
}

// DetectVerifyCommands detects build and test commands for a given worktree path.
// If a ServicesManifest is provided, it uses the custom commands from services.yaml
// instead of auto-detecting the build system.
// It auto-detects common build systems: Go, npm, etc.
func DetectVerifyCommands(worktreePath string, manifest *ServicesManifest) []string {
	var commands []string

	if manifest != nil {
		var cmds []string
		if b, ok := manifest.GetCommand("build"); ok {
			cmds = append(cmds, b)
		}
		if t, ok := manifest.GetCommand("test"); ok {
			cmds = append(cmds, t)
		}
		if len(cmds) > 0 {
			return cmds
		}
	}

	// Go project
	if _, err := os.Stat(filepath.Join(worktreePath, "go.mod")); err == nil {
		commands = append(commands, "go build ./...")
		commands = append(commands, "go vet ./...")
		commands = append(commands, "go test ./...")
	}

	// Node/npm project
	if _, err := os.Stat(filepath.Join(worktreePath, "package.json")); err == nil {
		commands = append(commands, "npm run build")
		commands = append(commands, "npm test")
	}

	return commands
}

// exitCodeFromError extracts an exit code from an exec.ExitError.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

// E2EVerifier is a pluggable E2E testing interface (FASE 5).
// Implementations can use browser automation, API testing tools, etc.
type E2EVerifier interface {
	// VerifyE2E runs end-to-end tests in the given worktree path.
	VerifyE2E(ctx context.Context, worktreePath string, mission *Mission, feature *Feature) (VerifyResult, error)
}

// CheckCleanStreak checks if a feature has met the required clean verify streak.
// Returns true if the feature has at least `required` consecutive clean runs.
func CheckCleanStreak(feature *Feature, required int) bool {
	if required <= 0 {
		return true
	}
	var consecutive int
	// Count from the end (most recent first)
	for i := len(feature.VerifyRuns) - 1; i >= 0; i-- {
		if feature.VerifyRuns[i].Passed {
			consecutive++
			if consecutive >= required {
				return true
			}
		} else {
			break // streak broken
		}
	}
	return false
}

// RunCleanStreak runs the verifier up to `required` consecutive passing times.
// It is the FASE 5 clean-streak gate: a feature is only considered verified
// when the verifier passes `required` times in a row.
//
// Behaviour:
//   - required <= 0 → returns a passing result without invoking the verifier.
//   - A verifier error aborts the loop and is returned wrapped.
//   - Any failing run aborts the loop and returns the failing VerifyResult
//     together with an error so callers can trigger self-correction/retry.
//
// Each run's VerifyRuns are NOT mutated onto the feature here; the caller owns
// persistence so it can fold them into a single atomic save.
func RunCleanStreak(ctx context.Context, verifier Verifier, worktreePath string, mission *Mission, feature *Feature, required int) (VerifyResult, error) {
	if required <= 0 {
		return VerifyResult{Passed: true, Combined: "clean streak disabled"}, nil
	}

	var combined []string
	for i := 0; i < required; i++ {
		if err := ctx.Err(); err != nil {
			return VerifyResult{Passed: false}, err
		}
		res, err := verifier.Verify(ctx, worktreePath, mission, feature)
		if err != nil {
			return res, fmt.Errorf("clean streak verify (run %d/%d): %w", i+1, required, err)
		}
		if !res.Passed {
			// Streak broken: surface the failure so the caller retries the worker.
			return res, fmt.Errorf("clean streak broken on run %d/%d", i+1, required)
		}
		combined = append(combined, res.Combined)
	}
	return VerifyResult{
		Passed:   true,
		Runs:     nil, // individual runs are owned by the caller's persistence
		Combined: strings.Join(combined, "\n---\n"),
	}, nil
}

// truncateOutput truncates a string to maxLen bytes for storage.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}

// RunVerifyOnFeature runs verification on a feature, persists the result, and
// transitions the feature to failed if verification fails.
//
// When required > 1, the FASE 5 clean-streak loop runs the verifier up to
// `required` consecutive passing times (see RunCleanStreak). required <= 1
// keeps the legacy single-run behaviour.
func RunVerifyOnFeature(store *MissionsStore, verifier Verifier, mission *Mission, feature *Feature, required int) (VerifyResult, error) {
	if required < 0 {
		required = 0
	}
	worktreePath := feature.WorktreePath
	if worktreePath == "" {
		// Fall back to mission artifacts dir for context files
		worktreePath = store.MissionDir(mission.ID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var result VerifyResult
	var err error
	if required > 1 {
		// Clean-streak loop (FASE 5): require N consecutive clean passes.
		result, err = RunCleanStreak(ctx, verifier, worktreePath, mission, feature, required)
	} else {
		result, err = verifier.Verify(ctx, worktreePath, mission, feature)
	}
	if err != nil {
		return result, fmt.Errorf("verification error: %w", err)
	}

	// Persist verify runs
	feature.VerifyRuns = append(feature.VerifyRuns, result.Runs...)
	mission.UpdatedAt = time.Now().UTC()

	if !result.Passed {
		log.Printf("Verification FAILED for feature %s: %s", feature.ID, result.Combined[:min(len(result.Combined), 200)])
		feature.LastError = result.Combined

		// Transition to failed
		_, fErr := FailFeature(store, mission, feature.ID)
		if fErr != nil {
			log.Printf("Warning: could not fail feature %s after verify fail: %v", feature.ID, fErr)
		}
	}

	return result, nil
}
