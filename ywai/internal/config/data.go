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
	getEmbeddedSkillsFS func() fs.FS
	getEmbeddedAgentsFS func() fs.FS
	fsMutex             sync.Mutex
)

func EnsureDataDir() error {
	fsMutex.Lock()
	defer fsMutex.Unlock()

	dirs := []string{DataDir(), DataSkillsDir(), DataAgentsDir()}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}
	}
	return nil
}

func ShouldSeedData() bool {
	return !IsDirPopulated(DataSkillsDir())
}

func SeedSkillsFrom(repoRoot string) error {
	if err := EnsureDataDir(); err != nil {
		return err
	}

	srcDir := SkillsSourceDir()
	dstDir := DataSkillsDir()

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		if err := copyDirRecursive(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to copy %s to %s: %w", srcPath, dstPath, err)
		}
	}
	return nil
}

func RegisterEmbeddedProviders(skillsFS func() fs.FS) {
	getEmbeddedSkillsFS = skillsFS
}

func RegisterEmbeddedAgents(agentsFS func() fs.FS) {
	getEmbeddedAgentsFS = agentsFS
}

func SeedAgentsFrom(repoRoot string) error {
	if err := EnsureDataDir(); err != nil {
		return err
	}

	srcDir := AgentsSourceDir()
	dstDir := DataAgentsDir()

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		if err := copyDirRecursive(srcPath, dstPath); err != nil {
			return fmt.Errorf("failed to copy %s to %s: %w", srcPath, dstPath, err)
		}
	}
	return nil
}

func SeedAgentsFromEmbedded() error {
	if err := EnsureDataDir(); err != nil {
		return fmt.Errorf("failed to ensure data directory: %w", err)
	}

	if fn := getEmbeddedAgentsFS; fn != nil {
		if fsys := fn(); fsys != nil {
			count := 0
			fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					count++
				}
				return nil
			})
			if count > 0 {
				fmt.Printf("  Seeding agents from embedded (%d files)...\n", count)
				if err := extractFS(fsys, ".", DataAgentsDir()); err != nil {
					return fmt.Errorf("failed to extract embedded agents: %w", err)
				}
				return nil
			}
		}
	}

	return fmt.Errorf("no embedded agents data available")
}

func SeedSkillsFromEmbedded() error {
	if err := EnsureDataDir(); err != nil {
		return fmt.Errorf("failed to ensure data directory: %w", err)
	}

	if fn := getEmbeddedSkillsFS; fn != nil {
		if fsys := fn(); fsys != nil {
			count := 0
			fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					count++
				}
				return nil
			})
			if count > 0 {
				fmt.Printf("  Seeding skills from embedded (%d files)...\n", count)
				if err := extractFS(fsys, ".", DataSkillsDir()); err != nil {
					return fmt.Errorf("failed to extract embedded skills: %w", err)
				}
				return nil
			}
		}
	}

	return fmt.Errorf("no embedded skills data available; binary was built without embedded resources.\n" +
		"  Reinstall with:\n" +
		"    macOS/Linux: curl -fsSL https://github.com/YoizenSA/dev-ai-workflow/releases/latest/download/install.sh | bash\n" +
		"    Source:      cd ywai && bash scripts/prepare-embedded.sh && go install -tags embedded ./cmd/ywai")
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

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading file %s: %w", path, err)
		}

		return os.WriteFile(dstPath, data, 0o644)
	})
}
