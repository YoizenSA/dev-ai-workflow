package agents

import (
	_ "embed"
	"os"
)

// agentsMdTemplate is the curated AGENTS.md ywai writes to the agent config
// directory during install. It contains only the engram protocol, skills,
// sub-agent delegation rules, and hooks — deliberately excluding SDD and
// persona, which ywai no longer installs by default.
//
//go:embed agents_md_template.md
var agentsMdTemplate string

// WriteAgentsMd writes the curated AGENTS.md to the given path. ywai owns this
// file instead of delegating to gentle-ai's sdd/persona components. The caller
// is responsible for ordering this BEFORE any component (e.g. codegraph) that
// appends its own marker-section to the file.
func WriteAgentsMd(path string) error {
	return os.WriteFile(path, []byte(agentsMdTemplate), 0o644)
}
