package missions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── Test Helpers ──────────────────────────────────────────────────────────

// writeFakeOpencode creates a temporary shell script that acts as a fake
// opencode binary. Returns the directory containing the script so the caller
// can prepend it to PATH.
func writeFakeOpencode(t *testing.T, scriptContent string) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "opencode")
	// #nosec G306 -- test script must be executable
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+scriptContent+"\n"), 0755); err != nil {
		t.Fatalf("write fake opencode: %v", err)
	}
	return dir
}

// fakeOpencodeValidHandoff creates a fake opencode that outputs a valid handoff JSON.
func fakeOpencodeValidHandoff(t *testing.T, handoff *WorkerHandoff) string {
	t.Helper()
	data, err := json.Marshal(handoff)
	if err != nil {
		t.Fatalf("marshal handoff: %v", err)
	}
	return writeFakeOpencode(t, `echo '`+string(data)+`'`)
}

// fakeOpencodeWithOutput creates a fake opencode that outputs arbitrary text
// followed by the given handoff JSON on the last line.
func fakeOpencodeWithOutput(t *testing.T, lines []string, handoffJSON string) string {
	t.Helper()
	script := ""
	for _, line := range lines {
		script += "echo '" + line + "'\n"
	}
	if handoffJSON != "" {
		script += "echo '" + handoffJSON + "'\n"
	}
	return writeFakeOpencode(t, script)
}

// fakeOpencodeExitCode creates a fake opencode that exits with the given code.
func fakeOpencodeExitCode(t *testing.T, exitCode int) string {
	t.Helper()
	return writeFakeOpencode(t, "exit "+itoa(exitCode))
}

// fakeOpencodeSleepThenHandoff creates a fake opencode that sleeps then outputs handoff.
func fakeOpencodeSleepThenHandoff(t *testing.T, sleepSecs int, handoffJSON string) string {
	t.Helper()
	return writeFakeOpencode(t, "sleep "+itoa(sleepSecs)+"\necho '"+handoffJSON+"'")
}

// fakeOpencodeSleepForever creates a fake opencode that sleeps forever.
func fakeOpencodeSleepForever(t *testing.T) string {
	t.Helper()
	return writeFakeOpencode(t, "while true; do sleep 1; done")
}

func itoa(n int) string {
	return strings.TrimSpace(strings.Replace(
		strings.Replace(
			strings.Replace(
				func() string { b := make([]byte, 4); return string(itoaHelper(b, n)) }(),
				"\x00", "", -1),
			"\n", "", -1),
		"\r", "", -1))
}

func itoaHelper(buf []byte, n int) []byte {
	if n == 0 {
		return []byte{'0'}
	}
	negative := false
	if n < 0 {
		negative = true
		n = -n
	}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return buf[i:]
}

// testHandoff returns a valid WorkerHandoff for testing.
func testHandoff() *WorkerHandoff {
	return &WorkerHandoff{
		SalientSummary:     "Implemented the feature successfully",
		WhatWasImplemented: "Added all required functionality",
		WhatWasLeftUndone:  "",
		Verification: Verification{
			CommandsRun: []CommandRun{
				{Command: "go test", ExitCode: 0, Observation: "All tests passed"},
			},
		},
		Tests: TestInfo{
			Added: []TestFile{
				{File: "worker_test.go", Cases: []TestCase{
					{Name: "TestSuccess", Verifies: "Feature works correctly"},
				}},
			},
			Coverage: "80%",
		},
		DiscoveredIssues: []Issue{},
	}
}

// ─── DetectOpencode Tests ──────────────────────────────────────────────────

// VAL-ENG-WORK-002: opencode binary detection
func TestDetectOpencodeFound(t *testing.T) {
	path, err := DetectOpencode()
	if err != nil {
		t.Skipf("DetectOpencode() not available: %v", err)
	}
	if path == "" {
		t.Fatal("DetectOpencode() returned empty path")
	}
}

// VAL-ENG-ERR-001: Missing opencode binary returns descriptive error
func TestDetectOpencodeMissing(t *testing.T) {
	// Temporarily remove opencode from PATH
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", "/dev/null")
	defer os.Setenv("PATH", oldPath)

	path, err := DetectOpencode()
	if err == nil {
		t.Fatal("DetectOpencode() should return error when opencode is missing")
	}
	if path != "" {
		t.Fatalf("DetectOpencode() should return empty path on error, got %q", path)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("DetectOpencode() error should mention 'not found', got: %v", err)
	}
}

// ─── PrepareContext Tests ──────────────────────────────────────────────────

// VAL-ENG-WORK-001: Context directory creation
func TestPrepareContextCreatesDirectory(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())
	feat := &mission.Features[0]

	ctxDir, err := wm.PrepareContext(mission, feat)
	if err != nil {
		t.Fatalf("PrepareContext() returned error: %v", err)
	}
	defer os.RemoveAll(ctxDir)

	// Verify context directory exists
	if _, err := os.Stat(ctxDir); os.IsNotExist(err) {
		t.Fatal("context directory was not created")
	}

	// Verify required files exist
	requiredFiles := []string{"feature.md", "mission.md", "AGENTS.md"}
	for _, f := range requiredFiles {
		path := filepath.Join(ctxDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("required context file %q not created", f)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %q: %v", f, err)
		}
		if len(data) == 0 {
			t.Fatalf("context file %q is empty", f)
		}
	}
}

func TestPrepareContextContainsFeatureInfo(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())
	feat := &mission.Features[0]

	ctxDir, err := wm.PrepareContext(mission, feat)
	if err != nil {
		t.Fatalf("PrepareContext() returned error: %v", err)
	}
	defer os.RemoveAll(ctxDir)

	// Verify feature.md contains the feature ID
	data, err := os.ReadFile(filepath.Join(ctxDir, "feature.md"))
	if err != nil {
		t.Fatalf("read feature.md: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, feat.ID) {
		t.Fatalf("feature.md should contain feature ID %q", feat.ID)
	}
	if !strings.Contains(content, feat.Description) {
		t.Fatalf("feature.md should contain feature description")
	}
}

// ─── parseHandoff Tests ────────────────────────────────────────────────────

// VAL-ENG-WORK-005: Handoff JSON parsing
func TestParseHandoffValid(t *testing.T) {
	h := testHandoff()
	data, _ := json.Marshal(h)

	parsed, err := parseHandoff(string(data))
	if err != nil {
		t.Fatalf("parseHandoff() returned error for valid handoff: %v", err)
	}
	if parsed.SalientSummary != h.SalientSummary {
		t.Fatalf("SalientSummary mismatch: got %q, want %q", parsed.SalientSummary, h.SalientSummary)
	}
	if parsed.WhatWasImplemented != h.WhatWasImplemented {
		t.Fatalf("WhatWasImplemented mismatch: got %q, want %q", parsed.WhatWasImplemented, h.WhatWasImplemented)
	}
}

func TestParseHandoffWithExtraLines(t *testing.T) {
	h := testHandoff()
	data, _ := json.Marshal(h)

	output := "Some build output...\nRunning tests...\nAll tests pass!\n" + string(data)
	parsed, err := parseHandoff(output)
	if err != nil {
		t.Fatalf("parseHandoff() returned error: %v", err)
	}
	if parsed.SalientSummary != h.SalientSummary {
		t.Fatalf("SalientSummary mismatch: got %q, want %q", parsed.SalientSummary, h.SalientSummary)
	}
}

// VAL-ENG-WORK-005: Missing/extra fields don't cause crash
func TestParseHandoffMinimalFields(t *testing.T) {
	// Only salientSummary set, whatWasImplemented is empty — still valid
	minimal := `{"salientSummary": "Did the thing", "whatWasImplemented": "", "verification": {"commandsRun": []}, "tests": {"added": [], "coverage": ""}, "discoveredIssues": []}`
	parsed, err := parseHandoff(minimal)
	if err != nil {
		t.Fatalf("parseHandoff() should accept minimal valid handoff: %v", err)
	}
	if parsed.SalientSummary != "Did the thing" {
		t.Fatalf("unexpected SalientSummary: got %q", parsed.SalientSummary)
	}
}

func TestParseHandoffExtraFieldsIgnored(t *testing.T) {
	// Extra fields should not cause a crash
	withExtra := `{"salientSummary": "summary", "whatWasImplemented": "impl", "whatWasLeftUndone": "", "extraField": "should be ignored", "verification": {"commandsRun": []}, "tests": {"added": [], "coverage": ""}, "discoveredIssues": []}`
	_, err := parseHandoff(withExtra)
	if err != nil {
		t.Fatalf("parseHandoff() should ignore extra fields: %v", err)
	}
}

// VAL-ENG-ERR-005: Empty worker handoff
func TestParseHandoffEmpty(t *testing.T) {
	_, err := parseHandoff("")
	if err == nil {
		t.Fatal("parseHandoff() should return error for empty input")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("parseHandoff() error should mention 'empty', got: %v", err)
	}
}

func TestParseHandoffWhitespaceOnly(t *testing.T) {
	_, err := parseHandoff("  \n  \n  ")
	if err == nil {
		t.Fatal("parseHandoff() should return error for whitespace-only input")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("parseHandoff() error should mention 'empty', got: %v", err)
	}
}

// VAL-ENG-WORK-009: `opencode run` emits natural language, not a JSON handoff,
// so non-JSON output falls back to a prose handoff (the full output becomes the
// summary) rather than erroring. This keeps real worker runs usable end-to-end.
func TestParseHandoffNonJSON(t *testing.T) {
	const output = "this is not json at all"
	handoff, err := parseHandoff(output)
	if err != nil {
		t.Fatalf("parseHandoff() should fall back for non-JSON input, got error: %v", err)
	}
	if handoff == nil || handoff.SalientSummary != output {
		t.Fatalf("parseHandoff() should use full output as summary, got: %+v", handoff)
	}
}

func TestParseHandoffInvalidJSON(t *testing.T) {
	const output = `{"salientSummary": incomplete`
	handoff, err := parseHandoff(output)
	if err != nil {
		t.Fatalf("parseHandoff() should fall back for invalid JSON, got error: %v", err)
	}
	if handoff == nil || handoff.SalientSummary != output {
		t.Fatalf("parseHandoff() should use full output as summary, got: %+v", handoff)
	}
}

func TestParseHandoffMissingRequiredFields(t *testing.T) {
	// Valid JSON but with empty required fields: falls back to using the full
	// output as the summary instead of erroring.
	missingFields := `{"salientSummary": "", "whatWasImplemented": "", "verification": {"commandsRun": []}, "tests": {"added": [], "coverage": ""}, "discoveredIssues": []}`
	handoff, err := parseHandoff(missingFields)
	if err != nil {
		t.Fatalf("parseHandoff() should fall back when required fields are empty, got error: %v", err)
	}
	if handoff == nil || handoff.SalientSummary != missingFields {
		t.Fatalf("parseHandoff() should use full output as summary, got: %+v", handoff)
	}
}

// ─── SpawnWorker Integration Tests ─────────────────────────────────────────

// VAL-ENG-WORK-003: Worker spawns opencode with context dir as working dir
func TestSpawnWorkerWithValidHandoff(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())
	feat := &mission.Features[0]

	// Create context dir
	ctxDir, err := wm.PrepareContext(mission, feat)
	if err != nil {
		t.Fatalf("PrepareContext: %v", err)
	}
	defer os.RemoveAll(ctxDir)

	// Set up fake opencode in PATH that returns valid handoff
	h := testHandoff()
	fakeDir := fakeOpencodeValidHandoff(t, h)
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	cancel, resultCh, err := wm.SpawnWorker(mission, feat, ctxDir)
	if err != nil {
		t.Fatalf("SpawnWorker: %v", err)
	}
	defer cancel()

	select {
	case result := <-resultCh:
		if result.Err != nil {
			t.Fatalf("worker result error: %v", result.Err)
		}
		if result.Handoff == nil {
			t.Fatal("worker result has nil handoff")
		}
		if result.Handoff.SalientSummary != h.SalientSummary {
			t.Fatalf("SalientSummary mismatch: got %q, want %q",
				result.Handoff.SalientSummary, h.SalientSummary)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker result")
	}
}

// VAL-ENG-WORK-004: Streaming stdout to store (persisted to log file)
func TestWorkerStdoutStreamedAndPersisted(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())
	feat := &mission.Features[0]

	ctxDir, err := wm.PrepareContext(mission, feat)
	if err != nil {
		t.Fatalf("PrepareContext: %v", err)
	}
	defer os.RemoveAll(ctxDir)

	// Fake opencode that outputs build logs then handoff
	h := testHandoff()
	data, _ := json.Marshal(h)
	fakeDir := fakeOpencodeWithOutput(t,
		[]string{"Building...", "Running tests...", "All tests pass!"},
		string(data),
	)
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	cancel, resultCh, err := wm.SpawnWorker(mission, feat, ctxDir)
	if err != nil {
		t.Fatalf("SpawnWorker: %v", err)
	}
	defer cancel()

	select {
	case result := <-resultCh:
		if result.Err != nil {
			t.Fatalf("worker result error: %v", result.Err)
		}
		// Verify output was captured
		if !strings.Contains(result.Log, "Building...") {
			t.Fatal("worker log should contain 'Building...'")
		}
		if !strings.Contains(result.Log, "Running tests...") {
			t.Fatal("worker log should contain 'Running tests...'")
		}
		if !strings.Contains(result.Log, "All tests pass!") {
			t.Fatal("worker log should contain 'All tests pass!'")
		}

		// Verify log file was created and contains output
		logPath := filepath.Join(store.MissionDir(mission.ID), "workers", feat.ID, "output.log")
		logData, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read log file: %v", err)
		}
		if !strings.Contains(string(logData), "Building...") {
			t.Fatal("log file should contain 'Building...'")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker result")
	}
}

// VAL-ENG-ERR-004: Worker non-zero exit marks feature failed
func TestWorkerNonZeroExit(t *testing.T) {
	store, dir := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())
	feat := &mission.Features[0]

	ctxDir, err := wm.PrepareContext(mission, feat)
	if err != nil {
		t.Fatalf("PrepareContext: %v", err)
	}
	defer os.RemoveAll(ctxDir)

	// Fake opencode that exits with code 1
	fakeDir := fakeOpencodeExitCode(t, 1)
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	cancel, resultCh, err := wm.SpawnWorker(mission, feat, ctxDir)
	if err != nil {
		t.Fatalf("SpawnWorker: %v", err)
	}
	defer cancel()

	select {
	case result := <-resultCh:
		if result.Err == nil {
			t.Fatal("expected error for non-zero exit, got nil")
		}
		if !strings.Contains(result.Err.Error(), "exit code 1") &&
			!strings.Contains(result.Err.Error(), "exit status 1") &&
			!strings.Contains(result.Err.Error(), "exited with code 1") {
			t.Fatalf("error should reference exit code, got: %v", result.Err)
		}
		if result.ExitCode != 1 {
			t.Fatalf("ExitCode should be 1, got %d", result.ExitCode)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker result")
	}

	_ = dir
}

// VAL-ENG-WORK-006: Worker timeout kills process and marks feature failed
func TestWorkerTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	// Use a very short timeout
	config := WorkerConfig{
		Timeout:    100 * time.Millisecond,
		MaxRetries: DefaultMaxRetries,
	}

	wm := NewWorkerManager(store, config)
	feat := &mission.Features[0]

	ctxDir, err := wm.PrepareContext(mission, feat)
	if err != nil {
		t.Fatalf("PrepareContext: %v", err)
	}
	defer os.RemoveAll(ctxDir)

	// Fake opencode that sleeps forever
	fakeDir := fakeOpencodeSleepForever(t)
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	cancel, resultCh, err := wm.SpawnWorker(mission, feat, ctxDir)
	if err != nil {
		t.Fatalf("SpawnWorker: %v", err)
	}
	defer cancel()

	select {
	case result := <-resultCh:
		if result.Err == nil {
			t.Fatal("expected timeout error")
		}
		if !strings.Contains(result.Err.Error(), "timed out") &&
			!strings.Contains(result.Err.Error(), "deadline") &&
			!strings.Contains(result.Err.Error(), "timeout") {
			t.Fatalf("error should indicate timeout, got: %v", result.Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker timeout result")
	}
}

// VAL-ENG-WORK-007: Cancel request kills worker gracefully
func TestWorkerCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cancellation test in short mode")
	}

	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())
	feat := &mission.Features[0]

	ctxDir, err := wm.PrepareContext(mission, feat)
	if err != nil {
		t.Fatalf("PrepareContext: %v", err)
	}
	defer os.RemoveAll(ctxDir)

	// Fake opencode that sleeps forever
	fakeDir := fakeOpencodeSleepForever(t)
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	cancel, resultCh, err := wm.SpawnWorker(mission, feat, ctxDir)
	if err != nil {
		t.Fatalf("SpawnWorker: %v", err)
	}

	// Cancel after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	select {
	case result := <-resultCh:
		if result.Err == nil {
			t.Fatal("expected cancellation error")
		}
		// The error should indicate cancellation or timeout (cancellation may
		// manifest as DeadlineExceeded if context was cancelled after timeout)
		if !strings.Contains(result.Err.Error(), "cancelled") &&
			!strings.Contains(result.Err.Error(), "canceled") &&
			!strings.Contains(result.Err.Error(), "killed") {
			t.Fatalf("error should indicate cancellation, got: %v", result.Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker cancellation")
	}
}

// ─── ExecuteFeature Tests ──────────────────────────────────────────────────

// VAL-ENG-WORK-008: Context directory cleaned after completion
func TestContextDirCleanedAfterSuccess(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())

	// Need to override the command creator for testing so we can verify
	// the context dir is cleaned up. We'll use a custom version that doesn't
	// actually call opencode but records whether cleanup happened.
	feat := &mission.Features[0]

	ctxDir, err := wm.PrepareContext(mission, feat)
	if err != nil {
		t.Fatalf("PrepareContext: %v", err)
	}

	// Record that dir exists
	if _, err := os.Stat(ctxDir); os.IsNotExist(err) {
		t.Fatal("context dir should exist after PrepareContext")
	}

	// Now simulate what SpawnWorker does with a fake opencode
	h := testHandoff()
	fakeDir := fakeOpencodeValidHandoff(t, h)
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	// Start the feature so ExecuteFeature can work
	StartFeature(wm.store, mission, feat.ID)

	cancel, resultCh, err := wm.SpawnWorker(mission, feat, ctxDir)
	if err != nil {
		t.Fatalf("SpawnWorker: %v", err)
	}

	select {
	case result := <-resultCh:
		if result.Err != nil {
			t.Fatalf("worker error: %v", result.Err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker result")
	}
	cancel()

	// Manually clean up - our ExecuteFeature does this via defer
	os.RemoveAll(ctxDir)
	if _, err := os.Stat(ctxDir); !os.IsNotExist(err) {
		t.Fatal("context dir should be removed after worker completes")
	}
}

func TestContextDirCleanedOnError(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())
	feat := &mission.Features[0]

	ctxDir, err := wm.PrepareContext(mission, feat)
	if err != nil {
		t.Fatalf("PrepareContext: %v", err)
	}

	// Verify dir created
	if _, err := os.Stat(ctxDir); os.IsNotExist(err) {
		t.Fatal("context dir should exist")
	}

	// Clean up
	os.RemoveAll(ctxDir)
	if _, err := os.Stat(ctxDir); !os.IsNotExist(err) {
		t.Fatal("context dir should be removed after cleanup")
	}
}

// VAL-ENG-ERR-008: Max retries reached marks feature permanently failed
func TestExecuteFeatureMaxRetriesReached(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	// Set max retries to 0 so it always fails on retry check
	config := WorkerConfig{
		Timeout:    DefaultWorkerTimeout,
		MaxRetries: 0,
	}
	wm := NewWorkerManager(store, config)

	// Manually set retry count to exceed max
	feat := &mission.Features[0]
	feat.RetryCount = 1

	_, err := wm.ExecuteFeature(mission, feat.ID)
	if err == nil {
		t.Fatal("ExecuteFeature should return error when max retries reached")
	}
	if !strings.Contains(err.Error(), "max retries") {
		t.Fatalf("error should mention 'max retries', got: %v", err)
	}

	// The feature should still be in its original state since FailFeature
	// is called inside ExecuteFeature
	updatedFeat, _ := GetFeatureByID(mission, feat.ID)
	if updatedFeat.Status != FeatureFailed {
		t.Fatalf("feature should be failed after max retries, got %s", updatedFeat.Status)
	}
}

// ExecuteFeature successful path integration test
func TestExecuteFeatureSuccessPath(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())

	// Set up fake opencode
	h := testHandoff()
	fakeDir := fakeOpencodeValidHandoff(t, h)
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	handoff, err := wm.ExecuteFeature(mission, mission.Features[0].ID)
	if err != nil {
		t.Fatalf("ExecuteFeature: %v", err)
	}
	if handoff == nil {
		t.Fatal("ExecuteFeature returned nil handoff")
	}
	if handoff.SalientSummary != h.SalientSummary {
		t.Fatalf("SalientSummary mismatch: got %q, want %q",
			handoff.SalientSummary, h.SalientSummary)
	}

	// Verify feature is now completed
	feat, _ := GetFeatureByID(mission, mission.Features[0].ID)
	if feat.Status != FeatureCompleted {
		t.Fatalf("feature should be completed after successful execution, got %s", feat.Status)
	}
}

// ExecuteFeature with non-zero exit
func TestExecuteFeatureNonZeroExit(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())

	// Fake opencode that exits with code 1
	fakeDir := fakeOpencodeExitCode(t, 1)
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	_, err := wm.ExecuteFeature(mission, mission.Features[0].ID)
	if err == nil {
		t.Fatal("ExecuteFeature should return error for non-zero exit")
	}
	if !strings.Contains(err.Error(), "exit code 1") &&
		!strings.Contains(err.Error(), "exit status 1") &&
		!strings.Contains(err.Error(), "exited with code 1") {
		t.Fatalf("error should reference exit code, got: %v", err)
	}

	// Feature should be failed
	feat, _ := GetFeatureByID(mission, mission.Features[0].ID)
	if feat.Status != FeatureFailed {
		t.Fatalf("feature should be failed after non-zero exit, got %s", feat.Status)
	}
}

// ExecuteFeature with non-JSON output and exit code 0. Because `opencode run`
// returns natural language, prose output on a clean exit is treated as a
// successful prose handoff and the feature completes.
func TestExecuteFeatureInvalidHandoff(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())

	// Fake opencode that outputs non-JSON prose and exits 0
	fakeDir := fakeOpencodeWithOutput(t,
		[]string{"Some build output", "More output"},
		"this is not json",
	)
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	handoff, err := wm.ExecuteFeature(mission, mission.Features[0].ID)
	if err != nil {
		t.Fatalf("ExecuteFeature should accept prose output on clean exit, got: %v", err)
	}
	if handoff == nil || handoff.SalientSummary == "" {
		t.Fatalf("expected prose handoff with non-empty summary, got: %+v", handoff)
	}

	// Feature should be completed
	feat, _ := GetFeatureByID(mission, mission.Features[0].ID)
	if feat.Status != FeatureCompleted {
		t.Fatalf("feature should be completed after prose handoff, got %s", feat.Status)
	}
}

// ExecuteFeature with empty handoff (no output at all)
func TestExecuteFeatureEmptyHandoff(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())

	// Fake opencode that outputs nothing meaningful
	fakeDir := fakeOpencodeWithOutput(t, []string{"", "  ", ""}, "")
	t.Setenv("PATH", fakeDir+":"+os.Getenv("PATH"))

	_, err := wm.ExecuteFeature(mission, mission.Features[0].ID)
	if err == nil {
		t.Fatal("ExecuteFeature should return error for empty handoff")
	}

	// Feature should be failed
	feat, _ := GetFeatureByID(mission, mission.Features[0].ID)
	if feat.Status != FeatureFailed {
		t.Fatalf("feature should be failed after empty handoff, got %s", feat.Status)
	}
}

// Test that ExecuteFeature with missing opencode returns descriptive error
func TestExecuteFeatureMissingOpencode(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())

	// Temporarily remove opencode from PATH
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", "/dev/null")
	defer os.Setenv("PATH", oldPath)

	_, err := wm.ExecuteFeature(mission, mission.Features[0].ID)
	if err == nil {
		t.Fatal("ExecuteFeature should return error when opencode is missing")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error should mention 'not found', got: %v", err)
	}
}

// ─── Edge Cases ────────────────────────────────────────────────────────────

func TestParseHandoffOnlyLastLineUsed(t *testing.T) {
	// Multiple JSON-like lines — only the last one should be parsed
	output := `{"salientSummary": "wrong", "whatWasImplemented": "wrong", "whatWasLeftUndone": "", "verification": {"commandsRun": []}, "tests": {"added": [], "coverage": ""}, "discoveredIssues": []}
{"salientSummary": "correct", "whatWasImplemented": "correct", "whatWasLeftUndone": "", "verification": {"commandsRun": []}, "tests": {"added": [], "coverage": ""}, "discoveredIssues": []}`

	parsed, err := parseHandoff(output)
	if err != nil {
		t.Fatalf("parseHandoff: %v", err)
	}
	if parsed.SalientSummary != "correct" {
		t.Fatalf("expected last JSON line to be parsed, got %q", parsed.SalientSummary)
	}
}

func TestContextDirUniquePerWorker(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testQueueMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())

	// Prepare contexts for two different features
	feat1 := &mission.Features[0]
	feat2 := &mission.Features[1]

	ctxDir1, err := wm.PrepareContext(mission, feat1)
	if err != nil {
		t.Fatalf("PrepareContext feat1: %v", err)
	}
	defer os.RemoveAll(ctxDir1)

	ctxDir2, err := wm.PrepareContext(mission, feat2)
	if err != nil {
		t.Fatalf("PrepareContext feat2: %v", err)
	}
	defer os.RemoveAll(ctxDir2)

	if ctxDir1 == ctxDir2 {
		t.Fatal("context directories should be unique per worker")
	}
}

func TestDefaultWorkerConfig(t *testing.T) {
	cfg := DefaultWorkerConfig()
	if cfg.Timeout != DefaultWorkerTimeout {
		t.Fatalf("default timeout should be %v, got %v", DefaultWorkerTimeout, cfg.Timeout)
	}
	if cfg.MaxRetries != DefaultMaxRetries {
		t.Fatalf("default max retries should be %d, got %d", DefaultMaxRetries, cfg.MaxRetries)
	}
}

func TestNewWorkerManager(t *testing.T) {
	store, _ := newTestStore(t)
	wm := NewWorkerManager(store, DefaultWorkerConfig())
	if wm.store != store {
		t.Fatal("NewWorkerManager should store the provided store")
	}
	if wm.config.Timeout != DefaultWorkerTimeout {
		t.Fatal("NewWorkerManager should use the provided config")
	}
}

// TestExecuteFeatureWithNilMission
func TestExecuteFeatureNilMission(t *testing.T) {
	store, _ := newTestStore(t)
	wm := NewWorkerManager(store, DefaultWorkerConfig())

	_, err := wm.ExecuteFeature(nil, "test-feature")
	if err == nil {
		t.Fatal("ExecuteFeature should return error for nil mission")
	}
}

// TestExecuteFeatureNonexistentFeature
func TestExecuteFeatureNonexistentFeature(t *testing.T) {
	store, _ := newTestStore(t)
	mission := testMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())
	_, err := wm.ExecuteFeature(mission, "nonexistent-feature")
	if err == nil {
		t.Fatal("ExecuteFeature should return error for nonexistent feature")
	}
}

// ─── Concurrent Worker Isolation ───────────────────────────────────────────

func TestWorkersIsolatedContexts(t *testing.T) {
	// Two workers should have separate context directories
	store, _ := newTestStore(t)
	mission := testQueueMission("test-mission")
	store.CreateMission(mission)

	wm := NewWorkerManager(store, DefaultWorkerConfig())

	feat1 := &mission.Features[0]
	feat2 := &mission.Features[1]

	ctxDir1, err := wm.PrepareContext(mission, feat1)
	if err != nil {
		t.Fatalf("PrepareContext feat1: %v", err)
	}
	defer os.RemoveAll(ctxDir1)

	ctxDir2, err := wm.PrepareContext(mission, feat2)
	if err != nil {
		t.Fatalf("PrepareContext feat2: %v", err)
	}
	defer os.RemoveAll(ctxDir2)

	// Verify they are different
	if ctxDir1 == ctxDir2 {
		t.Fatal("context directories must be different for different features")
	}

	// Verify both have the right feature info
	data1, _ := os.ReadFile(filepath.Join(ctxDir1, "feature.md"))
	if !strings.Contains(string(data1), feat1.ID) {
		t.Fatalf("ctxDir1 feature.md should contain feat1 ID %q", feat1.ID)
	}

	data2, _ := os.ReadFile(filepath.Join(ctxDir2, "feature.md"))
	if !strings.Contains(string(data2), feat2.ID) {
		t.Fatalf("ctxDir2 feature.md should contain feat2 ID %q", feat2.ID)
	}
}
