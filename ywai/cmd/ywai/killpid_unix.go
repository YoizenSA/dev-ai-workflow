//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// killPIDInt sends SIGTERM to a numeric PID. An already-finished process is
// not an error here (the caller's intent — free the port — is satisfied).
// Non-positive PIDs are rejected before touching the OS: on unix PID -1
// (and 0) name a process group, not one process — signaling them would hit
// every process the caller may signal.
func killPIDInt(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID %d: refusing to signal a process group", pid)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("cannot find process %d: %w", pid, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return err
	}
	return nil
}
