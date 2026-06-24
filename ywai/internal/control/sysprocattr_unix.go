//go:build !windows

package control

import "syscall"

// detachedSysProcAttr starts the child in a new session so it survives the
// parent server being killed during the update's restart step.
func detachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
