//go:build embedded

package main

import (
	"embed"
	"io/fs"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

//go:embed all:embedded_data
var embeddedFS embed.FS

func init() {
	skillsFS := func() fs.FS {
		sub, _ := fs.Sub(embeddedFS, "embedded_data/skills")
		return sub
	}
	config.RegisterEmbeddedProviders(skillsFS)
}
