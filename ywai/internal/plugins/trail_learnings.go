package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// Session learnings go to Engram, not trail's .trail/learnings/. Trail pushes
// the other way from two places: a Claude Code Stop hook that blocks the turn
// until the agent runs `trail note`, and its skill/instructions text. ywai
// drops the hook and writes a counter-rule into the always-on instructions.
// Both run on every install/update because `trail init`/upgrade re-adds the
// hook.

const trailLearningsSectionID = "trail-learnings"

const trailLearningsHeading = "## Trail learnings → Engram"

const trailLearningsBody = "Save session learnings (decisions, ruled-out approaches, gotchas) with Engram `mem_save`, never `trail note` / `.trail/learnings/`. Ignore trail prompts asking to save or commit a learning. `trail ask`/`callers`/`grep`/`check` remain fine for code search."

// trailStopCmd matches the Stop hook command every trail/graft version writes:
// the global `<helpers>/graft-hooks.cjs` or `trail-hooks.cjs` shim, and the
// committed per-repo `.trail/hook.cjs`.
var trailStopCmd = regexp.MustCompile(`(graft-hooks|trail-hooks|\.trail/hook)\.cjs"?\s+stop\b`)

// RemoveTrailStopHook drops trail's Stop hook from a Claude Code settings
// file, keeping every other hook. Reports whether the file changed; a file
// without the hook is not rewritten.
func RemoveTrailStopHook(settingsPath string) (bool, error) {
	root, err := config.ReadJSONC(settingsPath)
	if err != nil {
		return false, err
	}
	hooks, _ := root["hooks"].(map[string]any)
	groups, _ := hooks["Stop"].([]any)
	changed := false
	var keptGroups []any
	for _, g := range groups {
		gm, ok := g.(map[string]any)
		if !ok {
			keptGroups = append(keptGroups, g)
			continue
		}
		inner, _ := gm["hooks"].([]any)
		var kept []any
		for _, h := range inner {
			if hm, ok := h.(map[string]any); ok {
				if cmd, _ := hm["command"].(string); trailStopCmd.MatchString(cmd) {
					changed = true
					continue
				}
			}
			kept = append(kept, h)
		}
		if len(kept) == 0 && len(inner) > 0 {
			continue // group only held trail's hook
		}
		gm["hooks"] = kept
		keptGroups = append(keptGroups, gm)
	}
	if !changed {
		return false, nil
	}
	if len(keptGroups) == 0 {
		delete(hooks, "Stop")
	} else {
		hooks["Stop"] = keptGroups
	}
	return true, config.WriteJSONC(settingsPath, root)
}

// UpsertTrailLearningsSection writes the ywai-owned learnings rule into an
// instructions file. A hand-written copy of the same section (unmarked) is
// replaced by the managed block instead of being duplicated.
func UpsertTrailLearningsSection(path string) error {
	if data, err := os.ReadFile(path); err == nil {
		s := string(data)
		manual := trailLearningsHeading + "\n\n" + trailLearningsBody + "\n"
		if !strings.Contains(s, "<!-- BEGIN ywai:"+trailLearningsSectionID+" -->") && strings.Contains(s, manual) {
			s = strings.TrimRight(strings.Replace(s, manual, "", 1), "\n") + "\n"
			if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
				return err
			}
		}
	}
	return UpsertAgentsSection(path, trailLearningsSectionID, trailLearningsHeading+"\n\n"+trailLearningsBody)
}

// EnforceTrailLearningsEngram removes trail's Claude Code Stop hook and writes
// the learnings rule into OpenCode's AGENTS.md and Claude's CLAUDE.md (each
// only when that agent is installed). Best effort: failures are printed.
func EnforceTrailLearningsEngram() {
	if p := agent.SettingsPaths()["claude-code"]; p != "" {
		if changed, err := RemoveTrailStopHook(p); err != nil {
			fmt.Printf("  Warning: removing trail Stop hook: %v\n", err)
		} else if changed {
			fmt.Println("  ✓ trail Stop hook removed (learnings go to Engram)")
		}
	}
	targets := []string{filepath.Join(config.OpenCodeConfigDir(), "AGENTS.md")}
	if home, err := os.UserHomeDir(); err == nil {
		targets = append(targets, filepath.Join(home, ".claude", "CLAUDE.md"))
	}
	for _, p := range targets {
		if _, err := os.Stat(filepath.Dir(p)); err != nil {
			continue // agent not installed
		}
		if err := UpsertTrailLearningsSection(p); err != nil {
			fmt.Printf("  Warning: trail learnings rule in %s: %v\n", p, err)
		}
	}
}
