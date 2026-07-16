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
//   - CodeGraph usage rules (CODEGRAPH markers so re-wire can refresh the block)
//
// Persona, SDD, skill catalogs, and review hooks are NOT written here.
//
//go:embed agents_md_template.md
var agentsMdTemplate string

// WriteAgentsMd writes the curated AGENTS.md to the given path. ywai owns this
// file. Optional codegraph install may refresh the CODEGRAPH marker block;
// callers should still run WriteAgentsMd before plugin/MCP wiring.
func WriteAgentsMd(path string) error {
	return os.WriteFile(path, []byte(agentsMdTemplate), 0o644)
}
