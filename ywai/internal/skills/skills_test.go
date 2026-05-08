package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func TestSkillsSourceDirPrefersSeededHomeCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	repo := t.TempDir()
	t.Cleanup(func() { config.SetRepoRoot("") })
	config.SetRepoRoot(repo)

	repoSkillDir := filepath.Join(repo, "skills", "react-19")
	if err := os.MkdirAll(repoSkillDir, 0o755); err != nil {
		t.Fatalf("create repo skill dir: %v", err)
	}

	dataSkillDir := filepath.Join(config.DataSkillsDir(), "react-19")
	if err := os.MkdirAll(dataSkillDir, 0o755); err != nil {
		t.Fatalf("create data skill dir: %v", err)
	}

	if got, want := skillsSourceDir(), config.DataSkillsDir(); got != want {
		t.Fatalf("skillsSourceDir() = %q, want seeded data dir %q", got, want)
	}
}

func TestSkillsSourceDirFallsBackToRepoWhenCacheEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	repo := t.TempDir()
	t.Cleanup(func() { config.SetRepoRoot("") })
	config.SetRepoRoot(repo)

	repoSkillsDir := filepath.Join(repo, "skills")
	if err := os.MkdirAll(filepath.Join(repoSkillsDir, "react-19"), 0o755); err != nil {
		t.Fatalf("create repo skill dir: %v", err)
	}

	if got, want := skillsSourceDir(), repoSkillsDir; got != want {
		t.Fatalf("skillsSourceDir() = %q, want repo dir %q", got, want)
	}
}
