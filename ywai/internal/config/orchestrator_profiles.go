package config

import (
	_ "embed"
	"encoding/json"
)

const DefaultOrchestratorModelProfileName = "balanced"

// OrchestratorModelProfile is a named preset of per-agent model assignments.
// Keys of Agents are agent names (dev, qa, architect, qa-analyst, …); each
// entry's Model is a full opencode model id (e.g. "opencode-admin/deepseek-v4-pro").
// Activating a profile writes each agent's model into that agent's markdown
// config, so it applies both when the agent runs directly and when it is
// delegated to (the delegate tool falls back to the agent's configured model).
type OrchestratorModelProfile struct {
	DisplayName string       `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	Description string       `yaml:"description,omitempty" json:"description,omitempty"`
	Agents      RoleDefaults `yaml:"agents,omitempty" json:"agents,omitempty"`
	// Orchestration is the install-time behavior policy written into the
	// orchestrator agent markdown (generated policy section + edit/write
	// permission flip) when the profile is active. Missing → balanced-like
	// defaults via Normalize, so older profiles keep working unchanged.
	Orchestration OrchestrationPolicy `yaml:"orchestration,omitempty" json:"orchestration,omitempty"`
}

// OrchestrationPolicy is the orchestration behavior a profile ships. It is not
// consulted by ywai at runtime — it is materialized into the installed
// orchestrator agent (see configapi.ApplyActiveOrchestratorProfile).
type OrchestrationPolicy struct {
	// DefaultMode is the triage fallback when the user gives no explicit
	// signal: solo | thin | full.
	DefaultMode string `yaml:"default_mode,omitempty" json:"default_mode,omitempty"`
	// AllowSoloWrite lets the orchestrator edit/write directly in solo/thin
	// modes. Nil means true. false → install flips edit/write to deny.
	AllowSoloWrite *bool `yaml:"allow_solo_write,omitempty" json:"allow_solo_write,omitempty"`
	// MaxHopsBeforeEscalate caps delegation hops before escalating mode. Nil
	// means unset → default 1. Use an explicit 0 to escalate on any delegation.
	MaxHopsBeforeEscalate *int `yaml:"max_hops_before_escalate,omitempty" json:"max_hops_before_escalate,omitempty"`
	// RequireReview gates the review hop: never | on_ship | always.
	RequireReview string `yaml:"require_review,omitempty" json:"require_review,omitempty"`
	// EscalateOn lists the triggers that force escalation out of solo/thin.
	EscalateOn []string `yaml:"escalate_on,omitempty" json:"escalate_on,omitempty"`
}

// DefaultEscalateOn are the triggers every shipped profile escalates on.
var DefaultEscalateOn = []string{"multi_file_deps", "ui_design", "ship", "user_says_orchestrate"}

// DefaultOrchestrationPolicy is the fallback for profiles without an
// orchestration block (balanced-like).
func DefaultOrchestrationPolicy() OrchestrationPolicy {
	allow := true
	hops := 1
	return OrchestrationPolicy{
		DefaultMode:           "thin",
		AllowSoloWrite:        &allow,
		MaxHopsBeforeEscalate: &hops,
		RequireReview:         "on_ship",
		EscalateOn:            append([]string(nil), DefaultEscalateOn...),
	}
}

// Normalize fills zero-value fields with defaults so consumers read the policy
// without nil checks.
func (p OrchestrationPolicy) Normalize() OrchestrationPolicy {
	def := DefaultOrchestrationPolicy()
	if p.DefaultMode == "" {
		p.DefaultMode = def.DefaultMode
	}
	if p.AllowSoloWrite == nil {
		p.AllowSoloWrite = def.AllowSoloWrite
	}
	if p.MaxHopsBeforeEscalate == nil {
		p.MaxHopsBeforeEscalate = def.MaxHopsBeforeEscalate
	}
	if p.RequireReview == "" {
		p.RequireReview = def.RequireReview
	}
	if len(p.EscalateOn) == 0 {
		p.EscalateOn = append([]string(nil), def.EscalateOn...)
	}
	return p
}

// SoloWriteAllowed reports whether the policy lets the orchestrator edit/write
// directly in solo/thin modes.
func (p OrchestrationPolicy) SoloWriteAllowed() bool {
	p = p.Normalize()
	return p.AllowSoloWrite != nil && *p.AllowSoloWrite
}

// Clone returns a deep copy safe to mutate independently.
func (p OrchestrationPolicy) Clone() OrchestrationPolicy {
	if p.AllowSoloWrite != nil {
		v := *p.AllowSoloWrite
		p.AllowSoloWrite = &v
	}
	if p.MaxHopsBeforeEscalate != nil {
		v := *p.MaxHopsBeforeEscalate
		p.MaxHopsBeforeEscalate = &v
	}
	p.EscalateOn = append([]string(nil), p.EscalateOn...)
	return p
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
	p.Agents = cloneRoleDefaults(p.Agents)
	p.Orchestration = p.Orchestration.Clone()
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
