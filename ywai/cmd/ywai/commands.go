package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/gentlai"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/overrides"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/skills"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install gentle-ai + ecosystem + extra skills",
	Long:  "Full setup: gentle-ai, ecosystem, extra skills, and optional project init.",
	Run: func(cmd *cobra.Command, args []string) {
		agentFlag, _ := cmd.Flags().GetString("agent")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		tuiFlag, _ := cmd.Flags().GetBool("tui")
		mcpFlag, _ := cmd.Flags().GetBool("mcp")
		globalFlag, _ := cmd.Flags().GetBool("global")

		agents := detectAgents(cmd)
		if agents == nil {
			os.Exit(1)
		}

		var installMCP bool
		var globalOnly bool
		if tuiFlag || (agentFlag == "" && !dryRun && !globalFlag) {
			if !isInteractiveTerminal() {
				fmt.Fprintln(os.Stderr, "Error: install requires --agent or --dry-run when running non-interactively.")
				fmt.Fprintln(os.Stderr, "Run `ywai install --help` for flags, or run `ywai install` from an interactive terminal.")
				os.Exit(1)
			}
			selectedAgent, selectedMCP, selectedGlobal, err := runTUI(agents)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if selectedAgent == "" {
				fmt.Println("Installation cancelled.")
				return
			}
			agentFlag = selectedAgent
			installMCP = selectedMCP
			globalOnly = selectedGlobal
		} else {
			installMCP = mcpFlag
			globalOnly = globalFlag
		}

		installOpts := gentlai.InstallOptions{
			AgentName: agentFlag,
			Preset:    getStringFlag(cmd, "preset"),
			Scope:     getStringFlag(cmd, "scope"),
			SDDMode:   getStringFlag(cmd, "sdd-mode"),
			Persona:   getStringFlag(cmd, "persona"),
			DryRun:    dryRun,
		}

		executeInstall(installOpts, installMCP, globalOnly)
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Upgrade ywai + gentle-ai + re-seed + sync + copy skills",
	Run: func(cmd *cobra.Command, args []string) {
		var warnings []string
		warn := func(format string, args ...any) {
			msg := fmt.Sprintf(format, args...)
			warnings = append(warnings, msg)
			fmt.Printf("  Warning: %s\n", msg)
		}

		fmt.Println("=== ywai update ===")

		fmt.Println("\n[1/7] Self-updating ywai...")
		selfUpdate()

		fmt.Println("\n[2/7] Upgrading gentle-ai...")
		if !gentlai.IsInstalled() {
			fmt.Println("  gentle-ai not found, installing...")
			if err := gentlai.Install(); err != nil {
				warn("gentle-ai install failed: %v", err)
			}
		} else {
			if err := gentlai.Upgrade(); err != nil {
				warn("gentle-ai upgrade failed: %v", err)
			}
		}

		agents := agent.Detect()

		fmt.Println("\n[3/7] Re-seeding skills cache...")
		reseedData()

		fmt.Println("\n[4/7] Cleaning stale legacy links + pre-copying extra skills...")
		if len(agents) > 0 {
			for _, a := range agents {
				if configDir := filepath.Dir(a.SkillsDir); skills.IsLinkOrJunction(configDir) {
					warn("[%s] skipped stale legacy-link cleanup because config dir is a symlink/junction: %s", a.Name, configDir)
				} else {
					if removed, err := skills.RemoveStaleYwaiSkillLinks(a.SkillsDir); err != nil {
						warn("[%s] stale legacy-link cleanup failed: %v", a.Name, err)
					} else if len(removed) > 0 {
						fmt.Printf("  [%s] Removed stale legacy skill links: %s\n", a.Name, strings.Join(removed, ", "))
					}
				}
				if err := skills.CopyTo(a.SkillsDir); err != nil {
					warn("[%s] pre-sync skill copy failed: %v", a.Name, err)
				}
			}
		} else {
			fmt.Println("  No supported agents detected; skipping pre-sync skill copy.")
		}

		fmt.Println("\n[5/7] Syncing ecosystem...")
		if gentlai.IsInstalled() {
			syncOpts := gentlai.SyncOptions{
				SDDMode:      getStringFlag(cmd, "sdd-mode"),
				StrictTDD:    getBoolFlag(cmd, "strict-tdd"),
				IncludePerms: getBoolFlag(cmd, "include-permissions"),
				IncludeTheme: getBoolFlag(cmd, "include-theme"),
			}
			if err := gentlai.Sync(syncOpts); err != nil {
				warn("gentle-ai sync failed: %v", err)
				fmt.Println("  Continuing with ywai cache, skill copies, and overrides.")
			}
		} else {
			fmt.Println("  Skipping sync (gentle-ai not installed).")
		}

		fmt.Println("\n[6/7] Copying extra skills...")
		if len(agents) == 0 {
			fmt.Fprintln(os.Stderr, "Error: no supported agents detected.")
			os.Exit(1)
		}

		for _, a := range agents {
			if err := skills.CopyTo(a.SkillsDir); err != nil {
				warn("[%s] re-copy skills failed: %v", a.Name, err)
				continue
			}
			fmt.Printf("  [%s] Copied skills\n", a.Name)
		}

		fmt.Println("\n[6/7] Re-applying ywai overrides...")
		agentDirs := overrides.AgentSkillsDirs()
		for name, dir := range agentDirs {
			if _, err := os.Stat(dir); err == nil {
				agentDirs[name] = dir
			}
		}
		if err := overrides.ApplyOpenSpecToSDDOverride(agentDirs); err != nil {
			warn("failed to apply overrides: %v", err)
		}

		if len(warnings) > 0 {
			fmt.Println("\n=== Done with warnings ===")
			for _, msg := range warnings {
				fmt.Printf("  - %s\n", msg)
			}
			return
		}

		fmt.Println("\n=== Done! ===")
	},
}

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "List detected AI agents",
	Run: func(cmd *cobra.Command, args []string) {
		detected := agent.Detect()
		if len(detected) == 0 {
			fmt.Println("No agents detected.")
			fmt.Println("\nSupported agents:")
			for _, name := range agent.AvailableNames() {
				fmt.Printf("  - %s\n", name)
			}
			return
		}
		fmt.Printf("Detected %d agent(s):\n", len(detected))
		for _, a := range detected {
			fmt.Printf("  - %s\n", a.Name)
			if a.BinaryName != "" {
				fmt.Printf("    binary: %s\n", a.BinaryName)
			}
			fmt.Printf("    skills: %s\n", a.SkillsDir)
		}
	},
}

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "List available extra skills",
	Run: func(cmd *cobra.Command, args []string) {
		available, err := skills.ListAvailable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Extra skills available:")
		for _, s := range available {
			fmt.Printf("  - %s\n", s)
		}

		fmt.Printf("\nTotal: %d skills\n", len(available))
	},
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run gentle-ai health check",
	Long:  "Read-only ecosystem health diagnostics — tool binaries, state.json, Engram, disk space.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := gentlai.Doctor(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var skillRegistryCmd = &cobra.Command{
	Use:   "skill-registry",
	Short: "Refresh the project skill registry",
	Long:  "Scan project and global skills, build the registry used by SDD orchestrators.",
	Run: func(cmd *cobra.Command, args []string) {
		cwd, _ := cmd.Flags().GetString("cwd")
		if cwd == "" {
			cwd, _ = os.Getwd()
		}
		if err := gentlai.SkillRegistryRefresh(cwd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	installCmd.Flags().StringP("agent", "a", "", "Specific agent to install for")
	installCmd.Flags().Bool("dry-run", false, "Preview changes without applying")
	installCmd.Flags().Bool("tui", false, "Force TUI mode")
	installCmd.Flags().Bool("mcp", false, "Install Microsoft Learn MCP (for opencode/kilocode)")
	installCmd.Flags().Bool("global", false, "Install global skills only (skip AGENTS.md/REVIEW.md in project)")
	installCmd.Flags().String("preset", "full-gentleman", "Install preset: full-gentleman, ecosystem-only, minimal, custom")
	installCmd.Flags().String("scope", "", "Install scope: global (default) or workspace")
	installCmd.Flags().String("sdd-mode", "", "SDD orchestrator mode: single or multi")
	installCmd.Flags().String("persona", "", "Persona: gentleman, neutral, custom")

	updateCmd.Flags().String("sdd-mode", "", "SDD orchestrator mode: single or multi")
	updateCmd.Flags().Bool("strict-tdd", false, "Enable Strict TDD Mode for SDD agents")
	updateCmd.Flags().Bool("include-permissions", false, "Include permissions in sync")
	updateCmd.Flags().Bool("include-theme", false, "Include theme in sync")

	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(agentsCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(doctorCmd)

	skillRegistryCmd.Flags().String("cwd", "", "Project directory (defaults to current)")
	rootCmd.AddCommand(skillRegistryCmd)
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func getStringFlag(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

func getBoolFlag(cmd *cobra.Command, name string) bool {
	v, _ := cmd.Flags().GetBool(name)
	return v
}
