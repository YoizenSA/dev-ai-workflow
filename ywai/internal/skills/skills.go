package skills

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

const extraSkillMarkerFile = ".ywai-extra"

func CopyTo(agentSkillsDir string) error {
	return copyFiltered(agentSkillsDir, nil)
}

func CopyFiltered(agentSkillsDir string, filter []string) error {
	return copyFiltered(agentSkillsDir, filter)
}

func copyFiltered(agentSkillsDir string, filter []string) error {
	srcDir := skillsSourceDir()
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("skills source directory not found: %s", srcDir)
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("failed to read skills directory: %w", err)
	}

	filterSet := make(map[string]bool, len(filter))
	for _, f := range filter {
		filterSet[f] = true
	}

	extraSkills := ywaiExtraSkillNames(srcDir)
	copied := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !extraSkills[name] {
			continue
		}
		if len(filterSet) > 0 && !filterSet[name] {
			continue
		}

		src := filepath.Join(srcDir, name)
		dst := filepath.Join(agentSkillsDir, name)

		if pathExists(dst) {
			if err := removeExistingSkillPath(dst); err != nil {
				fmt.Printf("  Warning: failed to remove existing %s: %v\n", name, err)
				continue
			}
		}

		if err := copyDir(src, dst); err != nil {
			fmt.Printf("  Warning: failed to copy skill %s: %v\n", name, err)
			continue
		}

		fmt.Printf("  Copied skill: %s\n", name)
		copied++
	}

	if copied == 0 {
		fmt.Println("  All skills already up to date.")
	}
	return nil
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	return filepath.WalkDir(src, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func removeExistingSkillPath(path string) error {
	if config.IsWindows() {
		// Junctions are reparse points. `rmdir` removes the junction itself,
		// while recursive filesystem walks can be unreliable and may follow the
		// target depending on the existing path type.
		cmd := exec.Command("cmd", "/c", "rmdir", path)
		if output, err := cmd.CombinedOutput(); err == nil {
			return nil
		} else if !pathExists(path) {
			return nil
		} else if len(output) > 0 {
			// Fall through to RemoveAll for real directories/files, but preserve
			// the rmdir output if RemoveAll fails below.
			if removeErr := os.RemoveAll(path); removeErr != nil {
				return fmt.Errorf("rmdir failed: %s; remove all failed: %w", strings.TrimSpace(string(output)), removeErr)
			}
			return nil
		}
	}

	return os.RemoveAll(path)
}

// RemoveStaleYwaiSkillLinks removes only symlink/junction placeholders that
// point back into ywai's skill cache/source but are no longer ywai extra skills.
// Older ywai versions linked upstream-managed skills from ~/.ywai/skills; those
// links block gentle-ai's safe atomic writer. This is data-driven: ywai-owned
// skills are detected from marker data, not from an upstream denylist.
func RemoveStaleYwaiSkillLinks(agentSkillsDir string) ([]string, error) {
	entries, err := os.ReadDir(agentSkillsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read agent skills directory %s: %w", agentSkillsDir, err)
	}

	srcDir := skillsSourceDir()
	extraSkills := ywaiExtraSkillNames(srcDir)
	var removed []string
	for _, entry := range entries {
		name := entry.Name()
		if extraSkills[name] {
			continue
		}

		path := filepath.Join(agentSkillsDir, name)
		if !IsLinkOrJunction(path) {
			continue
		}
		if !linkTargetsYwaiSkills(path, srcDir) {
			continue
		}

		if err := removeExistingSkillPath(path); err != nil {
			return removed, fmt.Errorf("failed to remove stale ywai skill link %s: %w", path, err)
		}
		removed = append(removed, name)
	}

	sort.Strings(removed)
	return removed, nil
}

func IsLinkOrJunction(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}
	if config.IsWindows() {
		_, err := os.Readlink(path)
		return err == nil
	}
	return false
}

func linkTargetsYwaiSkills(path, srcDir string) bool {
	target, err := os.Readlink(path)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}

	for _, root := range uniquePaths([]string{srcDir, config.DataSkillsDir(), config.SkillsSourceDir()}) {
		if isPathWithin(target, root) {
			return true
		}
	}
	return false
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		clean := filepath.Clean(path)
		if clean == "." || clean == "" {
			continue
		}
		key := strings.ToLower(clean)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, clean)
	}
	return unique
}

func isPathWithin(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func ListAvailable() ([]string, error) {
	srcDir := skillsSourceDir()
	if _, err := os.ReadDir(srcDir); err != nil {
		return nil, err
	}

	extraSkills := ywaiExtraSkillNames(srcDir)
	names := make([]string, 0, len(extraSkills))
	for name := range extraSkills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func ywaiExtraSkillNames(srcDir string) map[string]bool {
	names := make(map[string]bool)

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return names
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if hasYwaiExtraMarker(filepath.Join(srcDir, name)) {
			names[name] = true
		}
	}

	return names
}

func hasYwaiExtraMarker(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, extraSkillMarkerFile))
	return err == nil
}

func skillsSourceDir() string {
	// Prefer the source checkout when available. Skills are copied, not linked,
	// so using the repo source no longer creates rollback/symlink issues for
	// upstream tools.
	repo := config.RepoRoot()
	for _, candidate := range []string{
		filepath.Join(repo, "ywai", config.SkillsDirName),
		filepath.Join(repo, config.SkillsDirName),
	} {
		if config.IsDirPopulated(candidate) {
			return candidate
		}
	}

	if config.IsDirPopulated(config.DataSkillsDir()) {
		return config.DataSkillsDir()
	}
	return config.SkillsSourceDir()
}

// sddAssetSubdirs are the sibling directories of an agent's skills dir where
// gentle-ai writes SDD assets (commands, skills, and agent definitions).
var sddAssetSubdirs = []string{"skills", "commands", "agents"}

// sddPrefix is the filename prefix of all SDD-managed assets.
const sddPrefix = "sdd-"

// RemoveSddAssets deletes every SDD-managed asset from an agent's config
// directory. agentSkillsDir is the agent's skills directory (e.g.
// ~/.claude/skills); the config dir is its parent. It scans the skills,
// commands, and agents subdirectories for entries named "sdd-*" and removes
// them, as well as the "sdd-*.md" files inside skills/_shared. Extra skills
// owned by ywai (angular, judgment-day, etc.) are never affected because they
// do not match the "sdd-" prefix.
//
// Returns the sorted list of removed relative paths (e.g. "claude-code/skills/sdd-init").
func RemoveSddAssets(agentSkillsDir string) ([]string, error) {
	configDir := filepath.Clean(filepath.Dir(agentSkillsDir))
	var removed []string

	for _, sub := range sddAssetSubdirs {
		subDir := filepath.Join(configDir, sub)
		entries, err := os.ReadDir(subDir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return removed, fmt.Errorf("failed to read %s: %w", subDir, err)
		}

		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasPrefix(name, sddPrefix) {
				continue
			}
			path := filepath.Join(subDir, name)
			if !isPathWithin(path, subDir) {
				continue
			}
			if err := removeExistingSkillPath(path); err != nil {
				return removed, fmt.Errorf("failed to remove %s: %w", path, err)
			}
			removed = append(removed, filepath.Join(sub, name))
		}

		// skills/_shared also holds sdd-*.md files written by gentle-ai.
		if sub == "skills" {
			sharedDir := filepath.Join(subDir, "_shared")
			sharedEntries, err := os.ReadDir(sharedDir)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return removed, fmt.Errorf("failed to read %s: %w", sharedDir, err)
			}
			for _, entry := range sharedEntries {
				name := entry.Name()
				if entry.IsDir() || !strings.HasPrefix(name, sddPrefix) {
					continue
				}
				path := filepath.Join(sharedDir, name)
				if err := removeExistingSkillPath(path); err != nil {
					return removed, fmt.Errorf("failed to remove %s: %w", path, err)
				}
				removed = append(removed, filepath.Join(sub, "_shared", name))
			}
		}
	}

	sort.Strings(removed)
	return removed, nil
}

// CountSddAssets counts SDD-managed assets present in an agent's config
// directory, using the same discovery rules as RemoveSddAssets. It is read-only.
func CountSddAssets(agentSkillsDir string) int {
	configDir := filepath.Clean(filepath.Dir(agentSkillsDir))
	count := 0

	for _, sub := range sddAssetSubdirs {
		subDir := filepath.Join(configDir, sub)
		entries, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), sddPrefix) {
				count++
			}
		}
		if sub == "skills" {
			sharedEntries, err := os.ReadDir(filepath.Join(subDir, "_shared"))
			if err != nil {
				continue
			}
			for _, entry := range sharedEntries {
				if !entry.IsDir() && strings.HasPrefix(entry.Name(), sddPrefix) {
					count++
				}
			}
		}
	}
	return count
}
