package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

func deleteSurface(t *testing.T, home, projectDir, path string) int {
	t.Helper()
	t.Setenv("USERPROFILE", home)
	srv := &Server{}
	req := httptest.NewRequest(http.MethodDelete, "/api/config/skills/surface?path="+url.QueryEscape(path)+"&project_dir="+url.QueryEscape(projectDir), nil)
	rec := httptest.NewRecorder()
	srv.handleSkillSurfaceDelete(rec, req)
	return rec.Code
}

func TestHandleSkillSurfaceDelete(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(t.TempDir(), "repo")
	skillDir := filepath.Join(proj, ".opencode", "skills", "gone")
	writeSkill(t, filepath.Join(proj, ".opencode", "skills"), "gone", "# gone")

	if code := deleteSurface(t, home, proj, skillDir); code != http.StatusOK {
		t.Fatalf("delete dir = %d, want 200", code)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatal("skill dir still exists after delete")
	}

	// Missing path param.
	t.Setenv("USERPROFILE", home)
	srv := &Server{}
	rec := httptest.NewRecorder()
	srv.handleSkillSurfaceDelete(rec, httptest.NewRequest(http.MethodDelete, "/api/config/skills/surface", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing path = %d, want 400", rec.Code)
	}

	// Outside every root.
	outside := filepath.Join(t.TempDir(), "evil")
	writeSkill(t, t.TempDir(), "evil", "# evil")
	if code := deleteSurface(t, home, proj, outside); code != http.StatusForbidden {
		t.Fatalf("outside roots = %d, want 403", code)
	}

	// Inside a root but no SKILL.md and not empty: refused.
	empty := filepath.Join(proj, ".opencode", "skills", "hollow")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	stuffed := filepath.Join(proj, ".opencode", "skills", "stuffed")
	if err := os.MkdirAll(stuffed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stuffed, "notes.txt"), []byte("not a skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := deleteSurface(t, home, proj, empty); code != http.StatusOK {
		t.Fatalf("empty dir = %d, want 200", code)
	}
	if code := deleteSurface(t, home, proj, stuffed); code != http.StatusNotFound {
		t.Fatalf("non-empty dir without SKILL.md = %d, want 404", code)
	}
	if _, err := os.Stat(stuffed); err != nil {
		t.Fatal("refused dir must be left intact")
	}
}

func TestPlanStandardize(t *testing.T) {
	surface := skillSurface{Skills: []surfaceSkill{
		{Name: "dup", Status: "shadowed", Entries: []surfaceEntry{
			{Location: "global-claude", Path: "/c/dup", Hash: "aa"},
			{Location: "global-opencode", Path: "/o/dup", Hash: "bb"},
		}},
		{Name: "same", Status: "unique", Entries: []surfaceEntry{
			{Location: "global-opencode", Path: "/o/same", Hash: "cc"},
			{Location: "global-claude", Path: "/c/same", Hash: "cc"},
		}},
		{Name: "dead", Status: "unreadable", Entries: []surfaceEntry{
			{Location: "global-opencode", Path: "/o/dead", Hash: ""},
		}},
	}}
	actions := planStandardize(surface)
	if len(actions) != 2 {
		t.Fatalf("actions = %+v, want 2", actions)
	}
	if actions[0].Kind != "delete-empty-dir" || actions[0].Path != "/o/dead" {
		t.Fatalf("actions[0] = %+v, want delete-empty-dir /o/dead", actions[0])
	}
	// Shadowed: global-opencode outranks global-claude, so the claude copy goes.
	if actions[1].Kind != "resolve-shadow" || actions[1].Path != "/c/dup" {
		t.Fatalf("actions[1] = %+v, want resolve-shadow /c/dup", actions[1])
	}
	if !strings.Contains(actions[1].Detail, "/o/dup") {
		t.Fatalf("detail = %q, must name the kept copy", actions[1].Detail)
	}
}

func TestHandleSkillSurfaceStandardize(t *testing.T) {
	home := t.TempDir()
	g1 := filepath.Join(home, ".config", "opencode", "skills")
	g2 := filepath.Join(home, ".claude", "skills")
	proj := filepath.Join(t.TempDir(), "repo")
	writeSkill(t, g1, "shadow", "# one")
	writeSkill(t, g2, "shadow", "# two")
	writeSkill(t, g1, "same", "# identical")
	writeSkill(t, g2, "same", "# identical")
	hollow := filepath.Join(g1, "hollow")
	if err := os.MkdirAll(hollow, 0o755); err != nil {
		t.Fatal(err)
	}

	standardize := func(dryRun string) (int, map[string]any) {
		t.Helper()
		t.Setenv("USERPROFILE", home)
		srv := &Server{}
		q := "?dry_run=" + dryRun + "&project_dir=" + url.QueryEscape(proj)
		rec := httptest.NewRecorder()
		srv.handleSkillSurfaceStandardize(rec, httptest.NewRequest(http.MethodPost, "/api/config/skills/surface/standardize"+q, nil))
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("bad json: %v", err)
		}
		return rec.Code, body
	}

	// Dry run plans without touching disk.
	code, body := standardize("1")
	if code != http.StatusOK {
		t.Fatalf("dry run = %d, want 200", code)
	}
	actions, _ := body["actions"].([]any)
	if len(actions) != 2 {
		t.Fatalf("dry-run actions = %v, want 2 (shadow + hollow)", body["actions"])
	}
	if _, err := os.Stat(filepath.Join(g2, "shadow", "SKILL.md")); err != nil {
		t.Fatal("dry run must not delete")
	}

	// Execute: claude shadow goes, opencode shadow stays, hollow goes,
	// identical copies stay.
	code, body = standardize("0")
	if code != http.StatusOK {
		t.Fatalf("execute = %d, want 200", code)
	}
	if _, err := os.Stat(filepath.Join(g2, "shadow")); !os.IsNotExist(err) {
		t.Fatal("losing shadow copy still exists")
	}
	if _, err := os.Stat(filepath.Join(g1, "shadow", "SKILL.md")); err != nil {
		t.Fatal("kept shadow copy is gone")
	}
	if _, err := os.Stat(hollow); !os.IsNotExist(err) {
		t.Fatal("hollow dir still exists")
	}
	if _, err := os.Stat(filepath.Join(g2, "same", "SKILL.md")); err != nil {
		t.Fatal("identical duplicate must be left alone")
	}
}

func TestHandleSkillSurfaceDelete_Symlink(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(t.TempDir(), "repo")
	realHolder := filepath.Join(t.TempDir(), "real")
	writeSkill(t, realHolder, "keepme", "# keepme")
	linkDir := filepath.Join(proj, ".opencode", "skills")
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(linkDir, "keepme")
	if err := os.Symlink(filepath.Join(realHolder, "keepme"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if code := deleteSurface(t, home, proj, link); code != http.StatusOK {
		t.Fatalf("delete link = %d, want 200", code)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatal("link still exists after delete")
	}
	if _, err := os.Stat(filepath.Join(realHolder, "keepme", "SKILL.md")); err != nil {
		t.Fatalf("link target was harmed: %v", err)
	}
}
