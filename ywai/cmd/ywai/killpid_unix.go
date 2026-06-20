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
func killPIDInt(pid int) error {
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
