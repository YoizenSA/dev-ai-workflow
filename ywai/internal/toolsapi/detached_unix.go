//go:build !windows

package toolsapi

import (
	"os/exec"
	"syscall"
)

// setDetached starts the child in a new session so it survives the ywai
// process. This mirrors control.detachedSysProcAttr, which toolsapi cannot
// import.
func setDetached(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}
