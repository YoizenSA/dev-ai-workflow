package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/gentlai"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/plugins"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/selfupdate"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/skills"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/tui"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/versionfile"
	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "ywai",
	Short: "One command to set up your AI dev environment",
	Long:  "ywai installs and manages your AI dev environment: engram, agent profiles, extra skills, and project tooling.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Lightweight read-only commands must not seed skills/agents/workflows
		// (and must not print "no embedded data" noise).
		if skipDataSeeding(cmd) {
			_ = versionfile.Touch(version)
			return
		}

		repo := config.RepoRoot()
		isRealRepo := config.IsOurRepoByPath(repo) && repo != config.DataDir()

		// Seed skills data if skills dir is empty
		if !config.IsDirPopulated(config.DataSkillsDir()) {
			if isRealRepo {
				if err := config.SeedSkillsFrom(repo); err != nil {
					fmt.Printf("Warning: failed to seed skills from repo: %v\n", err)
				}
				if len(config.AvailableSkills()) == 0 {
					if err := config.SeedSkillsFromEmbedded(); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
					}
				}
			} else {
				if err := config.SeedSkillsFromEmbedded(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
				}
			}

			if len(config.AvailableSkills()) == 0 && cmd.Name() != "update" {
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "Error: no skills available after seeding.")
				fmt.Fprintln(os.Stderr, "This usually means the binary was not built with embedded data.")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "Fix: reinstall ywai from the release installer:")
				fmt.Fprintln(os.Stderr, "  curl -fsSL https://github.com/YoizenSA/dev-ai-workflow/releases/latest/download/install.sh | bash")
				fmt.Fprintln(os.Stderr, "Or, from a source checkout:")
				fmt.Fprintln(os.Stderr, "  cd ywai && bash scripts/prepare-embedded.sh && go install -tags embedded ./cmd/ywai")
			}
		}

		// Seed agent profiles if agents dir is empty
		if !config.IsDirPopulated(config.DataAgentsDir()) {
			if isRealRepo {
				if err := config.SeedAgentsFrom(repo); err != nil {
					fmt.Printf("Warning: failed to seed agents from repo: %v\n", err)
				}
			}
			if !config.IsDirPopulated(config.DataAgentsDir()) {
				if err := config.SeedAgentsFromEmbedded(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to seed agent profiles: %v\n", err)
				}
			}
		}

		// Seed the bundled starter workflows (never overwrites user edits).
		if isRealRepo {
			if err := config.SeedWorkflowsFrom(repo); err != nil {
				fmt.Printf("Warning: failed to seed workflows from repo: %v\n", err)
			}
		} else {
			if err := config.SeedWorkflowsFromEmbedded(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to seed workflows: %v\n", err)
			}
		}

		// Keep ~/.ywai/version.json's installed version current for the TUI logo.
		// No network here (see versionfile.Touch); cheap enough for every command.
		_ = versionfile.Touch(version)
	},
}

func init() {
	rootCmd.Version = version
}

// skipDataSeeding reports commands that only need the binary + local DBs,
// not ~/.ywai skills/agents/workflows seeding.
func skipDataSeeding(cmd *cobra.Command) bool {
	// Walk to the leaf command name (e.g. "run" under "eval").
	names := make([]string, 0, 4)
	for c := cmd; c != nil; c = c.Parent() {
		if n := c.Name(); n != "" && n != "ywai" {
			names = append(names, n)
		}
	}
	// names are leaf→root; reverse sense via contains on the chain.
	joined := strings.Join(names, " ")
	// Direct leaves / parents we always skip.
	skip := map[string]bool{
		"eval": true, "completion": true, "help": true,
		"version": true, "stop": true, "ui": true,
	}
	for _, n := range names {
		if skip[n] {
			return true
		}
	}
	_ = joined
	return false
}

func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func detectAgents(cmd *cobra.Command) []agent.Agent {
	agentFlag, _ := cmd.Flags().GetString("agent")
	if agentFlag != "" {
		a, err := agent.FindByName(agentFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return nil
		}
		return []agent.Agent{*a}
	}

	agents := agent.FilterProfileInstallAgents(agent.Resolve())
	if len(agents) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no supported agents detected.")
		fmt.Fprintln(os.Stderr, "Supported (profile install): "+strings.Join(agent.ProfileInstallHosts, ", "))
		return nil
	}
	return agents
}

func installEcosystem(agents []agent.Agent, dryRun bool, opts gentlai.InstallOptions) {
	for _, a := range agents {
		if dryRun {
			continue
		}
		if removed, err := skills.RemoveStaleYwaiSkillLinks(a.SkillsDir); err != nil {
			fmt.Printf("  Warning: [%s] failed to clean stale legacy skill links: %v\n", a.Name, err)
		} else if len(removed) > 0 {
			fmt.Printf("  [%s] Removed stale legacy skill links: %s\n", a.Name, strings.Join(removed, ", "))
		}
	}

	if dryRun {
		fmt.Println("  Would install Engram once")
		if hosts := engramMCPHostNames(agents); len(hosts) > 0 {
			fmt.Printf("  Would wire engram MCP for %s\n", strings.Join(hosts, ", "))
		}
		return
	}
	if err := gentlai.InstallEcosystem(opts); err != nil {
		fmt.Printf("  Warning: failed to install Engram: %v\n", err)
	}

	hosts := engramMCPHostNames(agents)
	if len(hosts) == 0 {
		return
	}
	fmt.Println("  Wiring Engram MCP into agent configs...")
	if err := plugins.WireEngramMCP(hosts); err != nil {
		fmt.Printf("  Warning: %v\n", err)
		return
	}
	fmt.Printf("  ✓ engram MCP wired for %s\n", strings.Join(hosts, ", "))
}

func engramMCPHostNames(agents []agent.Agent) []string {
	var hosts []string
	for _, a := range agents {
		switch a.Name {
		case "opencode", "pi", "omp", "claude-code":
			hosts = append(hosts, a.Name)
		}
	}
	return hosts
}

// summarizeAgents prints one line per phase instead of chattering per agent.
func summarizeAgents(dryRun bool, what string, names []string) {
	if len(names) == 0 {
		return
	}
	verb := "installed"
	if dryRun {
		verb = "would install"
	}
	fmt.Printf("  %s %s for %d agents: %s\n", verb, what, len(names), strings.Join(names, ", "))
}

func copySkillsForAgents(agents []agent.Agent, dryRun bool) {
	var done []string
	for _, a := range agents {
		if skip, reason := skipSkillCopy(a, agents); skip {
			if !dryRun {
				if removed := skills.PruneYwaiSkills(a.SkillsDir); len(removed) > 0 {
					fmt.Printf("  [%s] removed %d duplicate skill(s): %s\n", a.Name, len(removed), reason)
				}
			}
			continue
		}
		if dryRun {
			done = append(done, a.Name)
			continue
		}
		if err := skills.CopyTo(a.SkillsDir); err != nil {
			fmt.Printf("  Warning: [%s] failed to copy extra skills: %v\n", a.Name, err)
			continue
		}
		done = append(done, a.Name)
	}
	summarizeAgents(dryRun, "ywai extra skills", done)
}

// skipSkillCopy reports whether a host already sees the skills through another
// host's directory, so copying them again only duplicates the catalog it loads.
//
// opencode reads ~/.agents/skills, ~/.claude/skills and its own dir (verified
// in the 1.18.29 binary), so with Claude Code installed its copy adds a second
// entry for every skill and nothing else.
//
// ponytail: one rule for the one overlap that exists today; generalize into a
// table if another host starts reading a sibling's directory.
func skipSkillCopy(a agent.Agent, all []agent.Agent) (bool, string) {
	if a.Name != "opencode" {
		return false, ""
	}
	for _, other := range all {
		if other.Name == "claude-code" {
			return true, "opencode reads ~/.claude/skills"
		}
	}
	return false, ""
}

func runTUI(agents []agent.Agent) (tui.TUIResult, error) {
	tuiAgents := make([]agent.Agent, len(agents))
	copy(tuiAgents, agents)

	return tui.Run(tuiAgents)
}

// executeInstall is kept as a thin wrapper for the shared applyManaged pipeline.
func executeInstall(opts gentlai.InstallOptions, installMCP, installMetaMCP, installPonytail bool, groupFilter agentprofiles.GroupFilter, overwriteAgents bool, autostart bool) applyResult {
	return applyManaged(applyOpts{
		Mode:            applyInstall,
		Opts:            opts,
		InstallMCP:      installMCP,
		InstallMetaMCP:  installMetaMCP,
		InstallPonytail: installPonytail,
		GroupFilter:     groupFilter,
		OverwriteAgents: overwriteAgents,
		Autostart:       autostart,
	})
}

func installAgentProfiles(agents []agent.Agent, dryRun bool, filter agentprofiles.GroupFilter, overwriteAgents bool) {
	// Read agent profiles: prefer source dir (has latest groups.json when running
	// from source checkout), fall back to seeded data dir.
	sourceDir := config.AgentsSourceDir()
	if !config.IsDirPopulated(sourceDir) {
		if err := config.SeedAgentsFromEmbedded(); err != nil {
			fmt.Printf("  Warning: no agent profiles available: %v\n", err)
			return
		}
		sourceDir = config.DataAgentsDir()
	}
	var profiles map[string]agentprofiles.AgentProfile
	var err error
	if filter.AllGroups {
		// --all-groups flag: install everything
		profiles, err = agentprofiles.LoadProfiles(sourceDir)
	} else if len(filter.Groups) == 0 {
		// Default: core + qa-automation (orchestrator stack + QA agents)
		profiles, err = agentprofiles.LoadProfilesByGroup(sourceDir, agentprofiles.GroupFilter{
			Groups: []string{"qa-automation"},
		})
	} else {
		profiles, err = agentprofiles.LoadProfilesByGroup(sourceDir, filter)
	}
	if err != nil {
		fmt.Printf("  Warning: failed to load agent profiles: %v\n", err)
		return
	}

	if len(profiles) == 0 {
		fmt.Println("  No agent profiles to install.")
		return
	}

	if dryRun {
		fmt.Printf("  Would install %d agent profiles (orchestrator, ask, dev, qa, architect, reviewer, devops)\n", len(profiles))
		return
	}

	home, _ := os.UserHomeDir()

	for _, a := range agents {
		switch a.Name {
		case "opencode":
			configPath := ""
			settingsPaths := agent.SettingsPaths()
			if p, ok := settingsPaths[a.Name]; ok && p != "" {
				configPath = p
			}
			if configPath == "" {
				continue
			}
			agentsDir := config.OpenCodeAgentsDir()

			// Install agents as markdown ONLY (no JSON fallback).
			// OpenCodeAgentsDir may resolve to a host-managed location (Orca's
			// shared hooks dir). opencode itself always reads ~/.config/opencode/
			// agents; when that is a different path it was previously only ever
			// patched in place by the delegation/permission rewriters, so its
			// prompt body went stale while its frontmatter kept being updated.
			// Write the full markdown to both.
			targets := []string{agentsDir}
			if home, err := os.UserHomeDir(); err == nil {
				canonical := filepath.Join(home, ".config", "opencode", "agents")
				if canonical != agentsDir {
					targets = append(targets, canonical)
				}
			}
			for _, target := range targets {
				if err := agentprofiles.InstallOpenCodeMarkdown(target, profiles, overwriteAgents); err != nil {
					fmt.Printf("  [%s] Warning: markdown install failed for %s: %v\n", a.Name, target, err)
				} else {
					fmt.Printf("  [%s] Agent profiles installed (markdown) → %s\n", a.Name, target)
				}
			}

			// Sweep up orphans from past installs: any .md whose frontmatter has no
			// description makes opencode reject the whole config ("Expected string |
			// undefined, got null description"). Run before applying delegations so
			// the delegation filter sees only valid installed agents.
			agentprofiles.RemoveAgentsWithoutDescription(agentsDir)

			// Apply the default delegation graph (agents/delegations.json): the
			// task map goes to opencode.json + agent markdown as v2 subagent
			// triggers are rendered into each agent's markdown prompt body.
			// Idempotent + safe to re-run.
			if doc, err := agentprofiles.LoadDelegations(sourceDir); err != nil {
				fmt.Printf("  [%s] Warning: failed to load delegations: %v\n", a.Name, err)
			} else if len(doc.Agents) > 0 {
				if err := agentprofiles.ApplyDelegations(configPath, agentsDir, doc); err != nil {
					fmt.Printf("  [%s] Warning: failed to apply delegations: %v\n", a.Name, err)
				}
			}

		case "claude-code":
			agentsDir := filepath.Join(home, ".claude", "agents")
			_ = agentprofiles.InstallClaude(agentsDir, profiles)

		case "vscode-copilot":
			promptsDir := agentprofiles.VSCodePromptsDir()
			if promptsDir != "" {
				_ = agentprofiles.InstallVSCode(promptsDir, profiles)
			}

		case "pi":
			agentsDir := filepath.Join(home, ".pi", "agent", "agents")
			if err := agentprofiles.InstallPi(agentsDir, profiles, overwriteAgents); err != nil {
				fmt.Printf("  [%s] Warning: %v\n", a.Name, err)
			} else {
				fmt.Printf("  [%s] Agent profiles installed\n", a.Name)
			}
			teamProfilesDir := filepath.Join(home, ".pi", "agent")
			if err := agentprofiles.InstallPiTeamProfiles(teamProfilesDir, profiles, overwriteAgents); err != nil {
				fmt.Printf("  [%s] Warning: teammate profiles: %v\n", a.Name, err)
			} else {
				fmt.Printf("  [%s] Teammate profiles generated\n", a.Name)
			}

		case "omp":
			// oh-my-pi: CORE agents only (fast harness host). Models via
			// `ywai tokenbank configure --agent omp`.
			agentsDir := filepath.Join(home, ".omp", "agent", "agents")
			if err := agentprofiles.InstallOmp(agentsDir, profiles, overwriteAgents); err != nil {
				fmt.Printf("  [%s] Warning: %v\n", a.Name, err)
			} else {
				fmt.Printf("  [%s] Core agent profiles installed → %s\n", a.Name, agentsDir)
			}
		}
	}
}

// selfUpdate upgrades the binary. When beta is true, uses the newest GitHub
// prerelease; otherwise uses /releases/latest (stable only).
func selfUpdate(beta bool) string {
	var (
		newVersion string
		err        error
	)
	if beta {
		fmt.Println("  Channel: beta (prerelease)")
		newVersion, err = selfupdate.RunBeta(version)
	} else {
		newVersion, err = selfupdate.Run(version)
	}
	if err != nil {
		fmt.Printf("  Warning: self-update failed: %v\n", err)
		if beta {
			fmt.Println("  Tip: pin a tag with install.sh, e.g.")
			fmt.Println("    curl -fsSL https://github.com/YoizenSA/dev-ai-workflow/releases/download/vX.Y.Z-beta.N/install.sh | bash -s -- vX.Y.Z-beta.N")
			return ""
		}
		fmt.Println("  Falling back to go install...")
		selfUpdateViaGo()
		return ""
	}

	if newVersion == "" {
		if beta {
			fmt.Println("  Already on latest beta.")
		} else {
			fmt.Println("  Already up to date.")
		}
		return ""
	}

	fmt.Printf("  Updated: %s → %s\n", version, newVersion)
	return newVersion
}

func selfUpdateViaGo() {
	_, err := os.Executable()
	if err != nil {
		return
	}

	fmt.Println("  Installing via go install with embedded data enabled...")
	cmd := exec.Command("go", "install", "-tags", "embedded", "github.com/Yoizen/dev-ai-workflow/ywai/cmd/ywai@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("  go install failed: %v\n", err)
		fmt.Println("  Try the release installer instead:")
		fmt.Println("    curl -fsSL https://github.com/YoizenSA/dev-ai-workflow/releases/latest/download/install.sh | bash")
	}
}

func reseedData() {
	os.RemoveAll(config.DataSkillsDir())
	os.RemoveAll(config.DataAgentsDir())

	if err := config.EnsureDataDir(); err != nil {
		fmt.Printf("  Warning: failed to create data directory: %v\n", err)
		return
	}

	repo := config.RepoRoot()
	isRealRepo := config.IsOurRepoByPath(repo) && repo != config.DataDir()

	// Reseed skills
	if isRealRepo {
		if err := config.SeedSkillsFrom(repo); err != nil {
			fmt.Printf("  Warning: seed skills from repo failed: %v\n", err)
		} else if len(config.AvailableSkills()) > 0 {
			fmt.Println("  Skills re-seeded from repo.")
		} else {
			fmt.Println("  Repo seed had no skills, falling back to embedded...")
			if err := config.SeedSkillsFromEmbedded(); err != nil {
				fmt.Printf("  Warning: seed skills from embedded failed: %v\n", err)
			}
		}
	} else {
		if err := config.SeedSkillsFromEmbedded(); err != nil {
			fmt.Printf("  Warning: seed skills from embedded failed: %v\n", err)
			fmt.Println("  The updated binary will seed data on next run.")
		} else {
			fmt.Println("  Skills re-seeded from embedded.")
		}
	}

	// Reseed agent profiles
	seededAgents := false
	if isRealRepo {
		if err := config.SeedAgentsFrom(repo); err == nil && config.IsDirPopulated(config.DataAgentsDir()) {
			fmt.Println("  Agent profiles re-seeded from repo.")
			seededAgents = true
		}
	}
	if !seededAgents {
		if err := config.SeedAgentsFromEmbedded(); err != nil {
			// Not fatal — agent profiles are optional
		} else {
			fmt.Println("  Agent profiles re-seeded from embedded.")
		}
	}

	// Reseed plugin bundles. The resolvers in config/data.go only seed when the
	// bundle is missing, so an upgrade used to keep serving the copy the first
	// install wrote — e.g. the v1 background-agents bundle survived the v2 port
	// and opencode v2 refused to load it.
	if err := config.SeedPluginsFromEmbedded(); err != nil {
		// Not fatal — a source checkout resolves bundles from plugins/*/dist.
	} else {
		fmt.Println("  Plugin bundles re-seeded from embedded.")
	}
}

func installPluginsForAgents(agents []agent.Agent, dryRun bool, installMCP, installMetaMCP, installPonytail bool) {
	agentSettingsPaths := agent.SettingsPaths()
	var done []string

	// sub-agent-statusline shows delegation activity in the sidebar and footer.
	// The published npm package's peer range excludes OpenCode 2, so the
	// vendored port is installed per-agent below by InstallSubagentStatusline.
	// The install used to strip the v1 entry from tui.json on every run, which
	// quietly undid the entry Engram's own installer had just written.

	// Resolve the install policy once: the ~/.ywai/plugins.json override when
	// valid, the embedded manifest otherwise. Warnings surface a bad override.
	mf, manifestWarnings := plugins.LoadManifest()
	for _, w := range manifestWarnings {
		fmt.Printf("  Warning: %v\n", w)
	}
	flags := map[string]bool{"mcp": installMCP, "meta-mcp": installMetaMCP, "ponytail": installPonytail}

	for _, a := range agents {
		// Plugin/MCP handling covers the config formats ywai writes entries
		// for: opencode-style JSON and the claude-code/pi mcpServers shape.
		if a.Name != "opencode" && a.Name != "claude-code" && a.Name != "pi" && a.Name != "omp" {
			continue
		}

		configPath, ok := agentSettingsPaths[a.Name]
		if !ok || configPath == "" {
			fmt.Printf("  [%s] No config path found, skipping plugins\n", a.Name)
			continue
		}

		if dryRun {
			done = append(done, a.Name)
			continue
		}

		// Drop MCP servers ywai used to install and has since removed, so the
		// agent stops advertising tools whose backing command is gone.
		if removed, err := plugins.RemoveRetiredMCPs(configPath, a.Name); err != nil {
			fmt.Printf("  [%s] Warning: failed to remove retired MCPs: %v\n", a.Name, err)
		} else if len(removed) > 0 {
			fmt.Printf("  [%s] Removed retired MCP(s): %s\n", a.Name, strings.Join(removed, ", "))
		}

		// Remove legacy mcp-vision MCP (replaced by vision-bridge plugin).
		if err := plugins.RemoveVisionMCP(configPath, a.Name); err != nil {
			fmt.Printf("  [%s] Warning: failed to remove mcp-vision MCP: %v\n", a.Name, err)
		}

		// What installs is policy, not code: the manifest (~/.ywai/plugins.json
		// override, embedded default otherwise) decides per id, agent, flag and
		// flavor. Executor wiring lives in internal/plugins/manifest.go.
		for _, r := range plugins.RunManifest(mf, a.Name, configPath, flags) {
			switch {
			case r.Skipped != "":
				fmt.Printf("  [%s] Skipped %s: %s\n", a.Name, r.ID, r.Skipped)
			case r.Err != nil:
				fmt.Printf("  [%s] Warning: failed to install %s: %v\n", a.Name, r.ID, r.Err)
			}
		}

		// Orca deploys a status plugin into the config dir ywai manages, and on
		// v2 it fails to load. Repair it here so the status bar is not dead
		// until Orca ships its own fix; re-applied because Orca redeploys it.
		if patched, err := plugins.RepairOrcaStatusPluginV2(configPath); err != nil {
			fmt.Printf("  [%s] Warning: %v\n", a.Name, err)
		} else if patched {
			fmt.Printf("  [%s] Repaired Orca status plugin for OpenCode v2\n", a.Name)
		}

		// Remove leftover Azure DevOps plugin entries from older installs. ywai
		// now drives Azure DevOps through the `ado` skill (Bash CLI) instead of
		// the in-process plugin, so its tools must not stay registered.
		if err := plugins.RemoveAdoPluginFromConfig(configPath, a.Name); err != nil {
			fmt.Printf("  [%s] Warning: failed to remove Azure DevOps plugin entries: %v\n", a.Name, err)
		}
		done = append(done, a.Name)
	}
	summarizeAgents(dryRun, "plugins + MCP", done)

	// Delete the standalone ADO plugin config older ywai installs wrote next to
	// opencode.json (~/.config/opencode/ado-plugin.json). Runs once, not per-agent.
	if !dryRun {
		if err := plugins.RemoveAdoPluginConfigFile(); err != nil {
			fmt.Printf("  Warning: failed to remove ADO plugin config file: %v\n", err)
		}

		// Install the `ado` CLI globally so the agent can drive Azure DevOps via
		// the `ado` skill (Bash). Non-fatal: if npm is missing or it fails, the
		// user can run `npm i -g @cioffinahuel/opencode-ado` manually later.
		fmt.Println("\n  Installing Azure DevOps CLI (`ado`)...")
		if err := plugins.InstallAdoCLI(); err != nil {
			fmt.Printf("  Warning: %v\n", err)
		} else if v, ok := plugins.AdoCLIInfo(); ok {
			if v == "" {
				fmt.Println("  ✓ ado CLI installed (version unknown)")
			} else {
				fmt.Printf("  ✓ ado CLI installed (v%s)\n", v)
			}
		}

		// Install the `graft` CLI (Graft from nanonets/graft).
		// Non-fatal: if npm is missing or the install fails, the user
		// can run `npm i -g @nanonets/graft` manually later.
		fmt.Println("\n  Installing Graft CLI (`graft`)...")
		if err := plugins.InstallGraftCLI(); err != nil {
			fmt.Printf("  Warning: %v\n", err)
		} else if v, ok := plugins.GraftInfo(); ok {
			if v == "" {
				fmt.Println("  ✓ graft CLI installed (version unknown)")
			} else {
				fmt.Printf("  ✓ graft CLI installed (v%s)\n", v)
			}
		}

		// Wire the graft MCP server into the agent config natively
		// (`graft mcp` entry written by ywai, not `graft init`), so no
		// instruction files are rewritten unexpectedly.
		fmt.Println("  Wiring Graft MCP into agent configs...")
		if err := plugins.WireGraftMCP(); err != nil {
			fmt.Printf("  Warning: %v\n", err)
		} else {
			fmt.Println("  ✓ graft MCP wired")
		}
	} else {
		fmt.Println("  Would install Azure DevOps CLI (`ado`)")
		fmt.Println("  Would install Graft CLI (`graft`)")
		fmt.Println("  Would wire Graft MCP into opencode")
	}
}

// managedDefaultAgents are the default_agent values ywai may replace with its
// own orchestrator: OpenCode's built-ins ("build" is what a fresh install
// lands on, "plan" its sibling), ywai's own profile, and the one gentle-ai
// auto-sets. None of these represents a deliberate choice by the user.
//
// Any other value does, so install leaves it alone — silently redirecting
// someone's default agent changes what every new session runs.
var managedDefaultAgents = map[string]bool{
	"":                    true,
	"build":               true,
	"plan":                true,
	"orchestrator":        true,
	"gentle-orchestrator": true,
}

func isManagedDefaultAgent(name string) bool {
	return managedDefaultAgents[strings.TrimSpace(name)]
}

// openCodeRootForWrite reads the active OpenCode config (JSONC-safe) and
// returns the parsed root, its path, and whether the file already exists. A
// missing file yields an empty root so the caller can create it.
func openCodeRootForWrite() (map[string]any, string, bool, error) {
	configDir := config.OpenCodeConfigDir()
	path := config.FindJSONCPath(configDir, "opencode")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, path, false, nil
		}
		return nil, "", false, fmt.Errorf("reading opencode config: %w", err)
	}
	cfg, err := config.ReadJSONC(path)
	if err != nil {
		return nil, "", false, fmt.Errorf("reading opencode config: %w", err)
	}
	return cfg, path, true, nil
}

// writeOpenCodeRoot writes the merged root back to the config path, creating
// the directory when needed. JSONC comments are not preserved: the file is
// rewritten as plain JSON.
func writeOpenCodeRoot(path string, cfg map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating opencode config dir: %w", err)
	}
	if err := config.WriteJSONC(path, cfg); err != nil {
		return fmt.Errorf("writing opencode config: %w", err)
	}
	return nil
}

func setDefaultAgent(agentName string, dryRun bool) error {
	cfg, path, existed, err := openCodeRootForWrite()
	if err != nil {
		return err
	}

	// Only claim the default when it is still one nobody deliberately picked:
	// OpenCode's own built-ins, ywai's own profile, or the one gentle-ai
	// auto-sets. Anything else is the user's decision and is left untouched —
	// overwriting it would silently redirect every session they start.
	if cur, ok := cfg["default_agent"]; ok {
		name, _ := cur.(string)
		if !isManagedDefaultAgent(name) {
			fmt.Printf("  default_agent already set to %q — leaving it\n", name)
			return nil
		}
		if name == agentName {
			fmt.Printf("  default_agent already %q\n", name)
			return nil
		}
	}

	cfg["default_agent"] = agentName

	if dryRun {
		fmt.Printf("  Would set default_agent to %q\n", agentName)
		return nil
	}
	if err := writeOpenCodeRoot(path, cfg); err != nil {
		return err
	}
	if existed {
		fmt.Printf("  default_agent set to %q\n", agentName)
	} else {
		fmt.Printf("  Created opencode config with default_agent=%q\n", agentName)
	}
	return nil
}

// setDefaultModel writes the root `model` key (the default model for a new
// session) when the key is absent or empty. A value the user already set is
// left untouched: it is their choice, and overwriting it would change what
// every new session runs.
func setDefaultModel(model string, dryRun bool) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}

	cfg, path, existed, err := openCodeRootForWrite()
	if err != nil {
		return err
	}

	if cur, ok := cfg["model"]; ok {
		s, isString := cur.(string)
		if !isString || strings.TrimSpace(s) != "" {
			fmt.Printf("  model already set — leaving it\n")
			return nil
		}
	}

	cfg["model"] = model

	if dryRun {
		fmt.Printf("  Would set model to %q\n", model)
		return nil
	}
	if err := writeOpenCodeRoot(path, cfg); err != nil {
		return err
	}
	if existed {
		fmt.Printf("  model set to %q\n", model)
	} else {
		fmt.Printf("  Created opencode config with model=%q\n", model)
	}
	return nil
}

// defaultRootModel returns the model ywai writes to the root `model` key: the
// active orchestrator profile's `orchestrator` role model, with the shipped
// `balanced` profile as fallback. A `#variant` suffix is stripped because the
// root key carries a plain provider/model while a variant belongs on an agent.
func defaultRootModel() string {
	if cfg, err := config.LoadConfig(); err == nil {
		if m := strings.TrimSpace(cfg.GetOrchestratorAgentModel("orchestrator")); m != "" {
			return stripModelVariant(m)
		}
	}
	seeded := config.DefaultOrchestratorModelProfiles()[config.DefaultOrchestratorModelProfileName]
	return stripModelVariant(seeded.Agents["orchestrator"].Model)
}

func stripModelVariant(model string) string {
	model = strings.TrimSpace(model)
	if i := strings.IndexByte(model, '#'); i >= 0 {
		return strings.TrimSpace(model[:i])
	}
	return model
}
