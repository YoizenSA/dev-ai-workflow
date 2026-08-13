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
	engramOwner = "Gentleman-Programming"
	engramRepo  = "engram"
	engramBin   = "engram"
)

var versionPattern = regexp.MustCompile(`v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?`)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func IsInstalled() bool {
	return findBinary(config.GentleAIBin) != ""
}

// Install installs gentle-ai only when it is missing. Upgrading an existing
// install is `ywai update`'s job (it calls Upgrade explicitly), so `ywai
// install` never moves a working gentle-ai version underneath the user.
// Install no longer provisions the gentle-ai binary. The gentle-ai binary is
// optional for ywai: engram is installed through ywai's own release path
// (InstallEngram) and skills/profiles/plugins are applied by the ywai
// pipeline. This is the slice-1 decoupling contract: ywai install must never
// install gentle-ai.
func Install() error {
	if IsInstalled() {
		if version := CurrentVersion(); version != "" {
			fmt.Printf("gentle-ai already installed (%s) — ywai does not manage it.\n", version)
		} else {
			fmt.Println("gentle-ai already installed — ywai does not manage it.")
		}
		return nil
	}
	fmt.Println("gentle-ai is not installed; ywai no longer installs it.")
	return nil
}

// InstallEngram installs the engram binary through ywai's own manual release
// path (installEngramReleaseBinary) and returns the directory it was
// installed into. It never invokes the gentle-ai binary. Slice 1 contract:
// ywai installs engram itself for non-minimal presets.
func InstallEngram() (string, error) {
	return installEngramReleaseBinary()
}

// InstallOptions holds all configurable options for gentle-ai install.
type InstallOptions struct {
	AgentName string
	Preset    string // full-gentleman, ecosystem-only, minimal, custom
	Scope     string // global, workspace
	WorkDir   string // working directory for gentle-ai (isolates workspace writes); empty = current dir
	DryRun    bool

	// Optional gentle-ai SDD (off by default). Installed AFTER ywai's curated
	// AGENTS.md so marker sections are not wiped. Persona is never installed —
	// ywai always owns tone via its own AGENTS.md.
	InstallSDD bool
	SDDMode    string // "single" or "multi" (default multi when InstallSDD)
}

// ComponentPlan is the gentle-ai component set derived from an install preset.
type ComponentPlan struct {
	IncludeEngram bool
	Ecosystem     []string // components installed after engram (or alone for minimal)
}

// PlanForPreset maps install presets to gentle-ai --component lists only.
// ywai extra skills are always copied by the apply pipeline and are not gated here.
//
//	full-gentleman / ecosystem-only / "" / custom → engram + skills + context7 + permissions
//	minimal                                       → skills only (no engram via this plan)
func PlanForPreset(preset string) ComponentPlan {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "minimal":
		return ComponentPlan{
			IncludeEngram: false,
			Ecosystem:     []string{"skills"},
		}
	case "ecosystem-only", "full-gentleman", "custom", "":
		return ComponentPlan{
			IncludeEngram: true,
			Ecosystem:     append([]string(nil), ecosystemComponents...),
		}
	default:
		return ComponentPlan{
			IncludeEngram: true,
			Ecosystem:     append([]string(nil), ecosystemComponents...),
		}
	}
}

// AllComponents returns engram (when included) plus ecosystem components.
func (p ComponentPlan) AllComponents() []string {
	if p.IncludeEngram {
		out := make([]string, 0, 1+len(p.Ecosystem))
		out = append(out, "engram")
		return append(out, p.Ecosystem...)
	}
	return append([]string(nil), p.Ecosystem...)
}

func InstallEcosystem(opts InstallOptions) error {
	plan := PlanForPreset(opts.Preset)

	// Slice 1 decoupling: engram is installed through ywai's own release
	// path, never via `gentle-ai install`. The gentle-ai ecosystem
	// components (skills/context7/permissions) have no ywai-native
	// replacement in this slice — context7 is out of scope and skills are
	// seeded by the apply pipeline — so they are reported and skipped
	// rather than delegated to the gentle-ai binary.
	if plan.IncludeEngram {
		if opts.DryRun {
			fmt.Println("  Would install engram (ywai release path).")
		} else {
			installDir, err := InstallEngram()
			if err != nil {
				return fmt.Errorf("failed to install engram: %w", err)
			}
			fmt.Printf("  Engram ready in %s\n", installDir)
			UpgradeEngram()
		}
	}

	return nil
}

// ecosystemComponents are gentle-ai components except engram (installed
// separately so Homebrew failures cannot abort the others).
//
// sdd is optional (InstallSDD); persona is never installed (ywai owns AGENTS.md tone).
var ecosystemComponents = []string{
	"skills", "context7", "permissions",
}

func (o InstallOptions) effectiveScope() string {
	if o.Scope == "" {
		return "global"
	}
	return o.Scope
}

func (o InstallOptions) buildArgs(components []string) []string {
	if len(components) == 0 {
		components = PlanForPreset(o.Preset).AllComponents()
	}
	args := []string{
		"install",
		"--agent", o.AgentName,
		"--scope", o.effectiveScope(),
	}
	for _, c := range components {
		args = append(args, "--component", c)
	}
	if o.DryRun {
		args = append(args, "--dry-run")
	}
	return args
}

// EffectiveSDDMode returns single|multi (default multi).
func (o InstallOptions) EffectiveSDDMode() string {
	m := strings.ToLower(strings.TrimSpace(o.SDDMode))
	if m == "single" || m == "multi" {
		return m
	}
	return "multi"
}

// HasOptionalComponents reports whether optional SDD should be installed
// after the base ecosystem + ywai AGENTS.md write.
func (o InstallOptions) HasOptionalComponents() bool {
	return o.InstallSDD
}

// optionalComponents returns sdd for a follow-up install pass.
func (o InstallOptions) optionalComponents() []string {
	if o.InstallSDD {
		return []string{"sdd"}
	}
	return nil
}

// InstallOptionalComponents installs SDD after the base ecosystem.
// Call this AFTER writing ywai's curated AGENTS.md so gentle-ai can re-inject
// SDD marker sections. Never installs persona. SDD may auto-pull engram.
func InstallOptionalComponents(opts InstallOptions) error {
	if !opts.HasOptionalComponents() {
		return nil
	}

	comps := opts.optionalComponents()
	// Slice 1 decoupling: optional components (SDD) are not delegated to
	// `gentle-ai install`; a ywai-native SDD flow lands in a later slice.
	fmt.Printf("Skipping optional SDD components [%s] — ywai-native SDD lands in a later slice.\n",
		strings.Join(comps, ", "))
	if opts.DryRun {
		return nil
	}
	return nil
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

// Upgrade no longer shells out to the gentle-ai binary. Slice 1 contract:
// `ywai update` must not run `gentle-ai upgrade`. It preserves only the
// ywai/engram-owned behavior — refreshing the engram binary when an update
// is available.
func Upgrade() error {
	UpgradeEngram()
	return nil
}

// Doctor runs ywai-native health checks. Slice 1 contract: it must not
// require the gentle-ai binary and must not depend on .gentle-ai paths.
// It reports on ywai data locations and the tool binaries ywai works with;
// gentle-ai is optional and its absence is not an error.
func Doctor() error {
	fmt.Println("Running ywai health checks...")

	if _, err := os.Stat(config.DataDir()); err == nil {
		fmt.Printf("  [ok]  ywai:data-dir          %s\n", config.DataDir())
	} else {
		fmt.Printf("  [warn] ywai:data-dir          %s missing (run `ywai install`)\n", config.DataDir())
	}

	for _, bin := range []string{"git", "go", "graft", "node", "npm", engramBin} {
		if _, err := exec.LookPath(bin); err == nil {
			fmt.Printf("  [ok]  %-22s found\n", bin)
		} else {
			fmt.Printf("  [warn] %-22s not found\n", bin)
		}
	}

	// gentle-ai is optional for ywai; report, never fail.
	if _, err := exec.LookPath(config.GentleAIBin); err == nil {
		fmt.Println("  [ok]  gentle-ai               found (optional)")
	} else {
		fmt.Println("  [info] gentle-ai               not found (optional; ywai does not require it)")
	}

	return nil
}

// SkillRegistryRefresh is a no-op: it never resolves or executes gentle-ai.
func SkillRegistryRefresh(cwd string) error {
	return nil
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
	return ""
}

func parseVersion(output string) string {
	match := versionPattern.FindString(output)
	return normalizeVersion(match)
}

func normalizeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

// latestRelease returns the tag name of the latest GitHub release for the given
// owner/repo.
func latestRelease(owner, repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

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

// assetName builds the release archive name for a binary following the
// "{bin}_{version}_{os}_{arch}.{ext}" convention shared by gentle-ai and engram.
func assetName(binName, version string) string {
	clean := normalizeVersion(version)
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("%s_%s_%s_%s.%s", binName, clean, runtime.GOOS, runtime.GOARCH, ext)
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

var (
	fetchLatestEngram   = latestEngramRelease
	downloadReleaseFile = downloadFile
)

func latestEngramRelease() (string, error) {
	return latestRelease(engramOwner, engramRepo)
}

func installedEngramVersion() string {
	exe := findBinary(engramBin)
	if exe == "" {
		return ""
	}
	out, err := exec.Command(exe, "version").Output()
	if err != nil {
		return ""
	}
	return versionPattern.FindString(string(out))
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

	version, err := fetchLatestEngram()
	if err != nil {
		return "", fmt.Errorf("failed to check latest engram release: %w", err)
	}

	if current := installedEngramVersion(); current != "" && normalizeVersion(current) == normalizeVersion(version) {
		fmt.Printf("  Engram already %s\n", version)
		return installDir, nil
	}

	archiveName := assetName(engramBin, version)
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
	if err := downloadReleaseFile(downloadURL, archivePath); err != nil {
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
