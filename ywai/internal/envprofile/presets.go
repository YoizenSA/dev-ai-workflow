package envprofile

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// Preset is the starting content of a profile: which agent groups, skills,
// MCP servers, defaults and UI theme a `create --preset` stamps into the
// manifest pipeline. Presets are data (JSON), not Go code.
//
// Two sources: builtins embedded in the binary (read-only) and user presets
// under UserPresetsDir() — copies of a builtin or of an env's customized
// content, created from the web Envs page. A user preset never shadows a
// builtin name.
//
// Render rule (enforced by the apply pipeline, not here): source registries
// may stay global as drafts, but rendered artifacts always live per profile.
// A skill or workflow absent from a profile must not be advertised by it.

//go:embed presets/*.json
var presetFS embed.FS

// UserPresetsDir is ~/.ywai/presets (inside the test profiles root when
// overridden, so hermetic tests never touch the real one).
func UserPresetsDir() string {
	if profilesRootOverride != "" {
		return filepath.Join(profilesRootOverride, ".presets")
	}
	return filepath.Join(config.DataDir(), "presets")
}

func userPresetPath(name string) string {
	return filepath.Join(UserPresetsDir(), name+".json")
}

// IsBuiltinPreset reports whether name is an embedded (read-only) preset.
func IsBuiltinPreset(name string) bool {
	_, err := presetFS.ReadFile("presets/" + name + ".json")
	return err == nil
}

// Preset returns one preset by name: builtin first, then user presets.
func Preset(name string) (map[string]any, error) {
	if !validProfileName.MatchString(name) {
		return nil, fmt.Errorf("unknown preset %q", name)
	}
	data, err := presetFS.ReadFile("presets/" + name + ".json")
	if err != nil {
		if data, err = os.ReadFile(userPresetPath(name)); err != nil {
			return nil, fmt.Errorf("unknown preset %q", name)
		}
	}
	var preset map[string]any
	if err := json.Unmarshal(data, &preset); err != nil {
		return nil, fmt.Errorf("parse preset %q: %w", name, err)
	}
	return preset, nil
}

// Presets returns builtin and user preset names, sorted and deduplicated.
func Presets() []string {
	seen := map[string]bool{}
	if entries, err := presetFS.ReadDir("presets"); err == nil {
		for _, e := range entries {
			if name, ok := strings.CutSuffix(e.Name(), ".json"); ok {
				seen[name] = true
			}
		}
	}
	if entries, err := os.ReadDir(UserPresetsDir()); err == nil {
		for _, e := range entries {
			if name, ok := strings.CutSuffix(e.Name(), ".json"); ok && !e.IsDir() && validProfileName.MatchString(name) {
				seen[name] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// validateNewPresetName rejects malformed or already-taken preset names.
func validateNewPresetName(name string) error {
	if !validProfileName.MatchString(name) {
		return fmt.Errorf("invalid preset name %q: use lowercase letters, digits and hyphens (max 32 chars)", name)
	}
	if _, err := Preset(name); err == nil {
		return fmt.Errorf("preset %q already exists", name)
	}
	return nil
}

// SavePresetCopy writes user preset `to`, copied from preset `from`, with an
// env's content overrides applied when ov is non-nil (same semantics as
// PresetSpec: nil list inherits, empty list means none / core only).
func SavePresetCopy(from, to, description string, ov *ProfileOverrides) error {
	spec, err := Preset(from)
	if err != nil {
		return err
	}
	if err := validateNewPresetName(to); err != nil {
		return err
	}
	spec["name"] = to
	spec["copied_from"] = from
	if description = strings.TrimSpace(description); description != "" {
		spec["description"] = description
	}
	if ov != nil {
		if ov.Groups != nil {
			groups := ov.Groups
			if len(groups) == 0 {
				groups = []string{"core"}
			}
			spec["groups"] = groups
		}
		for key, list := range map[string][]string{"skills": ov.Skills, "mcp": ov.MCP} {
			if list == nil {
				continue
			}
			spec[key] = list
			delete(spec, key+"_none")
			if len(list) == 0 {
				spec[key+"_none"] = true
			}
		}
	}
	return writeUserPreset(to, spec)
}

// RenamePreset renames a user preset and repoints every env that uses it.
// Builtins cannot be renamed (duplicate them instead).
func RenamePreset(oldName, newName string) error {
	if IsBuiltinPreset(oldName) {
		return fmt.Errorf("preset %q is built in and cannot be renamed; duplicate it instead", oldName)
	}
	spec, err := Preset(oldName)
	if err != nil {
		return err
	}
	if err := validateNewPresetName(newName); err != nil {
		return err
	}
	spec["name"] = newName
	if err := writeUserPreset(newName, spec); err != nil {
		return err
	}
	profiles, err := List()
	if err != nil {
		return err
	}
	for _, p := range profiles {
		if p.Preset != oldName {
			continue
		}
		p.Preset = newName
		if err := p.SaveManifest(profileDir(p.Name)); err != nil {
			return fmt.Errorf("repoint env %q: %w", p.Name, err)
		}
	}
	if err := os.Remove(userPresetPath(oldName)); err != nil {
		return fmt.Errorf("remove old preset file: %w", err)
	}
	return nil
}

// UpdatePresetDescription edits a user preset's description.
func UpdatePresetDescription(name, description string) error {
	if IsBuiltinPreset(name) {
		return fmt.Errorf("preset %q is built in and cannot be edited; duplicate it instead", name)
	}
	spec, err := Preset(name)
	if err != nil {
		return err
	}
	spec["description"] = strings.TrimSpace(description)
	return writeUserPreset(name, spec)
}

// DeletePreset removes a user preset. It refuses builtins and presets that
// an env still uses (switch those envs first).
func DeletePreset(name string) error {
	if IsBuiltinPreset(name) {
		return fmt.Errorf("preset %q is built in and cannot be deleted", name)
	}
	if _, err := Preset(name); err != nil {
		return err
	}
	var users []string
	if profiles, err := List(); err == nil {
		for _, p := range profiles {
			if p.Preset == name {
				users = append(users, p.Name)
			}
		}
	}
	if len(users) > 0 {
		return fmt.Errorf("preset %q is in use by env(s) %s", name, strings.Join(users, ", "))
	}
	return os.Remove(userPresetPath(name))
}

func writeUserPreset(name string, spec map[string]any) error {
	if err := os.MkdirAll(UserPresetsDir(), 0o755); err != nil {
		return fmt.Errorf("create presets dir: %w", err)
	}
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("encode preset: %w", err)
	}
	if err := os.WriteFile(userPresetPath(name), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write preset: %w", err)
	}
	return nil
}
