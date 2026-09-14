//go:build windows

package control

import "syscall"

// detachedSysProcAttr starts the child detached from the parent on Windows.
// CREATE_NEW_PROCESS_GROUP moves the child out of the parent's console
// control group, so console signals (Ctrl+C/Break) aimed at the parent do
// not take the update child down with it. This mirrors Setsid on Unix.
func detachedSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
