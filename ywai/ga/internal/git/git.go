package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yoizen/ga/internal/utils"
)

func GetStagedFiles(patterns, excludes string) ([]string, error) {
	// Validate input patterns
	if err := utils.IsValidPattern(patterns); err != nil {
		return nil, fmt.Errorf("invalid include patterns: %v", err)
	}
	if err := utils.IsValidPattern(excludes); err != nil {
		return nil, fmt.Errorf("invalid exclude patterns: %v", err)
	}

	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--diff-filter=ACM")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %v\nOutput: %s", err, string(out))
	}

	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	result := []string{}

	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		
		// Validate file path before processing
		if err := utils.ValidateFilePath(file); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping invalid file path %s: %v\n", file, err)
			continue
		}
		
		if matchesPattern(file, patterns) && !matchesPattern(file, excludes) {
			if _, err := os.Stat(file); err == nil {
				result = append(result, file)
			} else if !os.IsNotExist(err) {
				// Log unexpected stat errors but continue processing
				fmt.Fprintf(os.Stderr, "Warning: failed to stat file %s: %v\n", file, err)
			}
		}
	}

	return result, nil
}

func GetCIFiles(patterns, excludes string) ([]string, error) {
	// Validate input patterns
	if err := utils.IsValidPattern(patterns); err != nil {
		return nil, fmt.Errorf("invalid include patterns: %v", err)
	}
	if err := utils.IsValidPattern(excludes); err != nil {
		return nil, fmt.Errorf("invalid exclude patterns: %v", err)
	}

	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=ACM", "HEAD~1..HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %v\nOutput: %s", err, string(out))
	}

	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	result := []string{}

	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		
		// Validate file path before processing
		if err := utils.ValidateFilePath(file); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping invalid file path %s: %v\n", file, err)
			continue
		}
		
		// Check if file exists and matches patterns before including
		if matchesPattern(file, patterns) && !matchesPattern(file, excludes) {
			if _, err := os.Stat(file); err == nil {
				result = append(result, file)
			} else if !os.IsNotExist(err) {
				// Log unexpected stat errors but continue processing
				fmt.Fprintf(os.Stderr, "Warning: failed to stat file %s: %v\n", file, err)
			}
		}
	}

	return result, nil
}

func GetPRFiles(prRange, patterns, excludes string) ([]string, error) {
	// Validate input patterns
	if err := utils.IsValidPattern(patterns); err != nil {
		return nil, fmt.Errorf("invalid include patterns: %v", err)
	}
	if err := utils.IsValidPattern(excludes); err != nil {
		return nil, fmt.Errorf("invalid exclude patterns: %v", err)
	}

	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=ACM", prRange)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %v\nOutput: %s", err, string(out))
	}

	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	result := []string{}

	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		
		// Validate file path before processing
		if err := utils.ValidateFilePath(file); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping invalid file path %s: %v\n", file, err)
			continue
		}
		
		// Check if file exists and matches patterns before including
		if matchesPattern(file, patterns) && !matchesPattern(file, excludes) {
			if _, err := os.Stat(file); err == nil {
				result = append(result, file)
			} else if !os.IsNotExist(err) {
				// Log unexpected stat errors but continue processing
				fmt.Fprintf(os.Stderr, "Warning: failed to stat file %s: %v\n", file, err)
			}
		}
	}

	return result, nil
}

func GetPRDiff(prRange string) (string, error) {
	cmd := exec.Command("git", "diff", prRange)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %v\nOutput: %s", err, string(out))
	}
	result := string(out)
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("no diff found for range: %s", prRange)
	}
	return result, nil
}

func DetectBaseBranch() (string, error) {
	// First check if repository has any commits
	if err := exec.Command("git", "rev-parse", "--verify", "HEAD").Run(); err != nil {
		return "", fmt.Errorf("repository has no commits - cannot detect base branch for new repositories")
	}

	candidates := []string{"main", "master", "develop"}

	for _, branch := range candidates {
		// Check if branch exists locally
		cmd := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+branch)
		if err := cmd.Run(); err == nil {
			return branch, nil
		}

		// Check if branch exists remotely
		cmd = exec.Command("git", "rev-parse", "--verify", "refs/remotes/origin/"+branch)
		if err := cmd.Run(); err == nil {
			return branch, nil
		}
	}

	return "", fmt.Errorf(
		"no base branch found (checked: %s). Consider setting PR_BASE_BRANCH in config",
		strings.Join(candidates, ", "),
	)
}

func GetHooksDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-path", "hooks")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse failed: %v\nOutput: %s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

func matchesPattern(file, patterns string) bool {
	if patterns == "" || patterns == "*" {
		return true
	}

	patternList := strings.Split(patterns, ",")
	for _, p := range patternList {
		p = strings.TrimSpace(p)
		if p == "" || p == "*" {
			return true
		}
		// Use filepath.Match for proper glob pattern matching
		if matched, _ := filepath.Match(p, file); matched {
			return true
		}
	}
	return false
}

