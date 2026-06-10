package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
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
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func LatestVersion() (string, error) {
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

func Run(currentVersion string) (string, error) {
	latest, err := LatestVersion()
	if err != nil {
		return "", fmt.Errorf("checking latest version: %w", err)
	}

	normalized := latest
	if strings.HasPrefix(normalized, "v") {
		normalized = normalized[1:]
	}

	if normalized == currentVersion {
		return "", nil
	}

	return downloadAndReplace(latest)
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
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, filepath.Base(asset))
	if err := downloadFile(downloadURL, archivePath); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
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
	os.Remove(bakPath)
	if err := os.Rename(exe, bakPath); err != nil {
		return "", fmt.Errorf("cannot backup old binary: %w", err)
	}

	if err := replaceBinary(binaryPath, exe); err != nil {
		os.Rename(bakPath, exe)
		return "", fmt.Errorf("cannot replace binary: %w", err)
	}

	os.Remove(bakPath)

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
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer dstFile.Close()

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

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
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
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	return err
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

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
			rc.Close()
			return err
		}

		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
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
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

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
			out.Close()
			return err
		}
		out.Close()

		os.Chmod(outPath, 0o755)
		return nil
	}

	return fmt.Errorf("ywai binary not found in tarball")
}
