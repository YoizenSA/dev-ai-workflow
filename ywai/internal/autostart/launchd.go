package autostart

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

const launchdPlistName = "com.ywai.server.plist"

// configureLaunchd sets up a launchd agent for ywai serve.
func configureLaunchd() error {
	// Get ywai binary path
	binaryPath, err := getYwaiBinaryPath()
	if err != nil {
		return fmt.Errorf("failed to get ywai binary path: %w", err)
	}

	// Get user directory
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Create LaunchAgents directory
	agentsDir := filepath.Join(usr.HomeDir, "Library", "LaunchAgents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	// Create plist file
	plistPath := filepath.Join(agentsDir, launchdPlistName)
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.ywai.server</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>serve</string>
        <string>--background</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/ywai-server.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/ywai-server-error.log</string>
</dict>
</plist>
`, binaryPath)

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("failed to write plist file: %w", err)
	}

	// Load the agent
	if _, err := runCommand("launchctl", "load", plistPath); err != nil {
		return fmt.Errorf("failed to load launchd agent: %w", err)
	}

	return nil
}

// disableLaunchd removes the launchd agent.
func disableLaunchd() error {
	// Get user directory
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Unload the agent
	plistPath := filepath.Join(usr.HomeDir, "Library", "LaunchAgents", launchdPlistName)
	if _, err := runCommand("launchctl", "unload", plistPath); err != nil {
		// Ignore errors if agent doesn't exist
	}

	// Remove plist file
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove plist file: %w", err)
	}

	return nil
}

// isLaunchdEnabled checks if the launchd agent is loaded.
func isLaunchdEnabled() (bool, error) {
	usr, err := user.Current()
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}

	plistPath := filepath.Join(usr.HomeDir, "Library", "LaunchAgents", launchdPlistName)
	if _, err := os.Stat(plistPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
