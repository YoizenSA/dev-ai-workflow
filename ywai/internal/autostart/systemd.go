package autostart

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

const systemdServiceName = "ywai-server.service"

// configureSystemd sets up a systemd user service for ywai serve.
func configureSystemd() error {
	// Check if systemd is available
	if _, err := runCommand("systemctl", "--user", "--version"); err != nil {
		return fmt.Errorf("systemd not available: %w", err)
	}

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

	// Create systemd user service directory
	serviceDir := filepath.Join(usr.HomeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("failed to create systemd directory: %w", err)
	}

	// Create service file
	servicePath := filepath.Join(serviceDir, systemdServiceName)
	serviceContent := fmt.Sprintf(`[Unit]
Description=ywai Unified Server
After=network.target

[Service]
Type=simple
ExecStart=%s serve --background
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
`, binaryPath)

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd
	if _, err := runCommand("systemctl", "--user", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable service
	if _, err := runCommand("systemctl", "--user", "enable", systemdServiceName); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	// Start service
	if _, err := runCommand("systemctl", "--user", "start", systemdServiceName); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

// disableSystemd removes the systemd service.
func disableSystemd() error {
	// Stop service
	if _, err := runCommand("systemctl", "--user", "stop", systemdServiceName); err != nil {
		// Ignore errors if service doesn't exist
	}

	// Disable service
	if _, err := runCommand("systemctl", "--user", "disable", systemdServiceName); err != nil {
		// Ignore errors if service doesn't exist
	}

	// Remove service file
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	servicePath := filepath.Join(usr.HomeDir, ".config", "systemd", "user", systemdServiceName)
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd
	if _, err := runCommand("systemctl", "--user", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	return nil
}

// isSystemdEnabled checks if the systemd service is enabled.
func isSystemdEnabled() (bool, error) {
	output, err := runCommand("systemctl", "--user", "is-enabled", systemdServiceName)
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(output) == "enabled", nil
}
