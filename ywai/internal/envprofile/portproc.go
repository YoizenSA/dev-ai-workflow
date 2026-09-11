package envprofile

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// portproc.go — fork-aware process control for profile servers.
//
// opencode2 serve forks: the pid recorded at launch is the launcher, while
// the process actually listening on the profile port is its child. Killing
// only the pidfile pid orphans the listener, which keeps the port and the
// database/log files locked forever. Every stop path below therefore also
// resolves the listener by port and, only when its image is an opencode
// binary, stops that too. Foreign processes on the port are never touched:
// at worst Start fast-fails naming the conflict.

// listenerPID returns the pid listening on TCP port on localhost, or 0 when
// none is found or the platform lookup is unavailable.
func listenerPID(port int) int {
	if runtime.GOOS == "windows" {
		return windowsListenerPID(port)
	}
	return unixListenerPID(port)
}

func windowsListenerPID(port int) int {
	out, err := exec.Command("netstat", "-ano", "-p", "tcp").Output()
	if err != nil {
		return 0
	}
	want := ":" + strconv.Itoa(port)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		// TCP    127.0.0.1:5800    0.0.0.0:0    LISTENING    1234
		if len(fields) != 5 || fields[0] != "TCP" || fields[3] != "LISTENING" {
			continue
		}
		if !strings.HasSuffix(fields[1], want) {
			continue
		}
		if pid, err := strconv.Atoi(fields[4]); err == nil && pid > 0 {
			return pid
		}
	}
	return 0
}

func unixListenerPID(port int) int {
	for _, args := range [][]string{
		{"lsof", "-ti", fmt.Sprintf("tcp:%d", port)},
		{"ss", "-ltnp"},
	} {
		if pid := parsePortTool(args, port); pid != 0 {
			return pid
		}
	}
	return 0
}

func parsePortTool(args []string, port int) int {
	out, err := exec.Command(args[0], args[1:]...).Output()
	if err != nil {
		return 0
	}
	if args[0] == "lsof" {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(out))); err == nil && pid > 0 {
			return pid
		}
		return 0
	}
	// ss -ltnp line: ... pid=1234,fd=...
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, ":"+strconv.Itoa(port)) {
			continue
		}
		if i := strings.Index(line, "pid="); i >= 0 {
			rest := line[i+4:]
			digits := ""
			for _, c := range rest {
				if c < '0' || c > '9' {
					break
				}
				digits += string(c)
			}
			if pid, err := strconv.Atoi(digits); err == nil && pid > 0 {
				return pid
			}
		}
	}
	return 0
}

// isOpencodeProcess reports whether pid runs an opencode binary, by image
// name. Unknown (lookup failed) means false: never kill what we cannot name.
func isOpencodeProcess(pid int) bool {
	name := processImageName(pid)
	base := strings.ToLower(name)
	if i := strings.LastIndexAny(base, `/\`); i >= 0 {
		base = base[i+1:]
	}
	base = strings.TrimSuffix(base, ".exe")
	return base == "opencode" || base == "opencode2"
}

func processImageName(pid int) string {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
		if err != nil {
			return ""
		}
		// "opencode2.exe","1234",...
		var first []byte
		if i := bytes.IndexByte(out, '\n'); i >= 0 {
			first = out[:i]
		} else {
			first = out
		}
		fields := strings.Split(string(first), ",")
		if len(fields) == 0 {
			return ""
		}
		return strings.Trim(fields[0], `" `)
	}
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
