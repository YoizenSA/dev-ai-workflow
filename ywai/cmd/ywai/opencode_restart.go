package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// openCodeProcess is one running OpenCode process found on this machine.
type openCodeProcess struct {
	pid  int
	args []string
}

// isServer reports whether the process is a headless server rather than an
// interactive session. Only servers may be restarted: a TUI holds the user's
// conversation, and killing it throws away work that is not on disk anywhere.
func (p openCodeProcess) isServer() bool {
	// Only the subcommand position counts. Scanning the whole command line
	// would read `opencode2 run "... serve ..."` as a server and kill the
	// session that prompt belongs to.
	return len(p.args) > 1 && p.args[1] == "serve"
}

// findOpenCodeProcesses lists running OpenCode processes by scanning /proc.
// Returns nothing on platforms without /proc, where the caller falls back to
// telling the user to restart OpenCode by hand.
func findOpenCodeProcesses() []openCodeProcess {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	var found []openCodeProcess
	self := os.Getpid()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == self {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil || len(raw) == 0 {
			continue
		}
		args := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
		if len(args) == 0 {
			continue
		}
		// Match the executable name, not the whole command line: a shell
		// running `grep opencode2` must not look like an OpenCode process.
		base := filepath.Base(args[0])
		if base != "opencode" && base != "opencode2" {
			continue
		}
		found = append(found, openCodeProcess{pid: pid, args: args})
	}
	return found
}

// restartOpenCodeServers stops every running OpenCode server so it reloads the
// config this run just wrote. Plugins, MCP servers and agent frontmatter are
// read once at startup, so without this an install lands on disk and changes
// nothing until the next restart — which is what made a fresh install look
// like it had done nothing at all.
//
// Interactive sessions are reported, never killed: they hold conversation
// state that exists nowhere else, so reopening one is the user's call.
func restartOpenCodeServers(r *applyResult, dryRun bool) {
	procs := findOpenCodeProcesses()
	if len(procs) == 0 {
		fmt.Println("  No OpenCode process running; the next start reads the new config.")
		return
	}

	var servers, sessions []openCodeProcess
	for _, p := range procs {
		if p.isServer() {
			servers = append(servers, p)
			continue
		}
		sessions = append(sessions, p)
	}

	for _, p := range servers {
		if dryRun {
			fmt.Printf("  Would stop OpenCode server (PID %d)\n", p.pid)
			continue
		}
		if err := killPIDInt(p.pid); err != nil {
			r.warnf("could not stop the OpenCode server on PID %d (%v); it keeps serving the previous "+
				"config until you restart it yourself", p.pid, err)
			continue
		}
		fmt.Printf("  Stopped OpenCode server (PID %d) — it restarts on the next request\n", p.pid)
	}

	if len(sessions) > 0 {
		fmt.Printf("  %d interactive OpenCode session(s) still running the previous config: "+
			"close and reopen them to pick up plugins and agents (PID %s)\n",
			len(sessions), joinPIDs(sessions))
	}
}

func joinPIDs(procs []openCodeProcess) string {
	out := make([]string, 0, len(procs))
	for _, p := range procs {
		out = append(out, strconv.Itoa(p.pid))
	}
	return strings.Join(out, ", ")
}
