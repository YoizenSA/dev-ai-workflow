package config

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
)

// embeddedRoleDefaults is the canonical seed for per-role execution defaults,
// baked into the binary at build time. Models reference opencode-native
// providers so a fresh install runs without any paid provider balance.
//
//go:embed role_defaults.json
var embeddedRoleDefaults []byte

// RoleDefaultsPath returns the path to the optional external role-defaults
// override file. When this file exists it fully replaces the embedded seed,
// letting users retune models and skills without rebuilding ywai.
func RoleDefaultsPath() string {
	return filepath.Join(DataDir(), "role-defaults.json")
}

// DefaultRoleDefaults returns the seeded role defaults, resolved in order:
//
//	external override (~/.ywai/role-defaults.json) → embedded JSON → empty
//
// Any malformed source is skipped so a bad edit can never brick mission
// execution; the next valid source (or an empty map) is used instead.
func DefaultRoleDefaults() RoleDefaults {
	if rd := parseRoleDefaults(readExternalRoleDefaults()); rd != nil {
		return rd
	}
	if rd := parseRoleDefaults(embeddedRoleDefaults); rd != nil {
		return rd
	}
	return RoleDefaults{}
}

// readExternalRoleDefaults returns the bytes of the external override file, or
// nil when it is absent or unreadable.
func readExternalRoleDefaults() []byte {
	data, err := os.ReadFile(RoleDefaultsPath())
	if err != nil {
		return nil
	}
	return data
}

// roleDefaultsDoc is the structured role-defaults file shape: a shared
// `defaults` base merged into every entry under `roles`, so common fields
// (e.g. agent) are written once instead of repeated per role.
type roleDefaultsDoc struct {
	Defaults RoleDefault  `json:"defaults"`
	Roles    RoleDefaults `json:"roles"`
}

// parseRoleDefaults unmarshals role defaults JSON. It accepts the structured
// `{defaults, roles}` shape (default-merged) and the legacy flat map shape.
// Returns nil when the input is empty or invalid so callers can fall through to
// the next source.
func parseRoleDefaults(data []byte) RoleDefaults {
	if len(data) == 0 {
		return nil
	}
	// Structured shape: { "defaults": {...}, "roles": { "<role>": {...} } }.
	var doc roleDefaultsDoc
	if err := json.Unmarshal(data, &doc); err == nil && len(doc.Roles) > 0 {
		mergeRoleDefaults(doc.Roles, doc.Defaults)
		return doc.Roles
	}
	// Legacy flat shape: { "<role>": {...} }.
	var rd RoleDefaults
	if err := json.Unmarshal(data, &rd); err != nil {
		return nil
	}
	if len(rd) == 0 {
		return nil
	}
	return rd
}

// mergeRoleDefaults fills each role's empty fields from the shared base so
// common values are declared once in `defaults`.
func mergeRoleDefaults(roles RoleDefaults, base RoleDefault) {
	for name, rd := range roles {
		if rd.Agent == "" {
			rd.Agent = base.Agent
		}
		if rd.Model == "" {
			rd.Model = base.Model
		}
		if len(rd.Fallbacks) == 0 {
			rd.Fallbacks = base.Fallbacks
		}
		if len(rd.Skills) == 0 {
			rd.Skills = base.Skills
		}
		roles[name] = rd
	}
}
