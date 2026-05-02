package skills

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Yoizen/ywai/internal/config"
)

func LinkTo(agentSkillsDir string) error {
	return linkFiltered(agentSkillsDir, nil)
}

func LinkFiltered(agentSkillsDir string, filter []string) error {
	return linkFiltered(agentSkillsDir, filter)
}

func linkFiltered(agentSkillsDir string, filter []string) error {
	srcDir := config.SkillsSourceDir()
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

	linked := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if name == "_shared" {
			continue
		}
		if len(filterSet) > 0 && !filterSet[name] {
			continue
		}

		src := filepath.Join(srcDir, name)
		dst := filepath.Join(agentSkillsDir, name)

		if isCurrentLink(dst, src) {
			continue
		}

		if _, err := os.Stat(dst); err == nil {
			if err := os.RemoveAll(dst); err != nil {
				fmt.Printf("  Warning: failed to remove existing %s: %v\n", name, err)
				continue
			}
		}

		if err := createLink(src, dst); err != nil {
			fmt.Printf("  Warning: failed to link skill %s: %v\n", name, err)
			continue
		}

		fmt.Printf("  Linked skill: %s\n", name)
		linked++
	}

	if linked == 0 {
		fmt.Println("  All skills already linked.")
	}
	return nil
}

func createLink(src, dst string) error {
	if config.IsWindows() {
		return createJunction(src, dst)
	}
	return os.Symlink(src, dst)
}

func createJunction(src, dst string) error {
	cmd := exec.Command("cmd", "/c", "mklink", "/J", dst, src)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mklink /J failed: %w: %s", err, string(output))
	}
	return nil
}

func isCurrentLink(dst, src string) bool {
	if config.IsWindows() {
		return isCurrentJunction(dst, src)
	}
	target, err := os.Readlink(dst)
	if err != nil {
		return false
	}
	return filepath.Clean(target) == filepath.Clean(src)
}

func isCurrentJunction(dst, src string) bool {
	target, err := os.Readlink(dst)
	if err != nil {
		return false
	}
	return filepath.Clean(target) == filepath.Clean(src)
}

func ListAvailable() ([]string, error) {
	srcDir := config.SkillsSourceDir()
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}
