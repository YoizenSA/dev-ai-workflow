//go:build windows

package mcp

import (
	"os"
	"os/exec"
	"strconv"
)

// configureProcessGroup adapts command cancellation for Windows.
// Windows has no POSIX process groups, and Process.Kill terminates only the
// direct child (the `sh -c` shell), leaving grandchildren such as npx/go
// orphaned. Those orphans keep running — and on Windows hold a lock on their
// own executable, which breaks temp-dir cleanup and leaks install processes
// on timeout. We use `taskkill /T /F` to terminate the whole process tree.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if p := cmd.Process; p != nil {
			if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(p.Pid)).Run(); err != nil {
				// Fall back to killing just the direct child if taskkill
				// is unavailable for any reason.
				_ = p.Kill()
			}
		}
		return os.ErrProcessDone
	}
}
