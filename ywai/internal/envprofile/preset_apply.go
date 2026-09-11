package envprofile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// preset_apply.go — preset enforcement for profile-scoped apply.
//
// The preset stamped in the profile manifest (Profile.Preset) filters what a
// profile-scoped apply installs. Global applies (no YWAI_PROFILE) never call
// into here for behavior: every helper defaults to "no filter, keep current"
// when the spec is empty, and callers gate on InProfileScope.
//
// Preset keys (see presets/*.json): name, description, groups[], skills[],
// mcp[], default_agent, default_model, deny_bash[], cli_theme, bare,
// agents[]. Only the keys this file documents are enforced; cli_theme is
// intentionally not wired (see the follow-up note in cmd/ywai/apply.go).
//
// Opencode2-exclusive, zero V1.

// PresetSpec returns the preset stamped in p as a generic map, with the
// profile's manifest overrides applied on top: a non-nil override list
// replaces the preset list (empty means install none), a nil list inherits
// the preset. An empty preset name yields an empty spec (no filtering),
// never an error; an unknown name returns the Preset lookup error.
func PresetSpec(p Profile) (map[string]any, error) {
	name := strings.TrimSpace(p.Preset)
	if name == "" {
		return map[string]any{}, nil
	}
	spec, err := Preset(name)
	if err != nil {
		return nil, err
	}
	if p.Overrides != nil {
		if p.Overrides.Groups != nil {
			groups := append([]string(nil), p.Overrides.Groups...)
			if len(groups) == 0 {
				// core always installs, so "no groups" means core only.
				groups = []string{"core"}
			}
			spec["groups"] = groups
		}
		// An empty skills/mcp list would read as "no filter" (install all)
		// to every consumer, so it is flagged explicitly via <key>_none.
		for key, list := range map[string][]string{"skills": p.Overrides.Skills, "mcp": p.Overrides.MCP} {
			if list == nil {
				continue
			}
			spec[key] = append([]string(nil), list...)
			// A user preset may carry <key>_none; an env override replaces it.
			delete(spec, key+"_none")
			if len(list) == 0 {
				spec[key+"_none"] = true
			}
		}
	}
	return spec, nil
}

// PresetNone reports whether an env override explicitly emptied a list
// ("skills" or "mcp"): install none of it, as opposed to an empty preset
// list, which means no filter.
func PresetNone(spec map[string]any, key string) bool {
	none, _ := spec[key+"_none"].(bool)
	return none
}

// FilterableMCPIDs lists the MCP server ids the preset mcp[] allowlist can
// filter (manifest MCP entries plus graft), sorted. Other MCP wiring
// (engram, plugins) always installs.
func FilterableMCPIDs() []string {
	ids := []string{"graft"}
	for id := range mcpServerManifestIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// IsBareSpec reports whether a preset spec marks a bare environment: one
// that installs no agents, groups, skills or MCP servers.
func IsBareSpec(spec map[string]any) bool {
	if spec == nil {
		return false
	}
	bare, _ := spec["bare"].(bool)
	return bare
}

// IsBare reports whether the preset stamped in p is bare. Unknown presets
// fail closed to false so a typo never wipes an install.
func IsBare(p Profile) bool {
	spec, err := PresetSpec(p)
	if err != nil {
		return false
	}
	return IsBareSpec(spec)
}

// stringList reads spec[key] as a trimmed, non-empty string slice. It
// accepts the JSON-decoded []any form and a native []string; missing keys,
// wrong types and blank entries yield nil (no filter).
func stringList(spec map[string]any, key string) []string {
	if spec == nil {
		return nil
	}
	var out []string
	switch v := spec[key].(type) {
	case []string:
		for _, s := range v {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	case []any:
		for _, item := range v {
			s, _ := item.(string)
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// stringField reads spec[key] as a trimmed string; missing or blank yields "".
func stringField(spec map[string]any, key string) string {
	if spec == nil {
		return ""
	}
	s, _ := spec[key].(string)
	return strings.TrimSpace(s)
}

// PresetGroups returns the preset agent groups, or nil when the preset sets
// no filter (keep the caller's default).
func PresetGroups(spec map[string]any) []string { return stringList(spec, "groups") }

// PresetSkills returns the preset skills allowlist, or nil when unset.
func PresetSkills(spec map[string]any) []string { return stringList(spec, "skills") }

// PresetMCPIDs returns the preset MCP server allowlist, or nil when unset.
func PresetMCPIDs(spec map[string]any) []string { return stringList(spec, "mcp") }

// PresetDenyBash returns the preset shell-deny patterns to append to the
// profile's agent rules, or nil when unset.
func PresetDenyBash(spec map[string]any) []string { return stringList(spec, "deny_bash") }

// PresetDefaultAgent returns the preset default_agent, or "" when unset.
func PresetDefaultAgent(spec map[string]any) string { return stringField(spec, "default_agent") }

// PresetDefaultModel returns the preset default_model, or "" when unset.
func PresetDefaultModel(spec map[string]any) string { return stringField(spec, "default_model") }

// PresetCliTheme returns the preset cli_theme. Read-only helper for the
// follow-up: nothing in the apply pipeline writes it yet.
func PresetCliTheme(spec map[string]any) string { return stringField(spec, "cli_theme") }

// InProfileScope reports whether the process runs inside a profile sandbox.
func InProfileScope() bool { return strings.TrimSpace(os.Getenv("YWAI_PROFILE")) != "" }

// ScopedProfileName returns the active profile name, or "" when global.
func ScopedProfileName() string { return strings.TrimSpace(os.Getenv("YWAI_PROFILE")) }

// ScopedPreset returns the active profile and its preset spec. It errors
// when not in scope or when the profile/preset is unknown.
func ScopedPreset() (Profile, map[string]any, error) {
	name := ScopedProfileName()
	if name == "" {
		return Profile{}, nil, fmt.Errorf("no profile scope (YWAI_PROFILE unset)")
	}
	p, err := Get(name)
	if err != nil {
		return Profile{}, nil, err
	}
	spec, err := PresetSpec(p)
	if err != nil {
		return Profile{}, nil, err
	}
	return p, spec, nil
}

// mcpServerManifestIDs are the plugin-manifest entry ids that install an MCP
// server (mirrors internal/plugins/manifest.go + mcp.go). Plugin entries
// (background-agents, vision-bridge, advisor, tui-logo, ponytail) always
// install; only these ids are filtered by the preset mcp[] allowlist.
var mcpServerManifestIDs = map[string]bool{
	"chrome-devtools": true,
	"grafana":         true,
	"microsoft-learn": true,
	"meta-devtools":   true,
}

// IsMCPServerManifestID reports whether a manifest entry id installs an MCP
// server (and is therefore subject to the preset mcp[] filter).
func IsMCPServerManifestID(id string) bool { return mcpServerManifestIDs[id] }

// ShouldInstallMCP reports whether an MCP server id installs under the
// allowlist. An empty allowlist means no filter (keep current behavior).
// Callers never delete: a false return only skips the install.
func ShouldInstallMCP(serverID string, allow []string) bool {
	if len(allow) == 0 {
		return true
	}
	serverID = strings.TrimSpace(serverID)
	for _, a := range allow {
		if strings.TrimSpace(a) == serverID {
			return true
		}
	}
	return false
}

// AppendDenyBashToAgents appends shell-deny rules (action shell, effect deny)
// for patterns to every flat *.md agent file in agentsDir. Patterns already
// present in a file's frontmatter are skipped per file. It returns how many
// files changed.
//
// agentsDir must be the profile's agents dir (already sandbox-resolved by the
// caller); nothing outside it is read or written, so the global install is
// never touched. Last-match-wins makes appended denies override allows above.
func AppendDenyBashToAgents(agentsDir string, patterns []string) (int, error) {
	var want []string
	for _, p := range patterns {
		if p = strings.TrimSpace(p); p != "" {
			want = append(want, p)
		}
	}
	if len(want) == 0 {
		return 0, nil
	}
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read agents dir: %w", err)
	}
	changed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(agentsDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		updated, ok := appendShellDenies(string(data), want)
		if !ok || updated == string(data) {
			continue
		}
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return changed, fmt.Errorf("write %s: %w", path, err)
		}
		changed++
	}
	return changed, nil
}

// appendShellDenies inserts missing shell-deny rules into one agent file's
// frontmatter permissions list. ok is false when the file has no permissions
// block to extend. Existing patterns are left untouched.
func appendShellDenies(content string, patterns []string) (string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content, false
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return content, false
	}
	permIdx := -1
	for i := 1; i < end; i++ {
		if strings.TrimSpace(lines[i]) == "permissions:" {
			permIdx = i
			break
		}
	}
	if permIdx < 0 {
		return content, false
	}
	front := strings.Join(lines[1:end], "\n")
	var missing []string
	for _, p := range patterns {
		if !strings.Contains(front, p) {
			missing = append(missing, p)
		}
	}
	if len(missing) == 0 {
		return content, true
	}
	var ruleLines []string
	for _, p := range missing {
		ruleLines = append(ruleLines,
			"  - action: shell",
			fmt.Sprintf("    resource: %s", yamlQuote(p)),
			"    effect: deny",
		)
	}
	out := make([]string, 0, len(lines)+len(ruleLines))
	out = append(out, lines[:end]...)
	out = append(out, ruleLines...)
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n"), true
}

// yamlQuote quotes a scalar whenever a bare one would not survive a strict
// parser (globs, spaces, structural characters). Mirrors the agents package
// quoting so appended resources parse identically.
func yamlQuote(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, "*:#&!|>',[]{}%@`\"\\\n\r\t ") ||
		strings.HasPrefix(s, "?") || strings.HasPrefix(s, "!") ||
		strings.TrimSpace(s) != s {
		return fmt.Sprintf("%q", s)
	}
	return s
}
