package plugins

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// pluginSkills maps a manifest entry's Skill name to the resolver for its
// SKILL.md. A plugin that ships usage rules installs them the same way it
// installs its bundle: named by the manifest, behind the same flag.
var pluginSkills = map[string]func() (string, error){
	"jev-gate": config.JevGateSkillPath,
}

// installPluginSkill copies a plugin's SKILL.md into the shared agent skills
// directory, under a directory named after the skill.
//
// It deliberately does NOT write the `.ywai-extra` marker that the extra-skill
// copier uses. That marker is what pruneRetiredSkills keys on to delete skills
// no longer shipped in ywai/skills, and this skill does not live there - with
// the marker, the next install would delete it as retired.
func installPluginSkill(name string) error {
	resolve, ok := pluginSkills[name]
	if !ok {
		return fmt.Errorf("unknown plugin skill %q", name)
	}
	src, err := resolve()
	if err != nil {
		return err
	}
	body, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}

	dir := filepath.Join(config.AgentsSkillsDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	dst := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(dst, body, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}

// RemovePluginSkill deletes a plugin skill directory. Usage rules for tools
// that are gone are worse than none.
func RemovePluginSkill(name string) error {
	if _, ok := pluginSkills[name]; !ok {
		return fmt.Errorf("unknown plugin skill %q", name)
	}
	return os.RemoveAll(filepath.Join(config.AgentsSkillsDir(), name))
}
