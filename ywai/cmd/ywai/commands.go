package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/gentlai"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/orchestrator"
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
		projectType, _ := cmd.Flags().GetString("type")
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
		if tuiFlag || (projectType == "" && agentFlag == "" && !dryRun && !globalFlag) {
			if !isInteractiveTerminal() {
				fmt.Fprintln(os.Stderr, "Error: install requires --type/--agent or --dry-run when running non-interactively.")
				fmt.Fprintln(os.Stderr, "Run `ywai install --help` for flags, or run `ywai install` from an interactive terminal.")
				os.Exit(1)
			}
			selectedType, selectedAgent, selectedMCP, selectedGlobal, err := runTUI(agents)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if selectedType == "" || selectedAgent == "" {
				fmt.Println("Installation cancelled.")
				return
			}
			projectType = selectedType
			agentFlag = selectedAgent
			installMCP = selectedMCP
			globalOnly = selectedGlobal
		} else {
			installMCP = mcpFlag
			globalOnly = globalFlag
		}

		executeInstall(agentFlag, projectType, dryRun, installMCP, globalOnly)
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Upgrade ywai + gentle-ai + re-seed + sync + re-link + rename orchestrator",
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
		if err := gentlai.Upgrade(); err != nil {
			warn("gentle-ai upgrade failed: %v", err)
		}

		agents := agent.Detect()

		fmt.Println("\n[3/7] Re-seeding data cache...")
		reseedData()

		fmt.Println("\n[4/7] Normalizing extra skill links...")
		if len(agents) > 0 {
			for _, a := range agents {
				if err := skills.LinkTo(a.SkillsDir); err != nil {
					warn("[%s] pre-sync skill link normalization failed: %v", a.Name, err)
				}
			}
		} else {
			fmt.Println("  No supported agents detected; skipping pre-sync link normalization.")
		}

		fmt.Println("\n[5/7] Syncing ecosystem...")
		if err := gentlai.Sync(); err != nil {
			warn("gentle-ai sync failed: %v", err)
			fmt.Println("  Continuing with ywai cache, skill links, and overrides.")
		}

		fmt.Println("\n[6/7] Re-linking extra skills...")
		if len(agents) == 0 {
			fmt.Fprintln(os.Stderr, "Error: no supported agents detected.")
			os.Exit(1)
		}

		for _, a := range agents {
			if err := skills.LinkTo(a.SkillsDir); err != nil {
				warn("[%s] re-link skills failed: %v", a.Name, err)
				continue
			}
			fmt.Printf("  [%s] Re-linked skills\n", a.Name)
		}

		fmt.Println("\n[7/7] Renaming orchestrator...")
		results := orchestrator.RenameAll(orchestrator.AgentSettingsPaths())
		orchestrator.PrintResults(results)

		fmt.Println("\n[7.5/7] Re-applying ywai overrides...")
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
		projectType, _ := cmd.Flags().GetString("type")

		if projectType != "" {
			filter := config.ProfileSkills(projectType)
			if filter == nil {
				fmt.Println("all skills (generic profile)")
			} else {
				fmt.Printf("Skills for %s:\n", projectType)
				for _, s := range filter {
					fmt.Printf("  - %s\n", s)
				}
			}
			return
		}

		available, err := skills.ListAvailable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Extra skills available:")
		for _, s := range available {
			fmt.Printf("  - %s\n", s)
		}

		profiles := config.AvailableProfiles()
		if len(profiles) > 0 {
			fmt.Println("\nSkills by profile:")
			for _, p := range profiles {
				pSkills := config.ProfileSkills(p)
				if pSkills == nil {
					fmt.Printf("  %s: all skills\n", p)
				} else {
					fmt.Printf("  %s: %s\n", p, strings.Join(pSkills, ", "))
				}
			}
		} else {
			fmt.Println("\nNo project profiles found.")
			fmt.Println("Try running: ywai update")
			fmt.Println("Or reinstall:")
			fmt.Println("  macOS/Linux: curl -fsSL https://github.com/YoizenSA/dev-ai-workflow/releases/latest/download/install.sh | bash")
			fmt.Println("  Source:      cd ywai && bash scripts/prepare-embedded.sh && go install -tags embedded ./cmd/ywai")
		}

		fmt.Printf("\nTotal: %d skills\n", len(available))
	},
}

func init() {
	installCmd.Flags().StringP("type", "t", "", "Project type (e.g., react, nest, dotnet)")
	installCmd.Flags().StringP("agent", "a", "", "Specific agent to install for")
	installCmd.Flags().Bool("dry-run", false, "Preview changes without applying")
	installCmd.Flags().Bool("tui", false, "Force TUI mode")
	installCmd.Flags().Bool("mcp", false, "Install Microsoft Learn MCP (for opencode/kilocode)")
	installCmd.Flags().Bool("global", false, "Install global skills only (skip AGENTS.md/REVIEW.md in project)")
	skillsCmd.Flags().StringP("type", "t", "", "Filter skills by project type")

	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(agentsCmd)
	rootCmd.AddCommand(skillsCmd)
}

func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
