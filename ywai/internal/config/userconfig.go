package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Role identifiers for mission execution defaults.
const (
	RolePlanning  = "planning"
	RoleArchitect = "architect"
	RoleDev       = "dev"
	RoleFrontend  = "frontend"
	RoleBackend   = "backend"
	RoleQA        = "qa"
	RoleReviewer  = "reviewer"
	RoleDevops    = "devops"
)

// DefaultVisionModel is intentionally empty: mcp-vision resolves the default
// from TokenBank's live vision-capable model catalog at runtime.
const DefaultVisionModel = ""

// CanonicalRoles is the ordered set of supported role identifiers.
// Ordered roughly by delivery phase: plan → design → implement → verify → ship.
var CanonicalRoles = []string{
	RolePlanning, RoleArchitect, RoleDev, RoleFrontend, RoleBackend, RoleQA, RoleReviewer, RoleDevops,
}

// RoleDefault captures the default agent/model/skills assigned to a role.
type RoleDefault struct {
	Agent     string   `yaml:"agent,omitempty" json:"agent,omitempty"`
	Model     string   `yaml:"model,omitempty" json:"model,omitempty"`
	Fallbacks []string `yaml:"fallbacks,omitempty" json:"fallbacks,omitempty"`
	Skills    []string `yaml:"skills,omitempty" json:"skills,omitempty"`
}

// RoleDefaults maps role names to their default execution configuration.
type RoleDefaults map[string]RoleDefault

// UserConfig represents the user's ywai configuration file
type UserConfig struct {
	// Default preset for installations
	DefaultPreset string `yaml:"default_preset,omitempty" json:"default_preset,omitempty"`

	// Default SDD mode
	DefaultSDDMode string `yaml:"default_sdd_mode,omitempty" json:"default_sdd_mode,omitempty"`

	// Default persona
	DefaultPersona string `yaml:"default_persona,omitempty" json:"default_persona,omitempty"`

	// Default scope (global or workspace)
	DefaultScope string `yaml:"default_scope,omitempty" json:"default_scope,omitempty"`

	// Whether to use TUI by default
	DefaultTUI bool `yaml:"default_tui,omitempty" json:"default_tui,omitempty"`

	// Agents is an explicit list of agents ywai should manage.
	// When non-empty, ywai will only operate on these agents instead of auto-detecting.
	Agents []string `yaml:"agents,omitempty" json:"agents,omitempty"`

	// Whether to install MCP by default for opencode
	DefaultMCP bool `yaml:"default_mcp,omitempty" json:"default_mcp,omitempty"`

	// Whether to use colored output
	ColoredOutput *bool `yaml:"colored_output,omitempty" json:"colored_output,omitempty"`

	// Log level (debug, info, warn, error)
	LogLevel string `yaml:"log_level,omitempty" json:"log_level,omitempty"`

	// Custom agent profiles directory
	CustomAgentsDir string `yaml:"custom_agents_dir,omitempty" json:"custom_agents_dir,omitempty"`

	// Custom skills directory
	CustomSkillsDir string `yaml:"custom_skills_dir,omitempty" json:"custom_skills_dir,omitempty"`

	// TokenBank proxy configuration
	TokenBankURL    string `yaml:"tokenbank_url,omitempty" json:"tokenbank_url,omitempty"`
	TokenBankAPIKey string `yaml:"tokenbank_api_key,omitempty" json:"tokenbank_api_key,omitempty"`

	// Server configuration
	Server ServerConfig `yaml:"server,omitempty" json:"server,omitempty"`

	// RoleDefaults assigns default agent + model + fallbacks + skills per mission role.
	RoleDefaults RoleDefaults `yaml:"role_defaults,omitempty" json:"role_defaults,omitempty"`

	// ActiveOrchestratorProfile selects which orchestrator model profile is used at runtime.
	ActiveOrchestratorProfile string `yaml:"active_orchestrator_profile,omitempty" json:"active_orchestrator_profile,omitempty"`

	// OrchestratorProfiles contains user-overridable model profiles for orchestrator roles.
	OrchestratorProfiles map[string]OrchestratorModelProfile `yaml:"orchestrator_profiles,omitempty" json:"orchestrator_profiles,omitempty"`

	// VisionModel is the default vision model used by mcp-vision.
	VisionModel string `yaml:"vision_model,omitempty" json:"vision_model,omitempty"`

	// VisionModelOverride, when set, overrides VisionModel for mcp-vision calls.
	VisionModelOverride string `yaml:"vision_model_override,omitempty" json:"vision_model_override,omitempty"`
}

// ServerConfig contains configuration for the control server
type ServerConfig struct {
	// Port for the control server (default 5768)
	Port int `yaml:"port,omitempty" json:"port,omitempty"`

	// Whether to run in background mode
	Background bool `yaml:"background,omitempty" json:"background,omitempty"`

	// Whether to start MCP adapter
	MCP bool `yaml:"mcp,omitempty" json:"mcp,omitempty"`

	// Whether to configure autostart
	Autostart bool `yaml:"autostart,omitempty" json:"autostart,omitempty"`
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
	config.ensureDefaults()

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

	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *UserConfig {
	return &UserConfig{
		DefaultPreset:             "full-gentleman",
		DefaultSDDMode:            "single",
		DefaultPersona:            "gentleman",
		DefaultScope:              "global",
		DefaultTUI:                true,
		DefaultMCP:                false,
		ColoredOutput:             func() *bool { b := true; return &b }(),
		LogLevel:                  "info",
		RoleDefaults:              DefaultRoleDefaults(),
		ActiveOrchestratorProfile: DefaultOrchestratorModelProfileName,
		OrchestratorProfiles:      DefaultOrchestratorModelProfiles(),
		VisionModel:               DefaultVisionModel,
	}
}

func (c *UserConfig) ensureDefaults() {
	if c == nil {
		return
	}
	if c.RoleDefaults == nil {
		c.RoleDefaults = DefaultRoleDefaults()
	}
	if c.OrchestratorProfiles == nil {
		c.OrchestratorProfiles = DefaultOrchestratorModelProfiles()
	}
	if c.ActiveOrchestratorProfile == "" {
		c.ActiveOrchestratorProfile = DefaultOrchestratorModelProfileName
	}
	if _, ok := c.OrchestratorProfiles[c.ActiveOrchestratorProfile]; !ok {
		c.ActiveOrchestratorProfile = DefaultOrchestratorModelProfileName
	}
	// VisionModel stays empty when unset so mcp-vision can pick from
	// TokenBank's live catalog instead of a hardcoded product default.
}

// GetVisionModel returns the configured vision model preference (override wins),
// or empty when unset. Strips a provider prefix (e.g. "opencode-admin/mimo-v2.5"
// → "mimo-v2.5") so the value is a bare TokenBank model id. An empty return means
// the caller should resolve the default from TokenBank's vision catalog.
func (c *UserConfig) GetVisionModel() string {
	raw := ""
	if c != nil && c.VisionModelOverride != "" {
		raw = c.VisionModelOverride
	} else if c != nil && c.VisionModel != "" {
		raw = c.VisionModel
	}
	raw = strings.TrimSpace(raw)
	if i := strings.LastIndex(raw, "/"); i >= 0 && i+1 < len(raw) {
		return raw[i+1:]
	}
	return raw
}

// GetRoleDefault returns the configured RoleDefault for a role, lazily falling
// back to the seeded default when the user config has no override.
func (c *UserConfig) GetRoleDefault(role string) RoleDefault {
	if c != nil {
		if rd, ok := c.RoleDefaults[role]; ok && !rd.isEmpty() {
			return rd
		}
	}
	seed := DefaultRoleDefaults()
	if rd, ok := seed[role]; ok {
		return rd
	}
	return RoleDefault{}
}

// GetActiveOrchestratorProfile returns the active orchestrator profile,
// falling back to the seeded default when config is missing or invalid.
func (c *UserConfig) GetActiveOrchestratorProfile() OrchestratorModelProfile {
	if c != nil {
		if profile, ok := c.OrchestratorProfiles[c.ActiveOrchestratorProfile]; ok {
			return profile.Clone()
		}
	}
	return DefaultOrchestratorModelProfiles()[DefaultOrchestratorModelProfileName]
}

// GetOrchestratorAgentModel returns the active profile's model for an agent,
// or "" when the agent is not listed in the profile.
func (c *UserConfig) GetOrchestratorAgentModel(agent string) string {
	profile := c.GetActiveOrchestratorProfile()
	if profile.Agents == nil {
		return ""
	}
	return profile.Agents[agent].Model
}

// ResyncOrchestratorModelProfiles restores profiles from the embedded seed and
// removes user-created profiles. The active selection is preserved when still valid.
func (c *UserConfig) ResyncOrchestratorModelProfiles() {
	if c == nil {
		return
	}
	active := c.ActiveOrchestratorProfile
	c.OrchestratorProfiles = DefaultOrchestratorModelProfiles()
	if _, ok := c.OrchestratorProfiles[active]; ok {
		c.ActiveOrchestratorProfile = active
		return
	}
	c.ActiveOrchestratorProfile = DefaultOrchestratorModelProfileName
}

func (rd RoleDefault) isEmpty() bool {
	return rd.Agent == "" && rd.Model == "" && len(rd.Fallbacks) == 0 && len(rd.Skills) == 0
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
