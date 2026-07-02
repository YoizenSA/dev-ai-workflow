// Package git provides lightweight git status information for the kanban UI.
package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// GitStatus holds the working-tree state of a repository.
type GitStatus struct {
	Branch         string `json:"branch"`
	Ahead          int    `json:"ahead"`
	Behind         int    `json:"behind"`
	ChangedFiles   int    `json:"changed_files"`
	UntrackedFiles int    `json:"untracked_files"`
	Dirty          bool   `json:"dirty"`
	RemoteURL      string `json:"remote_url,omitempty"`
}

var (
	commandTimeout = 5 * time.Second
	errNotGitRepo  = errors.New("not a git repository")
)

// GetStatus runs a handful of fast git commands and returns the repo status.
func GetStatus(repoDir string) (*GitStatus, error) {
	st := &GitStatus{}

	branch, err := gitOutput(repoDir, "git", "branch", "--show-current")
	if err != nil {
		return nil, errNotGitRepo
	}
	if branch == "" {
		// Detached HEAD — show the short SHA.
		branch, _ = gitOutput(repoDir, "git", "rev-parse", "--short", "HEAD")
		if branch == "" {
			branch = "HEAD"
		}
	}
	st.Branch = branch

	// Porcelain status: count changed (M/D) and untracked (?) lines.
	porcelain, _ := gitOutput(repoDir, "git", "status", "--porcelain")
	if porcelain != "" {
		lines := strings.Split(strings.TrimRight(porcelain, "\n"), "\n")
		for _, l := range lines {
			if strings.HasPrefix(l, "??") {
				st.UntrackedFiles++
			} else {
				st.ChangedFiles++
			}
		}
		st.Dirty = st.ChangedFiles > 0 || st.UntrackedFiles > 0
	}

	// Ahead/behind relative to upstream.
	// ponytail: rev-list --left-right --count is the cheapest way; skip if no upstream.
	upstream, err := gitOutput(repoDir, "git", "rev-parse", "--abbrev-ref", "@{upstream}")
	if err == nil && upstream != "" {
		counts, err := gitOutput(repoDir, "git", "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
		if err == nil {
			parts := strings.Fields(counts)
			if len(parts) == 2 {
				st.Ahead, _ = strconv.Atoi(parts[0])
				st.Behind, _ = strconv.Atoi(parts[1])
			}
		}
	}

	// Remote URL (best-effort).
	remote, _ := gitOutput(repoDir, "git", "remote", "get-url", "origin")
	st.RemoteURL = strings.TrimSpace(remote)

	return st, nil
}

// gitOutput runs a command with a timeout and returns trimmed stdout.
func gitOutput(dir, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return strings.TrimSpace(string(out)), nil
}
