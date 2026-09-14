package main

import (
	"fmt"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/evals"
)

func fptr(f float64) *float64 { return &f }

func gateSummaries() []evals.ModelTaskSummary {
	return []evals.ModelTaskSummary{
		{Model: "model-a", Attempts: 2, AvgWeighted: 0.875, HardRate: 1.0, AvgTurns: 7, TotalCostUSD: 1.2, CostKnown: true},
		{Model: "model-b", Attempts: 1, AvgWeighted: 0.5, HardRate: 0.0, AvgTurns: 3, TotalCostUSD: 0, CostKnown: false},
	}
}

func TestBenchExitCode(t *testing.T) {
	if got := benchExitCode(nil); got != benchExitPass {
		t.Fatalf("nil err = %d, want %d", got, benchExitPass)
	}
	if got := benchExitCode(benchGateMiss("nope")); got != benchExitGateMiss {
		t.Fatalf("gate miss = %d, want %d", got, benchExitGateMiss)
	}
	if got := benchExitCode(benchHarnessErr("boom")); got != benchExitHarnessErr {
		t.Fatalf("harness err = %d, want %d", got, benchExitHarnessErr)
	}
	// Wrapping must preserve the class: the exit code is a property of where
	// the error was raised, not of its outermost layer.
	wrapped := fmt.Errorf("run failed: %w", benchGateMiss("nope"))
	if got := benchExitCode(wrapped); got != benchExitGateMiss {
		t.Fatalf("wrapped gate miss = %d, want %d", got, benchExitGateMiss)
	}
	// A foreign error is a harness fault, never a gate miss.
	if got := benchExitCode(fmt.Errorf("plain")); got != benchExitHarnessErr {
		t.Fatalf("plain err = %d, want %d", got, benchExitHarnessErr)
	}
}

func TestBenchCheckGates_MinScore(t *testing.T) {
	cfg := benchConfig{minScore: 0.8}
	if err := benchCheckGates(cfg, []string{"model-a"}, nil, gateSummaries(), nil, false); err != nil {
		t.Fatalf("passing model gated: %v", err)
	}
	err := benchCheckGates(cfg, []string{"model-a", "model-b"}, nil, gateSummaries(), nil, false)
	if err == nil {
		t.Fatal("model-b at 0.5 must miss --min-score 0.8")
	}
	if got := benchExitCode(err); got != 1 {
		t.Fatalf("gate miss exit = %d, want 1", got)
	}
	// A model with no summary has no score at all: silence is not a pass.
	if err := benchCheckGates(cfg, []string{"ghost"}, nil, gateSummaries(), nil, false); err == nil {
		t.Fatal("unscored model must miss --min-score 0.8")
	}
}

func TestBenchCheckGates_RequireHard(t *testing.T) {
	attempts := []evals.Attempt{
		{Model: "m", Round: 1, Score: evals.Score{Answered: true, GotHard: true}},
		{Model: "m", Round: 2, Score: evals.Score{Answered: false, GotHard: false}},
	}
	cfg := benchConfig{requireHard: true}
	if err := benchCheckGates(cfg, []string{"m"}, attempts, gateSummaries(), nil, false); err != nil {
		t.Fatalf("hard-hit + unanswered must pass: %v", err)
	}
	attempts[0].Score.GotHard = false
	if err := benchCheckGates(cfg, []string{"m"}, attempts, gateSummaries(), nil, false); err == nil {
		t.Fatal("answered attempt missing hard must miss --require-hard")
	}
	// Gate off: the same attempts pass.
	cfg.requireHard = false
	if err := benchCheckGates(cfg, []string{"m"}, attempts, gateSummaries(), nil, false); err != nil {
		t.Fatalf("require-hard off must pass: %v", err)
	}
}

func TestBenchCheckGates_Regression(t *testing.T) {
	deltas := []evals.ModelDelta{
		{Model: "model-a", WeightedDelta: fptr(-0.02)},
		{Model: "model-b", WeightedDelta: nil}, // no baseline: no comparison, no fail
	}
	cfg := benchConfig{maxRegression: 0.05}
	if err := benchCheckGates(cfg, []string{"model-a"}, nil, gateSummaries(), deltas, true); err != nil {
		t.Fatalf("-0.02 within 0.05 must pass: %v", err)
	}
	deltas[0].WeightedDelta = fptr(-0.06)
	if err := benchCheckGates(cfg, []string{"model-a"}, nil, gateSummaries(), deltas, true); err == nil {
		t.Fatal("-0.06 beyond 0.05 must miss --max-regression")
	}
	// Without a baseline the same deltas are ignored entirely.
	if err := benchCheckGates(cfg, []string{"model-a"}, nil, gateSummaries(), deltas, false); err != nil {
		t.Fatalf("no baseline must ignore deltas: %v", err)
	}
}

func TestBenchCheckGates_Cost(t *testing.T) {
	cfg := benchConfig{maxCostUSD: 2.0}
	if err := benchCheckGates(cfg, []string{"model-a", "model-b"}, nil, gateSummaries(), nil, false); err != nil {
		t.Fatalf("1.2 under 2.0 + unpriced model must pass: %v", err)
	}
	cfg.maxCostUSD = 1.0
	if err := benchCheckGates(cfg, []string{"model-a"}, nil, gateSummaries(), nil, false); err == nil {
		t.Fatal("1.2 over 1.0 with known cost must miss --max-cost-usd")
	}
	// Unpriced models never trip the gate, however high the real cost is.
	// (Summaries under test carry only the model under gate.)
	unpriced := []evals.ModelTaskSummary{gateSummaries()[1]}
	if err := benchCheckGates(cfg, []string{"model-b"}, nil, unpriced, nil, false); err != nil {
		t.Fatalf("unknown cost must skip the gate: %v", err)
	}
	// Gate off (0) passes regardless.
	cfg.maxCostUSD = 0
	if err := benchCheckGates(cfg, []string{"model-a"}, nil, gateSummaries(), nil, false); err != nil {
		t.Fatalf("cost gate off must pass: %v", err)
	}
}
