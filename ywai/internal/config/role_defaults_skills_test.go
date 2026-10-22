package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEmbeddedRoleSkillsExist keeps the seeded role skills pointing at skills
// that ship under skills/: a missing id loads nothing and fails silently.
func TestEmbeddedRoleSkillsExist(t *testing.T) {
	roles := parseRoleDefaults(embeddedRoleDefaults)
	if roles == nil {
		t.Fatal("embedded role defaults do not parse")
	}
	for role, rd := range roles {
		for _, skill := range rd.Skills {
			if _, err := os.Stat(filepath.Join("..", "..", "skills", skill, "SKILL.md")); err != nil {
				t.Errorf("role %s: skill %q does not exist", role, skill)
			}
		}
	}
}
