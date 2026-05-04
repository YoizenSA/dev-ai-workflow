package config

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	getEmbeddedSkillsFS       func() fs.FS
	getEmbeddedProjectTypesFS func() fs.FS
	fsMutex                   sync.Mutex
)

func EnsureDataDir() error {
	fsMutex.Lock()
	defer fsMutex.Unlock()

	dirs := []string{DataDir(), DataSkillsDir(), DataProjectTypesDir()}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}
	}
	return nil
}

func ShouldSeedData() bool {
	return !isDirPopulated(DataSkillsDir()) || !isDirPopulated(DataProjectTypesDir())
}

func SeedDataFrom(repoRoot string) error {
	if err := EnsureDataDir(); err != nil {
		return err
	}

	for _, pair := range []struct{ src, dst string }{
		{SkillsSourceDir(), DataSkillsDir()},
		{ProjectTypesSourceDir(), DataProjectTypesDir()},
	} {
		entries, err := os.ReadDir(pair.src)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			srcPath := filepath.Join(pair.src, entry.Name())
			dstPath := filepath.Join(pair.dst, entry.Name())

			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy %s to %s: %w", srcPath, dstPath, err)
			}
		}
	}
	return nil
}

func RegisterEmbeddedProviders(skillsFS, ptFS func() fs.FS) {
	getEmbeddedSkillsFS = skillsFS
	getEmbeddedProjectTypesFS = ptFS
}

func SeedFromEmbedded() error {
	if err := EnsureDataDir(); err != nil {
		return fmt.Errorf("failed to ensure data directories: %w", err)
	}

	if fn := getEmbeddedSkillsFS; fn != nil {
		if fsys := fn(); fsys != nil {
			if err := extractFS(fsys, ".", DataSkillsDir()); err != nil {
				return fmt.Errorf("failed to extract embedded skills: %w", err)
			}
		}
	}

	if fn := getEmbeddedProjectTypesFS; fn != nil {
		if fsys := fn(); fsys != nil {
			if err := extractFS(fsys, ".", DataProjectTypesDir()); err != nil {
				return fmt.Errorf("failed to extract embedded project-types: %w", err)
			}
		}
	}

	return nil
}

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
			return os.MkdirAll(dstPath, 0o755)
		}

		if _, err := os.Stat(dstPath); err == nil {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("error reading embedded file %s: %w", path, err)
		}

		return os.WriteFile(dstPath, data, 0o644)
	})
}

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
			return os.MkdirAll(dstPath, info.Mode())
		}

		if _, err := os.Stat(dstPath); err == nil {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading file %s: %w", path, err)
		}

		return os.WriteFile(dstPath, data, 0o644)
	})
}
