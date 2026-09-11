// Package cleanup removes artifacts retired by past ywai releases from a
// machine upgrading from an old install: dead plugin bundles, retired agents,
// skill-registry/.atl leftovers, retired hooks, and the CodeGraph CLI. It is
// the manual, one-shot replacement for the install-time sweeps ywai used to
// run on every install.
//
// Run collects every action it performs (or would perform) and returns them as
// human-readable strings. With Apply=false nothing is modified.
package cleanup

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// retiredAgents are agents removed from ywai that may still be installed by a
// previous release.
var retiredAgents = []string{"qa-finder"}

// legacyPluginFiles are plugin leftovers opencode auto-discovers but cannot
// load, relative to the opencode config directory. Never list a bundle ywai
// still installs here.
var legacyPluginFiles = []string{
	"plugins/codemod-periodic-update.js",
	"plugins/engram.ts",
	"plugins/herdr-agent-state.js",
	"plugins/model-variants.ts",
	"plugins/review-result-artifacts.ts",
	"plugins/skill-registry.ts",
}

// quotaPlugin is the retired opencode-quota npm plugin.
const quotaPlugin = "@slkiser/opencode-quota"

// retiredHookMarker matches the UserPromptSubmit hook that regenerated the
// skill registry on every prompt.
const retiredHookMarker = "skill-registry refresh"

// codegraphPackage is the retired CodeGraph indexer CLI. Graft replaced it.
const codegraphPackage = "@colbymchenry/codegraph"

// Options controls what Run cleans. Home is the machine home directory; Repo
// is walked for stray .atl directories. Apply=false runs a dry pass: the same
// actions are reported but nothing on disk changes.
type Options struct {
	Home  string
	Repo  string
	Apply bool
}

// Run executes every cleanup pass and returns the actions performed (Apply)
// or that would be performed (dry run).
func Run(opts Options) []string {
	ocDir := filepath.Join(opts.Home, ".config", "opencode")
	c := &collector{opts: opts}
	c.cleanSkillRegistry(opts.Home, opts.Repo)
	c.cleanRetiredConfigArtifacts(ocDir)
	c.cleanRetiredAgents(opts.Home)
	c.cleanLegacyPlugins(ocDir)
	c.cleanQuota(ocDir)
	c.cleanSubagentStatusline(ocDir)
	c.cleanRetiredHooks(filepath.Join(opts.Home, ".claude", "settings.json"))
	c.cleanCodegraph(opts.Repo)
	return c.actions
}

type collector struct {
	opts    Options
	actions []string
	seen    map[string]bool
}

func (c *collector) verb() string {
	if c.opts.Apply {
		return "removed"
	}
	return "would remove"
}

func (c *collector) logf(format string, a ...any) {
	c.actions = append(c.actions, fmt.Sprintf(format, a...))
}

func (c *collector) removePath(path string, desc string) {
	if c.seen == nil {
		c.seen = map[string]bool{}
	}
	if c.seen[path] {
		return
	}
	c.seen[path] = true
	if _, err := os.Lstat(path); err != nil {
		return
	}
	if c.opts.Apply {
		if err := os.RemoveAll(path); err != nil {
			fmt.Printf("  WARNING: could not remove %s: %v\n", path, err)
			return
		}
	}
	c.logf("%s %s (%s)", c.verb(), path, desc)
}

// --- skill registry + .atl -------------------------------------------------

func (c *collector) cleanSkillRegistry(home, repo string) {
	roots := []string{
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".agents", "skills"),
		filepath.Join(home, ".config", "agents", "skills"),
		filepath.Join(home, ".config", "opencode", "skills"),
		filepath.Join(home, ".kimi", "skills"),
		filepath.Join(home, ".openclaw", "skills"),
		filepath.Join(home, ".pi", "agent", "skills"),
		filepath.Join(home, ".cursor", "skills"),
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".gemini", "skills"),
		filepath.Join(home, ".copilot", "skills"),
		filepath.Join(home, ".codeium", "windsurf", "skills"),
	}
	for _, root := range roots {
		c.removePath(filepath.Join(root, "skill-registry"), "pre-v2 skill registry")
		c.removePath(filepath.Join(root, ".atl"), "skill-registry index")
	}
	for _, dir := range []string{
		filepath.Join(home, ".config", "opencode"),
		filepath.Join(home, ".claude"),
		filepath.Join(home, ".ywai"),
	} {
		c.removePath(filepath.Join(dir, ".atl"), "skill-registry index")
	}
	c.removeAtlDirs(repo)
}

func (c *collector) removeAtlDirs(root string) {
	if root == "" {
		return
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || !d.IsDir() {
			return nil
		}
		name := d.Name()
		if name == ".git" || name == "node_modules" || name == "vendor" {
			return filepath.SkipDir
		}
		if name == ".atl" {
			c.removePath(path, "skill-registry index")
			return filepath.SkipDir
		}
		return nil
	})
}

// --- retired config artifacts ----------------------------------------------

func (c *collector) cleanRetiredConfigArtifacts(ocDir string) {
	for _, rel := range []string{".atl", filepath.Join("skills", "skill-registry")} {
		c.removePath(filepath.Join(ocDir, rel), "pre-v2 skill registry artifact")
	}
}

// --- retired agents + backups ----------------------------------------------

func (c *collector) cleanRetiredAgents(home string) {
	agentsDirs := []string{
		filepath.Join(home, ".config", "opencode", "agents"),
		filepath.Join(home, ".claude", "agents"),
		filepath.Join(home, ".cursor", "agents"),
		filepath.Join(home, ".pi", "agent"),
	}
	backupDir := filepath.Join(home, ".ywai", "agent-backups")
	for _, dir := range agentsDirs {
		for _, base := range retiredAgents {
			c.removePath(filepath.Join(dir, base+".md"), "retired agent")
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".bak") {
				continue
			}
			src := filepath.Join(dir, e.Name())
			dest := filepath.Join(backupDir, e.Name())
			if c.opts.Apply {
				if err := os.MkdirAll(backupDir, 0o755); err != nil {
					fmt.Printf("  WARNING: could not create %s: %v\n", backupDir, err)
					continue
				}
				if _, err := os.Stat(dest); err == nil {
					dest = filepath.Join(backupDir, fmt.Sprintf("%s.%d.bak", strings.TrimSuffix(e.Name(), ".bak"), time.Now().UnixNano()))
				}
				if err := os.Rename(src, dest); err != nil {
					fmt.Printf("  WARNING: could not move %s: %v\n", src, err)
					continue
				}
			}
			c.logf("%s %s -> %s (agent backup)", c.verb(), src, backupDir)
		}
	}
}

// --- opencode.json plugin entries -------------------------------------------

// filterOpenCodePlugins rewrites opencode.json keeping only entries for which
// keep returns true. Missing files are ignored; unparsable files are reported
// and never touched.
func (c *collector) filterOpenCodePlugins(configPath string, keep func(entry any) bool) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		fmt.Printf("  WARNING: %s is not plain JSON (comments?), edit it by hand\n", configPath)
		return
	}
	changed := false
	for _, key := range []string{"plugins", "plugin"} {
		raw, ok := root[key]
		if !ok {
			continue
		}
		list, ok := raw.([]any)
		if !ok {
			continue
		}
		kept := make([]any, 0, len(list))
		for _, entry := range list {
			if keep(entry) {
				kept = append(kept, entry)
			} else {
				changed = true
				c.logf("%s plugin entry %v from %s", c.verb(), entry, configPath)
			}
		}
		if len(kept) == 0 && len(list) > 0 {
			delete(root, key)
			changed = true
		} else {
			root[key] = kept
		}
	}
	if !changed || !c.opts.Apply {
		return
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		fmt.Printf("  WARNING: could not re-encode %s: %v\n", configPath, err)
		return
	}
	if err := os.WriteFile(configPath, append(out, '\n'), 0o644); err != nil {
		fmt.Printf("  WARNING: could not write %s: %v\n", configPath, err)
	}
}

func entryText(entry any) string {
	switch v := entry.(type) {
	case string:
		return v
	case map[string]any:
		pkg, _ := v["package"].(string)
		return pkg
	default:
		return ""
	}
}

// cleanLegacyPlugins removes plugin bundles left by old ywai installs. This
// is cleanup of past artifacts (ywai clean's one-shot path), not v1 support:
// current installs never write these files again.
func (c *collector) cleanLegacyPlugins(ocDir string) {
	for _, rel := range legacyPluginFiles {
		c.removePath(filepath.Join(ocDir, rel), "broken legacy plugin bundle")
	}
	c.filterOpenCodePlugins(filepath.Join(ocDir, "opencode.json"), func(entry any) bool {
		return !strings.Contains(entryText(entry), "@dietrichgebert/ponytail")
	})
}

func (c *collector) cleanQuota(ocDir string) {
	c.removePath(filepath.Join(ocDir, "opencode-quota"), "opencode-quota plugin dir")
	c.filterOpenCodePlugins(filepath.Join(ocDir, "opencode.json"), func(entry any) bool {
		return entryText(entry) != quotaPlugin
	})
}

func (c *collector) cleanSubagentStatusline(ocDir string) {
	c.removePath(filepath.Join(ocDir, "plugins", "subagent-statusline-server.js"), "retired statusline bundle")
	c.removePath(filepath.Join(ocDir, "plugins", "subagent-statusline-tui"), "retired statusline bundle")
	c.removePath(filepath.Join(ocDir, "tui-plugins", "subagent-statusline-tui.tsx"), "retired statusline bundle")
	for _, name := range []string{"cli.json", "tui.json"} {
		path := filepath.Join(ocDir, name)
		c.filterOpenCodePlugins(path, func(entry any) bool {
			return !strings.Contains(entryText(entry), "subagent-statusline")
		})
	}
}

// --- retired hooks -----------------------------------------------------------

func (c *collector) cleanRetiredHooks(settingsPath string) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		fmt.Printf("  WARNING: %s is not plain JSON (comments?), edit it by hand\n", settingsPath)
		return
	}
	hooks, ok := root["hooks"].(map[string]any)
	if !ok {
		return
	}
	changed := false
	for event, raw := range hooks {
		groups, ok := raw.([]any)
		if !ok {
			continue
		}
		kept := make([]any, 0, len(groups))
		for _, g := range groups {
			if hookMatches(g, retiredHookMarker) {
				changed = true
				c.logf("%s retired %q hook group from %s", c.verb(), event, settingsPath)
				continue
			}
			kept = append(kept, g)
		}
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
	if !changed || !c.opts.Apply {
		return
	}
	if len(hooks) == 0 {
		delete(root, "hooks")
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(settingsPath, append(out, '\n'), 0o644); err != nil {
		fmt.Printf("  WARNING: could not write %s: %v\n", settingsPath, err)
	}
}

func hookMatches(group any, marker string) bool {
	g, ok := group.(map[string]any)
	if !ok {
		return false
	}
	entries, ok := g["hooks"].([]any)
	if !ok {
		return false
	}
	for _, e := range entries {
		entry, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if cmd, ok := entry["command"].(string); ok && strings.Contains(cmd, marker) {
			return true
		}
	}
	return false
}

// --- CodeGraph ----------------------------------------------------------------

func (c *collector) cleanCodegraph(repo string) {
	if _, err := exec.LookPath("codegraph"); err == nil {
		if c.opts.Apply {
			cmd := exec.Command("npm", "rm", "-g", codegraphPackage)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Printf("  WARNING: npm rm -g %s: %v\n", codegraphPackage, err)
			} else {
				c.logf("%s %s (global npm)", c.verb(), codegraphPackage)
			}
		} else {
			c.logf("%s %s (global npm)", c.verb(), codegraphPackage)
		}
	}
	c.removePath(filepath.Join(repo, ".codegraph"), "CodeGraph index")
}
