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
	MCP       bool     `json:"mcp"`
	MetaMCP   bool     `json:"meta_mcp"`
	Ponytail  bool     `json:"ponytail"`
	Autostart bool     `json:"autostart"`
	Groups    []string `json:"groups"`
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

	return &defaults, nil
}

// BuiltInDefaults returns the hard-coded default values.
func BuiltInDefaults() *TUIDefaults {
	return &TUIDefaults{
		MCP:       false,
		Ponytail:  true,
		Autostart: true,
		Groups:    []string{},
	}
}
