package missions

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeVerifier is a programmable Verifier stub. It returns a queued sequence of
// results, one per Verify() call, so tests can drive pass/fail transitions.
type fakeVerifier struct {
	results []VerifyResult
	calls   int
}

func (f *fakeVerifier) Verify(ctx context.Context, worktreePath string, mission *Mission, feature *Feature) (VerifyResult, error) {
	idx := f.calls
	f.calls++
	if idx >= len(f.results) {
		// Default to passing once we exhaust the scripted results.
		return VerifyResult{Passed: true, Runs: []VerifyRun{{Passed: true, RunAt: time.Now()}}}, nil
	}
	return f.results[idx], nil
}

// TestRunCleanStreakPassImmediately verifies that a required streak of N is met
// when the verifier passes N times in a row from the first call.
func TestRunCleanStreakPassImmediately(t *testing.T) {
	v := &fakeVerifier{results: []VerifyResult{
		{Passed: true, Runs: []VerifyRun{{Passed: true}}},
		{Passed: true, Runs: []VerifyRun{{Passed: true}}},
		{Passed: true, Runs: []VerifyRun{{Passed: true}}},
	}}

	feat := &Feature{ID: "f1"}
	res, err := RunCleanStreak(context.Background(), v, "/wt", &Mission{}, feat, 3)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !res.Passed {
		t.Fatal("expected streak to pass")
	}
	if v.calls != 3 {
		t.Errorf("expected 3 verify calls, got %d", v.calls)
	}
}

// TestRunCleanStreakFailsAndStops verifies the loop stops at the first failure
// and returns the failing result.
func TestRunCleanStreakFailsAndStops(t *testing.T) {
	v := &fakeVerifier{results: []VerifyResult{
		{Passed: true, Runs: []VerifyRun{{Passed: true}}},
		{Passed: false, Runs: []VerifyRun{{Passed: false, Output: "test fail"}}},
		{Passed: true, Runs: []VerifyRun{{Passed: true}}},
	}}

	feat := &Feature{ID: "f1"}
	res, err := RunCleanStreak(context.Background(), v, "/wt", &Mission{}, feat, 3)

	if err == nil {
		t.Fatal("expected error on streak failure, got nil")
	}
	if res.Passed {
		t.Fatal("expected result to be failed")
	}
	if v.calls != 2 {
		t.Errorf("expected loop to stop after 2 calls (pass then fail), got %d", v.calls)
	}
}

// TestRunCleanStreakRequiredOne verifies the common case (required=1) runs once.
func TestRunCleanStreakRequiredOne(t *testing.T) {
	v := &fakeVerifier{results: []VerifyResult{
		{Passed: true, Runs: []VerifyRun{{Passed: true}}},
	}}

	feat := &Feature{ID: "f1"}
	res, err := RunCleanStreak(context.Background(), v, "/wt", &Mission{}, feat, 1)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !res.Passed {
		t.Fatal("expected pass")
	}
	if v.calls != 1 {
		t.Errorf("expected 1 call for required=1, got %d", v.calls)
	}
}

// TestRunCleanStreakZeroRequiredIsNoop verifies required<=0 returns passed
// without invoking the verifier.
func TestRunCleanStreakZeroRequiredIsNoop(t *testing.T) {
	v := &fakeVerifier{}
	feat := &Feature{ID: "f1"}
	res, err := RunCleanStreak(context.Background(), v, "/wt", &Mission{}, feat, 0)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !res.Passed {
		t.Fatal("expected pass when streak disabled")
	}
	if v.calls != 0 {
		t.Errorf("expected 0 verifier calls when streak disabled, got %d", v.calls)
	}
}

// TestRunCleanStreakVerifierError verifies a verifier error aborts the loop.
func TestRunCleanStreakVerifierError(t *testing.T) {
	errV := &erroringVerifier{err: errors.New("boom")}
	feat := &Feature{ID: "f1"}
	_, err := RunCleanStreak(context.Background(), errV, "/wt", &Mission{}, feat, 2)

	if err == nil {
		t.Fatal("expected verifier error to propagate")
	}
}

// erroringVerifier always returns an error.
type erroringVerifier struct{ err error }

func (e *erroringVerifier) Verify(ctx context.Context, worktreePath string, mission *Mission, feature *Feature) (VerifyResult, error) {
	return VerifyResult{}, e.err
}
