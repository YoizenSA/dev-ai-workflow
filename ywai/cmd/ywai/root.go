package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/gentlai"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/overrides"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/plugins"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/selfupdate"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/skills"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/tui"
	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "ywai",
	Short: "One command to set up your AI dev environment",
	Long:  "ywai wraps gentle-ai and adds extra skills, project templates, and one-command install.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
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
	},
}

func init() {
	rootCmd.Version = version
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

	agents := agent.Resolve()
	if len(agents) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no supported agents detected in PATH.")
		fmt.Fprintln(os.Stderr, "Supported: opencode, claude-code, cursor, windsurf, gemini-cli, vscode-copilot, codex")
		return nil
	}
	return agents
}

func installEcosystem(agents []agent.Agent, dryRun bool, opts gentlai.InstallOptions) {
	for _, a := range agents {
		configDir := filepath.Dir(a.SkillsDir)
		if skills.IsLinkOrJunction(configDir) {
			fmt.Printf("  Warning: [%s] gentle-ai install skipped because config dir is a symlink/junction: %s\n", a.Name, configDir)
			fmt.Println("    gentle-ai currently refuses to atomically write through linked config directories; leaving existing upstream skills untouched.")
			continue
		}
		if dryRun {
			fmt.Printf("  [%s] Would remove stale legacy skill links that block gentle-ai...\n", a.Name)
			fmt.Printf("  [%s] Running gentle-ai install...\n", a.Name)
			continue
		}
		if removed, err := skills.RemoveStaleYwaiSkillLinks(a.SkillsDir); err != nil {
			fmt.Printf("  Warning: [%s] failed to clean stale legacy skill links: %v\n", a.Name, err)
		} else if len(removed) > 0 {
			fmt.Printf("  [%s] Removed stale legacy skill links: %s\n", a.Name, strings.Join(removed, ", "))
		}
		agentOpts := opts
		agentOpts.AgentName = a.Name
		if err := gentlai.InstallEcosystem(agentOpts); err != nil {
			fmt.Printf("  Warning: ecosystem install failed for %s: %v\n", a.Name, err)
		}
	}
}

func copySkillsForAgents(agents []agent.Agent, dryRun bool) {
	fmt.Println("  Copying all ywai extra skills.")

	for _, a := range agents {
		if dryRun {
			fmt.Printf("  [%s] Copying extra skills to %s...\n", a.Name, a.SkillsDir)
			continue
		}

		_ = skills.CopyTo(a.SkillsDir)
		fmt.Printf("  [%s] Copied extra skills to %s\n", a.Name, a.SkillsDir)
	}
}

func runTUI(agents []agent.Agent) (tui.TUIResult, error) {
	tuiAgents := make([]agent.Agent, len(agents))
	copy(tuiAgents, agents)

	return tui.Run(tuiAgents)
}

func executeInstall(opts gentlai.InstallOptions, installMCP bool, globalOnly bool, installADO bool, groupFilter agentprofiles.GroupFilter, overwriteAgents bool) {
	var agents []agent.Agent
	if opts.AgentName != "" {
		a, err := agent.FindByName(opts.AgentName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}
		agents = []agent.Agent{*a}
	} else {
		agents = agent.Resolve()
		if len(agents) == 0 {
			fmt.Fprintln(os.Stderr, "Error: no supported agents detected.")
			return
		}
	}

	// When global-only, run gentle-ai from a neutral directory so it does not
	// write workspace skills (skills/, .sdd/, AGENTS.md) into the current project.
	if globalOnly {
		neutralDir := filepath.Join(config.DataDir(), "global-workspace")
		if err := os.MkdirAll(neutralDir, 0o755); err != nil {
			fmt.Printf("  Warning: failed to create global-workspace dir: %v\n", err)
		} else {
			opts.WorkDir = neutralDir
		}
	}

	fmt.Println("=== ywai install ===")
	if opts.DryRun {
		fmt.Println("\n[DRY RUN] No changes will be made.")
	}
	if globalOnly {
		fmt.Println("  Global-only: gentle-ai will not write into the current project.")
	}

	fmt.Println("\n[1/3] Checking gentle-ai...")
	if opts.DryRun {
		fmt.Println("  Would install or update gentle-ai if needed.")
	} else {
		if err := gentlai.Install(); err != nil {
			fmt.Printf("  Warning: gentle-ai install/update failed: %v\n", err)
		}
		reseedData()
	}

	fmt.Println("\n[2/3] Detecting agents...")
	for _, a := range agents {
		fmt.Printf("  Found: %s (%s)\n", a.Name, a.BinaryName)
	}

	fmt.Println("\n[3/3] Installing ecosystem + copying extra skills...")
	installEcosystem(agents, opts.DryRun, opts)
	copySkillsForAgents(agents, opts.DryRun)

	if !opts.DryRun {
		agentDirs := make(map[string]string)
		for _, a := range agents {
			agentDirs[a.Name] = a.SkillsDir
		}

		fmt.Println("\n[3.5/3] Installing agent profiles...")
		installAgentProfiles(agents, opts.DryRun, groupFilter, overwriteAgents)

		fmt.Println("\n[3.6/3] Applying ywai overrides...")
		_ = overrides.ApplyOpenSpecToSDDOverride(agentDirs)

		fmt.Println("\n[3.7/3] Installing plugins...")
		installPluginsForAgents(agents, opts.DryRun, installMCP, installADO)

		fmt.Println("\n[3.8/3] Removing deprecated opencode-quota plugin...")
		removeQuotaForAgents(agents, opts.DryRun)

		fmt.Println("\n[3.9/3] Setting default_agent...")
		if err := setDefaultAgent("orchestrator", opts.DryRun); err != nil {
			fmt.Printf("  Warning: failed to set default_agent: %v\n", err)
		}
	}

	fmt.Println("\n=== Done! ===")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Open your AI agent in this project")
	fmt.Println("  2. Run `ywai skills` to see available skills")
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
		// No groups specified: install core only
		profiles, err = agentprofiles.LoadProfilesByGroup(sourceDir, agentprofiles.GroupFilter{Groups: []string{}})
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
			agentsDir := filepath.Join(home, ".config", "opencode", "agents")

			// Migrate existing agents from JSON to markdown
			if err := agentprofiles.MigrateOpenCodeAgents(configPath, agentsDir); err != nil {
				fmt.Printf("  [%s] Warning: migration failed: %v\n", a.Name, err)
			}

			// Install agents as markdown ONLY (no JSON fallback)
			if err := agentprofiles.InstallOpenCodeMarkdown(agentsDir, profiles, overwriteAgents); err != nil {
				fmt.Printf("  [%s] Warning: markdown install failed: %v\n", a.Name, err)
			} else {
				fmt.Printf("  [%s] Agent profiles installed (markdown)\n", a.Name)
			}

		case "kilocode":
			configPath := ""
			settingsPaths := agent.SettingsPaths()
			if p, ok := settingsPaths[a.Name]; ok && p != "" {
				configPath = p
			}
			if configPath == "" {
				continue
			}
			if err := agentprofiles.InstallOpenCode(configPath, profiles); err != nil {
				fmt.Printf("  [%s] Warning: %v\n", a.Name, err)
			} else {
				fmt.Printf("  [%s] Agent profiles installed\n", a.Name)
			}

		case "claude-code":
			agentsDir := filepath.Join(home, ".claude", "agents")
			_ = agentprofiles.InstallClaude(agentsDir, profiles)

		case "cursor":
			agentsDir := filepath.Join(home, ".cursor", "agents")
			_ = agentprofiles.InstallCursor(agentsDir, profiles)

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
		}
	}
}

func selfUpdate() {
	newVersion, err := selfupdate.Run(version)
	if err != nil {
		fmt.Printf("  Warning: self-update failed: %v\n", err)
		fmt.Println("  Falling back to go install...")
		selfUpdateViaGo()
		return
	}

	if newVersion == "" {
		fmt.Println("  Already up to date.")
		return
	}

	fmt.Printf("  Updated: %s → %s\n", version, newVersion)
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
}

func installPluginsForAgents(agents []agent.Agent, dryRun bool, installMCP bool, installADO bool) {
	agentSettingsPaths := agent.SettingsPaths()

	// Install sub-agent-statusline TUI plugin (global config, not per-agent)
	if !dryRun {
		if err := plugins.InstallSubAgentStatusline(); err != nil {
			fmt.Printf("  Warning: failed to install sub-agent-statusline plugin: %v\n", err)
		}
	} else {
		fmt.Println("  Would install sub-agent-statusline TUI plugin")
	}

	for _, a := range agents {
		// Install MCP for agents that support it
		if a.Name != "opencode" && a.Name != "kilocode" && a.Name != "claude-code" && a.Name != "pi" {
			continue
		}

		configPath, ok := agentSettingsPaths[a.Name]
		if !ok || configPath == "" {
			fmt.Printf("  [%s] No config path found, skipping plugins\n", a.Name)
			continue
		}

		// background-agents is an opencode plugin (delegate/delegation_* async
		// tools); it only applies to opencode-format configs (opencode/kilocode).
		supportsOpenCodePlugins := a.Name == "opencode" || a.Name == "kilocode"

		if dryRun {
			fmt.Printf("  [%s] Would install ywai-kanban MCP\n", a.Name)
			if supportsOpenCodePlugins {
				fmt.Printf("  [%s] Would install background-agents plugin\n", a.Name)
			}
			if installMCP {
				fmt.Printf("  [%s] Would install Microsoft Learn MCP\n", a.Name)
			}
			if installADO {
				fmt.Printf("  [%s] Would install Azure DevOps plugin\n", a.Name)
			}
			continue
		}

		// Install kanban MCP (always, required for orchestrator)
		if err := plugins.InstallKanbanMCP(configPath, a.Name); err != nil {
			fmt.Printf("  [%s] Warning: failed to install ywai-kanban MCP: %v\n", a.Name, err)
		} else {
			fmt.Printf("  [%s] Installed ywai-kanban MCP\n", a.Name)
		}

		// Install background-agents plugin (async delegation) for opencode-format configs.
		if supportsOpenCodePlugins {
			if err := plugins.InstallBackgroundAgents(configPath); err != nil {
				fmt.Printf("  [%s] Warning: failed to install background-agents plugin: %v\n", a.Name, err)
			} else {
				fmt.Printf("  [%s] Installed background-agents plugin\n", a.Name)
			}
		}

		// Install Microsoft Learn MCP if requested
		if installMCP {
			if err := plugins.InstallMicrosoftLearnMCP(configPath, a.Name); err != nil {
				fmt.Printf("  [%s] Warning: failed to install Microsoft Learn MCP: %v\n", a.Name, err)
			} else {
				fmt.Printf("  [%s] Installed Microsoft Learn MCP\n", a.Name)
			}
		}
	}

	// Install ADO plugin for opencode + pi
	if installADO && !dryRun {
		fmt.Println("\n  Configuring Azure DevOps plugin...")

		// 1. Interactive setup (collects org, project, repos, PAT)
		if err := plugins.InstallADOFromSetup(); err != nil {
			fmt.Printf("  Warning: ADO setup failed: %v\n", err)
			fmt.Println("  You can configure it later with: npx @nahuelcio/opencode-ado init")
		}

		// 2. Read back the config we just wrote
		adoConfig, _ := plugins.ReadExistingADOConfig()
		if adoConfig == nil {
			adoConfig = &plugins.ADOPluginConfig{}
		}

		// 3. Add to opencode config
		for _, a := range agents {
			if a.Name != "opencode" && a.Name != "kilocode" {
				continue
			}
			if _, ok := agentSettingsPaths[a.Name]; !ok {
				continue
			}
			if err := plugins.InstallADOOpenCode(*adoConfig); err != nil {
				fmt.Printf("  [%s] Warning: failed to install ADO plugin: %v\n", a.Name, err)
			} else {
				fmt.Printf("  [%s] ✓ Azure DevOps plugin installed\n", a.Name)
			}
		}

		// 4. Add to Pi
		home, _ := os.UserHomeDir()
		piSettingsPath := filepath.Join(home, ".pi", "agent", "settings.json")
		if _, err := os.Stat(piSettingsPath); err == nil {
			if err := plugins.InstallADOPi(piSettingsPath); err != nil {
				fmt.Printf("  [pi] Warning: failed to install ADO plugin: %v\n", err)
			} else {
				fmt.Println("  [pi] ✓ Azure DevOps plugin installed")
			}
		} else {
			fmt.Println("  [pi] Skipped — Pi not configured yet")
		}

		fmt.Println("  Azure DevOps plugin setup complete!")
	}
}

func removeQuotaForAgents(agents []agent.Agent, dryRun bool) {
	agentSettingsPaths := agent.SettingsPaths()

	for _, a := range agents {
		// Only remove quota for opencode
		if a.Name != "opencode" && a.Name != "kilocode" && a.Name != "claude-code" {
			continue
		}

		configPath, ok := agentSettingsPaths[a.Name]
		if !ok || configPath == "" {
			continue
		}

		if dryRun {
			fmt.Printf("  [%s] Would remove opencode-quota plugin\n", a.Name)
			continue
		}

		if err := plugins.RemoveQuota(configPath); err != nil {
			fmt.Printf("  [%s] Warning: failed to remove opencode-quota: %v\n", a.Name, err)
		} else {
			fmt.Printf("  [%s] Removed opencode-quota plugin\n", a.Name)
		}
	}
}

func setDefaultAgent(agentName string, dryRun bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(home, ".config", "opencode")
	path := config.FindJSONCPath(configDir, "opencode")

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading opencode config: %w", err)
		}
		// Config file does not exist — create it with default_agent.
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return fmt.Errorf("creating opencode config dir: %w", err)
		}
		cfg := map[string]any{"default_agent": agentName}
		updated, mErr := json.MarshalIndent(cfg, "", "\t")
		if mErr != nil {
			return mErr
		}
		if dryRun {
			fmt.Printf("  Would set default_agent to %q\n", agentName)
			return nil
		}
		if wErr := os.WriteFile(path, append(updated, '\n'), 0o644); wErr != nil {
			return fmt.Errorf("writing opencode config: %w", wErr)
		}
		fmt.Printf("  Created opencode config with default_agent=%q\n", agentName)
		return nil
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing opencode.json: %w", err)
	}

	if _, ok := cfg["default_agent"]; ok {
		fmt.Printf("  default_agent already set to %q\n", cfg["default_agent"])
		return nil
	}

	cfg["default_agent"] = agentName
	updated, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Printf("  Would set default_agent to %q\n", agentName)
		return nil
	}

	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return fmt.Errorf("writing opencode.json: %w", err)
	}
	fmt.Printf("  default_agent set to %q\n", agentName)
	return nil
}
