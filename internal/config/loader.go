package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var (
	activeConfig *Config
	configPaths  []string
)

func LoadConfig() *Config {
	if activeConfig != nil {
		return activeConfig
	}

	cfg := BuiltinConfig()

	paths := []string{}
	home, _ := os.UserHomeDir()
	if home != "" {
		paths = append(paths, filepath.Join(home, ".ywai.yaml"))
	}
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, ".ywai.yaml"))
	}
	configPaths = paths

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}

		fileCfg := &Config{}
		if err := unmarshalYAML(data, fileCfg); err != nil {
			fmt.Printf("Warning: invalid config %s: %v\n", p, err)
			continue
		}

		for name, profile := range fileCfg.Profiles {
			cfg.Profiles[name] = profile
		}
		if fileCfg.DefaultType != "" {
			cfg.DefaultType = fileCfg.DefaultType
		}
	}

	activeConfig = cfg
	return cfg
}

func unmarshalYAML(data []byte, out *Config) error {
	return yaml.Unmarshal(data, out)
}

func ProfileSkills(profileName string) []string {
	cfg := LoadConfig()
	p, ok := cfg.Profiles[profileName]
	if !ok || len(p.Skills) == 0 {
		return nil
	}
	return p.Skills
}

func AvailableProfiles() []string {
	cfg := LoadConfig()
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	return names
}

func ProfileDescription(name string) string {
	cfg := LoadConfig()
	if p, ok := cfg.Profiles[name]; ok {
		return p.Description
	}
	return ""
}

func ResetConfig() {
	activeConfig = nil
}
