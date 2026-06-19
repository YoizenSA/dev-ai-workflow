package missions

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// WorkspaceManager manages git worktrees for isolated feature execution.
type WorkspaceManager struct {
	repoPath string     // path to the real repository
	mergeMu  sync.Mutex // serializes integration branch checkouts

	// BaseRef is the git ref feature worktrees branch from. Empty = repo HEAD.
	BaseRef string

	// port allocation state (FASE 6)
	portMu       sync.Mutex
	portPool     map[int]bool // base ports in use
	nextPortBase int          // next candidate base port when pool is empty
}

// DefaultPortBase is the first base port ywai hands out to feature worktrees.
const DefaultPortBase = 30000

// PortBlockSize is the number of ports reserved per feature worktree.
const PortBlockSize = 20

// NewWorkspaceManager creates a WorkspaceManager for a given repo.
func NewWorkspaceManager(repoPath string) *WorkspaceManager {
	return &WorkspaceManager{
		repoPath:     repoPath,
		portPool:     make(map[int]bool),
		nextPortBase: DefaultPortBase,
	}
}

// CreateWorktree creates a git worktree at targetPath from branch.
// If branch does not exist it is created from BaseRef (or HEAD when empty).
func (wm *WorkspaceManager) CreateWorktree(targetPath, branch string) error {
	// Ensure target parent exists
	parent := filepath.Dir(targetPath)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("create worktree parent dir: %w", err)
	}

	// Determine the ref to branch from: explicit BaseRef, else HEAD.
	startPoint := wm.BaseRef
	if startPoint == "" {
		startPoint = "HEAD"
	}

	// Check if branch already exists
	checkCmd := exec.Command("git", "-C", wm.repoPath, "rev-parse", "--verify", branch)
	if err := checkCmd.Run(); err != nil {
		// Branch doesn't exist — create it from the chosen start point.
		createArgs := []string{"-C", wm.repoPath, "branch", branch}
		if startPoint != "HEAD" {
			createArgs = append(createArgs, startPoint)
		}
		createCmd := exec.Command("git", createArgs...)
		if out, err := createCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("create branch %s from %s: %w\noutput: %s", branch, startPoint, err, string(out))
		}
	}

	// Create worktree
	cmd := exec.Command("git", "-C", wm.repoPath, "worktree", "add", targetPath, branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		// If worktree already exists, that's fine
		if !strings.Contains(string(out), "already exists") {
			return fmt.Errorf("create worktree: %w\noutput: %s", err, string(out))
		}
	}
	return nil
}

// AllocatePortBlock reserves a contiguous block of PortBlockSize ports for a
// feature worktree and returns the base port. The returned release func frees
// the block back to the pool so concurrent features don't exhaust the range.
//
// Ports are handed out from DefaultPortBase upward. This lets parallel features
// each run their own services (app, db, etc.) without clashing on fixed ports.
func (wm *WorkspaceManager) AllocatePortBlock() (base int, release func()) {
	wm.portMu.Lock()
	defer wm.portMu.Unlock()

	if wm.portPool == nil {
		wm.portPool = make(map[int]bool)
	}

	// Find the next free base port, scanning upward in PortBlockSize strides.
	base = wm.nextPortBase
	for wm.portPool[base] {
		base += PortBlockSize
	}
	// Guard against overflow into privileged/ephemeral ranges.
	if base+PortBlockSize > 60000 {
		base = DefaultPortBase
		for wm.portPool[base] {
			base += PortBlockSize
		}
	}
	wm.portPool[base] = true
	if base >= wm.nextPortBase {
		wm.nextPortBase = base + PortBlockSize
	}

	return base, func() {
		wm.portMu.Lock()
		delete(wm.portPool, base)
		wm.portMu.Unlock()
	}
}

// ─── Git repo validation & introspection ───────────────────────────────────

// GitInfo describes the git state of a project's repo path. When IsGitRepo is
// false, the other fields are empty and callers should offer to initialize git.
type GitInfo struct {
	IsGitRepo     bool     `json:"isGitRepo"`
	CurrentBranch string   `json:"currentBranch,omitempty"`
	Branches      []string `json:"branches,omitempty"`
}

// ValidateGitRepo returns nil when repoPath is a valid git repository, or an
// error with a clear message when it is not. Used to fail early (before worktree
// creation) with actionable feedback instead of a cryptic git error mid-run.
func (wm *WorkspaceManager) ValidateGitRepo() error {
	cmd := exec.Command("git", "-C", wm.repoPath, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("path %q is not a git repository: run 'git init' first (or use a registered project)", wm.repoPath)
	}
	if !strings.Contains(strings.TrimSpace(string(out)), "true") {
		return fmt.Errorf("path %q is not a git repository: run 'git init' first", wm.repoPath)
	}
	return nil
}

// GitInfo inspects the repo and returns its current branch and local branches.
// On a non-git directory it returns IsGitRepo=false without erroring, so the UI
// can offer to initialize git. Errors only on unexpected git failures (e.g.
// corrupted repo), not on "no git here".
func (wm *WorkspaceManager) GitInfo() (GitInfo, error) {
	info := GitInfo{}

	// Detect git presence without erroring.
	detect := exec.Command("git", "-C", wm.repoPath, "rev-parse", "--is-inside-work-tree")
	if detectErr := detect.Run(); detectErr != nil {
		return info, nil // not a git repo — not an error condition
	}
	info.IsGitRepo = true

	// Current branch.
	if out, err := exec.Command("git", "-C", wm.repoPath, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		info.CurrentBranch = strings.TrimSpace(string(out))
	}

	// Local branches (strip the leading "* " marker from current + "  " from others).
	out, err := exec.Command("git", "-C", wm.repoPath, "branch", "--list").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
			line = strings.TrimSpace(line)
			if line != "" {
				info.Branches = append(info.Branches, line)
			}
		}
	}

	return info, nil
}

// InitGitRepo initializes a git repository at repoPath (git init + a baseline
// commit so HEAD exists and worktrees can branch from it). Idempotent: if the
// path is already a git repo, it is a no-op.
func (wm *WorkspaceManager) InitGitRepo() error {
	// Idempotent: skip if already a repo.
	if err := wm.ValidateGitRepo(); err == nil {
		return nil
	}

	mk := func(args ...string) error {
		cmd := exec.Command("git", args...)
		cmd.Dir = wm.repoPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
		}
		return nil
	}
	if err := mk("init", "-b", "main"); err != nil {
		return err
	}
	// Configure a default identity so the initial commit succeeds.
	_ = mk("config", "user.email", "ywai@local")
	_ = mk("config", "user.name", "ywai")
	// Seed a .gitkeep so there is something to commit (empty trees are allowed
	// by modern git, but a baseline file makes HEAD deterministic).
	keepPath := filepath.Join(wm.repoPath, ".gitkeep")
	if err := os.WriteFile(keepPath, []byte(""), 0644); err != nil {
		return fmt.Errorf("write .gitkeep: %w", err)
	}
	if err := mk("add", ".gitkeep"); err != nil {
		return err
	}
	if err := mk("commit", "-m", "ywai: initial commit"); err != nil {
		return err
	}
	return nil
}

// RemoveWorktree removes a git worktree and its branch.
func (wm *WorkspaceManager) RemoveWorktree(targetPath, branch string) error {
	// Remove worktree
	cmd := exec.Command("git", "-C", wm.repoPath, "worktree", "remove", targetPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Warning: removing worktree %s: %v\n%s", targetPath, err, string(out))
	}

	// Delete branch (force, as it might not be fully merged)
	delCmd := exec.Command("git", "-C", wm.repoPath, "branch", "-D", branch)
	if out, err := delCmd.CombinedOutput(); err != nil {
		log.Printf("Warning: deleting branch %s: %v\n%s", branch, err, string(out))
	}

	// Clean up directory if leftover
	_ = os.RemoveAll(targetPath)
	return nil
}

// worktreesDir returns the shared directory for all mission worktrees.
// Uses XDG_DATA_HOME / ~/.local/share/ywai/worktrees/ to avoid polluting the repo.
func (wm *WorkspaceManager) worktreesDir() string {
	// Prefer XDG_DATA_HOME, fall back to ~/.local/share
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			dataHome = filepath.Join(home, ".local", "share")
		} else {
			// Last resort: relative to repo (M1 suppression)
			return filepath.Join(wm.repoPath, ".ywai-worktrees")
		}
	}
	return filepath.Join(dataHome, "ywai", "worktrees")
}

// GetWorktreePath returns the worktree path for a given feature.
func (wm *WorkspaceManager) GetWorktreePath(missionID, featureID string) string {
	return filepath.Join(wm.worktreesDir(), missionID, featureID)
}

// BranchName returns the git branch name for a feature.
func (wm *WorkspaceManager) BranchName(missionID, featureID string) string {
	return fmt.Sprintf("ywai/%s/%s", missionID, featureID)
}

// IntegrationBranchName returns the integration branch for a mission.
func (wm *WorkspaceManager) IntegrationBranchName(missionID string) string {
	return fmt.Sprintf("ywai/%s/integration", missionID)
}

// EnsureIntegrationBranch creates the integration branch if it doesn't exist.
func (wm *WorkspaceManager) EnsureIntegrationBranch(missionID string) error {
	branch := wm.IntegrationBranchName(missionID)
	checkCmd := exec.Command("git", "-C", wm.repoPath, "rev-parse", "--verify", branch)
	if err := checkCmd.Run(); err != nil {
		// Create from current HEAD
		createCmd := exec.Command("git", "-C", wm.repoPath, "branch", branch)
		if out, err := createCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("create integration branch %s: %w\noutput: %s", branch, err, string(out))
		}
	}
	return nil
}

// MergeToIntegration merges the feature branch into the integration branch.
// Uses a per-merge dedicated worktree so the user's working tree is never touched.
// The mutex serializes concurrent calls — git disallows the same branch in two worktrees.
func (wm *WorkspaceManager) MergeToIntegration(missionID, featureID string) error {
	featureBranch := wm.BranchName(missionID, featureID)
	integrationBranch := wm.IntegrationBranchName(missionID)

	// C2b fix: serialize merges — git blocks concurrent checkout of the same branch.
	wm.mergeMu.Lock()
	defer wm.mergeMu.Unlock()

	if err := wm.EnsureIntegrationBranch(missionID); err != nil {
		return err
	}

	// M1 fix: integration worktree also lives outside the repo (XDG), unique per feature.
	integrationWorktree := wm.GetIntegrationWorktreePath(missionID, featureID)
	if err := wm.CreateWorktree(integrationWorktree, integrationBranch); err != nil {
		return fmt.Errorf("create integration worktree: %w", err)
	}
	defer func() {
		_ = exec.Command("git", "-C", wm.repoPath, "worktree", "remove", "--force", integrationWorktree).Run()
		_ = os.RemoveAll(integrationWorktree)
	}()

	cmd := exec.Command("git", "-C", integrationWorktree, "merge", featureBranch, "--no-ff", "-m",
		fmt.Sprintf("feat: merge %s into integration", featureID))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("merge %s to %s: %w\noutput: %s", featureBranch, integrationBranch, err, string(out))
	}
	return nil
}

// GetIntegrationWorktreePath returns the worktree path for a per-merge integration checkout.
// Includes featureID so concurrent merges never share a path.
func (wm *WorkspaceManager) GetIntegrationWorktreePath(missionID, featureID string) string {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			dataHome = filepath.Join(home, ".local", "share")
		} else {
			return filepath.Join(wm.repoPath, ".ywai-worktrees", missionID, "integration-"+featureID)
		}
	}
	return filepath.Join(dataHome, "ywai", "worktrees", missionID, "integration-"+featureID)
}

// CleanupMission removes all worktrees for a mission.
func (wm *WorkspaceManager) CleanupMission(missionID string, features []Feature) {
	for _, feat := range features {
		if feat.WorktreePath != "" {
			_ = wm.RemoveWorktree(feat.WorktreePath, feat.Branch)
		}
	}
}
