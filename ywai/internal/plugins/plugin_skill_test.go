package plugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func TestInstallPluginSkill_JevGate(t *testing.T) {
	if _, err := config.JevGateSkillPath(); err != nil {
		t.Skipf("no jev-gate skill available: %v", err)
	}
	isolateHome(t)

	if err := installPluginSkill("jev-gate"); err != nil {
		t.Fatalf("installPluginSkill() error = %v", err)
	}

	dir := filepath.Join(config.AgentsSkillsDir(), "jev-gate")
	body, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatalf("expected a SKILL.md in %s: %v", dir, err)
	}
	if !strings.Contains(string(body), "jev_review_diff") {
		t.Error("the installed skill does not describe the tools")
	}

	// The extra-skill copier deletes any directory carrying this marker that
	// is not shipped in ywai/skills. This skill is not, so the marker would
	// make the next install delete it.
	if _, err := os.Stat(filepath.Join(dir, ".ywai-extra")); err == nil {
		t.Error("plugin skills must not carry the .ywai-extra marker")
	}

	// Re-running install overwrites in place rather than failing.
	if err := installPluginSkill("jev-gate"); err != nil {
		t.Errorf("second installPluginSkill() error = %v", err)
	}

	if err := RemovePluginSkill("jev-gate"); err != nil {
		t.Fatalf("RemovePluginSkill() error = %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("the skill directory should be gone after removal")
	}
}

func TestInstallPluginSkill_UnknownName(t *testing.T) {
	if err := installPluginSkill("nope"); err == nil {
		t.Error("an unknown skill name must be an error, not a silent no-op")
	}
}

// Every skill a manifest entry names must resolve, or install fails late on a
// machine that is not this one.
func TestManifestPluginSkillsResolve(t *testing.T) {
	mf, warnings := LoadManifest()
	if len(warnings) > 0 {
		t.Fatalf("LoadManifest() warnings = %v", warnings)
	}
	for _, e := range mf.Install {
		if e.Skill == "" {
			continue
		}
		if _, ok := pluginSkills[e.Skill]; !ok {
			t.Errorf("entry %q names unknown plugin skill %q", e.ID, e.Skill)
		}
	}
}
