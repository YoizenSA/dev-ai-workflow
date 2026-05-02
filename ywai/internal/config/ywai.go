package config

import (
	"fmt"
	"os"
	"path/filepath"
	"gopkg.in/yaml.v3"
)

type Profile struct {
	Skills      []string `yaml:"skills"`
	Description string   `yaml:"description,omitempty"`
}

type Config struct {
	DefaultType string             `yaml:"default_type,omitempty"`
	Profiles    map[string]Profile `yaml:"profiles"`
}

type profileFile struct {
	Description string   `yaml:"description"`
	Skills      []string `yaml:"skills"`
}

func BuiltinConfig() *Config {
	cfg := &Config{
		Profiles: make(map[string]Profile),
	}

	dir := ProjectTypesSourceDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return cfg
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		profilePath := filepath.Join(dir, entry.Name(), "profile.yaml")
		data, err := os.ReadFile(profilePath)
		if err != nil {
			continue
		}

		var pf profileFile
		if err := yaml.Unmarshal(data, &pf); err != nil {
			fmt.Printf("Warning: invalid %s: %v\n", profilePath, err)
			continue
		}

		skills := pf.Skills
		if len(skills) == 0 {
			skills = nil
		}

		cfg.Profiles[entry.Name()] = Profile{
			Description: pf.Description,
			Skills:      skills,
		}
	}

	return cfg
}
