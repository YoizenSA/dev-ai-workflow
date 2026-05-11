package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/gentlai"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/orchestrator"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/overrides"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/plugins"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/project"
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
		// Seed data if project-types dir is empty OR no valid profiles found.
		// The profile check handles cases where the dir has content but no valid profile.yaml files
		// (e.g. stale state from a previous broken install).
		needsSeed := !config.IsDirPopulated(config.DataProjectTypesDir()) ||
			len(config.AvailableProfiles()) == 0

		if needsSeed {
			repo := config.RepoRoot()
			isRealRepo := config.IsOurRepoByPath(repo) && repo != config.DataDir()

			if isRealRepo {
				if err := config.SeedDataFrom(repo); err != nil {
					fmt.Printf("Warning: failed to seed data from repo: %v\n", err)
				}
				// Fallback: if repo seed ran but no valid profiles were loaded,
				// force embedded seed to avoid stale/local repo issues.
				config.ResetConfig()
				if len(config.AvailableProfiles()) == 0 {
					if err := config.SeedFromEmbedded(); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
					}
				}
			} else {
				if err := config.SeedFromEmbedded(); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
				}
			}
			// Invalidate config cache so TUI picks up freshly seeded profiles
			config.ResetConfig()

			// Verify seeding actually worked
			if len(config.AvailableProfiles()) == 0 && cmd.Name() != "update" {
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "Error: no project profiles available after seeding.")
				fmt.Fprintln(os.Stderr, "This usually means the binary was not built with embedded data.")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "Fix: reinstall ywai from the release installer:")
				fmt.Fprintln(os.Stderr, "  curl -fsSL https://github.com/YoizenSA/dev-ai-workflow/releases/latest/download/install.sh | bash")
				fmt.Fprintln(os.Stderr, "Or, from a source checkout:")
				fmt.Fprintln(os.Stderr, "  cd ywai && bash scripts/prepare-embedded.sh && go install -tags embedded ./cmd/ywai")
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
		configDir := filepath.Dir(a.SkillsDir)
		if skills.IsLinkOrJunction(configDir) {
			fmt.Printf("  Warning: [%s] gentle-ai install skipped because config dir is a symlink/junction: %s\n", a.Name, configDir)
			fmt.Println("    gentle-ai currently refuses to atomically write through linked config directories; leaving existing upstream skills untouched.")
			continue
		}
		if dryRun {
			fmt.Printf("  [%s] Would remove stale ywai skill links that block gentle-ai...\n", a.Name)
			fmt.Printf("  [%s] Running gentle-ai install...\n", a.Name)
			continue
		}
		if removed, err := skills.RemoveStaleYwaiSkillLinks(a.SkillsDir); err != nil {
			fmt.Printf("  Warning: [%s] failed to clean stale ywai skill links: %v\n", a.Name, err)
		} else if len(removed) > 0 {
			fmt.Printf("  [%s] Removed stale ywai skill links: %s\n", a.Name, strings.Join(removed, ", "))
		}
		if err := gentlai.InstallEcosystem(a.Name); err != nil {
			fmt.Printf("  Warning: ecosystem install failed for %s: %v\n", a.Name, err)
		}
	}
}

func linkSkillsForAgents(agents []agent.Agent, projectType string, dryRun bool) {
	filter := config.ProfileSkills(projectType)

	if filter == nil {
		fmt.Println("  copying all ywai extra skills (generic profile).")
	} else {
		fmt.Printf("  Skills for %s:\n", projectType)
		for _, s := range filter {
			fmt.Printf("    - %s\n", s)
		}
	}

	for _, a := range agents {
		if dryRun {
			fmt.Printf("  [%s] Copying extra skills to %s...\n", a.Name, a.SkillsDir)
			continue
		}

		if filter == nil {
			skills.LinkTo(a.SkillsDir)
		} else {
			skills.LinkFiltered(a.SkillsDir, filter)
		}
		fmt.Printf("  [%s] Copied extra skills to %s\n", a.Name, a.SkillsDir)
	}
}

func runTUI(agents []agent.Agent) (string, string, bool, bool, error) {
	// Convert internal agent.Agent to tui agent format
	tuiAgents := make([]agent.Agent, len(agents))
	copy(tuiAgents, agents)

	return tui.Run(tuiAgents)
}

func executeInstall(agentFlag, projectType string, dryRun bool, installMCP bool, globalOnly bool) {
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
		fmt.Println("  Would install or update gentle-ai if needed.")
	} else {
		if err := gentlai.Install(); err != nil {
			fmt.Printf("  Warning: gentle-ai install/update failed: %v\n", err)
		}
		reseedData()
	}

	fmt.Println("\n[2/4] Detecting agents...")
	for _, a := range agents {
		fmt.Printf("  Found: %s (%s)\n", a.Name, a.BinaryName)
	}

	fmt.Println("\n[3/4] Installing ecosystem + linking extra skills...")
	installEcosystem(agents, dryRun)
	linkSkillsForAgents(agents, projectType, dryRun)

	if !dryRun {
		results := orchestrator.RenameAll(orchestrator.AgentSettingsPaths())
		orchestrator.PrintResults(results)

		fmt.Println("\n[3.5/4] Applying ywai overrides...")
		applyOverrides(agents)

		fmt.Println("\n[3.6/4] Installing opencode-quota plugin...")
		installPluginsForAgents(agents, dryRun, installMCP)
	}

	if !globalOnly {
		fmt.Println("\n[4/4] Initializing project...")
		if projectType != "" && projectType != "generic" {
			if dryRun {
				fmt.Printf("  Would init project type %q in current directory.\n", projectType)
			} else {
				if err := project.Init(projectType, "."); err != nil {
					fmt.Printf("  Warning: project init failed: %v\n", err)
				}
			}
		}
	} else {
		fmt.Println("\n[4/4] Skipped (global-only mode).")
	}

	fmt.Println("\n=== Done! ===")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Open your AI agent in this project")
	fmt.Println("  2. Run `ywai skills` to see available skills")
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
	skillsDir := config.DataSkillsDir()
	ptDir := config.DataProjectTypesDir()

	os.RemoveAll(skillsDir)
	os.RemoveAll(ptDir)

	if err := config.EnsureDataDir(); err != nil {
		fmt.Printf("  Warning: failed to create data directories: %v\n", err)
		return
	}

	repo := config.RepoRoot()
	if config.IsOurRepoByPath(repo) && repo != config.DataDir() {
		if err := config.SeedDataFrom(repo); err != nil {
			fmt.Printf("  Warning: seed from repo failed: %v\n", err)
		} else {
			config.ResetConfig()
			if len(config.AvailableProfiles()) > 0 {
				fmt.Println("  Data re-seeded from repo.")
				return
			}
			fmt.Println("  Repo seed had no valid profiles, falling back to embedded...")
		}
	}

	if err := config.SeedFromEmbedded(); err != nil {
		fmt.Printf("  Warning: seed from embedded failed: %v\n", err)
		fmt.Println("  The updated binary will seed data on next run.")
		return
	}

	config.ResetConfig()
	fmt.Println("  Data re-seeded from embedded.")
}

func applyOverrides(agents []agent.Agent) {
	agentDirs := make(map[string]string)
	for _, a := range agents {
		agentDirs[a.Name] = a.SkillsDir
	}

	if err := overrides.ApplyOpenSpecToSDDOverride(agentDirs); err != nil {
		fmt.Printf("  Warning: failed to apply openspec→.sdd override: %v\n", err)
	}
}

func installPluginsForAgents(agents []agent.Agent, dryRun bool, installMCP bool) {
	agentSettingsPaths := orchestrator.AgentSettingsPaths()

	for _, a := range agents {
		// Only install plugins for opencode/kilocode
		if a.Name != "opencode" && a.Name != "kilocode" {
			continue
		}

		configPath, ok := agentSettingsPaths[a.Name]
		if !ok || configPath == "" {
			fmt.Printf("  [%s] No config path found, skipping plugins\n", a.Name)
			continue
		}

		if dryRun {
			fmt.Printf("  [%s] Would install opencode-quota plugin\n", a.Name)
			if installMCP {
				fmt.Printf("  [%s] Would install Microsoft Learn MCP\n", a.Name)
			}
			continue
		}

		// Install opencode-quota (always for opencode/kilocode)
		if err := plugins.InstallQuota(configPath); err != nil {
			fmt.Printf("  [%s] Warning: failed to install opencode-quota: %v\n", a.Name, err)
		} else {
			fmt.Printf("  [%s] Installed opencode-quota plugin\n", a.Name)
		}

		// Install Microsoft Learn MCP if requested
		if installMCP {
			if err := plugins.InstallMicrosoftLearnMCP(configPath); err != nil {
				fmt.Printf("  [%s] Warning: failed to install Microsoft Learn MCP: %v\n", a.Name, err)
			} else {
				fmt.Printf("  [%s] Installed Microsoft Learn MCP\n", a.Name)
			}
		}
	}
}
