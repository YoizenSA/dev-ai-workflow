//go:build windows

package control

import "syscall"

// detachedSysProcAttr starts the child detached from the parent on Windows.
func detachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}
