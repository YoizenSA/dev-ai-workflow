package skills

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
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

func TestLinkToSkipsNonYwaiExtraSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	repo := t.TempDir()
	t.Cleanup(func() {
		config.SetRepoRoot("")
		config.ResetConfig()
	})
	config.SetRepoRoot(repo)
	config.ResetConfig()

	repoSkillsDir := filepath.Join(repo, "skills")
	writeSkill(t, repoSkillsDir, "react-19", true)
	writeSkill(t, repoSkillsDir, "sdd-init", false)
	writeSkill(t, repoSkillsDir, "skill-creator", false)
	writeSkill(t, repoSkillsDir, "judgment-day", false)

	agentSkillsDir := filepath.Join(t.TempDir(), "agent-skills")
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		t.Fatalf("create agent skills dir: %v", err)
	}

	if err := LinkTo(agentSkillsDir); err != nil {
		t.Fatalf("LinkTo() error = %v", err)
	}

	if _, err := os.Lstat(filepath.Join(agentSkillsDir, "react-19")); err != nil {
		t.Fatalf("react-19 should be copied: %v", err)
	}
	if IsLinkOrJunction(filepath.Join(agentSkillsDir, "react-19")) {
		t.Fatal("react-19 should be a real directory, not a link/junction")
	}
	for _, name := range []string{"sdd-init", "skill-creator", "judgment-day"} {
		if _, err := os.Lstat(filepath.Join(agentSkillsDir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s should not be linked by ywai; err=%v", name, err)
		}
	}
}

func TestListAvailableSkipsNonYwaiExtraSkills(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	repo := t.TempDir()
	t.Cleanup(func() {
		config.SetRepoRoot("")
		config.ResetConfig()
	})
	config.SetRepoRoot(repo)
	config.ResetConfig()

	repoSkillsDir := filepath.Join(repo, "skills")
	writeSkill(t, repoSkillsDir, "react-19", true)
	writeSkill(t, repoSkillsDir, "yz-ui", true)
	writeSkill(t, repoSkillsDir, "sdd-init", false)
	writeSkill(t, repoSkillsDir, "skill-creator", false)
	writeSkill(t, repoSkillsDir, "judgment-day", false)

	got, err := ListAvailable()
	if err != nil {
		t.Fatalf("ListAvailable() error = %v", err)
	}

	if !slices.Contains(got, "react-19") {
		t.Fatalf("ListAvailable() = %v, want react-19", got)
	}
	if !slices.Contains(got, "yz-ui") {
		t.Fatalf("ListAvailable() = %v, want yz-ui", got)
	}
	for _, name := range []string{"sdd-init", "skill-creator", "judgment-day"} {
		if slices.Contains(got, name) {
			t.Fatalf("ListAvailable() = %v, must not include non-ywai extra %s", got, name)
		}
	}
}

func TestRemoveStaleYwaiSkillLinksRemovesOnlyYwaiSourceLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses symlinks; junction behavior is covered by production code path")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	repo := t.TempDir()
	t.Cleanup(func() {
		config.SetRepoRoot("")
		config.ResetConfig()
	})
	config.SetRepoRoot(repo)
	config.ResetConfig()

	repoSkillsDir := filepath.Join(repo, "skills")
	writeSkill(t, repoSkillsDir, "react-19", true)
	writeSkill(t, repoSkillsDir, "sdd-init", false)

	agentSkillsDir := filepath.Join(t.TempDir(), "agent-skills")
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		t.Fatalf("create agent skills dir: %v", err)
	}

	if err := os.Symlink(filepath.Join(repoSkillsDir, "sdd-init"), filepath.Join(agentSkillsDir, "sdd-init")); err != nil {
		t.Fatalf("create sdd-init symlink: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(agentSkillsDir, "judgment-day"), 0o755); err != nil {
		t.Fatalf("create real judgment-day dir: %v", err)
	}
	externalTarget := filepath.Join(t.TempDir(), "external-skill")
	if err := os.MkdirAll(externalTarget, 0o755); err != nil {
		t.Fatalf("create external target: %v", err)
	}
	if err := os.Symlink(externalTarget, filepath.Join(agentSkillsDir, "external-review")); err != nil {
		t.Fatalf("create external symlink: %v", err)
	}

	removed, err := RemoveStaleYwaiSkillLinks(agentSkillsDir)
	if err != nil {
		t.Fatalf("RemoveStaleYwaiSkillLinks() error = %v", err)
	}
	if !slices.Equal(removed, []string{"sdd-init"}) {
		t.Fatalf("removed = %v, want [sdd-init]", removed)
	}
	if _, err := os.Lstat(filepath.Join(agentSkillsDir, "sdd-init")); !os.IsNotExist(err) {
		t.Fatalf("sdd-init symlink should be removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(agentSkillsDir, "judgment-day")); err != nil {
		t.Fatalf("real judgment-day dir should remain: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(agentSkillsDir, "external-review")); err != nil {
		t.Fatalf("external symlink should remain: %v", err)
	}
}

func writeSkill(t *testing.T, root, name string, ywaiExtra bool) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create skill dir %s: %v", name, err)
	}
	content := `---
name: ` + name + `
---

# ` + name + `
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill %s: %v", name, err)
	}
	if ywaiExtra {
		if err := os.WriteFile(filepath.Join(dir, extraSkillMarkerFile), []byte("managed-by: ywai\n"), 0o644); err != nil {
			t.Fatalf("write marker %s: %v", name, err)
		}
	}
}
