package missions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pin the two behaviors where the CLI and API execution paths
// deliberately diverge, so a future collapse into a shared executor cannot
// change them silently:
//
//   - ExecuteViaCLI HARD-fails when PrepareContext fails.
//   - executeViaAPI SOFT-falls-back to a simple description prompt when
//     PrepareContext fails, and still completes.

// badWorktree returns a path to a regular file, so PrepareContext's
// MkdirAll(worktree/.ywai) fails on the non-temp branch.
func badWorktree(t *testing.T) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestExecuteViaCLIHardFailsOnPrepareContext(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("cli-prepctx")
	_ = store.CreateMission(mission)
	mission.Features[0].WorktreePath = badWorktree(t)

	wm := NewWorkerManager(store, DefaultWorkerConfig())
	fakeDir := fakeOpencodeValidHandoff(t, testHandoff())
	t.Setenv("PATH", fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := wm.ExecuteViaCLI(mission, "feat-1")
	if err == nil {
		t.Fatal("ExecuteViaCLI must hard-fail when PrepareContext fails")
	}
	if !strings.Contains(err.Error(), "prepare context") {
		t.Fatalf("error should reference prepare context, got: %v", err)
	}
	feat, _ := GetFeatureByID(mission, "feat-1")
	if feat.Status != FeatureFailed {
		t.Fatalf("feature should be failed after hard PrepareContext failure, got %s", feat.Status)
	}
}

func TestExecuteViaAPISoftFallsBackOnPrepareContext(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("api-prepctx")
	_ = store.CreateMission(mission)
	mission.Features[0].WorktreePath = badWorktree(t)
	mission.Features[0].Description = "Build the widget"

	wm := NewWorkerManager(store, DefaultWorkerConfig())
	handoffJSON, err := json.Marshal(testHandoff())
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeSessionAPI{responses: []string{string(handoffJSON)}}
	wm.SetClient(&fakePlannerClient{sessions: fake})

	handoff, err := wm.executeViaAPI(mission, "feat-1")
	if err != nil {
		t.Fatalf("executeViaAPI must soft-fall-back, not fail, on PrepareContext error: %v", err)
	}
	if handoff == nil {
		t.Fatal("expected a handoff from the soft-fallback path")
	}
	if len(fake.prompts) == 0 || !strings.Contains(fake.prompts[0], "Build the widget") {
		t.Fatalf("fallback prompt should contain the feature description, got: %v", fake.prompts)
	}
}
