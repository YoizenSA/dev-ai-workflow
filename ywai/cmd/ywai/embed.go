//go:build embedded

package main

import (
	"embed"
	"io/fs"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/unified"
)

//go:embed all:embedded_data
var embeddedFS embed.FS

func init() {
	skillsFS := func() fs.FS {
		sub, _ := fs.Sub(embeddedFS, "embedded_data/skills")
		return sub
	}
	agentsFS := func() fs.FS {
		sub, _ := fs.Sub(embeddedFS, "embedded_data/agents")
		return sub
	}
	defaultsFS := func() fs.FS {
		sub, _ := fs.Sub(embeddedFS, "embedded_data")
		return sub
	}
	uiFS := func() fs.FS {
		sub, _ := fs.Sub(embeddedFS, "embedded_data/ui")
		return sub
	}
	config.RegisterEmbeddedProviders(skillsFS)
	config.RegisterEmbeddedAgents(agentsFS)
	config.RegisterEmbeddedDefaults(defaultsFS)
	unified.RegisterEmbeddedUI(uiFS)
}
