package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/autostart"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/envprofile"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/mcp"
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

Run it in a terminal and it asks what to remove: global artifacts by group,
the ~/.ywai data directory, or individual environments. Scripts keep the
classic flags: --yes runs the whole plan (plus --purge), --dry-run previews it.

Removes, for every detected agent (or just --agent):

  - vendored plugins (vision-bridge, background-agents) and their entries
    in the agent config's "plugins" array
  - ywai agent profiles installed into the agent's agents directory
  - ywai skills: one canonical copy in ~/.agents/skills, Claude compat links
    into it, and legacy per-host copies — only links into ywai's skills
    dirs, or copies carrying ywai's marker file (never a skill you wrote)
  - the autostart service, and stops a running control server

Left alone:

  - the ywai binary itself — remove it with your package manager, or
    'rm $(which ywai)'
  - gentle-ai and its ecosystem — a separate tool with its own installer
  - ~/.ywai (config, TokenBank credentials) unless you pass --purge or pick
    it in the menu
  - your global opencode sessions (~/.local/share/opencode) — never touched
  - anything you wrote yourself: unmanaged agents, real skill directories,
    and unrelated config keys are never touched

When the data directory or an environment goes away, its session database is
archived under ~/.ywai-removed-sessions (the command asks: keep or delete),
so removing ywai never costs you your sessions unless you say so.
Use --discard-sessions to skip the archive in scripts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		assumeYes, _ := cmd.Flags().GetBool("yes")
		purge, _ := cmd.Flags().GetBool("purge")
		profileName, _ := cmd.Flags().GetString("profile")
		discard, _ := cmd.Flags().GetBool("discard-sessions")

		// Profile scope: stop the environment's own service and delete the
		// environment only. Global sweeps never run on this path.
		if strings.TrimSpace(profileName) != "" {
			return runUninstallProfile(strings.TrimSpace(profileName), dryRun, assumeYes, discard)
		}

		agents := detectAgents(cmd)
		if agents == nil {
			return fmt.Errorf("no agents detected")
		}

		envs, err := envprofile.List()
		if err != nil {
			envs = nil
		}

		// Interactive mode builds the plan without the purge steps: there the
		// data directory is a menu choice, and its steps run when picked.
		interactive := !dryRun && !assumeYes && isInteractiveTerminal()
		plan := buildUninstallPlan(agents, purge && !interactive, discard)

		if len(plan) == 0 && (dryRun || !interactive || len(envs) == 0) {
			fmt.Println("Nothing to uninstall — no ywai artifacts found.")
			return nil
		}
		if len(plan) > 0 {
			printUninstallPlan(plan)
		}

		if dryRun {
			fmt.Println("\nDry run — nothing was removed.")
			return nil
		}

		if !interactive {
			if !confirmUninstall(len(plan)) {
				fmt.Println("Cancelled. Nothing was removed.")
				return nil
			}
			done, failed := runRemovals(plan)
			fmt.Printf("\nRemoved %d of %d items.\n", done, len(plan))
			if !purge {
				fmt.Println("Kept ~/.ywai (config + credentials). Pass --purge to remove it too.")
			}
			fmt.Println("The ywai binary is still installed: rm $(which ywai) to finish.")
			if failed > 0 {
				return fmt.Errorf("%d item(s) could not be removed", failed)
			}
			return nil
		}

		return runInteractiveUninstall(plan, envs, purge, discard)
	},
}

// runRemovals executes plan items, printing one line each. It returns how many
// succeeded and how many failed.
func runRemovals(plan []removal) (done, failed int) {
	for _, r := range plan {
		if err := r.apply(); err != nil {
			fmt.Printf("  ✗ %s: %v\n", r.label, err)
			failed++
			continue
		}
		fmt.Printf("  ✓ %s\n", r.label)
		done++
	}
	return done, failed
}

// selectedRemovals is what the interactive menu resolved to.
type selectedRemovals struct {
	cancelled bool
	kinds     map[removalKind]bool // plan groups picked by kind
	data      bool                 // the ~/.ywai data directory
	envs      []envprofile.Profile // environments picked one by one
}

// runInteractiveUninstall asks what to remove, then removes exactly that.
// The data-directory choice asks what happens to the environments' session
// databases before anything is deleted.
func runInteractiveUninstall(plan []removal, envs []envprofile.Profile, purge, discard bool) error {
	sel, err := selectRemovalGroups(plan, envs, purge)
	if err != nil {
		return err
	}
	if sel.cancelled {
		fmt.Println("Cancelled. Nothing was removed.")
		return nil
	}

	// Sessions are precious: before the data directory goes away, ask whether
	// to keep them (archived) or delete them — unless --discard-sessions or
	// there is nothing to ask about.
	if sel.data && !discard && hasAnyEnvSessionDB(envs) {
		del, ok := askSessionDBFate()
		if !ok {
			fmt.Println("Cancelled. Nothing was removed.")
			return nil
		}
		discard = del
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}
	archiveBase := filepath.Join(home, removedSessionsDirName)

	var done, failed int
	if len(sel.kinds) > 0 {
		var group []removal
		for _, r := range plan {
			if sel.kinds[r.kind] {
				group = append(group, r)
			}
		}
		done, failed = runRemovals(group)
	}
	if sel.data {
		d, f := runRemovals(purgeRemovals(config.DataDir(), archiveBase, discard, envs))
		done += d
		failed += f
	}
	for _, p := range sel.envs {
		if err := removeEnvironment(p, archiveBase, discard); err != nil {
			fmt.Printf("  ✗ environment %q: %v\n", p.Name, err)
			failed++
			continue
		}
		fmt.Printf("  ✓ environment %q deleted\n", p.Name)
		done++
	}

	fmt.Printf("\nRemoved %d item(s).\n", done)
	if !sel.data {
		fmt.Println("Kept ~/.ywai (config + credentials, environments).")
	}
	fmt.Println("The ywai binary is still installed: rm $(which ywai) to finish.")
	if failed > 0 {
		return fmt.Errorf("%d item(s) could not be removed", failed)
	}
	return nil
}

// removalChoice is one line of the interactive menu.
type removalChoice struct {
	id    string
	label string
	kind  removalKind // valid when this is a plan group
	data  bool        // the ~/.ywai data directory
	env   *envprofile.Profile
}

// buildRemovalMenu turns the plan plus the environments into menu lines,
// grouped the same way printUninstallPlan groups them.
func buildRemovalMenu(plan []removal, envs []envprofile.Profile) []removalChoice {
	groupLabels := map[removalKind]string{
		kindPlugin:    "vendored plugins and their config entries",
		kindConfigRef: "ywai entries in agent configs",
		kindAgent:     "ywai agent profiles",
		kindSkill:     "ywai skills",
		kindAutostart: "autostart service, control server",
	}
	counts := map[removalKind]int{}
	for _, r := range plan {
		if r.kind != kindData {
			counts[r.kind]++
		}
	}
	var menu []removalChoice
	for _, k := range []removalKind{kindPlugin, kindConfigRef, kindAgent, kindSkill, kindAutostart} {
		if counts[k] > 0 {
			menu = append(menu, removalChoice{
				id:    string(k),
				label: fmt.Sprintf("%s (%d item(s))", groupLabels[k], counts[k]),
				kind:  k,
			})
		}
	}
	if _, err := os.Stat(config.DataDir()); err == nil {
		menu = append(menu, removalChoice{
			id:    "data",
			label: "ywai data directory ~/.ywai — config, credentials and ALL environments",
			data:  true,
		})
	}
	for _, p := range envs {
		p := p
		menu = append(menu, removalChoice{
			id:    "env:" + p.Name,
			label: fmt.Sprintf("environment %q only (preset %s, port %d)", p.Name, p.Preset, p.Port),
			env:   &p,
		})
	}
	return menu
}

// selectRemovalGroups shows the menu and reads the user's pick. --purge
// pre-selects the data group. An empty answer cancels.
func selectRemovalGroups(plan []removal, envs []envprofile.Profile, purge bool) (selectedRemovals, error) {
	menu := buildRemovalMenu(plan, envs)
	fmt.Println("\nWhat should be removed?")
	fmt.Println("Enter numbers (space or comma separated), 'a' for everything, or press Enter to cancel:")
	picked := map[string]bool{}
	if purge {
		picked["data"] = true
	}
	for i, c := range menu {
		mark := " "
		if picked[c.id] {
			mark = "*"
		}
		fmt.Printf("  %s %d) %s\n", mark, i+1, c.label)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\n> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return selectedRemovals{}, fmt.Errorf("read selection: %w", err)
		}
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" {
			return selectedRemovals{cancelled: true}, nil
		}
		if line == "a" || line == "all" {
			for _, c := range menu {
				picked[c.id] = true
			}
			break
		}
		nums, perr := parseSelection(line, len(menu))
		if perr != nil {
			fmt.Printf("  %v — try again\n", perr)
			continue
		}
		picked = map[string]bool{}
		for _, n := range nums {
			picked[menu[n-1].id] = true
		}
		break
	}

	sel := selectedRemovals{kinds: map[removalKind]bool{}}
	for _, c := range menu {
		if !picked[c.id] {
			continue
		}
		switch {
		case c.data:
			sel.data = true
		case c.env != nil:
			sel.envs = append(sel.envs, *c.env)
		default:
			sel.kinds[c.kind] = true
		}
	}
	return sel, nil
}

// parseSelection parses "1 3", "1,3" and "1-3" into 1-based menu indices.
func parseSelection(line string, max int) ([]int, error) {
	var out []int
	for _, tok := range strings.Fields(strings.ReplaceAll(line, ",", " ")) {
		if lo, hi, isRange := strings.Cut(tok, "-"); isRange {
			a, e1 := strconv.Atoi(lo)
			b, e2 := strconv.Atoi(hi)
			if e1 != nil || e2 != nil || a < 1 || b < a || b > max {
				return nil, fmt.Errorf("invalid range %q", tok)
			}
			for n := a; n <= b; n++ {
				out = append(out, n)
			}
			continue
		}
		n, err := strconv.Atoi(tok)
		if err != nil || n < 1 || n > max {
			return nil, fmt.Errorf("invalid number %q (expected 1-%d)", tok, max)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("nothing selected")
	}
	return out, nil
}

// askSessionDBFate asks what happens to the environments' session databases
// when their directory goes away. ok=false means the user cancelled.
func askSessionDBFate() (delete bool, ok bool) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("\nEnvironment session databases:\n  [k] keep them, archived under ~/%s (default)\n  [d] delete them too\n  [c] cancel\n> ", removedSessionsDirName)
		line, err := reader.ReadString('\n')
		if err != nil {
			return false, false
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "", "k", "keep":
			return false, true
		case "d", "delete":
			return true, true
		case "c", "cancel":
			return false, false
		default:
			fmt.Println("  Answer k, d or c.")
		}
	}
}

// buildUninstallPlan resolves every removal without performing any of them.
func buildUninstallPlan(agents []agent.Agent, purge, discardSessions bool) []removal {
	var plan []removal
	home, err := os.UserHomeDir()
	if err != nil {
		return plan
	}
	settingsPaths := agent.SettingsPaths()

	for _, a := range agents {
		configPath := settingsPaths[a.Name]
		if a.Name == "omp" {
			// SettingsPaths points omp at models.yml, an "is configured"
			// marker in YAML. Every reader below parses JSON and fails
			// silently on it (countYwaiConfigRefs and retiredMCPsIn both
			// return zero on a parse error), so uninstall left omp's ywai
			// entries behind. Point at the file they were written to.
			if ompPath, err := mcp.EntryTargetPath("omp"); err == nil {
				configPath = ompPath
			}
		}

		// Vendored plugin bundles + their entries in the "plugins" array.
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

			// Sub-agent statusline (v2 port) installs outside ywai-plugins:
			// the server bundle lands in the auto-discovered "plugins" dir, the
			// TUI bundle in tui-plugins/, registered in the TUI client config.
			// Uninstall has never swept either directory — ywai-logo.tsx and a
			// superseded minimal statusline stay where they are — so only these
			// two files are removed, each matched by its exact name rather than
			// by sweeping the directory.
			cfgDir := filepath.Dir(configPath)
			serverBundle := filepath.Join(cfgDir, plugins.AutoDiscoveredPluginsSubdir, retiredStatuslineServerBundle)
			if _, err := os.Stat(serverBundle); err == nil {
				p := serverBundle
				plan = append(plan, removal{
					kind:  kindPlugin,
					label: fmt.Sprintf("[%s] sub-agent statusline server bundle", a.Name),
					apply: func() error { return os.Remove(p) },
				})
			}
			tuiDir := filepath.Join(cfgDir, plugins.AutoDiscoveredPluginsSubdir, plugins.SubagentStatuslineTuiPluginDir)
			if _, err := os.Stat(tuiDir); err == nil {
				p := tuiDir
				plan = append(plan, removal{
					kind:  kindPlugin,
					label: fmt.Sprintf("[%s] sub-agent statusline TUI plugin", a.Name),
					apply: func() error { return os.RemoveAll(p) },
				})
			}
			// The TUI bundle's entry in the TUI client config. cli.json is v2's
			// client config (tui.json in v1); uninstall handles it only for this
			// one entry, the same scope as the two files above.
			tuiConfigPath := filepath.Join(cfgDir, "cli.json")
			if refs := countStatuslineRefs(tuiConfigPath); refs > 0 {
				plan = append(plan, removal{
					kind:  kindConfigRef,
					label: fmt.Sprintf("[%s] %d sub-agent statusline entr(ies) in %s", a.Name, refs, tuiConfigPath),
					apply: func() error { return stripStatuslineRefs(tuiConfigPath) },
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

			// Agents installed as JSON keys rather than files. Only opencode
			// takes this path (see the install switch in root.go); every
			// other agent's config may legitimately hold an "agents" object we
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

	// The canonical dir and the legacy host dirs are shared state, not owned
	// by one agent: with --agent they would otherwise be skipped, and legacy
	// copies from pre-canonical installs live outside every current SkillsDir.
	seen := map[string]bool{}
	for _, a := range agents {
		seen[filepath.Clean(a.SkillsDir)] = true
	}
	for _, dir := range []string{
		config.AgentsSkillsDir(),
		filepath.Join(config.OpenCodeUserConfigDir(), "skills"),
		config.ClaudeSkillsDir(),
	} {
		if dir == "" || seen[filepath.Clean(dir)] {
			continue
		}
		seen[filepath.Clean(dir)] = true
		for _, skill := range ywaiSkillsIn(dir) {
			path := skill
			plan = append(plan, removal{
				kind:  kindSkill,
				label: fmt.Sprintf("[shared] skill %s (%s)", filepath.Base(path), dir),
				apply: func() error { return os.RemoveAll(path) },
			})
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
		profiles, lerr := envprofile.List()
		if lerr != nil {
			profiles = nil
		}
		plan = append(plan, purgeRemovals(config.DataDir(),
			filepath.Join(home, removedSessionsDirName), discardSessions, profiles)...)
	}

	return plan
}

// profileDirsFor returns the directories where ywai writes agent profiles as
// files. Mirrors the install switch in root.go.
//
// Agents installed as JSON keys (opencode) are handled by
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
	case "omp":
		return []string{filepath.Join(home, ".omp", "agent", "agents")}
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
// "agents" object (the opencode JSON-key install path), plus leftover v1 "agent".
func ywaiAgentKeysIn(configPath string) []string {
	return ywaiAgentKeysWith(configPath, ywaiProfileNames())
}

// ywaiAgentKeysWith is ywaiAgentKeysIn with the owner set injected.
func ywaiAgentKeysWith(configPath string, owned map[string]bool) []string {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, key := range []string{"agents", "agent"} {
		agents, _ := root[key].(map[string]any)
		for name := range agents {
			if owned[filepath.Base(name)] && !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
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
	merged := map[string]any{}
	// Migration read: merge both spellings so a user agent stored only under
	// the legacy `agent` key (v1-era ywai installs) survives the strip.
	for _, key := range []string{"agents", "agent"} {
		if agents, ok := root[key].(map[string]any); ok {
			for name, val := range agents {
				if _, exists := merged[name]; !exists {
					merged[name] = val
				}
			}
		}
	}
	// The canonical `agents` key is what opencode reads; the legacy key is
	// deleted so both spellings never coexist.
	delete(root, "agent")
	delete(root, "agents")
	for name := range merged {
		if owned[filepath.Base(name)] {
			delete(merged, name)
		}
	}
	if len(merged) > 0 {
		root["agents"] = merged
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

// ywaiSkillsIn lists skills ywai installed, by the ways install can place
// them: a link into ywai's skills directory or into the canonical
// ~/.agents/skills (Claude compat links), or a copied directory carrying the
// ".ywai-extra" marker that copyDir brings along.
//
// All tests prove ownership from the artifact itself rather than from its
// name, so a skill the user wrote is never removed even when its name collides
// with one ywai ships.
func ywaiSkillsIn(skillsDir string) []string {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}
	src := config.SkillsSourceDir()
	canonical := config.AgentsSkillsDir()
	var out []string
	for _, e := range entries {
		path := filepath.Join(skillsDir, e.Name())

		if skills.IsLinkOrJunction(path) {
			target, err := os.Readlink(path)
			if err != nil {
				continue
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}
			target = filepath.Clean(target)
			if withinDir(target, src) || withinDir(target, canonical) {
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

// withinDir reports whether path sits inside root.
func withinDir(path, root string) bool {
	root = filepath.Clean(root)
	if root == "." || root == "" {
		return false
	}
	rel, err := filepath.Rel(root, filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// ywaiSkillMarker is the file copyDir copies into every skill ywai installs.
// Keep in sync with internal/skills.extraSkillMarkerFile.
const ywaiSkillMarker = ".ywai-extra"

func ywaiPluginLists(root map[string]any) []any {
	var out []any
	seen := map[string]bool{}
	for _, key := range []string{"plugin", "plugins"} {
		list, _ := root[key].([]any)
		for _, v := range list {
			s, ok := v.(string)
			if !ok {
				out = append(out, v)
				continue
			}
			if seen[s] {
				continue
			}
			seen[s] = true
			out = append(out, v)
		}
	}
	return out
}

// countYwaiConfigRefs reports how many plugin entries in the agent config
// point into the ywai plugins directory (v1 "plugin" plus leftover v2 "plugins").
func countYwaiConfigRefs(configPath string) int {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return 0
	}
	n := 0
	for _, v := range ywaiPluginLists(root) {
		if s, ok := v.(string); ok && strings.Contains(s, "ywai-plugins") {
			n++
		}
	}
	return n
}

// stripYwaiConfigRefs drops ywai plugin entries from "plugin", drains a
// leftover v2 "plugins" array, and never writes the v2 key back.
func stripYwaiConfigRefs(configPath string) error {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", configPath, err)
	}
	list := ywaiPluginLists(root)
	delete(root, "plugins")
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

// countStatuslineRefs reports how many entries in the TUI client config point
// at the vendored sub-agent statusline TUI bundle.
func countStatuslineRefs(tuiConfigPath string) int {
	root, err := config.ReadJSONC(tuiConfigPath)
	if err != nil {
		return 0
	}
	n := 0
	for _, v := range ywaiPluginLists(root) {
		if s, ok := v.(string); ok && isSubagentStatuslineTuiEntry(s) {
			n++
		}
	}
	return n
}

// stripStatuslineRefs drops the sub-agent statusline TUI bundle entry from the
// TUI client config, preserving every other entry and key. Unlike
// stripYwaiConfigRefs it keeps the spelling the config already carried (v2
// stores TUI plugins under "plugins", v1 under "plugin") instead of
// normalizing to the v1 key.
func stripStatuslineRefs(tuiConfigPath string) error {
	root, err := config.ReadJSONC(tuiConfigPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", tuiConfigPath, err)
	}
	list := ywaiPluginLists(root)
	kept := make([]any, 0, len(list))
	for _, v := range list {
		if s, ok := v.(string); ok && isSubagentStatuslineTuiEntry(s) {
			continue
		}
		kept = append(kept, v)
	}
	if _, ok := root["plugin"]; ok {
		root["plugin"] = kept
		delete(root, "plugins")
	} else {
		root["plugins"] = kept
		delete(root, "plugin")
	}
	return config.WriteJSONC(tuiConfigPath, root)
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
	uninstallCmd.Flags().Bool("purge", false, "Also remove ~/.ywai (config, credentials and environments; session databases archived first unless --discard-sessions)")
	uninstallCmd.Flags().Bool("discard-sessions", false, "Delete environments' session databases instead of archiving them")
	uninstallCmd.Flags().String("profile", "", "Remove only an isolated environment: stop its service, archive its session database, delete it (skips all global removal)")
	rootCmd.AddCommand(uninstallCmd)
}

// removedSessionsDirName is where removed environments' session databases are
// kept. It lives beside ~/.ywai, never inside it, so --purge cannot take the
// archive down together with the rest of the data directory.
const removedSessionsDirName = ".ywai-removed-sessions"

// envSessionDBPath returns an environment's opencode session database path.
func envSessionDBPath(p envprofile.Profile) string {
	return filepath.Join(envprofile.Dirs(p)["data"], "opencode", "opencode.db")
}

// hasAnyEnvSessionDB reports whether any environment still holds a session
// database — the case that makes the keep-or-delete question worth asking.
func hasAnyEnvSessionDB(profiles []envprofile.Profile) bool {
	for _, p := range profiles {
		if _, err := os.Stat(envSessionDBPath(p)); err == nil {
			return true
		}
	}
	return false
}

// anyUnarchivedSessionDB scans the profiles tree directly, manifest or not, so
// the purge never walks past a session database it cannot name.
func anyUnarchivedSessionDB() bool {
	root := envprofile.ProfilesDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, e.Name(), "data", "opencode", "opencode.db")); err == nil {
			return true
		}
	}
	return false
}

// archiveEnvSessionDBInto moves one environment's session database and its
// SQLite sidecars into base/<env>-<timestamp>/. No database means nothing to
// keep ("", nil). A missing sidecar is fine; a failing main database move is
// an error, and callers must treat it as "do not delete anything".
func archiveEnvSessionDBInto(base string, p envprofile.Profile) (string, error) {
	src := envSessionDBPath(p)
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("stat %s: %w", src, err)
	}
	stamp := time.Now().Format("20060102-150405")
	dst := filepath.Join(base, fmt.Sprintf("%s-%s", p.Name, stamp))
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return "", fmt.Errorf("create archive dir: %w", err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		from := src + suffix
		if err := os.Rename(from, filepath.Join(dst, "opencode.db"+suffix)); err != nil {
			if os.IsNotExist(err) && suffix != "" {
				continue
			}
			return "", fmt.Errorf("archive %s: %w", from, err)
		}
	}
	return dst, nil
}

// purgeRemovals builds the end-of-life steps for the ywai data directory:
// stop every environment service (Windows holds deleted SQLite files open),
// archive their session databases unless --discard-sessions, then remove the
// directory. The archive always precedes the purge, and the purge itself
// refuses to run while an un-archived session database remains — a failed
// archive must never turn into lost sessions.
func purgeRemovals(dataDir, archiveBase string, discardSessions bool, profiles []envprofile.Profile) []removal {
	var out []removal
	if len(profiles) > 0 {
		profiles := profiles
		out = append(out, removal{
			kind:  kindAutostart,
			label: fmt.Sprintf("%d environment service(s) (stopped)", len(profiles)),
			apply: func() error {
				for _, p := range profiles {
					if err := envprofile.Stop(p); err != nil {
						return fmt.Errorf("stop environment %q: %w", p.Name, err)
					}
				}
				return nil
			},
		})
	}
	if !discardSessions && len(profiles) > 0 {
		out = append(out, removal{
			kind:  kindData,
			label: fmt.Sprintf("environment session databases (kept under ~/%s)", removedSessionsDirName),
			apply: func() error {
				for _, p := range profiles {
					dst, err := archiveEnvSessionDBInto(archiveBase, p)
					if err != nil {
						return err
					}
					if dst != "" {
						fmt.Printf("  ✓ %s sessions → %s\n", p.Name, dst)
					}
				}
				return nil
			},
		})
	}
	out = append(out, removal{
		kind:  kindData,
		label: fmt.Sprintf("ywai data directory %s (config + credentials + environments)", dataDir),
		apply: func() error {
			if !discardSessions && anyUnarchivedSessionDB() {
				return fmt.Errorf("un-archived environment session databases remain — purge aborted to protect them; resolve the archive failure and retry")
			}
			return os.RemoveAll(dataDir)
		},
	})
	return out
}

// removeEnvironment stops an environment's service, keeps (archives) or
// discards its session database, and deletes the environment directory.
func removeEnvironment(p envprofile.Profile, archiveBase string, discardSessions bool) error {
	if err := envprofile.Stop(p); err != nil {
		return fmt.Errorf("could not stop environment %q: %w", p.Name, err)
	}
	if !discardSessions {
		dst, err := archiveEnvSessionDBInto(archiveBase, p)
		if err != nil {
			return fmt.Errorf("session database not archived — deletion aborted: %w", err)
		}
		if dst != "" {
			fmt.Printf("  ✓ session database archived to %s\n", dst)
		}
	} else {
		fmt.Printf("  ! session database deleted (--discard-sessions)\n")
	}
	if err := envprofile.Delete(p.Name); err != nil {
		return fmt.Errorf("could not delete environment %q: %w", p.Name, err)
	}
	return nil
}

// runUninstallProfile removes one isolated environment. It never touches the
// global install: no agent configs, no skills, no autostart, no control
// server, no ~/.ywai purge.
func runUninstallProfile(name string, dryRun, assumeYes, discardSessions bool) error {
	p, err := envprofile.Get(name)
	if err != nil {
		return fmt.Errorf("unknown environment %q: %w", name, err)
	}

	fmt.Println("=== ywai uninstall ===")
	fmt.Printf("\nThe following environment will be removed:\n\n  - %s (preset %s, port %d)\n", p.Name, p.Preset, p.Port)
	if discardSessions {
		fmt.Println("Its session database will be deleted (--discard-sessions).")
	} else {
		fmt.Printf("Its session database is kept: archived under ~/%s.\n", removedSessionsDirName)
	}
	fmt.Println()

	if dryRun {
		fmt.Println("Dry run — nothing was removed.")
		fmt.Printf("  Would stop the %q service and delete the environment.\n", p.Name)
		return nil
	}

	if !assumeYes {
		if !isInteractiveTerminal() {
			return fmt.Errorf("uninstall needs confirmation: re-run with --yes (or --dry-run to preview)")
		}
		if !confirmUninstall(1) {
			fmt.Println("Cancelled. Nothing was removed.")
			return nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}
	if err := removeEnvironment(p, filepath.Join(home, removedSessionsDirName), discardSessions); err != nil {
		return err
	}
	fmt.Printf("  ✓ environment %q deleted\n", p.Name)
	return nil
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
// the agent's JSON config rather than as files. Mirrors the opencode case in
// root.go's install switch; keep the two in sync.
func installsAgentsAsJSONKeys(agentName string) bool {
	return agentName == "opencode"
}

// retiredMCPsIn reports which retired ywai MCP servers a config still lists, so
// the plan can name them before anything is deleted.
func retiredMCPsIn(configPath, agentName string) []string {
	root, err := config.ReadJSONC(configPath)
	if err != nil {
		return nil
	}
	key := "mcp"
	if agentName == "claude-code" || agentName == "pi" || agentName == "omp" {
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

// retiredStatuslineBundleNames are the filenames the vendored sub-agent
// statusline (retired from ywai) was installed under: the server half's
// bundle and the TUI half's loose pre-v2 file. Uninstall still recognizes
// and removes what an older release left behind.
const (
	retiredStatuslineServerBundle = "subagent-statusline-server.js"
	retiredStatuslineTuiBundle    = "subagent-statusline-tui.tsx"
)

// isSubagentStatuslineTuiEntry reports whether a TUI client config entry names
// the vendored sub-agent statusline TUI half. v2 registers the plugin
// directory the loader resolves the entry against; older installs registered
// the loose bundle file, and uninstall must still recognize those.
func isSubagentStatuslineTuiEntry(entry string) bool {
	base := filepath.Base(entry)
	return base == plugins.SubagentStatuslineTuiPluginDir || base == retiredStatuslineTuiBundle
}
