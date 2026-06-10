package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// UserConfig represents the user's ywai configuration file
type UserConfig struct {
	// Default preset for installations
	DefaultPreset string `yaml:"default_preset,omitempty"`
	
	// Default SDD mode
	DefaultSDDMode string `yaml:"default_sdd_mode,omitempty"`
	
	// Default persona
	DefaultPersona string `yaml:"default_persona,omitempty"`
	
	// Default scope (global or workspace)
	DefaultScope string `yaml:"default_scope,omitempty"`
	
	// Whether to use TUI by default
	DefaultTUI bool `yaml:"default_tui,omitempty"`
	
	// Agents is an explicit list of agents ywai should manage.
	// When non-empty, ywai will only operate on these agents instead of auto-detecting.
	Agents []string `yaml:"agents,omitempty"`
	
	// Whether to install MCP by default for opencode
	DefaultMCP bool `yaml:"default_mcp,omitempty"`
	
	// Whether to use colored output
	ColoredOutput *bool `yaml:"colored_output,omitempty"`
	
	// Log level (debug, info, warn, error)
	LogLevel string `yaml:"log_level,omitempty"`
	
	// Custom agent profiles directory
	CustomAgentsDir string `yaml:"custom_agents_dir,omitempty"`
	
	// Custom skills directory
	CustomSkillsDir string `yaml:"custom_skills_dir,omitempty"`

	// TokenBank proxy configuration
	TokenBankURL    string `yaml:"tokenbank_url,omitempty"`
	TokenBankAPIKey string `yaml:"tokenbank_api_key,omitempty"`
}

// ConfigPath returns the path to the user config file
func ConfigPath() string {
	return filepath.Join(DataDir(), "config.yaml")
}

// LoadConfig loads the user configuration from ~/.ywai/config.yaml
// Returns a default config if the file doesn't exist
func LoadConfig() (*UserConfig, error) {
	configPath := ConfigPath()
	
	// If config doesn't exist, return defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var config UserConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	
	return &config, nil
}

// SaveConfig saves the user configuration to ~/.ywai/config.yaml
func SaveConfig(config *UserConfig) error {
	configPath := ConfigPath()
	
	// Ensure data directory exists
	if err := EnsureDataDir(); err != nil {
		return err
	}
	
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	return nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *UserConfig {
	return &UserConfig{
		DefaultPreset:  "full-gentleman",
		DefaultSDDMode: "single",
		DefaultPersona: "gentleman",
		DefaultScope:   "global",
		DefaultTUI:     true,
		DefaultMCP:     false,
		ColoredOutput:  func() *bool { b := true; return &b }(),
		LogLevel:       "info",
	}
}

// GetDefaultPreset returns the default preset from config or fallback
func GetDefaultPreset() string {
	config, err := LoadConfig()
	if err != nil {
		return "full-gentleman"
	}
	if config.DefaultPreset != "" {
		return config.DefaultPreset
	}
	return "full-gentleman"
}

// GetDefaultSDDMode returns the default SDD mode from config or fallback
func GetDefaultSDDMode() string {
	config, err := LoadConfig()
	if err != nil {
		return "single"
	}
	if config.DefaultSDDMode != "" {
		return config.DefaultSDDMode
	}
	return "single"
}

// GetDefaultPersona returns the default persona from config or fallback
func GetDefaultPersona() string {
	config, err := LoadConfig()
	if err != nil {
		return "gentleman"
	}
	if config.DefaultPersona != "" {
		return config.DefaultPersona
	}
	return "gentleman"
}

// GetDefaultScope returns the default scope from config or fallback
func GetDefaultScope() string {
	config, err := LoadConfig()
	if err != nil {
		return "global"
	}
	if config.DefaultScope != "" {
		return config.DefaultScope
	}
	return "global"
}

// ShouldUseColor returns whether to use colored output
func ShouldUseColor() bool {
	config, err := LoadConfig()
	if err != nil {
		return true // default to colored
	}
	if config.ColoredOutput != nil {
		return *config.ColoredOutput
	}
	return true
}
