package agents

import (
	"os"
	"path/filepath"
	"testing"
)

// The pre-v2 skill registry was deleted from the source, but that only stops
// new installs from creating it. Hosts that already have it keep feeding a
// stale index to their agents until an install or update sweeps it, so the
// sweep is what actually retires the mechanism.
func TestRemoveRetiredConfigArtifacts(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, body string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	mustWrite(".atl/skill-registry.md", "# Skill Registry\n")
	mustWrite(".atl/.skill-registry.cache.json", "{}\n")
	mustWrite("skills/skill-registry/SKILL.md", "retired\n")
	// A skill the user owns must survive the sweep.
	mustWrite("skills/my-own-skill/SKILL.md", "keep me\n")

	removed := RemoveRetiredConfigArtifacts(dir)
	if len(removed) != 2 {
		t.Fatalf("removed %v, want both retired paths", removed)
	}
	for _, rel := range []string{".atl", filepath.Join("skills", "skill-registry")} {
		if _, err := os.Stat(filepath.Join(dir, rel)); !os.IsNotExist(err) {
			t.Errorf("%s survived the sweep (err=%v)", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", "my-own-skill", "SKILL.md")); err != nil {
		t.Errorf("a user's own skill must not be swept: %v", err)
	}

	// Idempotent: the sweep runs on every install, and a clean host reports
	// nothing rather than warning about paths that are already gone.
	if again := RemoveRetiredConfigArtifacts(dir); len(again) != 0 {
		t.Errorf("second sweep removed %v, want nothing", again)
	}
}

func TestSweepRetiredSkillRegistry_RemovesAtlAndSkill(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	mustWrite := func(root, rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	mustWrite(home, filepath.Join(".claude", "skills", "skill-registry", "SKILL.md"), "write .atl\n")
	mustWrite(home, filepath.Join(".agents", "skills", "skill-registry", "SKILL.md"), "write .atl\n")
	mustWrite(home, filepath.Join(".claude", "skills", "ywai", "SKILL.md"), "keep\n")
	mustWrite(repo, filepath.Join(".atl", "skill-registry.md"), "# index\n")
	mustWrite(repo, filepath.Join("ywai", "internal", "control", "web", ".atl", "cache.json"), "{}\n")
	mustWrite(repo, filepath.Join("src", "keep.txt"), "ok\n")

	removed := SweepRetiredSkillRegistry(home, repo)
	if len(removed) < 4 {
		t.Fatalf("removed %v, want .atl (2) + skill-registry (2)", removed)
	}
	gone := []string{
		filepath.Join(home, ".claude", "skills", "skill-registry"),
		filepath.Join(home, ".agents", "skills", "skill-registry"),
		filepath.Join(repo, ".atl"),
		filepath.Join(repo, "ywai", "internal", "control", "web", ".atl"),
	}
	for _, p := range gone {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s survived (err=%v)", p, err)
		}
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "ywai", "SKILL.md")); err != nil {
		t.Errorf("unrelated skill must survive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "src", "keep.txt")); err != nil {
		t.Errorf("repo files must survive: %v", err)
	}
	if again := SweepRetiredSkillRegistry(home, repo); len(again) != 0 {
		t.Errorf("second sweep removed %v, want nothing", again)
	}
}
