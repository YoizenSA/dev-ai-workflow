package config

import "os"

func AvailableSkills() []string {
	entries, err := os.ReadDir(DataSkillsDir())
	if err != nil {
		return nil
	}
	var skills []string
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
