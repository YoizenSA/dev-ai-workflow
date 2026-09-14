package envprofile

import (
	"os"
	"path/filepath"
	"testing"
)

// PidAlive must never report a non-positive pid as alive: on unix signal 0
// against PID -1 addresses a process group and succeeds while anything runs.
func TestPidAlive_RejectsNonPositive(t *testing.T) {
	for _, pid := range []int{-1, 0} {
		if PidAlive(pid) {
			t.Errorf("PidAlive(%d) = true; want false", pid)
		}
	}
	if !PidAlive(os.Getpid()) {
		t.Error("PidAlive(self) = false; want true")
	}
}

// Stop with a "-1" pid file must succeed and drop the garbage file instead
// of signaling a process group. It once failed `ywai env init` for the env.
func TestStop_RemovesNonPositivePidFile(t *testing.T) {
	testRoot(t)
	p := Profile{Name: "pidguard-test", Port: 5999}
	if err := os.MkdirAll(filepath.Dir(PidFile(p)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(PidFile(p), []byte("-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Stop(p); err != nil {
		t.Fatalf("Stop with -1 pid file = %v; want nil", err)
	}
	if _, err := os.Stat(PidFile(p)); !os.IsNotExist(err) {
		t.Error("stale -1 pid file still present after Stop; want removed")
	}
}
