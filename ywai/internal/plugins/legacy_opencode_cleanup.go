package plugins

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// RemoveBrokenLegacyOpenCodePlugins deletes known v1 plugin leftovers that
// OpenCode2 auto-discovers but cannot load.
func RemoveBrokenLegacyOpenCodePlugins(configPath string) error {
	dir := filepath.Dir(configPath)
	for _, rel := range []string{
		"plugins/codemod-periodic-update.js", "plugins/engram.ts", "plugins/herdr-agent-state.js",
		"plugins/model-variants.ts", "plugins/review-result-artifacts.ts", "plugins/skill-registry.ts",
		"ywai-plugins/background-agents.js", "ywai-plugins/advisor.js",
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
	blocked := []string{"@dietrichgebert/ponytail", "background-agents.js", "advisor.js"}
	filtered := make([]any, 0)
	for _, raw := range v2Plugins(root) {
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
