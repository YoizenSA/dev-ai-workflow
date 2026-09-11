package envprofile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// opencode2's TUI and `run` never talk to OPENCODE_URL: they attach to a
// managed background service (`opencode2 serve --service`) whose port comes
// from <config>/opencode/service.json and defaults to 49374. Every env shares
// that default with the global install, so an env's TUI loops forever on
// "Starting background server..." while the global service holds the port.
// Each env therefore pins its own managed service port.

// servicePortOffset keeps the managed service clear of the env's own
// `serve --port` (5800-5899) and of VNC (5900-5999): 6800-6899.
const servicePortOffset = 1000

// ServicePort is the managed opencode2 service port of an env.
func ServicePort(p Profile) int { return p.Port + servicePortOffset }

// ServiceConfigFile is <config>/opencode/service.json of an env.
func ServiceConfigFile(p Profile) string {
	return filepath.Join(profileDir(p.Name), "config", "opencode", "service.json")
}

// EnsureServicePort writes the env's managed service port into its
// service.json, keeping every other key (opencode stores the service
// password there). A port the user set by hand is left alone.
func EnsureServicePort(p Profile) error {
	path := ServiceConfigFile(p)
	cfg := map[string]any{}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	}
	if port, ok := cfg["port"].(float64); ok && port > 0 && int(port) != defaultServicePort {
		return nil
	}
	cfg["port"] = ServicePort(p)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create service config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// defaultServicePort is opencode2's built-in managed service port: an env
// pinned to it would collide with the global install, so it is rewritten.
const defaultServicePort = 49374
