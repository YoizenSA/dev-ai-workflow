package skills

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unicode"

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

func TestCopyTo_SkipsUnchangedSkill(t *testing.T) {
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
	agentSkillsDir := filepath.Join(t.TempDir(), "agent-skills")
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CopyTo(agentSkillsDir); err != nil {
		t.Fatal(err)
	}
	stamp := filepath.Join(agentSkillsDir, "yz-ui", "keep.txt")
	if err := os.WriteFile(stamp, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyTo(agentSkillsDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stamp); err != nil {
		t.Fatal("unchanged skill was recopied")
	}
}

func TestCopyTo_RecopiesWhenSourceChanges(t *testing.T) {
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
	agentSkillsDir := filepath.Join(t.TempDir(), "agent-skills")
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CopyTo(agentSkillsDir); err != nil {
		t.Fatal(err)
	}
	stamp := filepath.Join(agentSkillsDir, "yz-ui", "keep.txt")
	if err := os.WriteFile(stamp, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoSkillsDir, "yz-ui", "SKILL.md"), []byte("# changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyTo(agentSkillsDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stamp); !os.IsNotExist(err) {
		t.Fatal("changed skill must be recopied")
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

func TestCopyToBundlesLearnYwaiDocs(t *testing.T) {
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
	writeSkill(t, repoSkillsDir, "learn-ywai", true)

	page := filepath.Join(repo, "docs", "src", "content", "docs", "getting-started", "index.mdx")
	if err := os.MkdirAll(filepath.Dir(page), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, []byte("# Primeros pasos\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	agentSkillsDir := filepath.Join(t.TempDir(), "agent-skills")
	if err := os.MkdirAll(agentSkillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CopyTo(agentSkillsDir); err != nil {
		t.Fatalf("CopyTo: %v", err)
	}

	got := filepath.Join(agentSkillsDir, "learn-ywai", "references", "docs", "getting-started", "index.mdx")
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("bundled doc missing: %v", err)
	}
	if string(data) != "# Primeros pasos\n" {
		t.Fatalf("bundled doc = %q", data)
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

// Deleting a skill from the source used to leave it installed forever:
// webapp-testing outlived its removal and kept contradicting the skill that
// replaced it. Pruning fixes that, and these pin the line it must not cross —
// removing something the user wrote.
func TestPruneRetiredSkills(t *testing.T) {
	dir := t.TempDir()

	mk := func(name string, ywai bool) string {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if ywai {
			if err := os.WriteFile(filepath.Join(p, extraSkillMarkerFile), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return p
	}

	stillShipped := mk("tdd", true)
	retired := mk("webapp-testing", true)
	userOwned := mk("my-skill", false)
	// The dangerous case: the user wrote a skill named like one we retired.
	collision := mk("dotnet", false)

	removed := pruneRetiredSkills(dir, map[string]bool{"tdd": true})

	if len(removed) != 1 || removed[0] != "webapp-testing" {
		t.Fatalf("removed = %v, want only the retired ywai skill", removed)
	}
	if _, err := os.Stat(retired); !os.IsNotExist(err) {
		t.Error("a retired ywai skill must be removed")
	}
	for _, keep := range []string{stillShipped, userOwned, collision} {
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("%s must survive: %v", filepath.Base(keep), err)
		}
	}
}

func TestPruneRetiredSkills_LeavesSymlinksAlone(t *testing.T) {
	// Links are RemoveStaleYwaiSkillLinks's job — it knows how to check where
	// they point. Treating one as a directory here could delete its target.
	dir := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(dir, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if removed := pruneRetiredSkills(dir, map[string]bool{}); len(removed) != 0 {
		t.Errorf("removed = %v, want nothing", removed)
	}
	if _, err := os.Stat(target); err != nil {
		t.Error("the link target must be untouched")
	}
}

func TestPruneRetiredSkills_MissingDirIsNoOp(t *testing.T) {
	if removed := pruneRetiredSkills(filepath.Join(t.TempDir(), "absent"), map[string]bool{}); removed != nil {
		t.Errorf("removed = %v, want nil", removed)
	}
}

func TestWorkLedgerSkillLayout(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "skills", "work-ledger")
	required := []string{
		"SKILL.md",
		extraSkillMarkerFile,
		"modules/gate.md",
		"modules/ledger.md",
		"modules/seams.md",
		"modules/ship.md",
		"modules/resume.md",
	}
	for _, rel := range required {
		path := filepath.Join(dir, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("work-ledger missing %s: %v", rel, err)
		}
		if strings.TrimSpace(string(data)) == "" {
			t.Fatalf("work-ledger %s is empty", rel)
		}
	}
	body, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "name: work-ledger") {
		t.Fatal("SKILL.md must declare name: work-ledger")
	}
	if !strings.Contains(text, "description:") {
		t.Fatal("SKILL.md must have a description")
	}
}

// maxSkillDescription caps the frontmatter description of a bundled skill.
//
// Every description is concatenated into the <available_skills> block that
// OpenCode injects into every request of every agent, so the catalog is paid
// on every turn whether or not a skill is ever loaded. Measured on a real
// payload the block was 5486 tokens; the descriptions had drifted to full
// paragraphs (diks alone spent 582 characters). The description only has to
// make the model pick the skill — the SKILL.md body is where the content
// lives, and it costs nothing until the skill is actually loaded.
const maxSkillDescription = 125

func TestBundledSkillDescriptionsStayShort(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	skillsDir := filepath.Join(filepath.Dir(thisFile), "..", "..", "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatalf("read skills dir: %v", err)
	}

	checked := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(skillsDir, e.Name(), "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			continue // not every directory ships a SKILL.md
		}
		desc, ok := frontmatterDescriptionForTest(string(data))
		if !ok {
			t.Errorf("%s: no description in frontmatter", e.Name())
			continue
		}
		checked++
		if len(desc) > maxSkillDescription {
			t.Errorf("%s: description is %d chars, max %d\n  %s",
				e.Name(), len(desc), maxSkillDescription, desc)
		}
	}
	if checked == 0 {
		t.Fatal("no bundled skills checked; the test is not looking where it thinks")
	}
}

// frontmatterDescriptionForTest reads the description from YAML frontmatter,
// covering the plain, quoted, and folded (">") forms the skills use.
func frontmatterDescriptionForTest(s string) (string, bool) {
	if !strings.HasPrefix(s, "---\n") {
		return "", false
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		return "", false
	}
	lines := strings.Split(s[4:4+end], "\n")
	for i, l := range lines {
		if !strings.HasPrefix(l, "description:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(l, "description:"))
		if val == ">" || val == "|" {
			var folded []string
			for _, next := range lines[i+1:] {
				if next == "" || !unicode.IsSpace(rune(next[0])) {
					break
				}
				folded = append(folded, strings.TrimSpace(next))
			}
			return strings.Join(folded, " "), true
		}
		return strings.Trim(val, `"'`), true
	}
	return "", false
}

// A one-level scan reported ~/.gemini clean while ten sdd-* skills sat in
// ~/.gemini/antigravity-cli/skills, and missed the loose sdd-orchestrator.md /
// sdd-*.config.toml gentle-ai drops in the config root. A cleanup that reports
// success while assets remain is worse than none: nobody looks again.
func TestRemoveSddAssetsReachesNestedAndLooseAssets(t *testing.T) {
	cfg := t.TempDir()
	write := func(rel string) {
		p := filepath.Join(cfg, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	// Retired assets, in all three shapes seen on real hosts.
	write("sdd-orchestrator.md")                         // loose in config root
	write("sdd-cheap.config.toml")                       // loose in config root
	write("skills/sdd-init/SKILL.md")                    // the already-covered shape
	write("antigravity-cli/skills/sdd-apply/SKILL.md")   // nested agent
	write("antigravity-cli/skills/_shared/sdd-phase.md") // nested _shared
	write("antigravity-cli/commands/sdd-new.md")         // nested commands

	// Must survive: a plugin marketplace owns its sdd-* skills, and ywai's own
	// skills never match the prefix anyway.
	write("plugins/marketplaces/engram/skills/sdd-flow/SKILL.md")
	write("skills/angular/SKILL.md")

	removed, err := RemoveSddAssets(filepath.Join(cfg, "skills"))
	if err != nil {
		t.Fatalf("RemoveSddAssets: %v", err)
	}
	if len(removed) != 6 {
		t.Errorf("removed %d assets, want 6: %v", len(removed), removed)
	}
	for _, rel := range []string{
		"sdd-orchestrator.md",
		"sdd-cheap.config.toml",
		"skills/sdd-init",
		"antigravity-cli/skills/sdd-apply",
		"antigravity-cli/skills/_shared/sdd-phase.md",
		"antigravity-cli/commands/sdd-new.md",
	} {
		if _, err := os.Stat(filepath.Join(cfg, rel)); !os.IsNotExist(err) {
			t.Errorf("%s survived (err=%v)", rel, err)
		}
	}
	for _, rel := range []string{
		"plugins/marketplaces/engram/skills/sdd-flow/SKILL.md",
		"skills/angular/SKILL.md",
	} {
		if _, err := os.Stat(filepath.Join(cfg, rel)); err != nil {
			t.Errorf("%s must not be touched: %v", rel, err)
		}
	}

	// The count the Settings panel shows must agree with what removal does,
	// or the UI says "Sin assets SDD" over a host that still has them.
	if got := CountSddAssets(filepath.Join(cfg, "skills")); got != 0 {
		t.Errorf("CountSddAssets after removal = %d, want 0", got)
	}
}
