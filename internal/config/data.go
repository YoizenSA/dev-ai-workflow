package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func EnsureDataDir() error {
	dirs := []string{DataDir(), DataSkillsDir(), DataProjectTypesDir()}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

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

func SeedDataFrom(repoRoot string) error {
	if err := EnsureDataDir(); err != nil {
		return err
	}

	for _, dir := range []string{SkillsDirName, ProjectTypesDir} {
		src := filepath.Join(repoRoot, dir)
		dst := filepath.Join(DataDir(), dir)

		entries, err := os.ReadDir(src)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			srcPath := filepath.Join(src, entry.Name())
			dstPath := filepath.Join(dst, entry.Name())

			if _, err := os.Stat(dstPath); err == nil {
				continue
			}

			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

var (
	getEmbeddedSkillsFS       func() fs.FS
	getEmbeddedProjectTypesFS func() fs.FS
)

func RegisterEmbeddedProviders(skillsFS, ptFS func() fs.FS) {
	getEmbeddedSkillsFS = skillsFS
	getEmbeddedProjectTypesFS = ptFS
}

func SeedFromEmbedded() error {
	if err := EnsureDataDir(); err != nil {
		return err
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
	return fs.WalkDir(fsys, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
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
			return err
		}
		return os.WriteFile(dstPath, data, 0o644)
	})
}

func copyDirRecursive(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}
