//go:build windows

package mcp

import (
	"os"
	"os/exec"
)

// configureProcessGroup adapts command cancellation for Windows.
// Windows has no POSIX process-group syscalls, so we rely on the standard
// Process.Kill behavior when the context is cancelled.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if p := cmd.Process; p != nil {
			_ = p.Kill()
		}
		return os.ErrProcessDone
	}
}
