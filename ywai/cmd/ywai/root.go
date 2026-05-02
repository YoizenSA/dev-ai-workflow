package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/gentlai"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/skills"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/tui"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "ywai",
	Short: "One command to set up your AI dev environment",
	Long:  "ywai wraps gentle-ai and adds extra skills, project templates, and one-command install.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		repo := config.RepoRoot()
		if config.ShouldSeedData() {
			if err := config.SeedDataFrom(repo); err != nil {
				fmt.Printf("Warning: failed to seed data: %v\n", err)
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

	agents := agent.Detect()
	if len(agents) == 0 {
		fmt.Fprintln(os.Stderr, "Error: no supported agents detected in PATH.")
		fmt.Fprintln(os.Stderr, "Supported: opencode, claude-code, cursor, windsurf, gemini-cli, vscode-copilot, codex")
		return nil
	}
	return agents
}

func installEcosystem(agents []agent.Agent, dryRun bool) {
	for _, a := range agents {
		if dryRun {
			fmt.Printf("  [%s] Running gentle-ai install...\n", a.Name)
			continue
		}
		if err := gentlai.InstallEcosystem(a.Name); err != nil {
			fmt.Printf("  Warning: ecosystem install failed for %s: %v\n", a.Name, err)
		}
	}
}

func linkSkillsForAgents(agents []agent.Agent, projectType string, dryRun bool) {
	filter := config.ProfileSkills(projectType)

	if filter == nil {
		fmt.Println("  linking all skills (generic profile).")
	} else {
		fmt.Printf("  linking %d skills (%s profile).\n", len(filter), projectType)
	}

	for _, a := range agents {
		if dryRun {
			fmt.Printf("  [%s] Linking extra skills to %s...\n", a.Name, a.SkillsDir)
			continue
		}

		if filter == nil {
			skills.LinkTo(a.SkillsDir)
		} else {
			skills.LinkFiltered(a.SkillsDir, filter)
		}
		fmt.Printf("  [%s] Linked extra skills to %s\n", a.Name, a.SkillsDir)
	}
}

func runTUI(agents []agent.Agent) error {
	m := tui.NewModel(agents)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}

func executeInstall(agentFlag, projectType string, dryRun bool) {
	var agents []agent.Agent
	if agentFlag != "" {
		a, err := agent.FindByName(agentFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}
		agents = []agent.Agent{*a}
	} else {
		agents = agent.Detect()
		if len(agents) == 0 {
			fmt.Fprintln(os.Stderr, "Error: no supported agents detected.")
			return
		}
	}

	fmt.Println("=== ywai install ===")
	if dryRun {
		fmt.Println("\n[DRY RUN] No changes will be made.")
	}

	fmt.Println("\n[1/4] Checking gentle-ai...")
	if dryRun {
		fmt.Println("  Would install gentle-ai if not present.")
	} else {
		gentlai.Install()
	}

	fmt.Println("\n[2/4] Detecting agents...")
	for _, a := range agents {
		fmt.Printf("  Found: %s (%s)\n", a.Name, a.BinaryName)
	}

	fmt.Println("\n[3/4] Installing ecosystem + linking extra skills...")
	installEcosystem(agents, dryRun)
	linkSkillsForAgents(agents, projectType, dryRun)

	fmt.Println("\n[4/4] Initializing project...")
	if projectType != "" && projectType != "generic" {
		fmt.Printf("  %sinit project type %q in current directory.\n", ternary(dryRun, "Would ", ""), projectType)
	}

	fmt.Println("\n=== Done! ===")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Open your AI agent in this project")
	fmt.Println("  2. Run /sdd-init to detect stack and testing capabilities")
	fmt.Println("  3. Run skill-registry to scan installed skills")
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func stringInSlice(s string, slice []string) bool {
	for _, v := range slice {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
