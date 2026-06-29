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
	getEmbeddedSkillsFS   func() fs.FS
	getEmbeddedAgentsFS   func() fs.FS
	getEmbeddedDefaultsFS func() fs.FS
	getEmbeddedPluginsFS  func() fs.FS
	fsMutex               sync.Mutex
)

// BackgroundAgentsBundleName is the filename of the bundled opencode
// background-agents plugin, both in the embedded FS and once seeded to disk.
const BackgroundAgentsBundleName = "background-agents.js"

// TuiLogoBundleName is the filename of the ywai TUI logo plugin, both in the
// embedded FS (under plugins/tui/) and once seeded/installed to disk.
const TuiLogoBundleName = "ywai-logo.tsx"

func EnsureDataDir() error {
	fsMutex.Lock()
	defer fsMutex.Unlock()

	dirs := []string{DataDir(), DataSkillsDir(), DataAgentsDir(), DataWorkflowsDir()}
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

func RegisterEmbeddedDefaults(defaultsFS func() fs.FS) {
	getEmbeddedDefaultsFS = defaultsFS
}

func RegisterEmbeddedPlugins(pluginsFS func() fs.FS) {
	getEmbeddedPluginsFS = pluginsFS
}

// GetEmbeddedDefaults reads the defaults.jsonc from embedded FS.
func GetEmbeddedDefaults() ([]byte, error) {
	if fn := getEmbeddedDefaultsFS; fn != nil {
		if fsys := fn(); fsys != nil {
			return fs.ReadFile(fsys, "defaults.jsonc")
		}
	}
	return nil, fmt.Errorf("no embedded defaults available")
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
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())

		if entry.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy %s to %s: %w", srcPath, dstPath, err)
			}
		} else {
			// Copy individual files (like groups.json)
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", srcPath, err)
			}
			if err := os.WriteFile(dstPath, data, 0o644); err != nil {
				return fmt.Errorf("failed to write %s: %w", dstPath, err)
			}
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
			if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					count++
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to walk embedded agents: %w", err)
			}
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
			if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					count++
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to walk embedded skills: %w", err)
			}
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

// SeedPluginsFromEmbedded extracts the embedded plugin bundles (flat .js files)
// into DataPluginsDir so source-less binaries can install them.
func SeedPluginsFromEmbedded() error {
	if err := os.MkdirAll(DataPluginsDir(), 0o755); err != nil {
		return fmt.Errorf("failed to create data plugins directory: %w", err)
	}

	if fn := getEmbeddedPluginsFS; fn != nil {
		if fsys := fn(); fsys != nil {
			count := 0
			if err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					count++
				}
				return nil
			}); err != nil {
				return fmt.Errorf("failed to walk embedded plugins: %w", err)
			}
			if count > 0 {
				if err := extractFS(fsys, ".", DataPluginsDir()); err != nil {
					return fmt.Errorf("failed to extract embedded plugins: %w", err)
				}
				return nil
			}
		}
	}

	return fmt.Errorf("no embedded plugins data available")
}

// BackgroundAgentsBundlePath resolves the path to the bundled background-agents
// plugin JS. It prefers the source checkout (ywai/plugins/background-agents/
// dist/), falls back to the seeded copy under DataPluginsDir, and seeds from the
// embedded FS on demand. Returns an error when no bundle exists (e.g. a source
// build where `bun` was unavailable at prepare-embedded time).
func BackgroundAgentsBundlePath() (string, error) {
	// 1. Source checkout: ywai/plugins/background-agents/dist/background-agents.js
	srcBundle := filepath.Join(PluginsSourceDir(), "background-agents", "dist", BackgroundAgentsBundleName)
	if _, err := os.Stat(srcBundle); err == nil {
		return srcBundle, nil
	}

	// 2. Already seeded to the data dir.
	seeded := filepath.Join(DataPluginsDir(), BackgroundAgentsBundleName)
	if _, err := os.Stat(seeded); err == nil {
		return seeded, nil
	}

	// 3. Seed from embedded FS, then re-check.
	if err := SeedPluginsFromEmbedded(); err == nil {
		if _, err := os.Stat(seeded); err == nil {
			return seeded, nil
		}
	}

	return "", fmt.Errorf("background-agents plugin bundle not found; rebuild embedded data with `bun` available (cd ywai && bash scripts/prepare-embedded.sh)")
}

// TuiLogoBundlePath resolves the path to the ywai TUI logo plugin source. It
// prefers the source checkout (ywai/plugins/tui/ywai-logo.tsx), falls back to
// the seeded copy under DataPluginsDir()/tui/, and seeds from the embedded FS
// on demand. Unlike the background-agents bundle the logo is a plain .tsx file
// (no build step), so it ships as source.
func TuiLogoBundlePath() (string, error) {
	// 1. Source checkout: ywai/plugins/tui/ywai-logo.tsx
	srcBundle := filepath.Join(PluginsSourceDir(), "tui", TuiLogoBundleName)
	if _, err := os.Stat(srcBundle); err == nil {
		return srcBundle, nil
	}

	// 2. Already seeded to the data dir.
	seeded := filepath.Join(DataPluginsDir(), "tui", TuiLogoBundleName)
	if _, err := os.Stat(seeded); err == nil {
		return seeded, nil
	}

	// 3. Seed from embedded FS, then re-check.
	if err := SeedPluginsFromEmbedded(); err == nil {
		if _, err := os.Stat(seeded); err == nil {
			return seeded, nil
		}
	}

	return "", fmt.Errorf("ywai TUI logo plugin not found; rebuild embedded data (cd ywai && bash scripts/prepare-embedded.sh)")
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
