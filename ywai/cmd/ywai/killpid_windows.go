//go:build windows

package main

import (
	"fmt"
	"os/exec"
)

// killPIDInt kills a process by PID on Windows using taskkill /F.
// Windows does not support syscall.SIGTERM, so we use taskkill which
// is the standard way to terminate a process from the command line.
func killPIDInt(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/PID", fmt.Sprintf("%d", pid))
	if err := cmd.Run(); err != nil {
		// If the process already exited, taskkill returns an error but
		// the port is free — treat exit code 128 as success.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			return nil
		}
		return fmt.Errorf("taskkill PID %d: %w", pid, err)
	}
	return nil
}
