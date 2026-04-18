package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func (i *Installer) installGA() error {
	if i.flags.SkipGA {
		return nil
	}

	i.logger.LogStep("Installing GA...")

	gaDir := i.getGADir()
	version := i.resolveVersion()

	// Check if we're running from the dev-ai-workflow repo with local GA source.
	// If yes, use the local source instead of downloading/cloning to avoid redundant downloads.
	if localGADir := i.findLocalGADir(); localGADir != "" && !i.flags.Force {
		i.logger.LogInfo("Using local GA source from: " + localGADir)
		if i.flags.DryRun {
			i.logger.Log("DRY RUN: Would build/install GA from local source")
			return nil
		}
		if err := i.installGAFromLocalSource(localGADir); err != nil {
			i.logger.LogWarning("Failed to install from local source: " + err.Error())
			i.logger.LogInfo("Falling back to download...")
		} else {
			i.logger.LogSuccess("GA installed successfully from local source")
			return nil
		}
	}

	if i.dirExists(gaDir) && !i.flags.Force {
		currentVersion := i.detectInstalledGAVersion(gaDir)
		if shouldPinVersion(version) && currentVersion != "" && currentVersion != version {
			i.logger.LogInfo(fmt.Sprintf("GA version mismatch (installed: %s, required: %s). Reinstalling pinned version...", currentVersion, version))
		} else if shouldPinVersion(version) && currentVersion == "" {
			i.logger.LogInfo(fmt.Sprintf("Could not detect installed GA version. Reinstalling pinned version %s...", version))
		} else {
			i.logger.LogSuccess("GA already installed at " + gaDir)
			return nil
		}
	}

	if i.flags.DryRun {
		i.logger.Log("DRY RUN: Would clone GA repository to " + gaDir + " @ " + version)
		return nil
	}

	if i.dirExists(gaDir) {
		if err := os.RemoveAll(gaDir); err != nil {
			return err
		}
	}

	i.logger.Log("Cloning GA repository...")
	if err := i.runCommand("git", "clone", "--depth", "1", "--branch", version, GA_REPO, gaDir); err != nil {
		i.logger.LogError("Failed to clone GA repository")
		return err
	}

	if err := i.installGASystemWide(gaDir); err != nil {
		i.logger.LogError("Failed to install GA system-wide")
		return err
	}

	i.logger.LogSuccess("GA installed successfully")
	return nil
}

// findLocalGADir returns the local ywai/ga directory if the wizard is being run
// from the dev-ai-workflow repository, otherwise returns empty string.
func (i *Installer) findLocalGADir() string {
	repoRoot := i.getRepoRoot()
	if repoRoot == "" {
		return ""
	}
	candidate := filepath.Join(repoRoot, "ywai", "ga")
	if i.dirExists(candidate) && i.fileExists(filepath.Join(candidate, "go.mod")) {
		return candidate
	}
	return ""
}

// installGAFromLocalSource builds GA from the local source directory and installs
// the resulting binary to the system-wide location.
func (i *Installer) installGAFromLocalSource(sourceDir string) error {
	home, _ := os.UserHomeDir()
	var binDir string
	if runtime.GOOS == "windows" {
		binDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "ywai")
	} else {
		binDir = filepath.Join(home, ".local", "bin")
	}

	if err := i.ensureDir(binDir); err != nil {
		return err
	}

	// Check if Go compiler is available
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("Go compiler not found in PATH, cannot build from local source: %w", err)
	}

	destBin := filepath.Join(binDir, "ga")
	if runtime.GOOS == "windows" {
		destBin += ".exe"
	}

	i.logger.Log("Building GA from local source...")
	ldflags := i.gaBuildLdflags(sourceDir)
	buildCmd := exec.Command("go", "build", "-ldflags="+ldflags, "-o", destBin, "./cmd/ga")
	buildCmd.Dir = sourceDir
	if err := i.runCommandWithCmd(buildCmd, "go", "build", "-ldflags="+ldflags, "-o", destBin, "./cmd/ga"); err != nil {
		return fmt.Errorf("failed to build GA from local source: %w", err)
	}

	if !i.fileExists(destBin) {
		return fmt.Errorf("GA binary not found after build at %s", destBin)
	}

	if err := os.Chmod(destBin, 0755); err != nil {
		return err
	}

	i.logger.LogSuccess("Installed GA (built locally) to " + destBin)
	return nil
}

func (i *Installer) updateGA() error {
	// Keep update flow deterministic with the same version pinning logic as install.
	// If binary is pinned to a tag, update should converge to that same tag.
	return i.installGA()
}

func shouldPinVersion(version string) bool {
	v := strings.TrimSpace(version)
	return v != "" && v != "main" && v != "master"
}

func (i *Installer) detectInstalledGAVersion(gaDir string) string {
	tag := strings.TrimSpace(i.commandOutput("git", "-C", gaDir, "describe", "--tags", "--exact-match"))
	if tag != "" && !strings.Contains(strings.ToLower(tag), "fatal") {
		return tag
	}
	branch := strings.TrimSpace(i.commandOutput("git", "-C", gaDir, "rev-parse", "--abbrev-ref", "HEAD"))
	if branch != "" && !strings.Contains(strings.ToLower(branch), "fatal") {
		return branch
	}
	return ""
}

func (i *Installer) installGASystemWide(gaDir string) error {
	home, _ := os.UserHomeDir()
	var binDir string
	if runtime.GOOS == "windows" {
		binDir = filepath.Join(os.Getenv("LOCALAPPDATA"), "ywai")
	} else {
		binDir = filepath.Join(home, ".local", "bin")
	}

	if err := i.ensureDir(binDir); err != nil {
		return err
	}

	// Try to download precompiled binary from releases
	version := i.resolveVersion()
	platform := i.getPlatform()

	displayVersion := version
	if displayVersion != "" &&
		!strings.HasPrefix(displayVersion, "v") &&
		displayVersion != "main" &&
		displayVersion != "master" {
		displayVersion = "v" + displayVersion
	}

	i.logger.Log("Downloading GA " + displayVersion + " for " + platform + "...")

	assetName := "ga-" + platform
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}

	// Try GitHub releases first
	versionTag := version
	if !strings.HasPrefix(versionTag, "v") &&
		versionTag != "main" &&
		versionTag != "master" {
		versionTag = "v" + versionTag
	}
	downloadURL := "https://github.com/Yoizen/dev-ai-workflow/releases/download/" + versionTag + "/" + assetName

	destBin := filepath.Join(binDir, "ga")
	if runtime.GOOS == "windows" {
		destBin += ".exe"
	}

	// Download binary
	if err := i.downloadFile(downloadURL, destBin); err != nil {
		i.logger.LogWarning("Could not download precompiled binary, using local build")
		return i.installGAFromSource(gaDir, binDir)
	}

	if err := os.Chmod(destBin, 0755); err != nil {
		return err
	}

	i.logger.LogSuccess("Installed GA to " + destBin)
	return nil
}

func (i *Installer) getPlatform() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	arch := "amd64"
	if goarch == "arm64" || goarch == "aarch64" {
		arch = "arm64"
	}

	switch goos {
	case "darwin":
		return "darwin-" + arch
	case "linux":
		return "linux-" + arch
	case "windows":
		return "windows-" + arch
	default:
		return "linux-amd64"
	}
}

func (i *Installer) downloadFile(url, dest string) error {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell", "-Command",
			fmt.Sprintf("(New-Object Net.WebClient).DownloadFile('%s', '%s')", url, dest))
		cmd.Stdout = i.out
		cmd.Stderr = i.out
		if err := cmd.Run(); err != nil {
			// Fallback to curl if available
			cmd = exec.Command("curl", "-fsSL", "-o", dest, url)
			cmd.Stdout = i.out
			cmd.Stderr = i.out
			if err2 := cmd.Run(); err2 != nil {
				return fmt.Errorf("download failed: %w", err)
			}
		}
	} else {
		cmd := exec.Command("curl", "-fsSL", "-o", dest, url)
		cmd.Stdout = i.out
		cmd.Stderr = i.out
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	// Guard against proxies or HTML error pages that still return a file.
	data, err := os.ReadFile(dest)
	if err != nil {
		return err
	}
	if looksLikeHTMLDocument(data) {
		os.Remove(dest)
		return fmt.Errorf("downloaded HTML instead of binary")
	}

	return nil
}

func looksLikeHTMLDocument(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}

	trimmed := strings.TrimSpace(string(sample))
	if trimmed == "" {
		return false
	}

	lower := strings.ToLower(trimmed)
	return strings.HasPrefix(lower, "<!doctype html") ||
		strings.HasPrefix(lower, "<html") ||
		strings.HasPrefix(lower, "<?xml")
}

func (i *Installer) installGAFromSource(gaDir, binDir string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("precompiled binary not available for Windows and Go compiler not found - please download manually from https://github.com/Yoizen/dev-ai-workflow/releases")
	}

	// Fallback: build from source (only if precompiled not available)
	sourceDir := i.getGASourceDir(gaDir)
	if sourceDir == "" {
		return fmt.Errorf("GA source directory not found")
	}

	gaBinary := filepath.Join(sourceDir, "bin", "ga")
	if err := os.MkdirAll(filepath.Dir(gaBinary), 0755); err != nil {
		return err
	}

	// Try local binary first
	if !i.fileExists(gaBinary) {
		i.logger.Log("Building GA from source...")
		ldflags := i.gaBuildLdflags(sourceDir)
		buildCmd := exec.Command("go", "build", "-ldflags="+ldflags, "-o", gaBinary, "./cmd/ga")
		buildCmd.Dir = sourceDir
		if err := i.runCommandWithCmd(buildCmd, "go", "build", "-ldflags="+ldflags, "-o", gaBinary, "./cmd/ga"); err != nil {
			return fmt.Errorf("failed to build GA: %w", err)
		}
	}

	if !i.fileExists(gaBinary) {
		return fmt.Errorf("GA binary not found")
	}

	destBin := filepath.Join(binDir, "ga")
	if err := i.copyFile(gaBinary, destBin); err != nil {
		return err
	}

	return os.Chmod(destBin, 0755)
}

func (i *Installer) gaBuildLdflags(sourceDir string) string {
	versionValue := i.gaBuildVersionValue()
	flags := []string{
		"-s",
		"-w",
		"-X=github.com/yoizen/ga/internal/version.Version=" + versionValue,
	}

	if commit := strings.TrimSpace(i.commandOutput("git", "-C", sourceDir, "rev-parse", "--short", "HEAD")); commit != "" && !strings.Contains(strings.ToLower(commit), "fatal") {
		flags = append(flags, "-X=github.com/yoizen/ga/internal/version.Commit="+commit)
	}

	return strings.Join(flags, " ")
}

func (i *Installer) gaBuildVersionValue() string {
	version := strings.TrimSpace(i.resolveVersion())
	version = strings.TrimPrefix(version, "v")
	switch version {
	case "", "main", "master":
		return "dev"
	default:
		return version
	}
}

func (i *Installer) getGASourceDir(gaDir string) string {
	// New Go structure: ywai/ga
	newLayout := filepath.Join(gaDir, "ywai", "ga")
	if i.dirExists(newLayout) && i.fileExists(filepath.Join(newLayout, "go.mod")) {
		return newLayout
	}

	// Legacy structure: bin/ga
	legacyLayout := gaDir
	if i.dirExists(filepath.Join(legacyLayout, "bin")) && i.fileExists(filepath.Join(legacyLayout, "bin", "ga")) {
		return legacyLayout
	}

	// Also check ywai/ga directly
	if i.dirExists(filepath.Join(gaDir, "ga")) && i.fileExists(filepath.Join(gaDir, "ga", "go.mod")) {
		return filepath.Join(gaDir, "ga")
	}

	return ""
}

func (i *Installer) gaPull(path string) error {
	cmd := exec.Command("git", "fetch", "origin", "-q")
	cmd.Dir = path
	if err := i.runCommandWithCmd(cmd, "git", "fetch", "origin", "-q"); err != nil {
		return err
	}

	stashed := false
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = path
	if output, _ := statusCmd.CombinedOutput(); len(strings.TrimSpace(string(output))) > 0 {
		stashCmd := exec.Command("git", "stash", "push", "-m", "Auto-stash before GA update", "--include-untracked", "-q")
		stashCmd.Dir = path
		if err := i.runCommandWithCmd(stashCmd, "git", "stash", "push", "-m", "Auto-stash before GA update", "--include-untracked", "-q"); err == nil {
			stashed = true
		}
	}

	mergeCmd := exec.Command("git", "merge", "--ff-only", "origin/main")
	mergeCmd.Dir = path
	if err := i.runCommandWithCmd(mergeCmd, "git", "merge", "--ff-only", "origin/main"); err != nil {
		mergeCmd = exec.Command("git", "merge", "--ff-only", "origin/master")
		mergeCmd.Dir = path
		if err := i.runCommandWithCmd(mergeCmd, "git", "merge", "--ff-only", "origin/master"); err != nil {
			if stashed {
				popCmd := exec.Command("git", "stash", "pop", "-q")
				popCmd.Dir = path
				i.runCommandWithCmd(popCmd, "git", "stash", "pop", "-q")
			}
			return err
		}
	}

	if stashed {
		popCmd := exec.Command("git", "stash", "pop", "-q")
		popCmd.Dir = path
		i.runCommandWithCmd(popCmd, "git", "stash", "pop", "-q")
	}

	return nil
}
