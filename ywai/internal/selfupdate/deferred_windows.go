//go:build windows

package selfupdate

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// deferredReplace handles the case where Windows refuses to rename the
// running executable ("Access denied"). It copies the new binary to
// <exe>.new, writes a batch script that waits for the current process to
// exit, swaps the files, re-launches ywai, and cleans up. The batch script
// is launched detached and this function returns the new version so the
// caller can exit immediately.
func deferredReplace(newBinary, exePath, version string) (string, error) {
	newPath := exePath + ".new"
	srcFile, err := os.Open(newBinary)
	if err != nil {
		return "", fmt.Errorf("open new binary: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(newPath)
	if err != nil {
		return "", fmt.Errorf("create .new file: %w", err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return "", fmt.Errorf("copy new binary: %w", err)
	}
	_ = dstFile.Close()

	if err := os.Chmod(newPath, 0o755); err != nil {
		return "", fmt.Errorf("chmod new binary: %w", err)
	}

	pid := os.Getpid()
	batPath := exePath + ".update.bat"
	exeName := filepath.Base(exePath)
	newName := filepath.Base(newPath)

	batContent := fmt.Sprintf(`@echo off
:wait
tasklist /FI "PID eq %d" 2>NUL | find "%d" >NUL
if not errorlevel 1 (
    timeout /t 1 /nobreak >NUL
    goto wait
)
copy /Y "%s" "%s" >NUL
del "%s" >NUL
start "" "%s"
del "%%~f0" >NUL
`, pid, pid, newName, exeName, newPath, exeName)

	if err := os.WriteFile(batPath, []byte(batContent), 0o644); err != nil {
		return "", fmt.Errorf("write update batch script: %w", err)
	}

	cmd := exec.Command("cmd", "/C", "start", "/B", batPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008, // DETACHED_PROCESS
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("launch update script: %w", err)
	}

	fmt.Printf("  Update staged: %s will be replaced on exit.\n", exeName)
	return version, nil
}
