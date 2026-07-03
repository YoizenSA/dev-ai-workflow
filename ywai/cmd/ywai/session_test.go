package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// executeCommand is a test helper that runs rootCmd with the given args
// and captures stdout + error. It sets both stdout and stderr to the same
// buffer so both standard output and error messages are captured.
func executeCommand(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

// ---------------------------------------------------------------------------
// ywai session — top-level command group
// ---------------------------------------------------------------------------

func TestSessionCmd_NoSubcommand_ShowsHelp(t *testing.T) {
	out, err := executeCommand("session")
	if err != nil {
		t.Fatalf("expected no error (help should print to stdout, not err), got: %v", err)
	}
	if !strings.Contains(out, "Usage") && !strings.Contains(out, "Available Commands") && !strings.Contains(out, "list") {
		t.Errorf("output should contain help text mentioning subcommands; got:\n%s", out)
	}
}

func TestSessionCmd_UnknownFlag_ReturnsError(t *testing.T) {
	out, err := executeCommand("session", "--bogus")
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
	if !strings.Contains(err.Error(), "unknown flag") && !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention unknown flag; got: %v\noutput: %s", err, out)
	}
}

// ---------------------------------------------------------------------------
// ywai session list — listing sessions
// ---------------------------------------------------------------------------

func TestSessionList_NoSessions_ShowsMessage(t *testing.T) {
	out, err := executeCommand("session", "list")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !strings.Contains(out, "No active sessions") {
		t.Errorf("expected 'No active sessions' message in output; got:\n%s", out)
	}
}

func TestSessionList_JSONFlag_OutputsValidJSON(t *testing.T) {
	out, err := executeCommand("session", "list", "--json")
	if err != nil {
		t.Fatalf("expected no error, got: %v\noutput: %s", err, out)
	}
	var parsed interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Errorf("output should be valid JSON; parse error: %v\noutput: %s", err, out)
	}
}

func TestSessionList_JSONFlag_EmptyArray(t *testing.T) {
	out, err := executeCommand("session", "list", "--json")
	if err != nil {
		t.Fatalf("expected no error, got: %v\noutput: %s", err, out)
	}
	// Trim whitespace ; JSON output with no sessions should be an empty array.
	got := strings.TrimSpace(out)
	if got != "[]" {
		t.Errorf("expected empty JSON array '[]' for no sessions, got: %s", got)
	}
}

func TestSessionList_UnknownFlag_ReturnsError(t *testing.T) {
	out, err := executeCommand("session", "list", "--bogus")
	if err == nil {
		t.Fatal("expected error for unknown flag, got nil")
	}
	if !strings.Contains(err.Error(), "unknown flag") && !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should mention unknown flag; got: %v\noutput: %s", err, out)
	}
}

// ---------------------------------------------------------------------------
// ywai session list — known flags defined by the spec
// ---------------------------------------------------------------------------

func TestSessionList_StatusFlag_IsRecognized(t *testing.T) {
	out, err := executeCommand("session", "list", "--status", "active")
	if err != nil {
		t.Fatalf("expected no error, got: %v\noutput: %s", err, out)
	}
	// The flag should be recognized and output should still be valid (no session rows).
	if !strings.Contains(out, "No active sessions") {
		t.Errorf("expected 'No active sessions' message; got:\n%s", out)
	}
}

func TestSessionList_LimitFlag_IsRecognized(t *testing.T) {
	out, err := executeCommand("session", "list", "--limit", "10")
	if err != nil {
		t.Fatalf("expected no error, got: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "No active sessions") {
		t.Errorf("expected 'No active sessions' message; got:\n%s", out)
	}
}

func TestSessionList_SinceFlag_IsRecognized(t *testing.T) {
	out, err := executeCommand("session", "list", "--since", "24h")
	if err != nil {
		t.Fatalf("expected no error, got: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "No active sessions") {
		t.Errorf("expected 'No active sessions' message; got:\n%s", out)
	}
}

func TestSessionList_RepoFlag_IsRecognized(t *testing.T) {
	out, err := executeCommand("session", "list", "--repo", "some-repo")
	if err != nil {
		t.Fatalf("expected no error, got: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "No active sessions") {
		t.Errorf("expected 'No active sessions' message; got:\n%s", out)
	}
}

func TestSessionList_PositionalArg_Ignored(t *testing.T) {
	out, err := executeCommand("session", "list", "extra-arg")
	if err != nil {
		t.Fatalf("expected no error (positional arg should be accepted/ignored), got: %v", err)
	}
	if !strings.Contains(out, "No active sessions") {
		t.Errorf("expected 'No active sessions' message; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Edge cases: combined flags and boundaries
// ---------------------------------------------------------------------------

func TestSessionList_JSONFlag_WithSinceFlag(t *testing.T) {
	out, err := executeCommand("session", "list", "--json", "--since", "24h")
	if err != nil {
		t.Fatalf("expected no error, got: %v\noutput: %s", err, out)
	}
	got := strings.TrimSpace(out)
	if got != "[]" {
		t.Errorf("expected empty JSON array '[]' when --json combined with --since, got: %s", got)
	}
}

func TestSessionList_JSONFlag_WithStatusFlag(t *testing.T) {
	out, err := executeCommand("session", "list", "--json", "--status", "active")
	if err != nil {
		t.Fatalf("expected no error, got: %v\noutput: %s", err, out)
	}
	got := strings.TrimSpace(out)
	if got != "[]" {
		t.Errorf("expected empty JSON array '[]' when --json combined with --status, got: %s", got)
	}
}

func TestSessionList_LimitFlag_ZeroValue(t *testing.T) {
	out, err := executeCommand("session", "list", "--limit", "0")
	if err != nil {
		t.Fatalf("expected no error for --limit 0 (boundary), got: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "No active sessions") {
		t.Errorf("expected 'No active sessions' message; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// ywai session start — starting sessions
// ---------------------------------------------------------------------------

func TestSessionStart_IsRecognized(t *testing.T) {
	out, err := executeCommand("session", "start", "--goal", "fix bug")
	if err != nil {
		t.Fatalf("expected no error, got: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "Started session") {
		t.Errorf("expected 'Started session' message; got:\n%s", out)
	}
}

func TestSessionStart_GoalFlag_Required(t *testing.T) {
	out, err := executeCommand("session", "start")
	if err == nil {
		t.Fatal("expected error when --goal is missing, got nil")
	}
	if !strings.Contains(err.Error(), "goal") && !strings.Contains(out, "goal") {
		t.Errorf("error should mention --goal is required; got: %v\noutput: %s", err, out)
	}
}

func TestSessionStart_JSONFlag_OutputsValidJSON(t *testing.T) {
	out, err := executeCommand("session", "start", "--goal", "fix bug", "--json")
	if err != nil {
		t.Fatalf("expected no error, got: %v\noutput: %s", err, out)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Errorf("expected valid JSON output, got parse error: %v\noutput: %s", err, out)
	}
	if _, ok := parsed["id"]; !ok {
		t.Errorf("expected JSON to contain 'id' field; got: %+v", parsed)
	}
	if _, ok := parsed["goal"]; !ok {
		t.Errorf("expected JSON to contain 'goal' field; got: %+v", parsed)
	}
}

func TestSessionStart_Help_ShowsUsage(t *testing.T) {
	out, err := executeCommand("session", "start", "--help")
	if err != nil {
		t.Fatalf("expected no error for --help, got: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "Usage") {
		t.Errorf("output should contain 'Usage'; got:\n%s", out)
	}
	if !strings.Contains(out, "start") {
		t.Errorf("output should mention 'start'; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// ywai session stop — stopping sessions
// ---------------------------------------------------------------------------

func TestSessionStop_IsRecognized(t *testing.T) {
	out, err := executeCommand("session", "stop", "test-session-123")
	if err != nil {
		t.Fatalf("expected no error, got: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "Stopped session") {
		t.Errorf("expected 'Stopped session' message; got:\n%s", out)
	}
}

func TestSessionStop_MissingID_ReturnsError(t *testing.T) {
	out, err := executeCommand("session", "stop")
	if err == nil {
		t.Fatal("expected error when session ID is missing, got nil")
	}
	if !strings.Contains(err.Error(), "accepts") && !strings.Contains(err.Error(), "arg") && !strings.Contains(err.Error(), "id") && !strings.Contains(out, "id") {
		t.Errorf("error should mention missing session ID; got: %v\noutput: %s", err, out)
	}
}

func TestSessionStop_JSONFlag_OutputsValidJSON(t *testing.T) {
	out, err := executeCommand("session", "stop", "test-session-123", "--json")
	if err != nil {
		t.Fatalf("expected no error, got: %v\noutput: %s", err, out)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Errorf("expected valid JSON output, got parse error: %v\noutput: %s", err, out)
	}
	if _, ok := parsed["id"]; !ok {
		t.Errorf("expected JSON to contain 'id' field; got: %+v", parsed)
	}
	if _, ok := parsed["status"]; !ok {
		t.Errorf("expected JSON to contain 'status' field; got: %+v", parsed)
	}
}

func TestSessionStop_Help_ShowsUsage(t *testing.T) {
	out, err := executeCommand("session", "stop", "--help")
	if err != nil {
		t.Fatalf("expected no error for --help, got: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "Usage") {
		t.Errorf("output should contain 'Usage'; got:\n%s", err)
	}
	if !strings.Contains(out, "stop") {
		t.Errorf("output should mention 'stop'; got:\n%s", err)
	}
}
