//go:build windows

package main

import "syscall"

// Windows process creation flags. Spelled out here because syscall exposes only
// some of them and x/sys is not a dependency.
const (
	// createNoWindow runs a console application without giving it a console
	// window. Without it a backgrounded server inherits the console of whatever
	// launched it and holds that window open for its whole life — which is why
	// the logon scheduled task left a visible "Server started in background"
	// terminal on every boot.
	createNoWindow = 0x08000000
	// createNewProcessGroup keeps the child out of the launcher's Ctrl+C group,
	// so closing the window that started it does not signal the server.
	createNewProcessGroup = 0x00000200
)

// sysProcAttr detaches the forked server from the console that started it.
//
// The Unix build uses Setsid for the same purpose. Windows has no session
// leader, so the equivalent is declining a console outright.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: createNoWindow | createNewProcessGroup,
		HideWindow:    true,
	}
}
