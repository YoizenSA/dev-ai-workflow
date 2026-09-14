package main

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// waitForOccupied polls until a TCP bind on port fails (something is listening),
// or returns false when the timeout elapses. Used to synchronize tests on the
// occupant child actually grabbing the port before assertions run.
func waitForOccupied(port int, timeout time.Duration) bool {
	addr := "127.0.0.1:" + itoa(port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return true // bind failed -> something holds the port
		}
		ln.Close()
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

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

// TestAcquirePort_KillsOccupantAndBinds reproduces the race that broke `ywai
// serve` after `ywai update`: killPort sends SIGTERM and returns at once, but
// the old process has not released the socket yet. An immediate single retry
// of net.Listen failed with "still in use after cleanup". acquirePort must kill
// the occupant and then retry the bind with backoff until the OS frees the
// port (or give up after a bounded timeout).
func TestAcquirePort_KillsOccupantAndBinds(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: spawns child processes")
	}
	port := freePort(t)

	// Occupy the port with a child that holds the socket until it is killed.
	// The child is this test binary itself (the canonical Go helper-process
	// pattern), so no python/POSIX tooling is needed on any GOOS.
	cmd := helperProcess(t, "^TestHelperPortOccupant$",
		envHelperPort+"="+itoa(port))
	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn occupant: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	// Wait until the child actually holds the port before we assert it's
	// occupied and try to acquire it.
	if !waitForOccupied(port, 10*time.Second) {
		t.Fatalf("occupant did not bind port %d", port)
	}

	// acquirePort must kill the occupant and win the bind within the retry window.
	ln, err := acquirePort(port)
	if err != nil {
		t.Fatalf("acquirePort(%d) failed: %v (the immediate-retry race)", port, err)
	}
	if ln == nil {
		t.Fatal("acquirePort returned nil listener with nil error")
	}
	defer ln.Close()
}

// TestAcquirePort_FreePortSucceeds is the common case: a free port binds on the
// first try with no killing, so acquirePort must not error or require a retry.
func TestAcquirePort_FreePortSucceeds(t *testing.T) {
	port := freePort(t)
	ln, err := acquirePort(port)
	if err != nil {
		t.Fatalf("acquirePort on free port %d: %v", port, err)
	}
	defer ln.Close()
}

// TestKillPIDs_KillsMultipleRealProcesses exercises killPIDs against two real
// child processes (the same shape as the bug, where lsof returned one PID per
// line: "PID1\nPID2"). The unit tests cover parsing; this confirms that every
// PID in the set is actually signalled, which is what freed the stuck port.
func TestKillPIDs_KillsMultipleRealProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test: spawns child processes")
	}
	// Spawn two dummy long-lived children (this test binary in sleeper mode,
	// so the test needs no `sleep` binary and runs on every GOOS). We keep
	// the *exec.Cmd handles so we can Wait() on them: a child that received
	// SIGTERM but was never waited for stays as a zombie, and Signal(0)
	// would still report it "alive", so probing the PID is unreliable.
	// Wait() reaps the zombie and reports the real exit status.
	type child struct {
		cmd *exec.Cmd
		pid int
	}
	var children []child
	for i := 0; i < 2; i++ {
		cmd := helperProcess(t, "^TestHelperSleeper$", envHelperPort+"=0")
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

	// Wait reaps each child; a signalled child exits non-cleanly (non-nil err
	// from Wait: "signal: terminated" on POSIX, "exit status 1" from
	// TerminateProcess on Windows), which is the success case here.
	for _, c := range children {
		err := c.cmd.Wait()
		if err == nil {
			// The sleeper exiting cleanly on its own means the kill never landed.
			t.Errorf("child %d exited without receiving a signal", c.pid)
		}
		// Any non-nil err (signal: terminated / killed) confirms we signalled it.
	}
}

// ─── helper-process plumbing ──────────────────────────────────────────────

// envHelperPort tells the port-occupant helper which port to bind. "0"
// (sleeper mode) binds nothing.
const envHelperPort = "YWAI_TEST_HELPER_PORT"

// helperProcess builds an exec.Cmd that re-runs this test binary with only
// the named test, guarded by envHelperPort so a plain `go test` run never
// enters helper mode. This replaces python3/sleep children, which do not
// exist on every machine the suite must pass on.
func helperProcess(t *testing.T, runPattern string, extraEnv string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run", runPattern, "-test.count=1")
	cmd.Env = append(os.Environ(), extraEnv)
	return cmd
}

// TestHelperPortOccupant binds 127.0.0.1:<envHelperPort> and holds it until
// killed. It is only ever selected as a child process of
// TestAcquirePort_KillsOccupantAndBinds.
func TestHelperPortOccupant(t *testing.T) {
	port := os.Getenv(envHelperPort)
	if port == "" || port == "0" {
		t.Skip("helper process only: set YWAI_TEST_HELPER_PORT")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		t.Fatalf("helper cannot bind port %s: %v", port, err)
	}
	defer ln.Close()
	// Hold the socket until the parent kills us. A plain sleep bounds the
	// process lifetime if the parent dies before killing us.
	time.Sleep(30 * time.Second)
}

// TestHelperSleeper just lives for a while so TestKillPIDs has a real
// process to signal.
func TestHelperSleeper(t *testing.T) {
	if os.Getenv(envHelperPort) == "" {
		t.Skip("helper process only")
	}
	time.Sleep(30 * time.Second)
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

func TestOpenCodeChildEnvClearsServerAuthentication(t *testing.T) {
	env := openCodeChildEnv([]string{
		"PATH=/bin",
		"OPENCODE_SERVER_PASSWORD=secret",
		"OPENCODE_SERVER_USERNAME=custom-user",
	}, "managed-password")

	for _, entry := range env {
		if entry == "OPENCODE_SERVER_PASSWORD=secret" || entry == "OPENCODE_SERVER_USERNAME=custom-user" {
			t.Fatalf("child environment retained server credentials: %q", entry)
		}
	}
	want := map[string]bool{
		"OPENCODE_SERVER_PASSWORD=managed-password": true,
		"OPENCODE_SERVER_USERNAME=opencode":         true,
	}
	for _, entry := range env {
		delete(want, entry)
	}
	if len(want) != 0 {
		t.Fatalf("child environment missing cleared auth variables: %v", want)
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
