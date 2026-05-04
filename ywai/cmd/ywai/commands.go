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

		agents := detectAgents(cmd)
		if agents == nil {
			os.Exit(1)
		}

		if tuiFlag || (projectType == "" && agentFlag == "" && !dryRun) {
			selectedType, selectedAgent, err := runTUI(agents)
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
		}

		executeInstall(agentFlag, projectType, dryRun)
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Upgrade ywai + gentle-ai + sync + re-seed + re-link + rename orchestrator",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== ywai update ===")

		fmt.Println("\n[1/6] Self-updating ywai...")
		selfUpdate()

		fmt.Println("\n[2/6] Upgrading gentle-ai...")
		gentlai.Upgrade()

		fmt.Println("\n[3/6] Syncing ecosystem...")
		gentlai.Sync()

		fmt.Println("\n[4/6] Re-seeding data cache...")
		reseedData()

		fmt.Println("\n[5/6] Re-linking extra skills...")
		agents := agent.Detect()
		if len(agents) == 0 {
			fmt.Fprintln(os.Stderr, "Error: no supported agents detected.")
			os.Exit(1)
		}

		for _, a := range agents {
			skills.LinkTo(a.SkillsDir)
			fmt.Printf("  [%s] Re-linked skills\n", a.Name)
		}

		fmt.Println("\n[6/6] Renaming orchestrator...")
		results := orchestrator.RenameAll(orchestrator.AgentSettingsPaths())
		orchestrator.PrintResults(results)

		fmt.Println("\n[6.5/6] Re-applying ywai overrides...")
		agentDirs := overrides.AgentSkillsDirs()
		for name, dir := range agentDirs {
			if _, err := os.Stat(dir); err == nil {
				agentDirs[name] = dir
			}
		}
		if err := overrides.ApplyOpenSpecToSDDOverride(agentDirs); err != nil {
			fmt.Printf("  Warning: failed to apply overrides: %v\n", err)
		}

		fmt.Println("\n=== Done! ===")
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
			fmt.Println("  macOS/Linux: curl -fsSL https://github.com/Yoizen/dev-ai-workflow/releases/latest/download/install.sh | bash")
			fmt.Println("  Go:          go install -tags embedded github.com/Yoizen/dev-ai-workflow/ywai/cmd/ywai@latest")
		}

		fmt.Printf("\nTotal: %d skills\n", len(available))
	},
}

func init() {
	installCmd.Flags().StringP("type", "t", "", "Project type (e.g., react, nest, dotnet)")
	installCmd.Flags().StringP("agent", "a", "", "Specific agent to install for")
	installCmd.Flags().Bool("dry-run", false, "Preview changes without applying")
	installCmd.Flags().Bool("tui", false, "Force TUI mode")
	skillsCmd.Flags().StringP("type", "t", "", "Filter skills by project type")

	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(skillsCmd)
}
