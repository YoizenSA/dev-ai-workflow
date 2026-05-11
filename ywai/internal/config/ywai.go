package config

import (
	"os"
)

func AvailableSkills() []string {
	dir := DataSkillsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	skills := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			skills = append(skills, entry.Name())
		}
	}
	return skills
}

func ResetConfig() {
	// No-op since config is now skills-only
}
