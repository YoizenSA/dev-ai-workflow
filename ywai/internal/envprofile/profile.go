// Package envprofile manages isolated opencode2 environments ("ywai env").
// Each profile owns a full XDG home, its own database and its own
// background service, stored under ~/.ywai/profiles/<name>/. The package is
// opencode2-exclusive: it contains no V1 code paths.
package envprofile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Profile is one isolated opencode2 environment.
type Profile struct {
	Name      string `json:"name"`
	Preset    string `json:"preset"`
	Port      int    `json:"port"`
	CreatedAt string `json:"created_at"` // RFC3339 UTC
	// Overrides replace the matching preset lists when non-nil. A nil list
	// inherits the preset; an empty (non-nil) list installs none. Edited
	// from the web Envs page (PATCH /api/envs/{name}) and honored by every
	// PresetGroups/Skills/MCPIDs consumer via PresetSpec.
	Overrides *ProfileOverrides `json:"overrides,omitempty"`
}

// ProfileOverrides replaces preset content lists per environment. No
// omitempty: an empty list ("install none") must survive the manifest
// roundtrip distinct from null ("inherit the preset").
type ProfileOverrides struct {
	Groups []string `json:"groups"`
	Skills []string `json:"skills"`
	MCP    []string `json:"mcp"`
}

// LoadManifest reads <dir>/manifest.json.
func LoadManifest(dir string) (Profile, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Profile{}, fmt.Errorf("read profile manifest: %w", err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{}, fmt.Errorf("parse profile manifest: %w", err)
	}
	if p.Name == "" {
		return Profile{}, fmt.Errorf("profile manifest has no name")
	}
	return p, nil
}

// SaveManifest writes <dir>/manifest.json (0644), creating dir when needed.
func (p Profile) SaveManifest(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create profile dir: %w", err)
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profile manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write profile manifest: %w", err)
	}
	return nil
}
