package autostart

import (
	"strings"
	"testing"
)

// A service manager supervises the process it starts. If the unit asks ywai to
// fork (--background), the supervised process exits immediately, the manager
// reads that as a crash and restarts it, and acquirePort kills the orphan the
// previous cycle left behind. That loop ran 10k+ times in the field before it
// was caught, so both unit templates are pinned here.

func TestSystemdUnitRunsInForeground(t *testing.T) {
	unit := systemdUnitContent("/usr/local/bin/ywai")

	if strings.Contains(unit, "--background") {
		t.Error("systemd unit must not fork: Type=simple + a forking ExecStart is a restart loop")
	}
	if !strings.Contains(unit, "ExecStart=/usr/local/bin/ywai serve") {
		t.Errorf("unexpected ExecStart in unit:\n%s", unit)
	}
	// Restart=always only behaves if the process actually stays alive.
	if strings.Contains(unit, "Restart=always") && strings.Contains(unit, "Type=forking") {
		t.Error("Type=forking needs a PIDFile for Restart to track the child")
	}
}

func TestLaunchdPlistRunsInForeground(t *testing.T) {
	plist := launchdPlistContent("/usr/local/bin/ywai", "/Users/ywai/Library/Logs")

	if strings.Contains(plist, "--background") {
		t.Error("launchd plist must not fork: KeepAlive + a forking process is a relaunch loop")
	}
	if !strings.Contains(plist, "<string>serve</string>") {
		t.Errorf("plist lost the serve argument:\n%s", plist)
	}
}

// The agent's stdout/stderr must not land in /tmp: the system may clean it
// and other local users can read it. configureLaunchd passes the user's
// ~/Library/Logs, and the plist template must use it verbatim.
func TestLaunchdPlistLogsGoToUserLogDir(t *testing.T) {
	const logsDir = "/Users/ywai/Library/Logs"
	plist := launchdPlistContent("/usr/local/bin/ywai", logsDir)

	if !strings.Contains(plist, logsDir+"/ywai-server.log") {
		t.Errorf("plist must send stdout to %s/ywai-server.log:\n%s", logsDir, plist)
	}
	if !strings.Contains(plist, logsDir+"/ywai-server-error.log") {
		t.Errorf("plist must send stderr to %s/ywai-server-error.log:\n%s", logsDir, plist)
	}
	if strings.Contains(plist, "/tmp/ywai") {
		t.Error("plist must not write agent logs to /tmp")
	}
}

func TestUnitsEmbedTheResolvedBinaryPath(t *testing.T) {
	const bin = "/opt/custom/ywai"
	if !strings.Contains(systemdUnitContent(bin), bin) {
		t.Error("systemd unit lost the binary path")
	}
	if !strings.Contains(launchdPlistContent(bin, "/Users/ywai/Library/Logs"), bin) {
		t.Error("launchd plist lost the binary path")
	}
}
