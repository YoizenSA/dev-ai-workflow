package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yoizen/ga/internal/utils"
)

const CacheDir = ".cache/ga"

var (
	cacheMutex sync.RWMutex
)

func GetProjectCacheDir() (string, error) {
	// Get project path without holding lock (git command can modify repo state)
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	projectPath := strings.TrimSpace(string(out))
	projectHash := fmt.Sprintf("%x", sha256.Sum256([]byte(projectPath)))

	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
		if home == "" {
			// Fallback for Windows
			home = os.Getenv("APPDATA")
			if home == "" {
				return "", fmt.Errorf("could not determine home directory")
			}
		}
	}

	return filepath.Join(home, ".cache", "ga", projectHash), nil
}

func GetFileHash(file string) (string, error) {
	// Validate file path before processing
	if err := utils.ValidateFilePath(file); err != nil {
		return "", fmt.Errorf("invalid file path: %v", err)
	}

	// Check if file exists and get its size
	info, err := os.Stat(file)
	if err != nil {
		return "", fmt.Errorf("cannot stat file: %v", err)
	}
	
	// Don't hash directories
	if info.IsDir() {
		return "", fmt.Errorf("cannot hash directory: %s", file)
	}

	// Open file for reading
	f, err := os.Open(file)
	if err != nil {
		return "", fmt.Errorf("cannot open file: %v", err)
	}
	defer f.Close()

	// Create hash calculator
	hasher := sha256.New()
	
	// Use streaming to avoid loading entire file into memory
	// Read in 64KB chunks to balance performance and memory usage
	buf := make([]byte, 64*1024) // 64KB buffer
	for {
		n, err := f.Read(buf)
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("error reading file: %v", err)
		}
		if n == 0 {
			break
		}
		
		// Write chunk to hash
		if _, err := hasher.Write(buf[:n]); err != nil {
			return "", fmt.Errorf("error updating hash: %v", err)
		}
	}

	// Get final hash as hex string
	hashBytes := hasher.Sum(nil)
	return hex.EncodeToString(hashBytes), nil
}

func GetStringHash(s string) string {
	hash := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", hash)
}

func GetMetadataHash(rulesFile, configFile string) (string, error) {
	rulesHash := ""
	configHash := ""

	if data, err := os.ReadFile(rulesFile); err == nil {
		rulesHash = fmt.Sprintf("%x", sha256.Sum256(data))
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if data, err := os.ReadFile(configFile); err == nil {
		configHash = fmt.Sprintf("%x", sha256.Sum256(data))
	} else if !os.IsNotExist(err) {
		return "", err
	}

	return GetStringHash(rulesHash + ":" + configHash), nil
}

func InitCache(rulesFile, configFile string) (string, error) {
	cacheDir, err := GetProjectCacheDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Join(cacheDir, "files"), 0755); err != nil {
		return "", err
	}

	metadataHash, err := GetMetadataHash(rulesFile, configFile)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(filepath.Join(cacheDir, "metadata"), []byte(metadataHash), 0644); err != nil {
		return "", err
	}

	return cacheDir, nil
}

func IsCacheValid(rulesFile, configFile string) bool {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	
	cacheDir, err := GetProjectCacheDir()
	if err != nil {
		return false
	}

	metadataFile := filepath.Join(cacheDir, "metadata")
	storedHash, err := os.ReadFile(metadataFile)
	if err != nil {
		return false
	}

	currentHash, err := GetMetadataHash(rulesFile, configFile)
	if err != nil {
		return false
	}

	return string(storedHash) == currentHash
}

func IsFileCached(file string) bool {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	
	cacheDir, err := GetProjectCacheDir()
	if err != nil {
		return false
	}

	filesDir := filepath.Join(cacheDir, "files")
	if _, err := os.Stat(filesDir); err != nil {
		return false
	}

	fileHash, err := GetFileHash(file)
	if err != nil {
		// File doesn't exist or can't be read - not cacheable
		return false
	}

	cacheFile := filepath.Join(filesDir, fileHash)
	status, err := os.ReadFile(cacheFile)
	if err != nil {
		// Cache file doesn't exist - file not cached
		return false
	}
	return string(status) == "PASSED"
}

func CacheFileResult(file, status string) error {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	
	cacheDir, err := GetProjectCacheDir()
	if err != nil {
		return err
	}

	filesDir := filepath.Join(cacheDir, "files")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return err
	}

	fileHash, err := GetFileHash(file)
	if err != nil {
		return err
	}

	cacheFile := filepath.Join(filesDir, fileHash)
	return os.WriteFile(cacheFile, []byte(status), 0644)
}

func InvalidateCache() error {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	
	cacheDir, err := GetProjectCacheDir()
	if err != nil {
		return err
	}

	if _, err := os.Stat(cacheDir); err == nil {
		return os.RemoveAll(cacheDir)
	}

	return nil
}

func ClearAllCache() error {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
		if home == "" {
			// Fallback for Windows
			home = os.Getenv("APPDATA")
			if home == "" {
				return fmt.Errorf("could not determine home directory")
			}
		}
	}

	cachePath := filepath.Join(home, ".cache", "ga")
	if _, err := os.Stat(cachePath); err == nil {
		return os.RemoveAll(cachePath)
	}

	return nil
}

func GetCacheStats() (int, int, string, error) {
	cacheDir, err := GetProjectCacheDir()
	if err != nil {
		return 0, 0, "", err
	}

	filesDir := filepath.Join(cacheDir, "files")
	if _, err := os.Stat(filesDir); err != nil {
		return 0, 0, "", nil
	}

	entries, err := os.ReadDir(filesDir)
	if err != nil {
		return 0, 0, "", err
	}

	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}

	var size string
	if stat, err := os.Stat(cacheDir); err == nil {
		size = formatSize(stat.Size())
	}

	return count, count, size, nil
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(bytes)/(1024*1024))
}
