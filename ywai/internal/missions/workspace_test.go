package missions

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a git repo in a temp dir with an initial commit on a
// "main" branch, then tags a commit as "base" so tests can branch from a
// non-HEAD ref. Returns the repo path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")

	// Initial commit.
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("init\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run("add", ".")
	run("commit", "-m", "initial")
	run("tag", "base-tag")

	// A second commit on HEAD so HEAD != base-tag.
	if err := os.WriteFile(filepath.Join(repo, "extra.md"), []byte("extra\n"), 0644); err != nil {
		t.Fatalf("write extra: %v", err)
	}
	run("add", ".")
	run("commit", "-m", "second")

	return repo
}

// gitHeadShort returns the short SHA of a ref in the repo.
func gitRefShort(t *testing.T, repo, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "rev-parse", "--short", ref)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

// TestCreateWorktreeFromBaseRef verifies the WorkspaceManager branches a worktree
// from the configured base ref rather than HEAD when BaseRef is set.
func TestCreateWorktreeFromBaseRef(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorkspaceManager(repo)
	wm.BaseRef = "base-tag"

	wtPath := filepath.Join(t.TempDir(), "wt")
	branch := "ywai/test/base"
	if err := wm.CreateWorktree(wtPath, branch); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	// The worktree branch must point at base-tag's commit, not HEAD.
	worktreeHead := gitRefShort(t, wtPath, "HEAD")
	baseTag := gitRefShort(t, repo, "base-tag")
	headNow := gitRefShort(t, repo, "HEAD")

	if worktreeHead != baseTag {
		t.Errorf("worktree HEAD = %s, want base-tag %s", worktreeHead, baseTag)
	}
	if worktreeHead == headNow {
		t.Errorf("worktree branched from HEAD (%s); expected base-tag", worktreeHead)
	}
}

// TestCreateWorktreeDefaultsToHEAD verifies the default (BaseRef empty) branches
// from HEAD, preserving existing behaviour.
func TestCreateWorktreeDefaultsToHEAD(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorkspaceManager(repo)

	wtPath := filepath.Join(t.TempDir(), "wt")
	branch := "ywai/test/head"
	if err := wm.CreateWorktree(wtPath, branch); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	worktreeHead := gitRefShort(t, wtPath, "HEAD")
	headNow := gitRefShort(t, repo, "HEAD")
	if worktreeHead != headNow {
		t.Errorf("default worktree HEAD = %s, want HEAD %s", worktreeHead, headNow)
	}
}

// ─── Port allocation (AllocatePortBlock) ────────────────────────────────────

// TestAllocatePortBlockReturnsContiguousRange verifies AllocatePortBlock returns
// a base port and that repeated calls advance the range so features don't clash.
func TestAllocatePortBlockReturnsContiguousRange(t *testing.T) {
	wm := NewWorkspaceManager(t.TempDir())

	base1, release1 := wm.AllocatePortBlock()
	if base1 <= 0 {
		t.Fatalf("expected positive base port, got %d", base1)
	}
	release1()

	base2, release2 := wm.AllocatePortBlock()
	defer release2()

	if base1 == base2 {
		t.Errorf("two allocations returned the same base %d; features would clash", base1)
	}
}

// TestAllocatePortBlockReleaseAllowsReuse verifies releasing a block frees the
// slot so it can be reallocated (bounded concurrency).
func TestAllocatePortBlockReleaseAllowsReuse(t *testing.T) {
	wm := NewWorkspaceManager(t.TempDir())

	// Reserve all slots in a tiny pool then release; the next alloc must succeed.
	for i := 0; i < 50; i++ {
		b, release := wm.AllocatePortBlock()
		if b <= 0 {
			t.Fatalf("alloc %d returned non-positive base", i)
		}
		release()
	}
}

// ─── Git repo validation & introspection ───────────────────────────────────

// TestValidateGitRepoRejectsNonGitDir verifies a plain directory (no .git) is
// rejected with a clear error.
func TestValidateGitRepoRejectsNonGitDir(t *testing.T) {
	plainDir := t.TempDir() // no git init
	wm := NewWorkspaceManager(plainDir)

	if err := wm.ValidateGitRepo(); err == nil {
		t.Fatal("expected error validating a non-git directory, got nil")
	}
}

// TestValidateGitRepoAcceptsGitDir verifies a real git repo passes validation.
func TestValidateGitRepoAcceptsGitDir(t *testing.T) {
	repo := initTestRepo(t) // has git init + commits
	wm := NewWorkspaceManager(repo)

	if err := wm.ValidateGitRepo(); err != nil {
		t.Fatalf("expected valid git repo, got error: %v", err)
	}
}

// TestGitInfoReturnsCurrentBranchAndBranches verifies GitInfo reports the
// current branch and the list of local branches.
func TestGitInfoReturnsCurrentBranchAndBranches(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorkspaceManager(repo)

	// Add a second branch so the list has >1 entry.
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("branch", "feature-x")

	info, err := wm.GitInfo()
	if err != nil {
		t.Fatalf("GitInfo: %v", err)
	}
	if info.CurrentBranch == "" {
		t.Error("expected non-empty current branch")
	}
	if info.CurrentBranch != "main" {
		t.Errorf("expected current branch 'main', got %q", info.CurrentBranch)
	}
	found := false
	for _, b := range info.Branches {
		if b == "feature-x" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected branches to include 'feature-x', got %v", info.Branches)
	}
	if !info.IsGitRepo {
		t.Error("expected IsGitRepo=true")
	}
}

// TestGitInfoOnNonGitRepo verifies GitInfo on a non-git dir returns IsGitRepo=false
// without erroring (so the UI can offer git init).
func TestGitInfoOnNonGitRepo(t *testing.T) {
	plainDir := t.TempDir()
	wm := NewWorkspaceManager(plainDir)

	info, err := wm.GitInfo()
	if err != nil {
		t.Fatalf("GitInfo should not error on non-git dir, got: %v", err)
	}
	if info.IsGitRepo {
		t.Error("expected IsGitRepo=false for plain dir")
	}
	if info.CurrentBranch != "" {
		t.Errorf("expected empty current branch, got %q", info.CurrentBranch)
	}
}

// TestInitGitRepo verifies InitGitRepo initializes a repo so ValidateGitRepo passes.
func TestInitGitRepo(t *testing.T) {
	plainDir := t.TempDir()
	wm := NewWorkspaceManager(plainDir)

	if err := wm.InitGitRepo(); err != nil {
		t.Fatalf("InitGitRepo: %v", err)
	}
	if err := wm.ValidateGitRepo(); err != nil {
		t.Errorf("expected valid git repo after init, got: %v", err)
	}
}
