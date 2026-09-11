package envprofile

import (
	"fmt"
	"os/exec"
	"strings"
)

// StopManagedService stops the env's opencode2 managed service (the one the
// TUI and `run` attach to) so it reloads config and logins on its next start;
// both restart it on demand. Stopping an already stopped service succeeds.
func StopManagedService(p Profile, opencodeBin string) error {
	env, err := profileCmdEnv(p)
	if err != nil {
		return err
	}
	cmd := exec.Command(opencodeBin, "service", "stop")
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stop env %q managed service: %v: %s", p.Name, err, strings.TrimSpace(string(out)))
	}
	return nil
}
