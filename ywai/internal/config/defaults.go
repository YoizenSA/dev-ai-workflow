package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TUIDefaults represents the default values for the TUI installation.
type TUIDefaults struct {
	Preset     string   `json:"preset"`
	Scope      string   `json:"scope"`
	GlobalOnly bool     `json:"global_only"`
	MCP        bool     `json:"mcp"`
	Ponytail   bool     `json:"ponytail"`
	Autostart  bool     `json:"autostart"`
	Groups     []string `json:"groups"`
}

// DefaultsPath returns the path to the defaults.jsonc file.
func DefaultsPath() string {
	return filepath.Join(DataDir(), "defaults.jsonc")
}

// LoadDefaults loads the TUI defaults from ~/.ywai/defaults.jsonc.
// Falls back to embedded defaults if the file doesn't exist.
func LoadDefaults() (*TUIDefaults, error) {
	defaultsPath := DefaultsPath()

	// Try user's custom defaults first
	if _, err := os.Stat(defaultsPath); err == nil {
		data, err := os.ReadFile(defaultsPath)
		if err != nil {
			return nil, fmt.Errorf("read defaults file: %w", err)
		}
		return parseDefaults(data)
	}

	// Try embedded defaults
	data, err := GetEmbeddedDefaults()
	if err != nil {
		// Return built-in defaults
		return BuiltInDefaults(), nil
	}
	return parseDefaults(data)
}

// parseDefaults parses JSONC (strips comments) and unmarshals to TUIDefaults.
func parseDefaults(data []byte) (*TUIDefaults, error) {
	// Strip // comments (simple JSONC support)
	lines := strings.Split(string(data), "\n")
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		// Remove inline comments
		if idx := strings.Index(line, "//"); idx != -1 {
			line = line[:idx]
		}
		cleanLines = append(cleanLines, line)
	}
	clean := strings.Join(cleanLines, "\n")

	var defaults TUIDefaults
	if err := json.Unmarshal([]byte(clean), &defaults); err != nil {
		return nil, fmt.Errorf("parse defaults: %w", err)
	}

	// Apply built-in defaults for empty fields
	builtin := BuiltInDefaults()
	if defaults.Preset == "" {
		defaults.Preset = builtin.Preset
	}
	if defaults.Scope == "" {
		defaults.Scope = builtin.Scope
	}

	return &defaults, nil
}

// BuiltInDefaults returns the hard-coded default values.
func BuiltInDefaults() *TUIDefaults {
	return &TUIDefaults{
		Preset:     "full-gentleman",
		Scope:      "global",
		GlobalOnly: true,
		MCP:        false,
		Ponytail:   false,
		Autostart:  true,
		Groups:     []string{},
	}
}

// SaveDefaults saves the TUI defaults to ~/.ywai/defaults.jsonc.
func SaveDefaults(defaults *TUIDefaults) error {
	defaultsPath := DefaultsPath()

	if err := EnsureDataDir(); err != nil {
		return err
	}

	// Build JSONC with comments
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString("  // ywai install defaults\n")
	b.WriteString("  // Edit this file to customize your default installation options.\n\n")
	b.WriteString(fmt.Sprintf("  \"preset\": %q,\n", defaults.Preset))
	b.WriteString(fmt.Sprintf("  \"scope\": %q,\n", defaults.Scope))
	b.WriteString(fmt.Sprintf("  \"global_only\": %v,\n", defaults.GlobalOnly))
	b.WriteString(fmt.Sprintf("  \"mcp\": %v,\n", defaults.MCP))
	b.WriteString(fmt.Sprintf("  \"ponytail\": %v,\n", defaults.Ponytail))

	// Groups array
	b.WriteString("  \"groups\": [")
	if len(defaults.Groups) > 0 {
		b.WriteString("\n")
		for i, g := range defaults.Groups {
			comma := ","
			if i == len(defaults.Groups)-1 {
				comma = ""
			}
			b.WriteString(fmt.Sprintf("    %q%s\n", g, comma))
		}
		b.WriteString("  ")
	}
	b.WriteString("]\n")
	b.WriteString("}\n")

	return os.WriteFile(defaultsPath, []byte(b.String()), 0o644)
}

// SeedDefaultsFromEmbedded copies the embedded defaults.jsonc to the data dir
// if it doesn't exist yet.
func SeedDefaultsFromEmbedded() error {
	defaultsPath := DefaultsPath()

	// Don't overwrite user's custom file
	if _, err := os.Stat(defaultsPath); err == nil {
		return nil
	}

	data, err := GetEmbeddedDefaults()
	if err != nil {
		return nil // No embedded defaults, skip
	}

	if err := EnsureDataDir(); err != nil {
		return err
	}

	return os.WriteFile(defaultsPath, data, 0o644)
}
