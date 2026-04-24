package review

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	MaxFileSizeBytes = 10 * 1024 * 1024 // 10MB limit per file
)

func BuildPrompt(rulesFile, commitMsg string, files []string, useStaged bool) string {
	rules := ""
	if data, err := os.ReadFile(rulesFile); err == nil {
		rules = string(data)
	}

	var b strings.Builder

	b.WriteString(
		"You are a code reviewer. Analyze the files below and validate " +
			"they comply with the coding standards provided.\n\n",
	)
	b.WriteString("=== CODING STANDARDS ===\n")
	b.WriteString(rules)
	b.WriteString("\n=== END CODING STANDARDS ===\n")

	if commitMsg != "" {
		b.WriteString("\n=== COMMIT MESSAGE ===\n")
		b.WriteString(commitMsg)
		b.WriteString("\n=== END COMMIT MESSAGE ===\n")
	}

	b.WriteString("\n=== FILES TO REVIEW ===\n")

	for _, file := range files {
		b.WriteString(fmt.Sprintf("\n--- FILE: %s ---\n", file))
		
		// Validate file path to prevent directory traversal
		if filepath.IsAbs(file) || strings.Contains(file, "..") {
			b.WriteString(fmt.Sprintf("(invalid file path: %s)\n", file))
			continue
		}
		
		var content []byte
		var err error

		if useStaged {
			cmd := exec.Command("git", "show", ":"+file)
			content, err = cmd.CombinedOutput()
			if err != nil {
				b.WriteString(fmt.Sprintf("(could not read staged version: %v)\n", err))
				continue
			}
		} else {
			// Check file size before reading to prevent memory exhaustion
			if info, err := os.Stat(file); err == nil {
				if info.Size() > MaxFileSizeBytes {
					b.WriteString(fmt.Sprintf("(file too large: %.2fMB, exceeds limit of %.2fMB)\n", 
						float64(info.Size())/1024/1024, float64(MaxFileSizeBytes)/1024/1024))
					continue
				}
			}
			
			content, err = os.ReadFile(file)
		}

		if err != nil {
			b.WriteString(fmt.Sprintf("(could not read: %v)\n", err))
		} else {
			b.WriteString(string(content))
		}
	}

	b.WriteString("\n=== END FILES ===\n\n")
	b.WriteString("**IMPORTANT: Your response MUST include one of these lines near the beginning:**\n")
	b.WriteString("STATUS: PASSED\n")
	b.WriteString("STATUS: FAILED\n\n")
	b.WriteString("**If FAILED:** List each violation with:\n")
	b.WriteString("- File name\n")
	b.WriteString("- Line number (if applicable)\n")
	b.WriteString("- Rule violated\n")
	b.WriteString("- Description of the issue\n\n")
	b.WriteString("**If PASSED:** Confirm all files comply with the coding standards.\n\n")
	b.WriteString("**Begin with STATUS:**\n")

	return b.String()
}

// PRPromptConfig holds configuration for PR prompt building
type PRPromptConfig struct {
	RulesFile  string
	PRDiff     string
	Files      []string
	DiffOnly   bool
	BaseBranch string
}

func BuildPRPrompt(cfg PRPromptConfig) string {
	rules := ""
	if data, err := os.ReadFile(cfg.RulesFile); err == nil {
		rules = string(data)
	}

	var b strings.Builder

	b.WriteString(fmt.Sprintf(
		"You are a code reviewer analyzing a pull request against the %s branch.\n\n",
		cfg.BaseBranch,
	))
	b.WriteString("=== CODING STANDARDS ===\n")
	b.WriteString(rules)
	b.WriteString("\n=== END CODING STANDARDS ===\n")

	b.WriteString("\n=== PR CONTEXT ===\n")
	b.WriteString(fmt.Sprintf(
		"This is a pull request review. The following files were changed "+
			"in this PR (compared to %s).\n",
		cfg.BaseBranch,
	))
	b.WriteString("=== END PR CONTEXT ===\n")

	if cfg.DiffOnly && cfg.PRDiff != "" {
		b.WriteString("\n=== PR DIFF ===\n")
		b.WriteString(cfg.PRDiff)
		b.WriteString("\n=== END PR DIFF ===\n")
		b.WriteString("\n=== FILES (complete content for context) ===\n")
	} else {
		b.WriteString("\n=== FILES TO REVIEW ===\n")
	}

	for _, file := range cfg.Files {
		b.WriteString(fmt.Sprintf("\n--- FILE: %s ---\n", file))

		// Validate file path to prevent directory traversal
		if filepath.IsAbs(file) || strings.Contains(file, "..") {
			b.WriteString(fmt.Sprintf("(invalid file path: %s)\n", file))
			continue
		}

		// Check file size before reading to prevent memory exhaustion
		if info, err := os.Stat(file); err == nil {
			if info.Size() > MaxFileSizeBytes {
				b.WriteString(fmt.Sprintf("(file too large: %.2fMB, exceeds limit of %.2fMB)\n",
					float64(info.Size())/1024/1024, float64(MaxFileSizeBytes)/1024/1024))
				continue
			}
		}

		if content, err := os.ReadFile(file); err == nil {
			b.WriteString(string(content))
		} else {
			b.WriteString(fmt.Sprintf("(could not read file: %v)\n", err))
		}
	}

	b.WriteString("\n=== END FILES ===\n\n")
	b.WriteString("**IMPORTANT: Your response MUST include one of these lines near the beginning:**\n")
	b.WriteString("STATUS: PASSED\n")
	b.WriteString("STATUS: FAILED\n\n")
	b.WriteString("**If FAILED:** List each violation with:\n")
	b.WriteString("- File name\n")
	b.WriteString("- Line number (if applicable)\n")
	b.WriteString("- Rule violated\n")
	b.WriteString("- Description of the issue\n\n")
	b.WriteString("**If PASSED:** Confirm all files comply with the coding standards.\n\n")
	b.WriteString("**Begin with STATUS:**\n")

	return b.String()
}

type ReviewResult int

const (
	ResultUnknown ReviewResult = iota
	ResultPassed
	ResultFailed
)

const (
	StatusCheckLines = 15
)

func ParseResult(output string) ReviewResult {
	lines := strings.Split(output, "\n")
	checkLines := StatusCheckLines
	if len(lines) < StatusCheckLines {
		checkLines = len(lines)
	}

	for _, line := range lines[:checkLines] {
		line := strings.ToUpper(line)
		if strings.Contains(line, "STATUS: PASSED") || strings.Contains(line, "**STATUS: PASSED**") {
			return ResultPassed
		}
		if strings.Contains(line, "STATUS: FAILED") || strings.Contains(line, "**STATUS: FAILED**") {
			return ResultFailed
		}
	}

	return ResultUnknown
}
