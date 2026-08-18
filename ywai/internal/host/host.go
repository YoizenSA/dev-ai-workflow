// Package host abstracts coding-agent runtimes (OpenCode, Pi, OMP) so the
// control UI and install paths can target more than one product.
package host

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ID is a stable runtime identifier used in APIs and CLI flags.
type ID string

const (
	OpenCode ID = "opencode"
	Pi       ID = "pi"
	OMP      ID = "omp"
	Claude   ID = "claude-code"
)

// All is the ordered list of hosts ywai treats as first-class for the web.
var All = []ID{OpenCode, Pi, OMP, Claude}

// Info is a serializable snapshot for health / settings.
type Info struct {
	ID         ID     `json:"id"`
	Detected   bool   `json:"detected"`
	Binary     string `json:"binary,omitempty"`
	AgentsDir  string `json:"agentsDir,omitempty"`
	MCPPath    string `json:"mcpPath,omitempty"`
	ModelsPath string `json:"modelsPath,omitempty"`
	// Capabilities the control plane can use for this host.
	Chat        bool `json:"chat"`        // live session proxy (OpenCode only today)
	WorkflowRun bool `json:"workflowRun"` // non-interactive workflow spawn
	Evals       bool `json:"evals"`       // session analytics DB
	MCP         bool `json:"mcp"`
	Profiles    bool `json:"profiles"` // write model: into agent markdown
	Team        bool `json:"team"`
}

// Home returns the user home directory or ".".
func Home() string {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "."
	}
	return h
}

// AgentsDir is where ywai installs agent markdown for the host.
func AgentsDir(id ID) string {
	h := Home()
	switch id {
	case OpenCode:
		return filepath.Join(h, ".config", "opencode", "agents")
	case Pi:
		return filepath.Join(h, ".pi", "agent", "agents")
	case OMP:
		return filepath.Join(h, ".omp", "agent", "agents")
	case Claude:
		return filepath.Join(h, ".claude", "agents")
	default:
		return ""
	}
}

// MCPConfigPath is the MCP server config file for the host (if any).
func MCPConfigPath(id ID) string {
	h := Home()
	switch id {
	case OpenCode:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "opencode", "opencode.json")
		}
		return filepath.Join(h, ".config", "opencode", "opencode.json")
	case Pi:
		return filepath.Join(h, ".pi", "agent", "mcp.json")
	case OMP:
		// Mirror Pi layout under ~/.omp/agent/
		return filepath.Join(h, ".omp", "agent", "mcp.json")
	case Claude:
		return filepath.Join(h, ".claude.json")
	default:
		return ""
	}
}

// ModelsPath is the host models/provider config (TokenBank writes here).
func ModelsPath(id ID) string {
	h := Home()
	switch id {
	case OpenCode:
		return MCPConfigPath(OpenCode) // providers live in opencode.json
	case Pi:
		return filepath.Join(h, ".pi", "agent", "models.json")
	case OMP:
		return filepath.Join(h, ".omp", "agent", "models.yml")
	default:
		return ""
	}
}

// BinaryName is the PATH binary for the host.
func BinaryName(id ID) string {
	switch id {
	case OpenCode:
		return "opencode2"
	case Pi:
		return "pi"
	case OMP:
		return "omp"
	case Claude:
		return "claude"
	default:
		return ""
	}
}

// FindBinary resolves the host CLI on PATH, or "".
func FindBinary(id ID) string {
	name := BinaryName(id)
	if name == "" {
		return ""
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return p
}

// Detected reports whether the host looks installed (binary and/or config tree).
func Detected(id ID) bool {
	if FindBinary(id) != "" {
		return true
	}
	// Config-dir fallback (same idea as agent.Detect).
	switch id {
	case OMP:
		for _, f := range []string{"models.yml", "models.yaml", "config.yml", "mcp.json"} {
			if _, err := os.Stat(filepath.Join(Home(), ".omp", "agent", f)); err == nil {
				return true
			}
		}
	case Pi:
		if _, err := os.Stat(filepath.Join(Home(), ".pi", "agent")); err == nil {
			return true
		}
	case OpenCode:
		if _, err := os.Stat(filepath.Join(Home(), ".config", "opencode")); err == nil {
			return true
		}
	case Claude:
		if _, err := os.Stat(filepath.Join(Home(), ".claude")); err == nil {
			return true
		}
	}
	return false
}

// DetectedList returns Info for every known host.
func DetectedList() []Info {
	out := make([]Info, 0, len(All))
	for _, id := range All {
		out = append(out, Snapshot(id))
	}
	return out
}

// Snapshot builds capability + path info for one host.
func Snapshot(id ID) Info {
	info := Info{
		ID:         id,
		Detected:   Detected(id),
		Binary:     FindBinary(id),
		AgentsDir:  AgentsDir(id),
		MCPPath:    MCPConfigPath(id),
		ModelsPath: ModelsPath(id),
		Profiles:   true, // all markdown hosts accept model: frontmatter writes
		MCP:        MCPConfigPath(id) != "",
	}
	switch id {
	case OpenCode:
		info.Chat = true
		info.WorkflowRun = true
		info.Evals = true
		info.Team = false
	case Pi:
		info.Chat = false
		info.WorkflowRun = true // one-shot CLI
		info.Evals = false
		info.Team = true
	case OMP:
		info.Chat = false
		info.WorkflowRun = true // omp -p
		info.Evals = false
		info.Team = false
	case Claude:
		info.Chat = false
		info.WorkflowRun = false
		info.Evals = false
		info.Team = false
	}
	return info
}

// ParseID validates a host id string.
func ParseID(s string) (ID, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch ID(s) {
	case OpenCode, Pi, OMP, Claude:
		return ID(s), nil
	case "oh-my-pi":
		return OMP, nil
	case "":
		return OpenCode, nil // default runtime for the control plane
	default:
		return "", fmt.Errorf("unknown host %q (want opencode|pi|omp|claude-code)", s)
	}
}

// AgentsDirs returns non-empty agent directories for hosts (for multi-list).
func AgentsDirs(ids ...ID) []string {
	if len(ids) == 0 {
		ids = All
	}
	var out []string
	for _, id := range ids {
		d := AgentsDir(id)
		if d == "" {
			continue
		}
		if _, err := os.Stat(d); err == nil {
			out = append(out, d)
		}
	}
	return out
}

// WorkflowExportTarget maps a host to the workflows.Exporter target string.
func WorkflowExportTarget(id ID) string {
	switch id {
	case Claude:
		return "claude-code"
	case Pi, OMP:
		// Pi-style markdown agents (name/description/tools).
		return "pi"
	default:
		return "opencode"
	}
}
