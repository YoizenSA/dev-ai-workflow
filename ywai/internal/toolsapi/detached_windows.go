//go:build windows

package toolsapi

import (
	"os/exec"
	"syscall"
)

// setDetached moves the child out of the parent console control group, so
// console signals (Ctrl+C/Break) aimed at ywai do not take the child down.
// This mirrors control.detachedSysProcAttr, which toolsapi cannot import.
func setDetached(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP
}
