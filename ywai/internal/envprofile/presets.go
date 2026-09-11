package envprofile

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
)

// Preset is the starting content of a profile: which agent groups, skills,
// MCP servers, defaults and UI theme a `create --preset` stamps into the
// manifest pipeline. Presets are data (embedded JSON), not Go code.
//
// Render rule (enforced by the apply pipeline, not here): source registries
// may stay global as drafts, but rendered artifacts always live per profile.
// A skill or workflow absent from a profile must not be advertised by it.

//go:embed presets/*.json
var presetFS embed.FS

// Preset returns one preset by name.
func Preset(name string) (map[string]any, error) {
	data, err := presetFS.ReadFile("presets/" + name + ".json")
	if err != nil {
		return nil, fmt.Errorf("unknown preset %q: %w", name, err)
	}
	var preset map[string]any
	if err := json.Unmarshal(data, &preset); err != nil {
		return nil, fmt.Errorf("parse preset %q: %w", name, err)
	}
	return preset, nil
}

// Presets returns the embedded preset names, sorted.
func Presets() []string {
	entries, err := presetFS.ReadDir("presets")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if len(name) > 5 && name[len(name)-5:] == ".json" {
			out = append(out, name[:len(name)-5])
		}
	}
	sort.Strings(out)
	return out
}
