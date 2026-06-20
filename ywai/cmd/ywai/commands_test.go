package main

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// freePort grabs an unused TCP port for tests that need to bind a real listener.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// TestKillPIDs_KillsMultipleRealProcesses exercises killPIDs against two real
// child processes (the same shape as the bug, where lsof returned one PID per
// line: "PID1\nPID2"). The unit tests cover parsing; this confirms that every
// PID in the set is actually signalled, which is what freed the stuck port.
func TestKillPIDs_KillsMultipleRealProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: spawns child processes")
	}
	// Spawn two dummy long-lived children. We keep the *exec.Cmd handles so we
	// can Wait() on them: a child that received SIGTERM but was never waited
	// for stays as a zombie, and Signal(0) would still report it "alive", so
	// probing the PID is unreliable. Wait() reaps the zombie and reports the
	// real exit status.
	type child struct {
		cmd *exec.Cmd
		pid int
	}
	var children []child
	for i := 0; i < 2; i++ {
		cmd := exec.Command("sleep", "30")
		if err := cmd.Start(); err != nil {
			t.Fatalf("spawn child %d: %v", i, err)
		}
		children = append(children, child{cmd: cmd, pid: cmd.Process.Pid})
	}
	pids := []int{children[0].pid, children[1].pid}

	// killPIDs must signal both, not Atoi-fail on the first as the old code did.
	if err := killPIDs(pids); err != nil {
		t.Fatalf("killPIDs(%v) failed: %v", pids, err)
	}

	// Wait reaps each child; a SIGTERM'd sleep exits with "signal: terminated"
	// (non-nil err from Wait), which is the success case here. An error of
	// "signal: terminated" means the signal landed — that's what we want.
	for _, c := range children {
		err := c.cmd.Wait()
		if err == nil {
			// sleep 30 exiting cleanly on its own means it ignored SIGTERM — bad.
			t.Errorf("child %d exited without receiving a signal", c.pid)
		}
		// Any non-nil err (signal: terminated / killed) confirms we signalled it.
	}
}

// itoa formats a non-negative int without pulling in strconv for a single use.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// TestKillPIDsFromOutput_ParsesMultipleLines reproduces the bug where
// `lsof -ti :<port>` returns one PID per line (e.g. "1040\n19768\n" when both
// a parent and a child process hold the port). killPort previously passed the
// whole multiline string to killPID, which did strconv.Atoi("1040\n19768")
// and failed with: invalid PID "1040\n19768".
//
// parsePIDs must split on any whitespace/newline so every PID is attempted.
func TestParsePIDs_SplitsMultipleLines(t *testing.T) {
	raw := "1040\n19768\n"
	got := parsePIDs(raw)
	want := []int{1040, 19768}
	if len(got) != len(want) {
		t.Fatalf("parsePIDs(%q) = %v, want %v", raw, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("parsePIDs(%q)[%d] = %d, want %d", raw, i, got[i], want[i])
		}
	}
}

// TestParsePIDs_HandlesSinglePID keeps the common-case behavior intact.
func TestParsePIDs_SinglePID(t *testing.T) {
	got := parsePIDs("49923\n")
	if len(got) != 1 || got[0] != 49923 {
		t.Errorf("parsePIDs single = %v, want [49923]", got)
	}
}

// TestParsePIDs_IgnoresGarbage skips non-numeric tokens (e.g. lsof error text)
// instead of returning an invalid entry that breaks the kill loop.
func TestParsePIDs_IgnoresGarbage(t *testing.T) {
	got := parsePIDs("not-a-pid\n  49923  \n\n")
	if len(got) != 1 || got[0] != 49923 {
		t.Errorf("parsePIDs garbage = %v, want [49923]", got)
	}
}

// TestParsePIDs_Empty returns nil for blank output, so the caller can fall
// through to the next lookup strategy.
func TestParsePIDs_Empty(t *testing.T) {
	if got := parsePIDs("   \n  "); got != nil {
		t.Errorf("parsePIDs blank = %v, want nil", got)
	}
}

// TestReadStopPIDFile_HandlesMultiplePIDs guards stop against a PID file that
// ever contains more than one PID (e.g. written by a buggy background launch).
// readStopPIDFile must return the first valid PID rather than Atoi-failing on
// the whole blob.
func TestReadStopPIDFile_HandlesMultiplePIDs(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "serve.pid")
	if err := os.WriteFile(pidFile, []byte("1040\n19768\n"), 0o644); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	got, err := readStopPIDFile(pidFile)
	if err != nil {
		t.Fatalf("readStopPIDFile unexpected error: %v", err)
	}
	if got != 1040 {
		t.Errorf("readStopPIDFile = %d, want first PID 1040", got)
	}
}

// TestReadStopPIDFile_InvalidReturnsError surfaces a real parse failure when
// the file contains no numeric PID at all (so we don't silently treat garbage
// as PID 0).
func TestReadStopPIDFile_InvalidReturnsError(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "serve.pid")
	if err := os.WriteFile(pidFile, []byte("not-a-number"), 0o644); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	if _, err := readStopPIDFile(pidFile); err == nil {
		t.Fatal("readStopPIDFile expected error for non-numeric content, got nil")
	} else if !strings.Contains(err.Error(), "invalid PID") {
		t.Errorf("readStopPIDFile error = %v, want it to mention 'invalid PID'", err)
	}
}
