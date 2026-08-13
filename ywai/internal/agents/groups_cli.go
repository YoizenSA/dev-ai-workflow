package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ErrUnknownGroup is returned when the name is not in groups.json.
var ErrUnknownGroup = fmt.Errorf("unknown group")

// ErrCoreGroupRequired is returned when trying to disable core.
var ErrCoreGroupRequired = fmt.Errorf("core group cannot be disabled")

// GroupAgentBasenames returns the installed .md basenames for a group.
func GroupAgentBasenames(sourceDir, group string) ([]string, error) {
	manifest, err := LoadGroupManifest(sourceDir)
	if err != nil {
		return nil, err
	}
	def, ok := manifest.Groups[group]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownGroup, group)
	}
	seen := map[string]bool{}
	var names []string
	for _, agentName := range def.Agents {
		base := filepath.Base(agentName)
		if seen[base] {
			continue
		}
		seen[base] = true
		names = append(names, base)
	}
	sort.Strings(names)
	return names, nil
}

// DisableGroupAgents removes that group's profile files from agentsDir.
// It never touches core.
func DisableGroupAgents(agentsDir, group string, basenames []string) ([]string, error) {
	if group == "core" {
		return nil, ErrCoreGroupRequired
	}
	var removed []string
	for _, base := range basenames {
		path := filepath.Join(agentsDir, base+".md")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := os.Remove(path); err != nil {
			return removed, fmt.Errorf("remove %s: %w", path, err)
		}
		removed = append(removed, base)
	}
	sort.Strings(removed)
	return removed, nil
}
