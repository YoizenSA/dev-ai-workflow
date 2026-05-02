package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yoizen/ywai/internal/config"
)

var KnownTypes = []string{"generic", "react", "nest", "dotnet", "devops"}

func SkillsForType(projectType string) []string {
	return config.ProfileSkills(projectType)
}

func ValidateType(projectType string) error {
	cfg := config.LoadConfig()
	if _, ok := cfg.Profiles[projectType]; ok {
		return nil
	}
	available := AvailableTypes()
	return fmt.Errorf("unknown project type %q. Available: %v", projectType, available)
}

func AvailableTypes() []string {
	return config.AvailableProfiles()
}

func Init(projectType, targetDir string) error {
	if projectType == "" {
		return fmt.Errorf("project type is required. Available: %v", AvailableTypes())
	}

	if err := ValidateType(projectType); err != nil {
		return err
	}

	srcDir := filepath.Join(config.ProjectTypesSourceDir(), projectType)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("project type directory not found: %s", srcDir)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
	}

	copied := 0
	for _, file := range []string{"AGENTS.md", "REVIEW.md"} {
		src := filepath.Join(srcDir, file)
		dst := filepath.Join(targetDir, file)

		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}

		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", src, err)
		}

		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", dst, err)
		}

		fmt.Printf("Copied %s -> %s\n", file, dst)
		copied++
	}

	if copied == 0 {
		return fmt.Errorf("no AGENTS.md or REVIEW.md found in %s", srcDir)
	}

	if err := ensureGitignore(targetDir); err != nil {
		fmt.Printf("Warning: failed to update .gitignore: %v\n", err)
	}

	fmt.Printf("Project initialized as %q.\n", projectType)
	return nil
}

var gitignoreEntries = []string{
	".sdd/",
	".ywai.yaml",
	".copilot/",
}

func ensureGitignore(targetDir string) error {
	gitignorePath := filepath.Join(targetDir, ".gitignore")

	existing := ""
	data, err := os.ReadFile(gitignorePath)
	if err == nil {
		existing = string(data)
	}

	var toAdd []string
	for _, entry := range gitignoreEntries {
		if !containsLine(existing, entry) {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	var b strings.Builder
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n# ywai\n")
	for _, entry := range toAdd {
		b.WriteString(entry + "\n")
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(b.String())
	if err != nil {
		return err
	}

	fmt.Printf("Updated .gitignore with ywai entries.\n")
	return nil
}

func containsLine(content, line string) bool {
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == line {
			return true
		}
	}
	return false
}
