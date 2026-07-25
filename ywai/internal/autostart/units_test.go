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
	plist := launchdPlistContent("/usr/local/bin/ywai")

	if strings.Contains(plist, "--background") {
		t.Error("launchd plist must not fork: KeepAlive + a forking process is a relaunch loop")
	}
	if !strings.Contains(plist, "<string>serve</string>") {
		t.Errorf("plist lost the serve argument:\n%s", plist)
	}
}

func TestUnitsEmbedTheResolvedBinaryPath(t *testing.T) {
	const bin = "/opt/custom/ywai"
	if !strings.Contains(systemdUnitContent(bin), bin) {
		t.Error("systemd unit lost the binary path")
	}
	if !strings.Contains(launchdPlistContent(bin), bin) {
		t.Error("launchd plist lost the binary path")
	}
}
