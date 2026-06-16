package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// Configure sets up the control server to start automatically on system boot.
func Configure() error {
	switch runtime.GOOS {
	case "linux":
		return configureSystemd()
	case "darwin":
		return configureLaunchd()
	case "windows":
		return configureWindows()
	default:
		return fmt.Errorf("autostart not supported on %s", runtime.GOOS)
	}
}

// Disable removes the autostart configuration.
func Disable() error {
	switch runtime.GOOS {
	case "linux":
		return disableSystemd()
	case "darwin":
		return disableLaunchd()
	case "windows":
		return disableWindows()
	default:
		return fmt.Errorf("autostart not supported on %s", runtime.GOOS)
	}
}

// IsEnabled checks if autostart is currently configured.
func IsEnabled() (bool, error) {
	switch runtime.GOOS {
	case "linux":
		return isSystemdEnabled()
	case "darwin":
		return isLaunchdEnabled()
	case "windows":
		return isWindowsEnabled()
	default:
		return false, fmt.Errorf("autostart not supported on %s", runtime.GOOS)
	}
}

// getYwaiBinaryPath returns the path to the ywai binary.
func getYwaiBinaryPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	// If the binary is a symlink, resolve it
	if link, err := os.Readlink(execPath); err == nil {
		execPath = link
	}

	return execPath, nil
}

// runCommand executes a command and returns its output.
func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}
