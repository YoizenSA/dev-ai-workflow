//go:build !windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

// reexecSelf replaces the current process with the binary at exePath.
// On Unix, syscall.Exec atomically replaces the process image.
func reexecSelf(exePath string) {
	if err := syscall.Exec(exePath, os.Args, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "failed to re-exec after update: %v\n", err)
		os.Exit(1)
	}
}
