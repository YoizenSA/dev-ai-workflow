package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/autostart"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/plugins"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/skills"
	"github.com/spf13/cobra"
)

// removalKind groups planned removals so the confirmation prompt reads as a
// summary instead of a wall of paths.
type removalKind string

const (
	kindPlugin    removalKind = "plugin"
	kindAgent     removalKind = "agent profile"
	kindSkill     removalKind = "skill"
	kindConfigRef removalKind = "config entry"
	kindAutostart removalKind = "autostart"
	kindData      removalKind = "ywai data"
)

// removal is one thing uninstall will delete, resolved before anything is
// touched so the whole plan can be shown and confirmed up front.
type removal struct {
	kind  removalKind
	label string // what the user sees
	apply func() error
}

// uninstallCmd reverses `ywai install`: it removes the artifacts ywai vendored
// into each agent's configuration. It never deletes files it cannot prove are
// ywai's — skill links are matched by their target, agent profiles by the
// shipped profile names, and plugin entries by their path inside the ywai
// plugins directory.
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove what ywai installed into your agents",
	Long: `Reverse a ywai install.

Removes, for every detected agent (or just --agent):

  - vendored plugins (vision-bridge, background-agents) and their entries
    in the agent config's "plugin" array
  - ywai agent profiles installed into the agent's agents directory
  - ywai skills (only links into ywai's skills dir, or copies carrying
    ywai's marker file — never a skill you wrote)
  - the autostart service, and stops a running control server

Left alone:

  - the ywai binary itself — remove it with your package manager, or
    'rm $(which ywai)'
  - gentle-ai and its ecosystem — a separate tool with its own installer
  - ~/.ywai (config, TokenBank credentials) unless you pass --purge
  - anything you wrote yourself: unmanaged agents, real skill directories,
    and unrelated config keys are never touched

The plan is printed and confirmed before anything is removed. Use --dry-run to
see it without confirming, or --yes to skip the prompt in scripts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		assumeYes, _ := cmd.Flags().GetBool("yes")
		purge, _ := cmd.Flags().GetBool("purge")

		agents := detectAgents(cmd)
		if agents == nil {
			return fmt.Errorf("no agents detected")
		}

		plan := buildUninstallPlan(agents, purge)
		if len(plan) == 0 {
			fmt.Println("Nothing to uninstall — no ywai artifacts found.")
			return nil
		}

		printUninstallPlan(plan)

		if dryRun {
			fmt.Println("\nDry run — nothing was removed.")
			return nil
		}

		if !assumeYes {
			if !isInteractiveTerminal() {
				return fmt.Errorf("uninstall needs confirmation: re-run with --yes (or --dry-run to preview)")
			}
			if !confirmUninstall(len(plan)) {
				fmt.Println("Cancelled. Nothing was removed.")
				return nil
			}
		}

		var failed int
		for _, r := range plan {
			if err := r.apply(); err != nil {
				fmt.Printf("  ✗ %s: %v\n", r.label, err)
				failed++
				continue
			}
			fmt.Printf("  ✓ %s\n", r.label)
		}

		fmt.Printf("\nRemoved %d of %d items.\n", len(plan)-failed, len(plan))
		if !purge {
			fmt.Println("Kept ~/.ywai (config + credentials). Pass --purge to remove it too.")
		}
		fmt.Println("The ywai binary is still installed: rm $(which ywai) to finish.")

		if failed > 0 {
			return fmt.Errorf("%d item(s) could not be removed", failed)
		}
		return nil
	},
}

// buildUninstallPlan resolves every removal without performing any of them.
func buildUninstallPlan(agents []agent.Agent, purge bool) []removal {
	var plan []removal
	home, err := os.UserHomeDir()
	if err != nil {
		return plan
	}
	settingsPaths := agent.SettingsPaths()

	for _, a := range agents {
		configPath := settingsPaths[a.Name]

		// Vendored plugin bundles + their entries in the "plugin" array.
		if configPath != "" {
			pluginsDir := filepath.Join(filepath.Dir(configPath), "ywai-plugins")
			if entries, err := os.ReadDir(pluginsDir); err == nil {
				for _, e := range entries {
					p := filepath.Join(pluginsDir, e.Name())
					plan = append(plan, removal{
						kind:  kindPlugin,
						label: fmt.Sprintf("[%s] plugin %s", a.Name, e.Name()),
						apply: func() error { return os.Remove(p) },
					})
				}
				plan = append(plan, removal{
					kind:  kindPlugin,
					label: fmt.Sprintf("[%s] plugins directory %s", a.Name, pluginsDir),
					apply: func() error { return os.RemoveAll(pluginsDir) },
				})
			}

			cfg := configPath
			name := a.Name
			if refs := countYwaiConfigRefs(cfg); refs > 0 {
				plan = append(plan, removal{
					kind:  kindConfigRef,
					label: fmt.Sprintf("[%s] %d ywai plugin entr(ies) in %s", name, refs, cfg),
					apply: func() error { return stripYwaiConfigRefs(cfg) },
				})
			}

			// MCP servers ywai installed for features it has since removed.
			// Uninstall is the last chance to take them out — nothing else will
			// run afterwards to clean them up.
			if retired := retiredMCPsIn(cfg, name); len(retired) > 0 {
				plan = append(plan, removal{
					kind:  kindConfigRef,
					label: fmt.Sprintf("[%s] retired MCP entr(ies) %s in %s", name, strings.Join(retired, ", "), cfg),
					apply: func() error {
						_, err := plugins.RemoveRetiredMCPs(cfg, name)
						return err
					},
				})
			}

			// Agents installed as JSON keys rather than files. Only opencode and
			// kilocode take this path (see the install switch in root.go); every
			// other agent's config may legitimately hold an "agent" object we
			// never wrote, and a name collision there must not delete it.
			if installsAgentsAsJSONKeys(name) {
				if keys := ywaiAgentKeysIn(cfg); len(keys) > 0 {
					plan = append(plan, removal{
						kind:  kindAgent,
						label: fmt.Sprintf("[%s] %d agent profile(s) in %s", name, len(keys), cfg),
						apply: func() error { return stripYwaiAgentKeys(cfg) },
					})
				}
			}
		}

		// Agent profiles ywai wrote, matched by the shipped profile names.
		for _, dir := range profileDirsFor(a.Name, home) {
			for _, f := range ywaiProfileFilesIn(dir) {
				path := f
				plan = append(plan, removal{
					kind:  kindAgent,
					label: fmt.Sprintf("[%s] agent profile %s", a.Name, filepath.Base(path)),
					apply: func() error { return os.Remove(path) },
				})
			}
		}

		// Skill links pointing into ywai's skills directory.
		if a.SkillsDir != "" {
			for _, skill := range ywaiSkillsIn(a.SkillsDir) {
				path := skill
				plan = append(plan, removal{
					kind:  kindSkill,
					label: fmt.Sprintf("[%s] skill %s", a.Name, filepath.Base(path)),
					// RemoveAll: copied skills are directories, links are not.
					apply: func() error { return os.RemoveAll(path) },
				})
			}
		}
	}

	if enabled, err := autostart.IsEnabled(); err == nil && enabled {
		plan = append(plan, removal{
			kind:  kindAutostart,
			label: "autostart service",
			apply: autostart.Disable,
		})
	}

	// A server left running keeps serving the config we are about to remove,
	// and holds the port against a later reinstall.
	if pidFile := filepath.Join(config.DataDir(), "serve.pid"); pidExists(pidFile) {
		plan = append(plan, removal{
			kind:  kindAutostart,
			label: "running control server",
			apply: func() error { return stopRunningServer(pidFile) },
		})
	}

	if purge {
		dir := config.DataDir()
		if _, err := os.Stat(dir); err == nil {
			plan = append(plan, removal{
				kind:  kindData,
				label: fmt.Sprintf("ywai data directory %s (config + credentials)", dir),
				apply: func() error { return os.RemoveAll(dir) },
			})
		}
	}

	return plan
}

// profileDirsFor returns the directories where ywai writes agent profiles as
// files. Mirrors the install switch in root.go.
//
// kilocode is deliberately absent: it installs profiles as keys inside its JSON
// config (InstallOpenCode), not as files, so it is handled by
// ywaiAgentKeysIn/stripYwaiAgentKeys instead.
func profileDirsFor(agentName, home string) []string {
	switch agentName {
	case "opencode":
		return []string{filepath.Join(home, ".config", "opencode", "agents")}
	case "claude-code":
		return []string{filepath.Join(home, ".claude", "agents")}
	case "cursor":
		return []string{filepath.Join(home, ".cursor", "agents")}
	case "pi":
		return []string{filepath.Join(home, ".pi", "agent", "agents")}
	default:
		return nil
	}
}

// ywaiProfileNames is the set of agent names ywai ships, used to tell our
// profiles apart from the user's own.
func ywaiProfileNames() map[string]bool {
	profiles, err := agentprofiles.LoadProfiles(config.AgentsSourceDir())
	if err != nil {
		return nil
	}
	owned := make(map[string]bool, len(profiles))
	for name := range profiles {
		owned[filepath.Base(name)] = true
	}
	return owned
}

// ywaiAgentKeysIn lists ywai-installed agent keys inside a JSON config's
// "agent" object (the kilocode install path).
func ywaiAgentKeysIn(configPath string) []string {
	return ywaiAgentKeysWith(configPath, ywaiProfileNames())
}

// ywaiAgentKeysWith is ywaiAgentKeysIn with the owner set injected.
func ywaiAgentKeysWith(configPath string, owned map[string]bool) []string {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return nil
	}
	agents, _ := root["agent"].(map[string]any)
	if len(agents) == 0 {
		return nil
	}
	var out []string
	for name := range agents {
		if owned[filepath.Base(name)] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// stripYwaiAgentKeys removes ywai's agent entries from a JSON config, leaving
// the user's own agents and every other key intact.
func stripYwaiAgentKeys(configPath string) error {
	return stripYwaiAgentKeysWith(configPath, ywaiProfileNames())
}

// stripYwaiAgentKeysWith is stripYwaiAgentKeys with the owner set injected.
func stripYwaiAgentKeysWith(configPath string, owned map[string]bool) error {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", configPath, err)
	}
	agents, _ := root["agent"].(map[string]any)
	if len(agents) == 0 {
		return nil
	}
	for name := range agents {
		if owned[filepath.Base(name)] {
			delete(agents, name)
		}
	}
	if len(agents) == 0 {
		delete(root, "agent")
	} else {
		root["agent"] = agents
	}
	return config.WriteJSONC(configPath, root)
}

// ywaiProfileFilesIn lists files in dir whose basename matches a profile ywai
// ships. Anything else in that directory is the user's own agent and is left
// in place.
func ywaiProfileFilesIn(dir string) []string {
	owned := ywaiProfileNames()
	if len(owned) == 0 {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		base := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if owned[base] {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	return out
}

// ywaiSkillsIn lists skills ywai installed, by the two ways install can place
// them: a link into ywai's skills directory, or a copied directory carrying the
// ".ywai-extra" marker that copyDir brings along.
//
// Both tests prove ownership from the artifact itself rather than from its
// name, so a skill the user wrote is never removed even when its name collides
// with one ywai ships.
func ywaiSkillsIn(skillsDir string) []string {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}
	src := config.SkillsSourceDir()
	var out []string
	for _, e := range entries {
		path := filepath.Join(skillsDir, e.Name())

		if skills.IsLinkOrJunction(path) {
			target, err := os.Readlink(path)
			if err != nil {
				continue
			}
			if strings.HasPrefix(filepath.Clean(target), filepath.Clean(src)) {
				out = append(out, path)
			}
			continue
		}

		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(path, ywaiSkillMarker)); err == nil {
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return out
}

// ywaiSkillMarker is the file copyDir copies into every skill ywai installs.
// Keep in sync with internal/skills.extraSkillMarkerFile.
const ywaiSkillMarker = ".ywai-extra"

// countYwaiConfigRefs reports how many "plugin" entries in the agent config
// point into the ywai plugins directory.
func countYwaiConfigRefs(configPath string) int {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return 0
	}
	list, _ := root["plugin"].([]any)
	n := 0
	for _, v := range list {
		if s, ok := v.(string); ok && strings.Contains(s, "ywai-plugins") {
			n++
		}
	}
	return n
}

// stripYwaiConfigRefs drops ywai plugin entries from the config's "plugin"
// array, leaving every other entry and every unrelated key untouched.
func stripYwaiConfigRefs(configPath string) error {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", configPath, err)
	}
	list, _ := root["plugin"].([]any)
	kept := make([]any, 0, len(list))
	for _, v := range list {
		if s, ok := v.(string); ok && strings.Contains(s, "ywai-plugins") {
			continue
		}
		kept = append(kept, v)
	}
	if len(kept) == 0 {
		delete(root, "plugin")
	} else {
		root["plugin"] = kept
	}
	return config.WriteJSONC(configPath, root)
}

// printUninstallPlan shows the plan grouped by kind.
func printUninstallPlan(plan []removal) {
	fmt.Println("=== ywai uninstall ===")
	fmt.Printf("\nThe following %d item(s) will be removed:\n\n", len(plan))

	order := []removalKind{kindPlugin, kindConfigRef, kindAgent, kindSkill, kindAutostart, kindData}
	for _, kind := range order {
		var group []string
		for _, r := range plan {
			if r.kind == kind {
				group = append(group, r.label)
			}
		}
		if len(group) == 0 {
			continue
		}
		fmt.Printf("  %s (%d):\n", kind, len(group))
		for _, label := range group {
			fmt.Printf("    - %s\n", label)
		}
		fmt.Println()
	}
}

// confirmUninstall asks for an explicit yes. Anything else cancels.
func confirmUninstall(n int) bool {
	fmt.Printf("Remove these %d item(s)? [y/N]: ", n)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func init() {
	uninstallCmd.Flags().StringP("agent", "a", "", "Limit removal to one agent (default: all detected)")
	uninstallCmd.Flags().Bool("dry-run", false, "Show what would be removed without removing it")
	uninstallCmd.Flags().Bool("yes", false, "Skip the confirmation prompt")
	uninstallCmd.Flags().Bool("purge", false, "Also remove ~/.ywai (config and TokenBank credentials)")
	rootCmd.AddCommand(uninstallCmd)
}

// pidExists reports whether a serve PID file holds a usable PID.
func pidExists(pidFile string) bool {
	_, err := readStopPIDFile(pidFile)
	return err == nil
}

// stopRunningServer sends SIGTERM and clears the PID file. An already-exited
// process is success: the goal is that nothing is left running.
func stopRunningServer(pidFile string) error {
	pid, err := readStopPIDFile(pidFile)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(pidFile) }()
	return killPIDInt(pid)
}

// installsAgentsAsJSONKeys reports whether ywai installs agent profiles into
// the agent's JSON config rather than as files. Mirrors the opencode/kilocode
// cases in root.go's install switch; keep the two in sync.
func installsAgentsAsJSONKeys(agentName string) bool {
	return agentName == "opencode" || agentName == "kilocode"
}

// retiredMCPsIn reports which retired ywai MCP servers a config still lists, so
// the plan can name them before anything is deleted.
func retiredMCPsIn(configPath, agentName string) []string {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return nil
	}
	key := "mcp"
	if agentName == "claude-code" || agentName == "pi" {
		key = "mcpServers"
	}
	mcp, _ := root[key].(map[string]any)
	if mcp == nil {
		return nil
	}
	var found []string
	for _, id := range config.RetiredMCPServers {
		if _, exists := mcp[id]; exists {
			found = append(found, id)
		}
	}
	return found
}
