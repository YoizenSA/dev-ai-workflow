package config

import (
	_ "embed"
	"encoding/json"
)

const DefaultOrchestratorModelProfileName = "balanced"

// OrchestratorModelProfile groups model defaults for orchestrator-specific roles.
type OrchestratorModelProfile struct {
	DisplayName  string       `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	Description  string       `yaml:"description,omitempty" json:"description,omitempty"`
	RoleDefaults RoleDefaults `yaml:"role_defaults,omitempty" json:"role_defaults,omitempty"`
}

type orchestratorProfilesDoc struct {
	Profiles map[string]OrchestratorModelProfile `json:"profiles"`
}

//go:embed orchestrator_profiles.json
var embeddedOrchestratorProfiles []byte

// DefaultOrchestratorModelProfiles returns the embedded seed profiles.
func DefaultOrchestratorModelProfiles() map[string]OrchestratorModelProfile {
	var doc orchestratorProfilesDoc
	if err := json.Unmarshal(embeddedOrchestratorProfiles, &doc); err != nil || len(doc.Profiles) == 0 {
		return map[string]OrchestratorModelProfile{}
	}
	return cloneOrchestratorProfiles(doc.Profiles)
}

func cloneOrchestratorProfiles(src map[string]OrchestratorModelProfile) map[string]OrchestratorModelProfile {
	out := make(map[string]OrchestratorModelProfile, len(src))
	for name, profile := range src {
		out[name] = profile.Clone()
	}
	return out
}

func (p OrchestratorModelProfile) Clone() OrchestratorModelProfile {
	p.RoleDefaults = cloneRoleDefaults(p.RoleDefaults)
	return p
}

func cloneRoleDefaults(src RoleDefaults) RoleDefaults {
	if src == nil {
		return nil
	}
	out := make(RoleDefaults, len(src))
	for role, rd := range src {
		rd.Fallbacks = append([]string(nil), rd.Fallbacks...)
		rd.Skills = append([]string(nil), rd.Skills...)
		out[role] = rd
	}
	return out
}
