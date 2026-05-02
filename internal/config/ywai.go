package config

type Profile struct {
	Skills      []string `yaml:"skills"`
	Description string   `yaml:"description,omitempty"`
}

type Config struct {
	DefaultType string             `yaml:"default_type,omitempty"`
	Profiles    map[string]Profile `yaml:"profiles"`
}

func BuiltinConfig() *Config {
	return &Config{
		Profiles: map[string]Profile{
			"generic": {
				Description: "All skills enabled",
				Skills:      nil,
			},
			"react": {
				Description: "React + TypeScript frontend",
				Skills:      []string{"react-19", "tailwind-4", "typescript", "biome", "playwright", "git-commit"},
			},
			"nest": {
				Description: "NestJS + TypeScript backend",
				Skills:      []string{"typescript", "biome", "playwright", "git-commit"},
			},
			"dotnet": {
				Description: ".NET / C# backend",
				Skills:      []string{"dotnet", "git-commit"},
			},
			"devops": {
				Description: "Azure Pipelines, Helm, K8s",
				Skills:      []string{"devops", "git-commit"},
			},
		},
	}
}
