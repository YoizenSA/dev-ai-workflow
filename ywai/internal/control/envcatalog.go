package control

import (
	"sort"

	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/envprofile"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/skills"
)

// envGroupOption is one agent group the Envs editor can pick.
type envGroupOption struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Agents      int    `json:"agents"`
}

// envCatalog lists what an env override can choose from, so the web editor
// offers real options instead of free text: agent groups from groups.json,
// ywai extra skills, and the MCP servers the preset filter can skip. A
// missing source yields an empty list, never an error.
func envCatalog() map[string]any {
	groups := []envGroupOption{}
	if m, err := agentprofiles.LoadGroupManifest(config.AgentsSourceDir()); err == nil {
		for name, def := range m.Groups {
			groups = append(groups, envGroupOption{Name: name, Description: def.Description, Agents: len(def.Agents)})
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		// core first: it always installs.
		if (groups[i].Name == "core") != (groups[j].Name == "core") {
			return groups[i].Name == "core"
		}
		return groups[i].Name < groups[j].Name
	})
	skillMeta, err := skills.ListAvailableMeta()
	if err != nil || skillMeta == nil {
		skillMeta = []skills.SkillMeta{}
	}
	return map[string]any{
		"groups": groups,
		"skills": skillMeta,
		"mcp":    envprofile.FilterableMCPIDs(),
	}
}
