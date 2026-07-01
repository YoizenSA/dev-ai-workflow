package skills

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func TestSkillsSourceDirPrefersRepoWhenAvailable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	repo := t.TempDir()
	t.Cleanup(func() { config.SetRepoRoot("") })
	config.SetRepoRoot(repo)

	repoSkillDir := filepath.Join(repo, "skills", "yz-ui")
	if err := os.MkdirAll(repoSkillDir, 0o755); err != nil {
		t.Fatalf("create repo skill dir: %v", err)
	}

	dataSkillDir := filepath.Join(config.DataSkillsDir(), "yz-ui")
	if err := os.MkdirAll(dataSkillDir, 0o755); err != nil {
		t.Fatalf("create data skill dir: %v", err)
	}

	if got, want := skillsSourceDir(), filepath.Join(repo, "skills"); got != want {
		t.Fatalf("skillsSourceDir() = %q, want repo skills dir %q", got, want)
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
	if err := os.MkdirAll(filepath.Join(repoSkillsDir, "yz-ui"), 0o755); err != nil {
		t.Fatalf("create repo skill dir: %v", err)
	}

	if got, want := skillsSourceDir(), repoSkillsDir; got != want {
		t.Fatalf("skillsSourceDir() = %q, want repo dir %q", got, want)
	}
}

func TestCopyToSkipsNonYwaiExtraSkills(t *testing.T) {
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
	writeSkill(t, repoSkillsDir, "yz-ui", true)
	writeSkill(t, repoSkillsDir, "sdd-init", false)
	writeSkill(t, repoSkillsDir, "skill-creator", false)
	writeSkill(t, repoSkillsDir, "judgment-day", false)

	agentSkillsDir := filepath.Join(t.TempDir(), "agent-skills")
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		t.Fatalf("create agent skills dir: %v", err)
	}

	if err := CopyTo(agentSkillsDir); err != nil {
		t.Fatalf("CopyTo() error = %v", err)
	}

	if _, err := os.Lstat(filepath.Join(agentSkillsDir, "yz-ui")); err != nil {
		t.Fatalf("yz-ui should be copied: %v", err)
	}
	if IsLinkOrJunction(filepath.Join(agentSkillsDir, "yz-ui")) {
		t.Fatal("yz-ui should be a real directory, not a link/junction")
	}
	for _, name := range []string{"sdd-init", "skill-creator", "judgment-day"} {
		if _, err := os.Lstat(filepath.Join(agentSkillsDir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s should not be copied by ywai; err=%v", name, err)
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
	writeSkill(t, repoSkillsDir, "yz-ui", true)
	writeSkill(t, repoSkillsDir, "sdd-init", false)
	writeSkill(t, repoSkillsDir, "skill-creator", false)
	writeSkill(t, repoSkillsDir, "judgment-day", false)

	got, err := ListAvailable()
	if err != nil {
		t.Fatalf("ListAvailable() error = %v", err)
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
	writeSkill(t, repoSkillsDir, "yz-ui", true)
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

// TestRemoveSddAssets verifies that RemoveSddAssets deletes every SDD-managed
// entry (skills/sdd-*, skills/_shared/sdd-*.md, commands/sdd-*.md,
// agents/sdd-*.md) while preserving unrelated skills (judgment-day, ywai
// extra skills) and non-SDD shared files.
func TestRemoveSddAssets(t *testing.T) {
	// Layout: <configDir>/skills, <configDir>/commands, <configDir>/agents
	configDir := t.TempDir()
	skillsDir := filepath.Join(configDir, "skills")
	commandsDir := filepath.Join(configDir, "commands")
	agentsDir := filepath.Join(configDir, "agents")
	sharedDir := filepath.Join(skillsDir, "_shared")

	for _, dir := range []string{skillsDir, commandsDir, agentsDir, sharedDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	// SDD assets that must be removed.
	sddDirs := []string{
		filepath.Join(skillsDir, "sdd-init"), // skill dir
		filepath.Join(skillsDir, "sdd-verify"),
	}
	sddFiles := []string{
		filepath.Join(sharedDir, "sdd-phase-common.md"),
		filepath.Join(sharedDir, "sdd-status-contract.md"),
		filepath.Join(commandsDir, "sdd-new.md"),
		filepath.Join(commandsDir, "sdd-status.md"),
		filepath.Join(agentsDir, "sdd-spec.md"),
	}
	for _, p := range sddDirs {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
	}
	for _, p := range sddFiles {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	mustRemove := append(append([]string{}, sddDirs...), sddFiles...)

	// Non-SDD assets that must be preserved.
	mustKeep := []string{
		filepath.Join(skillsDir, "judgment-day"),
		filepath.Join(skillsDir, "angular"),
		filepath.Join(sharedDir, "engram-convention.md"),
		filepath.Join(sharedDir, "SKILL.md"),
		filepath.Join(commandsDir, "skill-creator.md"),
		filepath.Join(agentsDir, "my-custom-agent.md"),
	}
	for _, p := range mustKeep {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatalf("create keep-dir %s: %v", p, err)
		}
	}

	removed, err := RemoveSddAssets(skillsDir)
	if err != nil {
		t.Fatalf("RemoveSddAssets: %v", err)
	}

	// Every SDD entry should be gone.
	for _, p := range mustRemove {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed, still exists", p)
		}
	}
	// Every non-SDD entry should survive.
	for _, p := range mustKeep {
		if _, err := os.Lstat(p); err != nil {
			t.Errorf("expected %s to be preserved, got error: %v", p, err)
		}
	}

	// All reported removed paths must start with "sdd".
	for _, r := range removed {
		base := filepath.Base(r)
		if !strings.HasPrefix(base, "sdd-") {
			t.Errorf("removed entry %q is not an SDD asset", r)
		}
	}
	wantCount := len(mustRemove)
	if len(removed) != wantCount {
		t.Errorf("removed count = %d, want %d (got %v)", len(removed), wantCount, removed)
	}

	// CountSddAssets should now report zero.
	if got := CountSddAssets(skillsDir); got != 0 {
		t.Errorf("CountSddAssets after removal = %d, want 0", got)
	}
}

// TestCountSddAssetsMatchesRemoval verifies the count equals the number of
// entries RemoveSddAssets will delete.
func TestCountSddAssetsMatchesRemoval(t *testing.T) {
	configDir := t.TempDir()
	skillsDir := filepath.Join(configDir, "skills")
	commandsDir := filepath.Join(configDir, "commands")
	agentsDir := filepath.Join(configDir, "agents")
	sharedDir := filepath.Join(skillsDir, "_shared")

	for _, dir := range []string{skillsDir, commandsDir, agentsDir, sharedDir} {
		os.MkdirAll(dir, 0o755)
	}
	os.MkdirAll(filepath.Join(skillsDir, "sdd-init"), 0o755)
	os.MkdirAll(filepath.Join(skillsDir, "sdd-spec"), 0o755)
	os.WriteFile(filepath.Join(sharedDir, "sdd-phase-common.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(commandsDir, "sdd-apply.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(agentsDir, "sdd-design.md"), []byte("x"), 0o644)
	// Non-SDD entries that must not be counted.
	os.MkdirAll(filepath.Join(skillsDir, "judgment-day"), 0o755)
	os.WriteFile(filepath.Join(sharedDir, "SKILL.md"), []byte("x"), 0o644)

	count := CountSddAssets(skillsDir)
	removed, err := RemoveSddAssets(skillsDir)
	if err != nil {
		t.Fatalf("RemoveSddAssets: %v", err)
	}
	if count != len(removed) {
		t.Errorf("CountSddAssets = %d, but RemoveSddAssets removed %d", count, len(removed))
	}
}
