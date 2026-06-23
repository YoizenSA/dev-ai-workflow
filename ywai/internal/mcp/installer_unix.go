//go:build !windows

package mcp

import (
	"os"
	"os/exec"
	"syscall"
)

// configureProcessGroup puts the command in its own process group so that
// SIGKILL on the leader propagates to the whole group and no orphaned
// grandchildren hold the write ends of stdout/stderr open (which would hang
// cmd.Wait).
func configureProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.Cancel = func() error {
		if p := cmd.Process; p != nil {
			_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
		}
		return os.ErrProcessDone
	}
}
