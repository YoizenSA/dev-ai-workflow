package evals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goldenTasks are the synthetic tasks the scoring fixtures score against. They
// live here — not in tasks/*.json — so a fixture's expectations are pinned to
// this file: editing a needle, a flag, or a weight below must turn
// TestScoreGoldenFixtures red against the checked-in expected Score.
var goldenTasks = map[string]Task{
	"golden-mixed": {
		ID: "golden-mixed", Name: "Golden mixed defaults", Agent: "finder", Brief: "score golden fixtures",
		Expect: []Expectation{
			{Needle: "config.load", Label: "config.load"},
			{Needle: "retryloop", Label: "retryLoop", Hard: true},
			{Needle: "shutdown", Label: "graceful shutdown"},
		},
	},
	"golden-weights": {
		ID: "golden-weights", Name: "Golden explicit weights", Agent: "finder", Brief: "score golden fixtures",
		Expect: []Expectation{
			{Needle: "ratelimiter", Label: "rateLimiter", Hard: true, Weight: 5},
			{Needle: "backoff", Label: "exponential backoff", Weight: 3},
			{Needle: "jitter", Label: "jitter"},
		},
	},
}

// goldenFixture is one file of testdata/scoring: a raw response and the exact
// Score the task must produce for it.
type goldenFixture struct {
	Name     string `json:"name"`
	TaskID   string `json:"taskId"`
	Response string `json:"response"`
	Expected Score  `json:"expected"`
}

// TestScoreGoldenFixtures replays every fixture in testdata/scoring against
// its named synthetic task and compares the full Score — hits and missed in
// order, the answered rule, and the weighted share to a 1e-9 tolerance, so a
// mutated weight or needle cannot pass silently.
func TestScoreGoldenFixtures(t *testing.T) {
	entries, err := filepath.Glob(filepath.Join("testdata", "scoring", "*.json"))
	if err != nil {
		t.Fatalf("glob scoring fixtures: %v", err)
	}
	if len(entries) < 6 {
		t.Fatalf("got %d scoring fixtures in testdata/scoring, want at least 6", len(entries))
	}
	seen := map[string]bool{}
	for _, path := range entries {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var fx goldenFixture
		if err := json.Unmarshal(raw, &fx); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if fx.Name == "" {
			fx.Name = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		if seen[fx.Name] {
			t.Errorf("%s: duplicate fixture name %q", path, fx.Name)
		}
		seen[fx.Name] = true
		t.Run(fx.Name, func(t *testing.T) {
			task, ok := goldenTasks[fx.TaskID]
			if !ok {
				t.Fatalf("fixture names unknown task %q", fx.TaskID)
			}
			if err := task.validate(); err != nil {
				t.Fatalf("golden task %q invalid: %v", fx.TaskID, err)
			}
			assertScoreEqual(t, task.Score(fx.Response), fx.Expected)
		})
	}
}

// assertScoreEqual deep-compares got against the fixture's expected Score.
// Slice order is part of the contract (hits follow expectation order), and the
// weighted share must match within the same 1e-9 tolerance the pricing tests
// use.
func assertScoreEqual(t *testing.T, got, want Score) {
	t.Helper()
	if !sameStrings(got.Hits, want.Hits) {
		t.Errorf("hits = %v, want %v", got.Hits, want.Hits)
	}
	if !sameStrings(got.Missed, want.Missed) {
		t.Errorf("missed = %v, want %v", got.Missed, want.Missed)
	}
	if got.Total != want.Total {
		t.Errorf("total = %d, want %d", got.Total, want.Total)
	}
	if got.GotHard != want.GotHard {
		t.Errorf("gotHard = %v, want %v", got.GotHard, want.GotHard)
	}
	if got.Answered != want.Answered {
		t.Errorf("answered = %v, want %v", got.Answered, want.Answered)
	}
	if !costNearlyEqual(got.Weighted, want.Weighted) {
		t.Errorf("weighted = %v, want %v", got.Weighted, want.Weighted)
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
