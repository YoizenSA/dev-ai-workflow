package agents

import (
	_ "embed"
	"os"
)

// agentsMdTemplate is the curated AGENTS.md ywai writes to the agent config
// directory during install. Scope is intentionally narrow:
//
//   - Engram memory protocol
//   - Sub-agent launch strategy
//
// Persona, SDD, skill catalogs, review hooks, and CodeGraph are NOT written
// here. CodeGraph in particular owns its own AGENTS.md marker section, written
// by its installer (see plugins.WireCodegraphMCP).
//
//go:embed agents_md_template.md
var agentsMdTemplate string

// WriteAgentsMd writes the curated AGENTS.md to the given path. ywai owns this
// file. Run it before plugin/MCP wiring so codegraph's installer can append its
// own marker section afterwards.
func WriteAgentsMd(path string) error {
	return os.WriteFile(path, []byte(agentsMdTemplate), 0o644)
}
