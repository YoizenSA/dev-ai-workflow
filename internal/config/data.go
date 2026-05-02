package config

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// This package handles data directory management, seeding, and filesystem operations
// for the ywai AI workflow tool. It ensures proper directory creation with secure
// permissions and supports both local filesystem and embedded resource seeding.

// EnsureDataDir creates all required data directories if they don't exist.
// It uses a mutex to prevent race conditions during directory creation.
func EnsureDataDir() error {
	fsMutex.Lock()
	defer fsMutex.Unlock()

	dirs := []string{DataDir(), DataSkillsDir(), DataProjectTypesDir()}
	for _, d := range dirs {
		if err := os.MkdirAll(d, getFilePermissions()); err != nil {
			log.Printf("Failed to create directory %s: %v", d, err)
			return fmt.Errorf("failed to create data directory: %w", err)
		}
		log.Printf("Ensured directory exists: %s", d)
	}
	return nil
}

// ShouldSeedData checks if the data directory needs to be seeded.
// It returns true if the skills directory doesn't exist or is empty.
func ShouldSeedData() bool {
	if _, err := os.Stat(DataSkillsDir()); os.IsNotExist(err) {
		return true
	}
	entries, err := os.ReadDir(DataSkillsDir())
	if err != nil || len(entries) == 0 {
		return true
	}
	return false
}

// SeedDataFrom copies skill and project type directories from a repository root
// to the local data directory. It only copies directories that don't already exist.
func SeedDataFrom(repoRoot string) error {
	if err := EnsureDataDir(); err != nil {
		return err
	}

	for _, dir := range []string{SkillsDirName, ProjectTypesDir} {
		src := filepath.Join(repoRoot, dir)
		dst := filepath.Join(DataDir(), dir)

		entries, err := os.ReadDir(src)
		if err != nil {
			log.Printf("Warning: Could not read source directory %s: %v", src, err)
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			srcPath := filepath.Join(src, entry.Name())
			dstPath := filepath.Join(dst, entry.Name())

			if _, err := os.Stat(dstPath); err == nil {
				log.Printf("Skipping existing directory: %s", dstPath)
				continue
			}

			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy %s to %s: %w", srcPath, dstPath, err)
			}
		}
	}
	return nil
}

var (
	// getEmbeddedSkillsFS provides access to embedded skills filesystem
	getEmbeddedSkillsFS func() fs.FS

	// getEmbeddedProjectTypesFS provides access to embedded project types filesystem
	getEmbeddedProjectTypesFS func() fs.FS

	// fsMutex prevents concurrent directory creation conflicts
	fsMutex sync.Mutex
)

// RegisterEmbeddedProviders registers functions that provide access to embedded
// filesystems for skills and project types. These are used when seeding from embedded resources.
func RegisterEmbeddedProviders(skillsFS, ptFS func() fs.FS) {
	getEmbeddedSkillsFS = skillsFS
	getEmbeddedProjectTypesFS = ptFS
}

// getFilePermissions returns appropriate file permissions for the current operating system.
// On Windows, it returns 700 (owner rwx only). On Unix-like systems, it returns 750
// (owner rwx, group r-x, others read/execute).
func getFilePermissions() os.FileMode {
	if IsWindows() {
		// Windows: More restrictive for security
		return 0o700
	}
	// Unix-like: 750 (owner rwx, group r-x, others ---)
	return 0o750
}

func SeedFromEmbedded() error {
	log.Println("Starting data seeding from embedded resources...")

	if err := EnsureDataDir(); err != nil {
		return fmt.Errorf("failed to ensure data directories: %w", err)
	}

	if fn := getEmbeddedSkillsFS; fn != nil {
		if fsys := fn(); fsys != nil {
			log.Println("Extracting embedded skills...")
			if err := extractFS(fsys, ".", DataSkillsDir()); err != nil {
				return fmt.Errorf("failed to extract embedded skills: %w", err)
			}
			log.Println("Successfully extracted embedded skills")
		} else {
			log.Println("Warning: embedded skills filesystem is nil")
		}
	} else {
		log.Println("Warning: embedded skills provider not registered")
	}

	if fn := getEmbeddedProjectTypesFS; fn != nil {
		if fsys := fn(); fsys != nil {
			log.Println("Extracting embedded project types...")
			if err := extractFS(fsys, ".", DataProjectTypesDir()); err != nil {
				return fmt.Errorf("failed to extract embedded project-types: %w", err)
			}
			log.Println("Successfully extracted embedded project types")
		} else {
			log.Println("Warning: embedded project types filesystem is nil")
		}
	} else {
		log.Println("Warning: embedded project types provider not registered")
	}

	log.Println("Data seeding completed successfully")
	return nil
}

// extractFS extracts files and directories from an embedded filesystem to the destination directory.
// It respects file existence checks, uses appropriate permissions, and includes timeout protection.
func extractFS(fsys fs.FS, srcDir, dstDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	return fs.WalkDir(fsys, srcDir, func(path string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled: %w", ctx.Err())
		default:
		}

		if err != nil {
			return fmt.Errorf("error walking embedded filesystem: %w", err)
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("error getting relative path: %w", err)
		}
		if rel == "." {
			return nil
		}

		dstPath := filepath.Join(dstDir, rel)

		if d.IsDir() {
			if err := os.MkdirAll(dstPath, getFilePermissions()); err != nil {
				return fmt.Errorf("error creating directory %s: %w", dstPath, err)
			}
			log.Printf("Created directory: %s", dstPath)
			return nil
		}

		// Skip if file already exists
		if _, err := os.Stat(dstPath); err == nil {
			log.Printf("Skipping existing file: %s", dstPath)
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("error reading embedded file %s: %w", path, err)
		}

		if err := os.WriteFile(dstPath, data, getFilePermissions()); err != nil {
			return fmt.Errorf("error writing embedded file %s: %w", dstPath, err)
		}

		log.Printf("Extracted file: %s", dstPath)
		return nil
	})
}

// copyDirRecursive recursively copies a directory from source to destination.
// It preserves directory structure, respects file existence checks, and includes timeout protection.
// Files are written with secure permissions ( getFilePermissions() ) instead of original permissions.
func copyDirRecursive(src, dst string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled: %w", ctx.Err())
		default:
		}

		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("error getting relative path: %w", err)
		}
		dstPath := filepath.Join(dst, rel)

		if info.IsDir() {
			if err := os.MkdirAll(dstPath, info.Mode()); err != nil {
				return fmt.Errorf("error creating directory %s: %w", dstPath, err)
			}
			return nil
		}

		// Check if file already exists and skip if it does
		if _, err := os.Stat(dstPath); err == nil {
			log.Printf("Skipping existing file: %s", dstPath)
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading file %s: %w", path, err)
		}

		if err := os.WriteFile(dstPath, data, getFilePermissions()); err != nil {
			return fmt.Errorf("error writing file %s: %w", dstPath, err)
		}

		log.Printf("Copied file: %s -> %s", path, dstPath)
		return nil
	})
}
