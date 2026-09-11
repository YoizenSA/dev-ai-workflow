package evals

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

var lbNow = time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

func lbAttempt(model string, weighted float64, answered, gotHard, costKnown bool, cost float64) Attempt {
	return Attempt{
		Model:     model,
		Score:     Score{Answered: answered, Weighted: weighted, GotHard: gotHard},
		CostUSD:   cost,
		CostKnown: costKnown,
	}
}

func lbRun(id, taskID string, startedAt time.Time, attempts ...Attempt) Run {
	return Run{ID: id, TaskID: taskID, StartedAt: startedAt, Attempts: attempts}
}

func TestBuildLeaderboard_Windowing(t *testing.T) {
	runs := []Run{
		lbRun("old", "t1", lbNow.AddDate(0, 0, -10), lbAttempt("m1", 0.0, true, false, true, 1.0)),
		lbRun("new", "t1", lbNow.AddDate(0, 0, -1), lbAttempt("m1", 1.0, true, true, true, 3.0)),
	}

	row := BuildLeaderboard(runs, "t1", 7, lbNow)
	if len(row) != 1 {
		t.Fatalf("rows=%d want 1", len(row))
	}
	if row[0].Runs != 1 {
		t.Fatalf("Runs=%d want 1 (10-day-old run is outside the window)", row[0].Runs)
	}
	if row[0].AvgWeighted != 1.0 || row[0].HardRate != 1.0 || row[0].AvgCostUSD != 3.0 {
		t.Fatalf("windowed row = %+v, want only the new run's numbers", row[0])
	}

	all := BuildLeaderboard(runs, "t1", 0, lbNow)
	if all[0].Runs != 2 || all[0].AvgWeighted != 0.5 || all[0].HardRate != 0.5 || all[0].AvgCostUSD != 2.0 {
		t.Fatalf("all-time row = %+v, want both runs averaged", all[0])
	}
}

func TestBuildLeaderboard_TrendOrder(t *testing.T) {
	var runs []Run
	for i := 1; i <= 7; i++ {
		runs = append(runs, lbRun(fmt.Sprintf("r%d", i), "t1",
			lbNow.Add(time.Duration(i)*time.Hour),
			lbAttempt("m1", float64(i)/10, true, false, true, 0)))
	}
	row := BuildLeaderboard(runs, "t1", 0, lbNow)
	if row[0].Runs != 7 {
		t.Fatalf("Runs=%d want 7", row[0].Runs)
	}
	want := []float64{0.3, 0.4, 0.5, 0.6, 0.7} // last five, oldest first
	if !reflect.DeepEqual(row[0].Trend, want) {
		t.Fatalf("Trend=%v want %v", row[0].Trend, want)
	}
}

func TestBuildLeaderboard_TrendPerRunAverage(t *testing.T) {
	runs := []Run{
		lbRun("r1", "t1", lbNow.Add(-2*time.Hour),
			lbAttempt("m1", 1.0, true, false, true, 0),
			lbAttempt("m1", 0.0, true, false, true, 0)),
		lbRun("r2", "t1", lbNow.Add(-1*time.Hour),
			lbAttempt("m1", 0.8, true, false, true, 0)),
	}
	row := BuildLeaderboard(runs, "t1", 0, lbNow)
	want := []float64{0.5, 0.8} // r1's two attempts average into one point
	if !reflect.DeepEqual(row[0].Trend, want) {
		t.Fatalf("Trend=%v want %v", row[0].Trend, want)
	}
}

func TestBuildLeaderboard_EmptyWindow(t *testing.T) {
	runs := []Run{
		lbRun("old", "t1", lbNow.AddDate(0, 0, -30), lbAttempt("m1", 1.0, true, false, true, 0)),
	}
	rows := BuildLeaderboard(runs, "t1", 7, lbNow)
	if rows == nil {
		t.Fatal("rows is nil, want empty non-nil slice")
	}
	if len(rows) != 0 {
		t.Fatalf("rows=%d want 0", len(rows))
	}
}

func TestBuildLeaderboard_CostKnownAND(t *testing.T) {
	runs := []Run{
		lbRun("r1", "t1", lbNow.Add(-time.Hour),
			lbAttempt("known", 1.0, true, false, true, 2.0),
			lbAttempt("known", 1.0, true, false, true, 4.0),
			lbAttempt("partial", 1.0, true, false, true, 0),
			lbAttempt("partial", 1.0, true, false, false, 0),
			lbAttempt("silent", 0.9, false, false, true, 1.0),
		),
	}
	rows := BuildLeaderboard(runs, "t1", 0, lbNow)
	if len(rows) != 2 {
		t.Fatalf("rows=%d want 2 (an unanswered attempt makes no row)", len(rows))
	}
	byModel := map[string]LeaderRow{}
	for _, r := range rows {
		byModel[r.Model] = r
	}
	if _, ok := byModel["silent"]; ok {
		t.Fatal("unanswered attempt must not create a row")
	}
	known := byModel["known"]
	if !known.CostKnown || known.AvgCostUSD != 3.0 {
		t.Fatalf("known = %+v, want CostKnown with AvgCostUSD 3.0", known)
	}
	if partial := byModel["partial"]; partial.CostKnown {
		t.Fatalf("partial = %+v, want CostKnown=false when any counted attempt lacks a price", partial)
	}
}

func TestBuildLeaderboard_DeterministicTies(t *testing.T) {
	runs := []Run{
		lbRun("r1", "t1", lbNow.Add(-time.Hour),
			lbAttempt("zeta", 0.7, true, false, true, 0),
			lbAttempt("alpha", 0.7, true, false, true, 0),
		),
	}
	rows := BuildLeaderboard(runs, "t1", 0, lbNow)
	if rows[0].Model != "alpha" || rows[1].Model != "zeta" {
		t.Fatalf("tie order = [%s, %s], want [alpha, zeta] (model asc)", rows[0].Model, rows[1].Model)
	}
}

func TestBuildLeaderboard_TaskFilter(t *testing.T) {
	runs := []Run{
		lbRun("r1", "t1", lbNow.Add(-time.Hour), lbAttempt("m1", 1.0, true, false, true, 0)),
		lbRun("r2", "t2", lbNow.Add(-time.Hour), lbAttempt("m1", 0.0, true, false, true, 0)),
	}

	one := BuildLeaderboard(runs, "t1", 0, lbNow)
	if len(one) != 1 || one[0].Runs != 1 || one[0].AvgWeighted != 1.0 {
		t.Fatalf("task-scoped = %+v, want only t1's run", one)
	}

	all := BuildLeaderboard(runs, "", 0, lbNow)
	if len(all) != 1 || all[0].Runs != 2 || all[0].AvgWeighted != 0.5 {
		t.Fatalf("empty taskID = %+v, want both tasks ranked together", all)
	}
}
