package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/installer"
	"github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/installer/api"
	"github.com/Yoizen/dev-ai-workflow/ywai/setup/wizard/pkg/installer/version"
)

const (
	repoOwner = "Yoizen"
	repoName  = "dev-ai-workflow"
)

func runSelfUpdate(flags *installer.Flags) error {
	fmt.Println("Checking for updates...")

	githubAPI := api.NewGitHubAPI(fmt.Sprintf("%s/%s", repoOwner, repoName))
	resolver := version.NewResolver(fmt.Sprintf("%s/%s", repoOwner, repoName))
	
	// Get current version
	currentVersion := flags.BuildVersion
	if currentVersion == "" || currentVersion == "dev" {
		fmt.Println("Current version: dev (development build)")
		fmt.Println("Self-update is only available for released versions")
		return nil
	}

	// Get latest version
	latestVersion, err := resolver.ResolveVersion("", flags.Channel)
	if err != nil {
		return fmt.Errorf("failed to resolve latest version: %w", err)
	}

	if latestVersion == "main" || latestVersion == "master" {
		fmt.Println("Latest version is main branch (no releases available)")
		return nil
	}

	fmt.Printf("Current version: %s\n", currentVersion)
	fmt.Printf("Latest version: %s\n", latestVersion)

	// Check if update is needed
	isNewer, err := api.IsNewerVersion(latestVersion, currentVersion)
	if err != nil {
		return fmt.Errorf("failed to compare versions: %w", err)
	}

	if !isNewer {
		fmt.Println("Already up to date")
		return nil
	}

	fmt.Println("Update available")

	// Download and install update
	if flags.DryRun {
		fmt.Println("DRY RUN: Would download and install update")
		return nil
	}

	return downloadAndInstall(latestVersion, githubAPI, flags)
}

func downloadAndInstall(version string, githubAPI *api.GitHubAPI, flags *installer.Flags) error {
	// Determine platform
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Map arch names
	archMap := map[string]string{
		"amd64": "amd64",
		"arm64": "arm64",
	}
	archName, ok := archMap[arch]
	if !ok {
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	// Construct download URL.
	// Format: https://github.com/Yoizen/dev-ai-workflow/releases/download/vX.Y.Z/setup-wizard-{os}-{arch}[.exe]
	//
	// The release workflow (`.github/workflows/release.yml`) uses the
	// Makefile `build-all` target which appends ".exe" for Windows builds
	// (see ywai/setup/Makefile). The asset uploaded to the GitHub Release is
	// therefore "setup-wizard-windows-amd64.exe" — without the extension
	// the download URL returns a 404.
	binaryName := fmt.Sprintf("setup-wizard-%s-%s", osName, archName)
	if osName == "windows" {
		binaryName += ".exe"
	}
	downloadURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", repoOwner, repoName, version, binaryName)

	fmt.Printf("Downloading from: %s\n", downloadURL)

	// Download to temp file. Keep the ".exe" suffix on Windows so SmartScreen
	// and Defender treat the downloaded file as an executable, not an
	// unknown-type blob (which some AV setups quarantine).
	tmpDir, err := os.MkdirTemp("", "ywai-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tmpFileName := "setup-wizard"
	if osName == "windows" {
		tmpFileName += ".exe"
	}
	tmpFile := filepath.Join(tmpDir, tmpFileName)
	if err := downloadFile(downloadURL, tmpFile); err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	// Make executable
	if err := os.Chmod(tmpFile, 0755); err != nil {
		return fmt.Errorf("failed to make executable: %w", err)
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	fmt.Printf("Installing to: %s\n", execPath)

	// Replace executable
	if err := replaceExecutable(tmpFile, execPath); err != nil {
		return fmt.Errorf("failed to replace executable: %w", err)
	}

	fmt.Printf("Successfully updated to %s\n", version)
	
	// Update global agents if not skipped
	if !flags.SkipGlobalAgentsUpdate {
		fmt.Println("\nUpdating global agents...")
		if err := runUpdateGlobalAgents(flags); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to update global agents: %v\n", err)
		}
	}
	
	return nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func replaceExecutable(src, dest string) error {
	// On Unix systems we can't overwrite the running executable directly, so
	// we hand off the move to a shell script that runs after this process
	// exits. On Windows we can rename the running .exe but we can't
	// overwrite it — so we first move the current exe out of the way, then
	// rename the freshly downloaded binary into place.

	if runtime.GOOS == "windows" {
		// Best-effort cleanup of a stale backup from a previous update.
		oldPath := dest + ".old"
		_ = os.Remove(oldPath)

		// Rename the running exe out of the way. Windows allows renaming a
		// locked/running .exe even though it blocks overwrite and delete.
		if err := os.Rename(dest, oldPath); err != nil {
			return fmt.Errorf("failed to rename current executable (%s -> %s): %w", dest, oldPath, err)
		}

		// Move the new binary into place. If this fails, try to roll back
		// the rename above so the user is not left without an executable.
		if err := os.Rename(src, dest); err != nil {
			if rollbackErr := os.Rename(oldPath, dest); rollbackErr != nil {
				return fmt.Errorf("failed to install new executable (%w); rollback also failed (%v)", err, rollbackErr)
			}
			return fmt.Errorf("failed to install new executable: %w", err)
		}

		// The .old file is still locked by the running process; we can't
		// delete it here. Schedule a best-effort cleanup by writing a
		// one-shot .cmd that retries the delete a moment after we exit.
		cleanupCmd := dest + ".cleanup.cmd"
		script := fmt.Sprintf("@echo off\r\nping -n 2 127.0.0.1 >nul\r\ndel /f /q \"%s\" >nul 2>&1\r\ndel /f /q \"%%~f0\" >nul 2>&1\r\n", oldPath)
		if err := os.WriteFile(cleanupCmd, []byte(script), 0644); err == nil {
			// Fire-and-forget; `start /B` already detaches the child from
			// this console. If this fails we simply leave an .old file
			// which the next update will remove via os.Remove above.
			cmd := exec.Command("cmd.exe", "/C", "start", "/B", "", cleanupCmd)
			_ = cmd.Start()
		}
		return nil
	}

	// On Unix, create a shell script to do the replacement
	scriptPath := dest + ".update.sh"
	script := fmt.Sprintf(`#!/bin/bash
sleep 1
mv "%s" "%s"
rm "%s"
`, src, dest, scriptPath)

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return err
	}

	// Execute the script in background
	cmd := exec.Command("bash", scriptPath)
	cmd.Start()

	return nil
}
