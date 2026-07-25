package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const (
	owner = "YoizenSA"
	repo  = "dev-ai-workflow"
)

type releaseInfo struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// githubGET performs an authenticated (when possible) GET against the GitHub API.
func githubGET(url string) (*http.Response, error) {
	return githubGETWithTimeout(url, 15*time.Second)
}

// githubGETWithTimeout is githubGET with a caller-chosen timeout: asset
// downloads need minutes, API calls need seconds.
func githubGETWithTimeout(url string, timeout time.Duration) (*http.Response, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ywai")

	// Attach a GitHub token if available so we get the 5000/hour limit
	// instead of the 60/hour unauthenticated limit (which 403s easily).
	if token := githubToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return client.Do(req)
}

func LatestVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	resp, err := githubGET(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release info: %w", err)
	}

	return release.TagName, nil
}

// LatestPrereleaseVersion returns the newest GitHub release marked as
// prerelease (or with a beta/rc/pre tag). Releases are newest-first from the API.
// Does not fall back to stable — callers that want stable use LatestVersion.
func LatestPrereleaseVersion() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=30", owner, repo)

	resp, err := githubGET(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var releases []releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", fmt.Errorf("failed to parse releases: %w", err)
	}

	tag, ok := pickLatestPrerelease(releases)
	if !ok {
		return "", fmt.Errorf("no prerelease (beta) found for %s/%s", owner, repo)
	}
	return tag, nil
}

// pickLatestPrerelease returns the first release in list that is a beta channel
// candidate. list is assumed newest-first (GitHub default).
func pickLatestPrerelease(releases []releaseInfo) (tag string, ok bool) {
	for _, r := range releases {
		if r.TagName == "" {
			continue
		}
		if r.Prerelease || isPrereleaseTag(r.TagName) {
			return r.TagName, true
		}
	}
	return "", false
}

// isPrereleaseTag reports whether a tag looks like a beta/rc/pre channel even
// if the GitHub "prerelease" flag was not set.
func isPrereleaseTag(tag string) bool {
	t := strings.ToLower(strings.TrimPrefix(tag, "v"))
	// semver pre-release segment starts after the first '-'
	i := strings.IndexByte(t, '-')
	if i < 0 {
		return false
	}
	pre := t[i+1:]
	return strings.HasPrefix(pre, "beta") ||
		strings.HasPrefix(pre, "rc") ||
		strings.HasPrefix(pre, "pre") ||
		strings.HasPrefix(pre, "alpha")
}

// githubToken returns a GitHub token from the environment if present.
// Supports GH_TOKEN, GITHUB_TOKEN, and the gh CLI config (GH_ENTERPRISE_TOKEN
// is ignored here since we target github.com). Empty string means no token.
func githubToken() string {
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

// ResolvedExecutable returns the path to the ywai binary that should be
// used for re-execution after a self-update.
//
// After selfupdate.Run replaces the running binary, os.Executable() on Linux
// returns a stale path: /proc/self/exe follows the rename to the .bak file
// which has already been removed. This helper detects that situation (the
// reported path no longer exists on disk) and falls back to exec.LookPath so
// callers get the real, current binary path.
func ResolvedExecutable() (string, error) {
	exe, err := os.Executable()
	if err == nil {
		if resolved, e := filepath.EvalSymlinks(exe); e == nil {
			exe = resolved
		}
		if _, statErr := os.Stat(exe); statErr == nil {
			return exe, nil
		}
		// Path from os.Executable() no longer exists (stale .bak after
		// self-update). Fall through to LookPath.
	}
	if path, err := exec.LookPath("ywai"); err == nil {
		return path, nil
	}
	if exe != "" {
		return exe, nil
	}
	return "", fmt.Errorf("cannot resolve ywai executable path")
}

// DevVersion is the version stamped into builds that were not produced by the
// release pipeline (goreleaser injects the real tag via ldflags).
const DevVersion = "dev"

// IsDevBuild reports whether a version string came from a local build rather
// than a release. Auto-update must skip these, or `ywai serve` overwrites the
// binary a developer just compiled with whatever is on GitHub — which is
// exactly what happened to a build stamped "8.16.5-dev+ff82a7b": it did not
// equal the latest tag, so the updater treated it as a version behind.
//
// Any marker a local build can carry counts, not just the bare word: the
// version is stamped by scripts/dev.sh and by hand, and the failure is silent
// and destructive when one of those forms is missed.
func IsDevBuild(v string) bool {
	t := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(v, "v")))
	if t == "" || t == DevVersion {
		return true
	}
	// Build metadata (`+sha`) only ever comes from a local build; releases are
	// plain tags.
	if strings.Contains(t, "+") {
		return true
	}
	// Pre-release segment naming a local build, e.g. 8.16.5-dev, 8.16.6-next,
	// 1.2.3-dirty. Release channels use beta/rc/alpha/pre, which must still
	// auto-update.
	for _, marker := range []string{"-dev", "-next", "-dirty", "-local", "-snapshot"} {
		if strings.Contains(t, marker) {
			return true
		}
	}
	return false
}

// checksumsAsset is the goreleaser-generated manifest published with every
// release (see .goreleaser.yaml `checksum.name_template`).
const checksumsAsset = "checksums.txt"

// verifyChecksum fails when the downloaded archive does not match the SHA-256
// the release published for it.
//
// Without this, the updater downloads a binary over the network and executes
// it on the strength of TLS alone — anything that can serve those bytes (a
// compromised mirror, a proxy with a trusted cert, a tampered release asset)
// gets code execution as the user. The manifest is already published; the only
// thing missing was reading it.
//
// A missing or unparsable manifest is a hard failure, not a warning: silently
// installing an unverified binary is exactly the outcome this prevents.
func verifyChecksum(archivePath, version, asset string) error {
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s",
		owner, repo, version, checksumsAsset)

	manifest, err := fetchText(url)
	if err != nil {
		return fmt.Errorf("cannot fetch %s for %s: %w", checksumsAsset, version, err)
	}
	return verifyAgainstManifest(archivePath, asset, manifest)
}

// verifyAgainstManifest is the pure half of verifyChecksum: no network, so the
// accept and reject paths are both directly testable.
func verifyAgainstManifest(archivePath, asset, manifest string) error {
	want, ok := checksumFor(manifest, asset)
	if !ok {
		return fmt.Errorf("%s has no entry for %s", checksumsAsset, asset)
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open download: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash download: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))

	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", asset, want, got)
	}
	return nil
}

// checksumFor pulls one asset's hash out of a `<sha256>  <filename>` manifest.
func checksumFor(manifest, asset string) (string, bool) {
	for _, line := range strings.Split(manifest, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		// goreleaser writes "hash  name"; some tools prefix the name with '*'.
		if strings.TrimPrefix(fields[1], "*") == asset {
			return fields[0], true
		}
	}
	return "", false
}

func fetchText(url string) (string, error) {
	resp, err := githubGET(url)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func assetName(version string) string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	clean := strings.TrimPrefix(version, "v")

	ext := "tar.gz"
	if osName == "windows" {
		ext = "zip"
	}

	return fmt.Sprintf("ywai_%s_%s_%s.%s", clean, osName, arch, ext)
}

// Run upgrades to the latest stable release (GitHub /releases/latest).
// Returns ("", nil) when already on that version.
func Run(currentVersion string) (string, error) {
	return run(currentVersion, false)
}

// RunBeta upgrades to the newest prerelease (beta) on GitHub.
// Stable latest is intentionally not used. Returns ("", nil) when already
// on that prerelease tag.
func RunBeta(currentVersion string) (string, error) {
	return run(currentVersion, true)
}

func run(currentVersion string, beta bool) (string, error) {
	var (
		target string
		err    error
	)
	if beta {
		target, err = LatestPrereleaseVersion()
		if err != nil {
			return "", fmt.Errorf("checking latest beta: %w", err)
		}
	} else {
		target, err = LatestVersion()
		if err != nil {
			return "", fmt.Errorf("checking latest version: %w", err)
		}
	}

	normalized := strings.TrimPrefix(target, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	if normalized == current {
		return "", nil
	}

	return downloadAndReplace(target)
}

func downloadAndReplace(version string) (string, error) {
	asset := assetName(version)
	downloadURL := fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/%s",
		owner, repo, version, asset,
	)

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find current executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		exe, _ = os.Executable()
	}

	tmpDir, err := os.MkdirTemp("", "ywai-update-*")
	if err != nil {
		return "", fmt.Errorf("cannot create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	archivePath := filepath.Join(tmpDir, filepath.Base(asset))
	if err := downloadFile(downloadURL, archivePath); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	// Verify before extracting: nothing from the archive is touched, let alone
	// made executable, until the bytes match what the release published.
	if err := verifyChecksum(archivePath, version, asset); err != nil {
		return "", fmt.Errorf("refusing to install %s: %w", version, err)
	}

	binaryPath := filepath.Join(tmpDir, "ywai")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	if runtime.GOOS == "windows" {
		if err := extractZip(archivePath, tmpDir); err != nil {
			return "", fmt.Errorf("extract failed: %w", err)
		}
	} else {
		if err := extractTarGz(archivePath, tmpDir); err != nil {
			return "", fmt.Errorf("extract failed: %w", err)
		}
	}

	if _, err := os.Stat(binaryPath); err != nil {
		return "", fmt.Errorf("binary not found in archive (looked for %s)", filepath.Base(binaryPath))
	}

	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return "", fmt.Errorf("cannot set permissions: %w", err)
	}

	bakPath := exe + ".bak"
	_ = os.Remove(bakPath)
	if err := os.Rename(exe, bakPath); err != nil {
		// Windows locks the running executable and may refuse the rename
		// with "Access denied". Fall back to a platform-specific handler.
		if runtime.GOOS == "windows" {
			return deferredReplace(binaryPath, exe, version)
		}
		return "", fmt.Errorf("cannot backup old binary: %w", err)
	}

	if err := replaceBinary(binaryPath, exe); err != nil {
		_ = os.Rename(bakPath, exe)
		return "", fmt.Errorf("cannot replace binary: %w", err)
	}

	_ = os.Remove(bakPath)

	return version, nil
}

// replaceBinary replaces src with dst, attempting an atomic rename first.
// If the rename fails due to a cross-device link (different filesystems),
// it falls back to copying the file contents and removing the source.
func replaceBinary(src, dst string) error {
	// Try atomic rename first (same filesystem).
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return err
	}

	// Cross-device link: copy file contents, set permissions, remove source.
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer func() { _ = dstFile.Close() }()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy binary: %w", err)
	}

	if err := os.Chmod(dst, 0o755); err != nil {
		return fmt.Errorf("set permissions: %w", err)
	}

	if err := os.Remove(src); err != nil {
		return fmt.Errorf("remove source: %w", err)
	}

	return nil
}

// downloadFile fetches a release asset. It goes through githubGET so a token,
// when present, is attached: the API calls are authenticated, and an
// unauthenticated download would be the one step that breaks if the repo ever
// stops being public.
func downloadFile(url, dest string) error {
	resp, err := githubGETWithTimeout(url, 5*time.Minute)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(f, resp.Body)
	return err
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		name := filepath.Base(f.Name)
		if name != "ywai" && name != "ywai.exe" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		outPath := filepath.Join(dest, name)
		out, err := os.Create(outPath)
		if err != nil {
			_ = rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		_ = out.Close()
		_ = rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if hdr.Typeflag != tar.TypeReg {
			continue
		}

		name := filepath.Base(hdr.Name)
		if name != "ywai" {
			continue
		}

		outPath := filepath.Join(dest, name)
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}

		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return err
		}
		_ = out.Close()

		if err := os.Chmod(outPath, 0o755); err != nil {
			return fmt.Errorf("chmod %s: %w", outPath, err)
		}
		return nil
	}

	return fmt.Errorf("ywai binary not found in tarball")
}
