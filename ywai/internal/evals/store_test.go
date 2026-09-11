package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func makeRun(id string, started time.Time) Run {
	return Run{
		ID:        id,
		TaskID:    "t1",
		TaskName:  "Task One",
		Agent:     "dev",
		Provider:  "test",
		Models:    []string{"m/a"},
		Attempts:  []Attempt{},
		Status:    "done",
		StartedAt: started,
	}
}

func mustOpenStore(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("OpenStore(%s): %v", dir, err)
	}
	return s
}

func mustListRuns(t *testing.T, s *Store) []Run {
	t.Helper()
	runs, err := s.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	return runs
}

func TestStorePersistsAndListsRuns(t *testing.T) {
	dir := t.TempDir()
	s := mustOpenStore(t, dir)
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 60; i++ {
		run := makeRun(fmt.Sprintf("run-%02d", i), base.Add(time.Duration(i)*time.Minute))
		if err := s.UpsertRun(run); err != nil {
			t.Fatalf("UpsertRun(%s): %v", run.ID, err)
		}
	}

	// A fresh store on the same dir reads back what the first one wrote.
	reopened := mustOpenStore(t, dir)
	runs := mustListRuns(t, reopened)
	if len(runs) != 60 {
		t.Fatalf("ListRuns: got %d runs, want 60", len(runs))
	}
	// Newest first: the list the leaderboard and summary render.
	if runs[0].ID != "run-59" || runs[59].ID != "run-00" {
		t.Errorf("ListRuns order: first=%s last=%s, want run-59..run-00", runs[0].ID, runs[59].ID)
	}

	got, err := reopened.GetRun("run-07")
	if err != nil {
		t.Fatalf("GetRun(run-07): %v", err)
	}
	if got.TaskName != "Task One" || got.Provider != "test" {
		t.Errorf("GetRun round trip: got %+v", got)
	}
	if !got.StartedAt.Equal(base.Add(7 * time.Minute)) {
		t.Errorf("GetRun StartedAt: got %v want %v", got.StartedAt, base.Add(7*time.Minute))
	}

	// Upserting the same ID replaces instead of duplicating.
	updated := makeRun("run-07", base.Add(7*time.Minute))
	updated.Status = "failed"
	updated.Error = "boom"
	if err := reopened.UpsertRun(updated); err != nil {
		t.Fatalf("UpsertRun replace: %v", err)
	}
	if runs = mustListRuns(t, reopened); len(runs) != 60 {
		t.Errorf("ListRuns after replace: got %d runs, want 60", len(runs))
	}
	if got, err = reopened.GetRun("run-07"); err != nil || got.Status != "failed" {
		t.Errorf("GetRun after replace: run=%+v err=%v", got, err)
	}
}

func TestStoreRetentionEnvOverride(t *testing.T) {
	t.Setenv("EVAL_RUNS_KEEP", "5")
	dir := t.TempDir()
	s := mustOpenStore(t, dir)
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 8; i++ {
		if err := s.UpsertRun(makeRun(fmt.Sprintf("run-%d", i), base.Add(time.Duration(i)*time.Minute))); err != nil {
			t.Fatalf("UpsertRun(run-%d): %v", i, err)
		}
	}

	if runs := mustListRuns(t, s); len(runs) != 5 {
		t.Fatalf("ListRuns: got %d runs, want 5 after retention", len(runs))
	}
	// The five oldest are gone from disk, not just from the index.
	for _, id := range []string{"run-0", "run-1", "run-2"} {
		if _, err := os.Stat(filepath.Join(dir, id+".json")); !os.IsNotExist(err) {
			t.Errorf("purged %s still on disk (stat err=%v)", id, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "run-7.json")); err != nil {
		t.Errorf("newest run missing: %v", err)
	}

	// The cap survives a reopen.
	if runs := mustListRuns(t, mustOpenStore(t, dir)); len(runs) != 5 {
		t.Errorf("ListRuns after reopen: got %d runs, want 5", len(runs))
	}
}

func TestRetentionLimitDefaults(t *testing.T) {
	for _, tc := range []struct {
		env, want string
	}{
		{"", fmt.Sprint(defaultRunRetention)},
		{"not-a-number", fmt.Sprint(defaultRunRetention)},
		{"0", fmt.Sprint(defaultRunRetention)},
		{"-3", fmt.Sprint(defaultRunRetention)},
		{"7", "7"},
	} {
		t.Setenv("EVAL_RUNS_KEEP", tc.env)
		if got := retentionLimit(); fmt.Sprint(got) != tc.want {
			t.Errorf("retentionLimit with EVAL_RUNS_KEEP=%q: got %d want %s", tc.env, got, tc.want)
		}
	}
}

func TestStoreMigratesLegacyRunsFileOnce(t *testing.T) {
	dir := t.TempDir()
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	legacy := []Run{makeRun("legacy-a", base), makeRun("legacy-b", base.Add(time.Minute))}
	data, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy: %v", err)
	}
	legacyPath := filepath.Join(dir, legacyRunsFile)
	if err := os.WriteFile(legacyPath, data, 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	s := mustOpenStore(t, dir)
	runs := mustListRuns(t, s)
	if len(runs) != 2 {
		t.Fatalf("ListRuns after migration: got %d runs, want 2", len(runs))
	}
	if runs[0].ID != "legacy-b" { // newest first
		t.Errorf("ListRuns order after migration: first=%s, want legacy-b", runs[0].ID)
	}
	if got, err := s.GetRun("legacy-a"); err != nil || got.TaskName != "Task One" {
		t.Errorf("GetRun(legacy-a): run=%+v err=%v", got, err)
	}
	// The original is renamed, never deleted, and its bytes are untouched.
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Errorf("legacy file still at original path (stat err=%v)", err)
	}
	imported, err := os.ReadFile(legacyPath + ".imported")
	if err != nil {
		t.Fatalf("read imported archive: %v", err)
	}
	if string(imported) != string(data) {
		t.Errorf("imported archive differs from the original legacy file")
	}

	// A second open is a no-op: no re-import, no duplicate, no resurrection.
	reopened := mustOpenStore(t, dir)
	if runs = mustListRuns(t, reopened); len(runs) != 2 {
		t.Errorf("ListRuns after second open: got %d runs, want 2", len(runs))
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Errorf("eval-runs.json resurrected by second open (stat err=%v)", err)
	}
	if imported, err = os.ReadFile(legacyPath + ".imported"); err != nil || string(imported) != string(data) {
		t.Errorf("imported archive changed by second open (err=%v)", err)
	}
}

func TestStoreRebuildsCorruptOrMissingIndex(t *testing.T) {
	dir := t.TempDir()
	s := mustOpenStore(t, dir)
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if err := s.UpsertRun(makeRun(fmt.Sprintf("run-%d", i), base.Add(time.Duration(i)*time.Minute))); err != nil {
			t.Fatalf("UpsertRun(run-%d): %v", i, err)
		}
	}

	indexPath := filepath.Join(dir, indexFile)
	// Corrupt index: the next open must rebuild by scanning, not fail.
	if err := os.WriteFile(indexPath, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("corrupt index: %v", err)
	}
	reopened := mustOpenStore(t, dir)
	if runs := mustListRuns(t, reopened); len(runs) != 3 {
		t.Fatalf("ListRuns after corrupt index: got %d runs, want 3", len(runs))
	}
	if got, err := reopened.GetRun("run-1"); err != nil || got.Status != "done" {
		t.Errorf("GetRun after rebuild: run=%+v err=%v", got, err)
	}
	// The rebuilt index is persisted and parses again.
	data, err := os.ReadFile(indexPath)
	if err != nil || !json.Valid(data) {
		t.Errorf("index.json not healed on disk (valid=%v err=%v)", json.Valid(data), err)
	}

	// Missing index: same story.
	if err := os.Remove(indexPath); err != nil {
		t.Fatalf("remove index: %v", err)
	}
	if runs := mustListRuns(t, mustOpenStore(t, dir)); len(runs) != 3 {
		t.Errorf("ListRuns after missing index: got %d runs, want 3", len(runs))
	}
}

func TestStoreBaselineRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := mustOpenStore(t, dir)
	if got, err := s.GetBaseline("t1"); err != nil || got != "" {
		t.Errorf("GetBaseline on empty store: got %q err=%v, want \"\"", got, err)
	}
	if err := s.SetBaseline("t1", "run-1"); err != nil {
		t.Fatalf("SetBaseline: %v", err)
	}
	if err := s.SetBaseline("t1", "run-2"); err != nil { // overwrite wins
		t.Fatalf("SetBaseline overwrite: %v", err)
	}
	if err := s.SetBaseline("t2", "run-3"); err != nil {
		t.Fatalf("SetBaseline t2: %v", err)
	}
	if got, err := s.GetBaseline("t1"); err != nil || got != "run-2" {
		t.Errorf("GetBaseline(t1): got %q err=%v, want run-2", got, err)
	}
	// Clearing an unknown task is a no-op, not an error.
	if err := s.ClearBaseline("unknown"); err != nil {
		t.Errorf("ClearBaseline(unknown): %v", err)
	}
	if err := s.ClearBaseline("t1"); err != nil {
		t.Fatalf("ClearBaseline: %v", err)
	}
	if got, err := s.GetBaseline("t1"); err != nil || got != "" {
		t.Errorf("GetBaseline(t1) after clear: got %q err=%v, want \"\"", got, err)
	}

	// Baselines persist across a reopen.
	reopened := mustOpenStore(t, dir)
	if got, err := reopened.GetBaseline("t2"); err != nil || got != "run-3" {
		t.Errorf("GetBaseline(t2) after reopen: got %q err=%v, want run-3", got, err)
	}
	if got, _ := reopened.GetBaseline("t1"); got != "" {
		t.Errorf("GetBaseline(t1) after reopen: got %q, want \"\"", got)
	}
}

// TestStoreConcurrentUpsertList is a smoke test, not a race detector (cgo is
// disabled in this build, so -race cannot run): it hammers one store from
// several goroutines and asserts the final count is exact, so lost updates or
// deadlock show up as a failure rather than as luck.
func TestStoreConcurrentUpsertList(t *testing.T) {
	dir := t.TempDir()
	s := mustOpenStore(t, dir)
	const writers = 8
	const perWriter = 10
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				run := makeRun(fmt.Sprintf("run-%d-%02d", w, i),
					base.Add(time.Duration(w*perWriter+i)*time.Second))
				if err := s.UpsertRun(run); err != nil {
					t.Errorf("upsert %s: %v", run.ID, err)
					return
				}
				if _, err := s.ListRuns(); err != nil {
					t.Errorf("list while writing: %v", err)
					return
				}
				if _, err := s.GetRun(run.ID); err != nil {
					t.Errorf("get own run %s: %v", run.ID, err)
					return
				}
				if i%5 == 0 {
					if err := s.SetBaseline(run.TaskID, run.ID); err != nil {
						t.Errorf("set baseline while writing: %v", err)
						return
					}
				}
			}
		}(w)
	}
	wg.Wait()

	// Every write survived: no data loss under contention.
	runs := mustListRuns(t, s)
	if len(runs) != writers*perWriter {
		t.Fatalf("ListRuns after concurrency: got %d runs, want %d", len(runs), writers*perWriter)
	}
}

func TestStoreRejectsUnsafeRunIDs(t *testing.T) {
	dir := t.TempDir()
	s := mustOpenStore(t, dir)
	for _, id := range []string{"", ".", "..", "a/b", `a\b`, "../escape"} {
		if err := s.UpsertRun(makeRun(id, time.Now())); err == nil {
			t.Errorf("UpsertRun(%q): expected error, got nil", id)
		}
		if _, err := s.GetRun(id); err == nil {
			t.Errorf("GetRun(%q): expected error, got nil", id)
		}
	}
	// Nothing escaped the run directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != indexFile {
		t.Errorf("unexpected files after rejected IDs: %v", entries)
	}
}
