package gentlai

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

const (
	gentleAIOwner = "Gentleman-Programming"
	gentleAIRepo  = "gentle-ai"

	engramOwner = "Gentleman-Programming"
	engramRepo  = "engram"
	engramBin   = "engram"
)

var versionPattern = regexp.MustCompile(`v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?`)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// gentleAIBinaryPath resolves the full path to the gentle-ai binary.
// It checks PATH first, then falls back to known install locations
// (~/.local/bin, ~/go/bin, ~/.bin) where the release binary may have
// been placed by installGentleAIReleaseBinaryFirstTime when Go was
// not available. This is critical on Windows where ~/.local/bin is
// typically not in PATH.
func gentleAIBinaryPath() string {
	if path, err := exec.LookPath(config.GentleAIBin); err == nil {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	exeName := config.GentleAIBin
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	for _, dir := range []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, "go", "bin"),
		filepath.Join(home, ".bin"),
	} {
		candidate := filepath.Join(dir, exeName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func IsInstalled() bool {
	return gentleAIBinaryPath() != ""
}

func Install() error {
	if IsInstalled() {
		if version := CurrentVersion(); version != "" {
			fmt.Printf("gentle-ai already installed (%s).\n", version)
		} else {
			fmt.Println("gentle-ai already installed.")
		}
		fmt.Println("Checking gentle-ai for updates...")
		if err := Upgrade(); err != nil {
			return fmt.Errorf("gentle-ai upgrade failed: %w", err)
		}
		return nil
	}

	// Prefer go install when Go is available.
	_, err := exec.LookPath("go")
	if err == nil {
		fmt.Println("Installing gentle-ai...")
		cmd := exec.Command("go", "install", "github.com/Gentleman-Programming/gentle-ai/cmd/gentle-ai@latest")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install gentle-ai: %w", err)
		}
		fmt.Println("gentle-ai installed successfully.")
		return nil
	}

	// Fallback: download pre-built release binary.
	fmt.Println("Go not found. Downloading pre-built gentle-ai binary...")
	if err := installGentleAIReleaseBinaryFirstTime(); err != nil {
		return fmt.Errorf("failed to install gentle-ai: %w", err)
	}
	fmt.Println("gentle-ai installed successfully.")
	return nil
}

// InstallOptions holds all configurable options for gentle-ai install.
type InstallOptions struct {
	AgentName string
	Preset    string // full-gentleman, ecosystem-only, minimal, custom
	Scope     string // global, workspace
	SDDMode   string // single, multi
	Persona   string // gentleman, neutral, custom
	WorkDir   string // working directory for gentle-ai (isolates workspace writes); empty = current dir
	DryRun    bool
}

func InstallEcosystem(opts InstallOptions) error {
	if !IsInstalled() {
		return fmt.Errorf("gentle-ai is not installed. Run install first.")
	}

	// engram is installed via Homebrew by gentle-ai. On machines without a C
	// compiler (and without Go) the bottle build fails, which would abort the
	// whole multi-component install. Install engram on its own so a failure
	// there never blocks the other components, and fall back to a prebuilt
	// release binary when gentle-ai cannot install it.
	var extraEnv []string
	if !opts.DryRun {
		if err := installEngramComponent(opts); err != nil {
			fmt.Printf("  Warning: engram install via gentle-ai failed: %v\n", err)
			installDir, ferr := installEngramReleaseBinary()
			if ferr != nil {
				fmt.Printf("  Warning: engram prebuilt-binary fallback failed: %v\n", ferr)
			} else {
				fmt.Println("  engram installed from prebuilt release binary.")
				extraEnv = pathEnvWith(installDir)
				// Retry the engram component now that the binary exists so
				// gentle-ai can wire up the engram MCP config without brew.
				if rerr := installEngramComponent(opts, extraEnv...); rerr != nil {
					fmt.Printf("  Warning: engram MCP wiring still failed: %v\n", rerr)
				}
			}
		}
	}

	// Install the remaining components together.
	args := opts.buildArgs(ecosystemComponents)
	fmt.Printf("Running gentle-ai install --agent %s (%d components)...\n", opts.AgentName, len(ecosystemComponents))
	cmd := exec.Command(gentleAIBinaryPath(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}
	if len(extraEnv) > 0 {
		cmd.Env = extraEnv
	}
	if err := cmd.Run(); err != nil {
		return err
	}

	UpgradeEngram()
	return nil
}

// installEngramComponent runs `gentle-ai install --component engram` in
// isolation. extraEnv, when provided, replaces the subprocess environment
// (used to inject a PATH that includes a freshly downloaded engram binary).
func installEngramComponent(opts InstallOptions, extraEnv ...string) error {
	args := opts.buildArgs([]string{"engram"})
	fmt.Printf("Running gentle-ai install --agent %s --component engram...\n", opts.AgentName)
	cmd := exec.Command(gentleAIBinaryPath(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}
	if len(extraEnv) > 0 {
		cmd.Env = extraEnv
	}
	return cmd.Run()
}

// Components to install via gentle-ai (explicit list, no gga).
var installComponents = []string{
	"engram", "sdd", "skills", "context7",
	"persona", "permissions",
}

// ecosystemComponents are all components except engram, which is installed
// separately so its (Homebrew-based) failure cannot abort the others.
var ecosystemComponents = []string{
	"sdd", "skills", "context7",
	"persona", "permissions",
}

func (o InstallOptions) effectivePersona() string {
	if o.Persona == "" {
		return "neutral"
	}
	return o.Persona
}

func (o InstallOptions) effectiveScope() string {
	if o.Scope == "" {
		return "global"
	}
	return o.Scope
}

func (o InstallOptions) buildArgs(components []string) []string {
	if len(components) == 0 {
		components = installComponents
	}
	args := []string{
		"install",
		"--agent", o.AgentName,
		"--persona", o.effectivePersona(),
		"--scope", o.effectiveScope(),
	}
	for _, c := range components {
		args = append(args, "--component", c)
	}
	if o.SDDMode != "" {
		args = append(args, "--sdd-mode", o.SDDMode)
	}
	if o.DryRun {
		args = append(args, "--dry-run")
	}
	return args
}

func UpgradeEngram() {
	engram := findBinary("engram")
	if engram == "" {
		return
	}

	fmt.Println("Checking for engram updates...")
	cmd := exec.Command(engram, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}

	if strings.Contains(string(output), "Update available") {
		fmt.Println("Updating engram...")
		if runtime.GOOS == "windows" {
			engramExe := engram
			if strings.HasSuffix(engram, ".ps1") || strings.HasSuffix(engram, ".cmd") {
				return
			}
			oldPath := engramExe + ".bak"
			_ = os.Rename(engramExe, oldPath)
			if err := runCommand("go", "install", "github.com/Gentleman-Programming/engram/cmd/engram@latest"); err != nil {
				fmt.Printf("  Warning: engram update failed: %v\n", err)
				_ = os.Rename(oldPath, engramExe)
			} else {
				_ = os.Remove(oldPath)
				fmt.Println("  engram updated successfully.")
			}
		} else {
			if err := runCommand("go", "install", "github.com/Gentleman-Programming/engram/cmd/engram@latest"); err != nil {
				fmt.Printf("  Warning: engram update failed: %v\n", err)
			} else {
				fmt.Println("  engram updated successfully.")
			}
		}
	}
}

func Upgrade() error {
	if !IsInstalled() {
		return fmt.Errorf("gentle-ai is not installed")
	}

	beforeVersion := CurrentVersion()
	fmt.Println("Upgrading gentle-ai...")
	cmd := exec.Command(gentleAIBinaryPath(), "upgrade")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	upgradeErr := cmd.Run()

	// gentle-ai intentionally reports "manual update required" for some Windows
	// install methods while still exiting successfully. ywai owns the one-command
	// update experience, so close that gap by replacing the resolved .exe with
	// the latest release binary when the version did not change.
	if runtime.GOOS == "windows" {
		if err := ensureLatestWindowsBinary(beforeVersion); err != nil {
			if upgradeErr != nil {
				return fmt.Errorf("%v; direct Windows update also failed: %w", upgradeErr, err)
			}
			return err
		}
	}

	return upgradeErr
}

// SyncOptions holds configurable options for gentle-ai sync.
type SyncOptions struct {
	SDDMode       string // single, multi
	StrictTDD     bool
	Profiles      []string // e.g. "cheap:openrouter/qwen/qwen3-30b-a3b:free"
	ProfilePhases []string // e.g. "cheap:sdd-design:anthropic/claude-sonnet-4"
	IncludePerms  bool
	IncludeTheme  bool
	DryRun        bool
}

func Sync(opts SyncOptions) error {
	if !IsInstalled() {
		return fmt.Errorf("gentle-ai is not installed")
	}

	if version := CurrentVersion(); version != "" {
		fmt.Printf("Syncing gentle-ai assets with gentle-ai %s...\n", version)
	} else {
		fmt.Println("Syncing gentle-ai assets...")
	}

	args := opts.buildArgs()

	cmd := exec.Command(gentleAIBinaryPath(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (o SyncOptions) buildArgs() []string {
	args := []string{"sync"}
	if o.SDDMode != "" {
		args = append(args, "--sdd-mode", o.SDDMode)
	}
	if o.StrictTDD {
		args = append(args, "--strict-tdd")
	}
	for _, p := range o.Profiles {
		args = append(args, "--profile", p)
	}
	for _, pp := range o.ProfilePhases {
		args = append(args, "--profile-phase", pp)
	}
	if o.IncludePerms {
		args = append(args, "--include-permissions")
	}
	if o.IncludeTheme {
		args = append(args, "--include-theme")
	}
	if o.DryRun {
		args = append(args, "--dry-run")
	}
	return args
}

// Doctor runs gentle-ai doctor for a read-only health check.
func Doctor() error {
	if !IsInstalled() {
		return fmt.Errorf("gentle-ai is not installed")
	}
	cmd := exec.Command(gentleAIBinaryPath(), "doctor")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SkillRegistryRefresh runs gentle-ai skill-registry refresh.
func SkillRegistryRefresh(cwd string) error {
	if !IsInstalled() {
		return fmt.Errorf("gentle-ai is not installed")
	}
	args := []string{"skill-registry", "refresh"}
	if cwd != "" {
		args = append(args, "--cwd", cwd)
	}
	cmd := exec.Command(gentleAIBinaryPath(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func findBinary(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	if runtime.GOOS == "windows" {
		for _, ext := range []string{".cmd", ".ps1", ".bat", ".exe"} {
			if path, err := exec.LookPath(name + ext); err == nil {
				return path
			}
		}
	}
	return ""
}

func CurrentVersion() string {
	for _, args := range [][]string{{"--version"}, {"version"}} {
		cmd := exec.Command(gentleAIBinaryPath(), args...)
		out, err := cmd.CombinedOutput()
		if err != nil && len(out) == 0 {
			continue
		}
		if version := parseVersion(string(out)); version != "" {
			return version
		}
	}
	return ""
}

func parseVersion(output string) string {
	match := versionPattern.FindString(output)
	return normalizeVersion(match)
}

func normalizeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

func ensureLatestWindowsBinary(beforeVersion string) error {
	latest, err := latestGentleAIRelease()
	if err != nil {
		return fmt.Errorf("failed to check latest gentle-ai release: %w", err)
	}

	afterVersion := CurrentVersion()
	if normalizeVersion(afterVersion) == normalizeVersion(latest) {
		if beforeVersion != "" && normalizeVersion(beforeVersion) != normalizeVersion(afterVersion) {
			fmt.Printf("  gentle-ai updated: %s → %s\n", beforeVersion, afterVersion)
		}
		return nil
	}

	currentLabel := afterVersion
	if currentLabel == "" {
		currentLabel = "unknown"
	}
	fmt.Printf("  gentle-ai is still %s after upstream upgrade; installing release binary %s...\n", currentLabel, latest)

	if err := installGentleAIReleaseBinary(latest); err != nil {
		return err
	}

	finalVersion := CurrentVersion()
	if normalizeVersion(finalVersion) != normalizeVersion(latest) {
		resolved := findBinary(config.GentleAIBin)
		return fmt.Errorf("installed gentle-ai %s, but PATH still resolves %q as version %q", latest, resolved, finalVersion)
	}

	fmt.Printf("  gentle-ai updated: %s → %s\n", currentLabel, finalVersion)
	return nil
}

func latestGentleAIRelease() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", gentleAIOwner, gentleAIRepo)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ywai")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("latest release did not include tag_name")
	}
	return release.TagName, nil
}

func installGentleAIReleaseBinary(version string) error {
	target := findBinary(config.GentleAIBin)
	if target == "" {
		return fmt.Errorf("gentle-ai binary not found in PATH")
	}

	if isScoopShim(target) {
		fmt.Println("  Detected Scoop-managed gentle-ai; running scoop update gentle-ai...")
		return runCommand("scoop", "update", config.GentleAIBin)
	}

	lowerTarget := strings.ToLower(target)
	if strings.HasSuffix(lowerTarget, ".ps1") || strings.HasSuffix(lowerTarget, ".cmd") || strings.HasSuffix(lowerTarget, ".bat") {
		return fmt.Errorf("resolved gentle-ai is a shim (%s); use the official installer or package manager for that install method", target)
	}

	archiveName := gentleAIAssetName(version)
	downloadURL := fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/%s",
		gentleAIOwner,
		gentleAIRepo,
		version,
		archiveName,
	)

	tmpDir, err := os.MkdirTemp("", "gentle-ai-update-*")
	if err != nil {
		return fmt.Errorf("cannot create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	archivePath := filepath.Join(tmpDir, archiveName)
	fmt.Printf("  Downloading %s...\n", downloadURL)
	if err := downloadFile(downloadURL, archivePath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	binaryPath, err := extractGentleAIBinary(archivePath, tmpDir)
	if err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	if err := replaceBinary(target, binaryPath); err != nil {
		return fmt.Errorf("replace failed: %w", err)
	}

	return nil
}

func isScoopShim(path string) bool {
	normalized := strings.ReplaceAll(strings.ToLower(filepath.Clean(path)), "\\", "/")
	return strings.Contains(normalized, "/scoop/shims/")
}

func gentleAIAssetName(version string) string {
	clean := normalizeVersion(version)
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("%s_%s_%s_%s.%s", config.GentleAIBin, clean, runtime.GOOS, runtime.GOARCH, ext)
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractGentleAIBinary(archivePath, destDir string) (string, error) {
	if runtime.GOOS == "windows" {
		return extractBinaryFromZip(archivePath, destDir)
	}
	return extractBinaryFromTarGz(archivePath, destDir)
}

func extractBinaryFromZip(archivePath, destDir string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = reader.Close() }()

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(file.Name) != config.GentleAIBin+".exe" {
			continue
		}

		src, err := file.Open()
		if err != nil {
			return "", err
		}
		defer func() { _ = src.Close() }()

		outPath := filepath.Join(destDir, config.GentleAIBin+".exe")
		out, err := os.Create(outPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, src); err != nil {
			_ = out.Close()
			return "", err
		}
		if err := out.Close(); err != nil {
			return "", err
		}
		return outPath, nil
	}

	return "", fmt.Errorf("%s.exe not found in archive", config.GentleAIBin)
}

func extractBinaryFromTarGz(archivePath, destDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != config.GentleAIBin {
			continue
		}

		outPath := filepath.Join(destDir, config.GentleAIBin)
		out, err := os.Create(outPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return "", err
		}
		if err := out.Close(); err != nil {
			return "", err
		}
		if err := os.Chmod(outPath, 0o755); err != nil {
			return "", err
		}
		return outPath, nil
	}

	return "", fmt.Errorf("%s not found in archive", config.GentleAIBin)
}

// installGentleAIReleaseBinaryFirstTime downloads the latest release binary
// to a user-local bin directory (~/.local/bin or ~/go/bin) when Go is not available.
func installGentleAIReleaseBinaryFirstTime() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	// Pick a writable install directory.
	var installDir string
	for _, dir := range []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, "go", "bin"),
		filepath.Join(home, ".bin"),
	} {
		if err := os.MkdirAll(dir, 0o755); err == nil {
			installDir = dir
			break
		}
	}
	if installDir == "" {
		return fmt.Errorf("no writable bin directory found in home")
	}

	version, err := latestGentleAIRelease()
	if err != nil {
		return fmt.Errorf("failed to check latest release: %w", err)
	}

	archiveName := gentleAIAssetName(version)
	downloadURL := fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/%s",
		gentleAIOwner,
		gentleAIRepo,
		version,
		archiveName,
	)

	tmpDir, err := os.MkdirTemp("", "gentle-ai-install-*")
	if err != nil {
		return fmt.Errorf("cannot create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	archivePath := filepath.Join(tmpDir, archiveName)
	fmt.Printf("  Downloading %s...\n", downloadURL)
	if err := downloadFile(downloadURL, archivePath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	binaryPath, err := extractGentleAIBinary(archivePath, tmpDir)
	if err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return err
	}

	target := filepath.Join(installDir, config.GentleAIBin)
	if err := os.Rename(binaryPath, target); err != nil {
		return fmt.Errorf("cannot move binary into place: %w", err)
	}

	fmt.Printf("  Installed to %s\n", target)

	// Warn if the install directory is not in PATH.
	pathEnv := os.Getenv("PATH")
	if !strings.Contains(pathEnv, installDir) {
		fmt.Printf("  Warning: %s is not in your PATH. Add it or restart your shell.\n", installDir)
	}

	return nil
}

func replaceBinary(target, replacement string) error {
	if err := os.Chmod(replacement, 0o755); err != nil {
		return err
	}

	backup := target + ".bak"
	_ = os.Remove(backup)

	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("cannot backup old binary: %w", err)
	}

	if err := os.Rename(replacement, target); err != nil {
		_ = os.Rename(backup, target)
		return fmt.Errorf("cannot move new binary into place: %w", err)
	}

	_ = os.Remove(backup)
	return nil
}

func runCommand(name string, args ...string) error {
	bin := findBinary(name)
	if bin == "" {
		return fmt.Errorf("%s not found", name)
	}

	if runtime.GOOS == "windows" && (strings.HasSuffix(bin, ".ps1") || strings.HasSuffix(bin, ".cmd")) {
		if strings.HasSuffix(bin, ".ps1") {
			fullArgs := append([]string{"-ExecutionPolicy", "Bypass", "-File", bin}, args...)
			cmd := exec.Command("powershell", fullArgs...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		fullArgs := append([]string{"/c", bin}, args...)
		cmd := exec.Command("cmd", fullArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// pathEnvWith returns a copy of the current environment with dir prepended to
// PATH, so a freshly downloaded binary in dir is found by child processes.
func pathEnvWith(dir string) []string {
	if dir == "" {
		return os.Environ()
	}
	env := os.Environ()
	out := make([]string, 0, len(env))
	replaced := false
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			out = append(out, "PATH="+dir+string(os.PathListSeparator)+strings.TrimPrefix(kv, "PATH="))
			replaced = true
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, "PATH="+dir)
	}
	return out
}

func latestEngramRelease() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", engramOwner, engramRepo)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ywai")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("latest release did not include tag_name")
	}
	return release.TagName, nil
}

func engramAssetName(version string) string {
	clean := normalizeVersion(version)
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("%s_%s_%s_%s.%s", engramBin, clean, runtime.GOOS, runtime.GOARCH, ext)
}

// installEngramReleaseBinary downloads the latest prebuilt engram binary into a
// user-local bin directory. This is the fallback for machines without a C
// compiler (Homebrew bottle build fails) and without Go (`go install` fails).
// It returns the directory the binary was installed into.
func installEngramReleaseBinary() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	var installDir string
	for _, dir := range []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, "go", "bin"),
		filepath.Join(home, ".bin"),
	} {
		if err := os.MkdirAll(dir, 0o755); err == nil {
			installDir = dir
			break
		}
	}
	if installDir == "" {
		return "", fmt.Errorf("no writable bin directory found in home")
	}

	version, err := latestEngramRelease()
	if err != nil {
		return "", fmt.Errorf("failed to check latest engram release: %w", err)
	}

	archiveName := engramAssetName(version)
	downloadURL := fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/%s",
		engramOwner,
		engramRepo,
		version,
		archiveName,
	)

	tmpDir, err := os.MkdirTemp("", "engram-install-*")
	if err != nil {
		return "", fmt.Errorf("cannot create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	archivePath := filepath.Join(tmpDir, archiveName)
	fmt.Printf("  Downloading %s...\n", downloadURL)
	if err := downloadFile(downloadURL, archivePath); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	binaryPath, err := extractNamedBinary(archivePath, tmpDir, engramBin)
	if err != nil {
		return "", fmt.Errorf("extract failed: %w", err)
	}

	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return "", err
	}

	binName := engramBin
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	target := filepath.Join(installDir, binName)
	if err := os.Rename(binaryPath, target); err != nil {
		return "", fmt.Errorf("cannot move binary into place: %w", err)
	}

	fmt.Printf("  Installed engram to %s\n", target)

	pathEnv := os.Getenv("PATH")
	if !strings.Contains(pathEnv, installDir) {
		fmt.Printf("  Warning: %s is not in your PATH. Add it or restart your shell.\n", installDir)
	}

	return installDir, nil
}

// extractNamedBinary extracts the file whose base name matches binName (or
// binName+".exe" on Windows) from a .tar.gz or .zip archive into destDir.
func extractNamedBinary(archivePath, destDir, binName string) (string, error) {
	if runtime.GOOS == "windows" {
		return extractNamedBinaryFromZip(archivePath, destDir, binName+".exe")
	}
	return extractNamedBinaryFromTarGz(archivePath, destDir, binName)
}

func extractNamedBinaryFromTarGz(archivePath, destDir, binName string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != binName {
			continue
		}

		outPath := filepath.Join(destDir, binName)
		out, err := os.Create(outPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return "", err
		}
		if err := out.Close(); err != nil {
			return "", err
		}
		if err := os.Chmod(outPath, 0o755); err != nil {
			return "", err
		}
		return outPath, nil
	}

	return "", fmt.Errorf("%s not found in archive", binName)
}

func extractNamedBinaryFromZip(archivePath, destDir, binName string) (string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = reader.Close() }()

	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(f.Name) != binName {
			continue
		}

		src, err := f.Open()
		if err != nil {
			return "", err
		}
		defer func() { _ = src.Close() }()

		outPath := filepath.Join(destDir, binName)
		out, err := os.Create(outPath)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, src); err != nil {
			_ = out.Close()
			return "", err
		}
		if err := out.Close(); err != nil {
			return "", err
		}
		return outPath, nil
	}

	return "", fmt.Errorf("%s not found in archive", binName)
}
