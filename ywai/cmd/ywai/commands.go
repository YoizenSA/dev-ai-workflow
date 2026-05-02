package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/gentlai"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/skills"
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
			if err := runTUI(agents); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		executeInstall(agentFlag, projectType, dryRun)
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Upgrade gentle-ai + sync + re-link skills",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== ywai update ===")

		fmt.Println("\n[1/3] Upgrading gentle-ai...")
		gentlai.Upgrade()

		fmt.Println("\n[2/3] Syncing ecosystem...")
		gentlai.Sync()

		fmt.Println("\n[3/3] Re-linking extra skills...")
		agents := agent.Detect()
		if len(agents) == 0 {
			fmt.Fprintln(os.Stderr, "Error: no supported agents detected.")
			os.Exit(1)
		}

		for _, a := range agents {
			skills.LinkTo(a.SkillsDir)
			fmt.Printf("  [%s] Re-linked skills\n", a.Name)
		}

		fmt.Println("\n=== Done! ===")
	},
}

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "List available extra skills",
	Run: func(cmd *cobra.Command, args []string) {
		projectType, _ := cmd.Flags().GetString("type")
		available, err := skills.ListAvailable()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Extra skills available:")
		for _, s := range available {
			fmt.Printf("  - %s\n", s)
		}

		if projectType != "" {
			filter := config.ProfileSkills(projectType)
			if filter == nil {
				fmt.Println("\nAll skills (generic profile)")
			} else {
				fmt.Printf("\nSkills for %s profile:\n", projectType)
				for _, s := range filter {
					fmt.Printf("  - %s\n", s)
				}
			}
			return
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
