package evals

import "testing"

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
