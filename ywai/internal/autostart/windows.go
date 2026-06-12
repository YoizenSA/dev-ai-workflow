package autostart

import (
	"fmt"
	"os/exec"
)

const windowsTaskName = "ywai-server"

// configureWindows sets up a Windows scheduled task for ywai serve.
func configureWindows() error {
	// Get ywai binary path
	binaryPath, err := getYwaiBinaryPath()
	if err != nil {
		return fmt.Errorf("failed to get ywai binary path: %w", err)
	}

	// Create scheduled task using schtasks
	// This creates a task that runs at user logon
	cmd := exec.Command("schtasks", "/create",
		"/tn", windowsTaskName,
		"/tr", fmt.Sprintf(`"%s" serve --background`, binaryPath),
		"/sc", "onlogon",
		"/rl", "highest",
		"/f")

	if _, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create scheduled task: %w", err)
	}

	return nil
}

// disableWindows removes the Windows scheduled task.
func disableWindows() error {
	// Delete scheduled task
	cmd := exec.Command("schtasks", "/delete",
		"/tn", windowsTaskName,
		"/f")

	if _, err := cmd.CombinedOutput(); err != nil {
		// Ignore errors if task doesn't exist
		return nil
	}

	return nil
}

// isWindowsEnabled checks if the Windows scheduled task exists.
func isWindowsEnabled() (bool, error) {
	// Query scheduled task
	cmd := exec.Command("schtasks", "/query",
		"/tn", windowsTaskName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, nil
	}

	// If the task exists, the output will contain the task name
	return len(output) > 0, nil
}
