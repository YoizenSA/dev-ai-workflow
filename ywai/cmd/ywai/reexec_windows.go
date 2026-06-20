//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

// reexecSelf re-launches the binary at exePath on Windows.
// Windows does not support syscall.Exec, so we spawn a new process
// and exit the current one. If a deferred replace was staged, the
// batch script handles the swap; otherwise we just start the new binary.
func reexecSelf(exePath string) {
	cmd := exec.Command(exePath, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to re-exec after update: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
