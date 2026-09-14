package evals

import (
	"encoding/json"
	"testing"
)

func TestLoadTasksParsesBuiltins(t *testing.T) {
	tasks, err := LoadTasks("")
	if err != nil {
		t.Fatalf("LoadTasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("no built-in tasks embedded")
	}
	for _, task := range tasks {
		if err := task.validate(); err != nil {
			t.Errorf("built-in task invalid: %v", err)
		}
	}
}

func TestScoreCountsHitsAndHard(t *testing.T) {
	task := Task{
		ID: "x", Agent: "finder", Brief: "b",
		Expect: []Expectation{
			{Needle: "handletimeout", Label: "handleTimeout"},
			{Needle: "maybecleanupchildsession", Label: "maybeCleanup", Hard: true},
			{Needle: "metadata.ts", Label: "metadata"},
		},
	}

	// Matching is case-insensitive, so a model's own capitalisation cannot cost it points.
	full := task.Score("Found handleTimeout, maybeCleanupChildSession and metadata.ts here.")
	if len(full.Hits) != 3 || !full.GotHard || !full.Answered {
		t.Fatalf("complete answer mis-scored: %+v", full)
	}

	partial := task.Score("Only handleTimeout and metadata.ts are relevant in this plugin.")
	if len(partial.Hits) != 2 || partial.GotHard {
		t.Fatalf("partial answer mis-scored: %+v", partial)
	}
	if len(partial.Missed) != 1 || partial.Missed[0] != "maybeCleanup" {
		t.Fatalf("missed list wrong: %+v", partial.Missed)
	}
}

func TestScoreTreatsEmptyAndErrorAsUnanswered(t *testing.T) {
	task := Task{ID: "x", Agent: "finder", Brief: "b",
		Expect: []Expectation{{Needle: "handletimeout", Label: "handleTimeout"}}}

	// A run that produced nothing is a missing measurement, not a wrong answer — and a
	// harness error string must never be mined for accidental keyword hits.
	for _, resp := range []string{"", "   ", "ERROR: timed out"} {
		s := task.Score(resp)
		if s.Answered {
			t.Errorf("response %q should count as unanswered", resp)
		}
		if len(s.Hits) != 0 {
			t.Errorf("response %q must not score hits: %+v", resp, s.Hits)
		}
	}
}

func TestValidateRejectsUnscorableTask(t *testing.T) {
	if err := (Task{ID: "a", Agent: "finder", Brief: "b"}).validate(); err == nil {
		t.Error("a task with no expectations cannot be scored and must be rejected")
	}
	if err := (Task{ID: "a", Brief: "b", Expect: []Expectation{{Needle: "x"}}}).validate(); err == nil {
		t.Error("a task with no agent must be rejected")
	}
}

func TestWeightedScoreCountsHardDouble(t *testing.T) {
	// One soft and one hard expectation weigh 1 and 2, so hitting only the soft one
	// is a third of the weighted score even though it is half the raw score.
	task := Task{ID: "x", Agent: "finder", Brief: "b",
		Expect: []Expectation{
			{Needle: "alpha", Label: "alpha"},
			{Needle: "beta", Label: "beta", Hard: true},
		}}

	if got := task.Score("alpha is mentioned here in a long enough response.").Weighted; got != 1.0/3.0 {
		t.Errorf("soft-only answer should score 1/3 weighted, got %v", got)
	}
	if got := task.Score("beta appears here, the other one never does, in this response.").Weighted; got != 2.0/3.0 {
		t.Errorf("hard-only answer should score 2/3 weighted, got %v", got)
	}
	if got := task.Score("alpha and beta both appear in this long enough response.").Weighted; got != 1 {
		t.Errorf("complete answer should score 1 weighted, got %v", got)
	}
}

func TestWeightedScoreHonorsExplicitWeight(t *testing.T) {
	// An explicit weight overrides the defaults, including on a hard expectation —
	// the author, not the hard flag, decides how heavy it is.
	task := Task{ID: "x", Agent: "finder", Brief: "b",
		Expect: []Expectation{
			{Needle: "alpha", Label: "alpha", Hard: true, Weight: 5},
			{Needle: "beta", Label: "beta"},
		}}

	if got := task.Score("only alpha is mentioned in this long enough response.").Weighted; got != 5.0/6.0 {
		t.Errorf("explicit weight 5 should beat the hard default of 2, got %v", got)
	}
	if got := task.Score("only beta is mentioned in this long enough response.").Weighted; got != 1.0/6.0 {
		t.Errorf("soft expectation should carry the remaining 1/6, got %v", got)
	}
}

func TestWeightedScoreIsZeroWhenUnanswered(t *testing.T) {
	task := Task{ID: "x", Agent: "finder", Brief: "b",
		Expect: []Expectation{
			{Needle: "alpha", Label: "alpha", Hard: true},
			{Needle: "beta", Label: "beta"},
		}}

	// An errored response must not leak needle hits into the weighted score either.
	for _, resp := range []string{"", "   ", "ERROR: timed out"} {
		s := task.Score(resp)
		if s.Answered {
			t.Errorf("response %q should count as unanswered", resp)
		}
		if s.Weighted != 0 {
			t.Errorf("response %q must score weighted 0, got %v", resp, s.Weighted)
		}
	}
}

func TestTaggedBuiltinTaskLoads(t *testing.T) {
	// The one built-in task now carries tags and a difficulty; classification must
	// not have broken its parse, its validation, or its needles.
	task, err := FindTask("", "find-session-deletes")
	if err != nil {
		t.Fatalf("FindTask(find-session-deletes): %v", err)
	}
	if err := task.validate(); err != nil {
		t.Fatalf("tagged built-in task invalid: %v", err)
	}
	if task.Difficulty != "hard" {
		t.Errorf("find-session-deletes difficulty = %q, want hard", task.Difficulty)
	}
	if len(task.Tags) == 0 {
		t.Error("find-session-deletes has no tags")
	}
	if len(task.Expect) != 5 || !task.Expect[2].Hard {
		t.Fatalf("expect needles changed: %+v", task.Expect)
	}
}

func TestScoreUnmarshalsLegacyRunWithoutWeighted(t *testing.T) {
	// A run JSON scored before Phase 1 has no weighted field; decoding it must
	// succeed with Weighted at its zero value so history stays readable.
	legacy := `{"hits":["alpha"],"missed":["beta"],"total":2,"gotHard":false,"answered":true}`
	var s Score
	if err := json.Unmarshal([]byte(legacy), &s); err != nil {
		t.Fatalf("legacy score JSON: %v", err)
	}
	if s.Total != 2 || !s.Answered || len(s.Hits) != 1 || s.Weighted != 0 {
		t.Errorf("legacy score decoded wrong: %+v", s)
	}
}

func TestValidateRejectsUnknownDifficulty(t *testing.T) {
	task := Task{ID: "a", Agent: "finder", Brief: "b", Difficulty: "brutal",
		Expect: []Expectation{{Needle: "x"}}}
	if err := task.validate(); err == nil {
		t.Error("an unknown difficulty must be rejected so typos cannot fragment run groups")
	}
	task.Difficulty = "medium"
	if err := task.validate(); err != nil {
		t.Errorf("medium is a valid difficulty: %v", err)
	}
}
