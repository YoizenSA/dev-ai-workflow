package plugins

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// RemoveBrokenLegacyOpenCodePlugins deletes plugin leftovers opencode
// auto-discovers but cannot load. It must never list a bundle ywai still
// installs: background-agents.js and advisor.js are the v1 builds this branch
// ships, so removing them here would delete the plugin the installer just
// wrote and leave the agents with no delegation tools.
func RemoveBrokenLegacyOpenCodePlugins(configPath string) error {
	dir := filepath.Dir(configPath)
	for _, rel := range []string{
		"plugins/codemod-periodic-update.js", "plugins/engram.ts", "plugins/herdr-agent-state.js",
		"plugins/model-variants.ts", "plugins/review-result-artifacts.ts", "plugins/skill-registry.ts",
	} {
		if err := os.Remove(filepath.Join(dir, rel)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	root, err := config.ReadJSONC(configPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	blocked := []string{"@dietrichgebert/ponytail"}
	filtered := make([]any, 0)
	for _, raw := range openCodePlugins(root) {
		s, ok := raw.(string)
		if ok && anyContains(s, blocked) {
			continue
		}
		filtered = append(filtered, raw)
	}
	writePlugins(root, filtered)
	return config.WriteJSONC(configPath, root)
}

func anyContains(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
