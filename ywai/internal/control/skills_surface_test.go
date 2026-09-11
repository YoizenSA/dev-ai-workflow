package control

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSkill(t *testing.T, dir, name, body string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func findSkill(skills []surfaceSkill, name string) *surfaceSkill {
	for i := range skills {
		if skills[i].Name == name {
			return &skills[i]
		}
	}
	return nil
}

func TestScanSkillSurface_UniqueAndMissing(t *testing.T) {
	root := t.TempDir()
	global := filepath.Join(root, "g")
	writeSkill(t, global, "alpha", "# alpha")
	writeSkill(t, global, "beta", "# beta")

	surface := scanSkillSurface(
		map[string]string{"global-opencode": global, "global-claude": filepath.Join(root, "nope")},
		map[string][]string{},
	)

	if len(surface.Locations) != 2 {
		t.Fatalf("locations = %d, want 2", len(surface.Locations))
	}
	alpha := findSkill(surface.Skills, "alpha")
	if alpha == nil || alpha.Status != "unique" || len(alpha.Entries) != 1 {
		t.Fatalf("alpha = %+v, want unique with 1 entry", alpha)
	}
	if len(alpha.Entries[0].Hash) != 12 {
		t.Fatalf("hash = %q, want 12 hex chars", alpha.Entries[0].Hash)
	}
}

func TestScanSkillSurface_Shadowed(t *testing.T) {
	root := t.TempDir()
	g1 := filepath.Join(root, "g1")
	g2 := filepath.Join(root, "g2")
	writeSkill(t, g1, "same", "# version one")
	writeSkill(t, g2, "same", "# version two")
	writeSkill(t, g2, "copy", "# identical")
	writeSkill(t, g1, "copy", "# identical")

	surface := scanSkillSurface(
		map[string]string{"global-opencode": g1, "global-claude": g2},
		map[string][]string{},
	)

	same := findSkill(surface.Skills, "same")
	if same == nil || same.Status != "shadowed" || len(same.Entries) != 2 {
		t.Fatalf("same = %+v, want shadowed with 2 entries", same)
	}
	cp := findSkill(surface.Skills, "copy")
	if cp == nil || cp.Status != "unique" || len(cp.Entries) != 2 {
		t.Fatalf("copy = %+v, want unique (same bytes) with 2 entries", cp)
	}
}

func TestScanSkillSurface_ProjectWalkUp(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "repo", "sub")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(root, "repo", ".opencode", "skills"), "local", "# local")

	surface := scanSkillSurface(map[string]string{}, projectSkillChains(proj))

	local := findSkill(surface.Skills, "local")
	if local == nil || local.Status != "unique" {
		t.Fatalf("local = %+v, want unique", local)
	}
	if got := local.Entries[0].Location; got != "project-opencode" {
		t.Fatalf("location = %q, want project-opencode", got)
	}
}

func TestScanSkillSurface_Symlink(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	writeSkill(t, real, "linked", "# linked")
	linkDir := filepath.Join(root, "links")
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(real, "linked"), filepath.Join(linkDir, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	surface := scanSkillSurface(
		map[string]string{"global-opencode": linkDir},
		map[string][]string{},
	)

	linked := findSkill(surface.Skills, "linked")
	if linked == nil || len(linked.Entries) != 1 {
		t.Fatalf("linked = %+v, want 1 entry", linked)
	}
	if linked.Entries[0].Symlink == "" || linked.Entries[0].Broken {
		t.Fatalf("entry = %+v, want live symlink", linked.Entries[0])
	}
}
