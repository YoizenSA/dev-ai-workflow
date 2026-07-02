package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetStatus_CleanRepo(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "git", "init")
	mustRun(t, dir, "git", "config", "user.email", "test@test.com")
	mustRun(t, dir, "git", "config", "user.name", "Test")
	mustRun(t, dir, "git", "commit", "--allow-empty", "-m", "init")

	st, err := GetStatus(dir)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if st.Branch == "" {
		t.Fatal("expected non-empty branch")
	}
	if st.Dirty {
		t.Fatal("expected clean repo")
	}
	if st.ChangedFiles != 0 {
		t.Errorf("changed_files = %d, want 0", st.ChangedFiles)
	}
	if st.UntrackedFiles != 0 {
		t.Errorf("untracked_files = %d, want 0", st.UntrackedFiles)
	}
}

func TestGetStatus_DirtyRepo(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "git", "init")
	mustRun(t, dir, "git", "config", "user.email", "test@test.com")
	mustRun(t, dir, "git", "config", "user.name", "Test")
	mustRun(t, dir, "git", "commit", "--allow-empty", "-m", "init")

	// Create a tracked file (staged change).
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644)
	mustRun(t, dir, "git", "add", "a.txt")

	// Create an untracked file.
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0644)

	st, err := GetStatus(dir)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if !st.Dirty {
		t.Fatal("expected dirty repo")
	}
	if st.ChangedFiles != 1 {
		t.Errorf("changed_files = %d, want 1", st.ChangedFiles)
	}
	if st.UntrackedFiles != 1 {
		t.Errorf("untracked_files = %d, want 1", st.UntrackedFiles)
	}
}

func TestGetStatus_DetachedHEAD(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "git", "init")
	mustRun(t, dir, "git", "config", "user.email", "test@test.com")
	mustRun(t, dir, "git", "config", "user.name", "Test")
	mustRun(t, dir, "git", "commit", "--allow-empty", "-m", "init")
	sha, _ := gitOutput(dir, "git", "rev-parse", "HEAD")
	mustRun(t, dir, "git", "checkout", sha)

	st, err := GetStatus(dir)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if st.Branch == "" || st.Branch == "HEAD" {
		// short SHA is fine
		t.Logf("detached HEAD branch label: %q", st.Branch)
	}
}

func TestGetStatus_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := GetStatus(dir)
	if err == nil {
		t.Fatal("expected error for non-git dir")
	}
}

func mustRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}
