// Package cli provides Cobra commands for ywai Missions.
// Each subcommand wires to the missions engine API.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencode"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// DefaultBaseDir is the default location for missions store.
// Must match missions.DefaultBaseDir (~/.local/share/ywai/missions).
const DefaultBaseDir = "~/.local/share/ywai/missions"

// ─── Command Registration ──────────────────────────────────────────────────

// RegisterCommands adds all missions subcommands to the given parent command.
func RegisterCommands(parent *cobra.Command) {
	missionsCmd := &cobra.Command{
		Use:   "missions",
		Short: "Manage ywai missions",
		Long: `Mission Control for ywai — orchestrate multi-agent software missions.

Subcommands:
  start              Start a new mission (interactive planning or from file)
  list               List all missions
  show               Show mission detail
  resume             Resume a paused mission
  cancel             Cancel a mission
  serve              Start the Mission Control Web UI server
  validate-contract  Check validation contract coverage
  show-contract      Show validation contract
  show-architecture  Show architecture document`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown subcommand %q for missions", args[0])
			}
			return cmd.Help()
		},
		// Suppress Cobra's default "unknown command" printing; we handle it
		// ourselves via RunE above.
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	missionsCmd.AddCommand(newStartCmd())
	missionsCmd.AddCommand(newListCmd())
	missionsCmd.AddCommand(newRunCmd())
	missionsCmd.AddCommand(newShowCmd())
	missionsCmd.AddCommand(newResumeCmd())
	missionsCmd.AddCommand(newCancelCmd())
	missionsCmd.AddCommand(newValidateContractCmd())
	missionsCmd.AddCommand(newShowContractCmd())
	missionsCmd.AddCommand(newAutoCmd())
	missionsCmd.AddCommand(newShowArchitectureCmd())
	missionsCmd.AddCommand(newProjectCmd())

	parent.AddCommand(missionsCmd)
}

// ─── Helpers ───────────────────────────────────────────────────────────────

// openStore opens the missions store at the default location.
func openStore() (*missions.MissionsStore, error) {
	return missions.OpenStore()
}

// isInteractiveTerminal returns true if stdin is a terminal.
func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// formatTime formats a time.Time for display.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

// statusIcon returns a human-readable status label with optional ANSI color.
func statusIcon(status string) string {
	switch status {
	case "planning":
		return "📋 planning"
	case "active":
		return "🔄 active"
	case "paused":
		return "⏸️  paused"
	case "completed":
		return "✅ completed"
	case "failed":
		return "❌ failed"
	case "cancelled":
		return "🚫 cancelled"
	case "validating":
		return "🔍 validating"
	default:
		return status
	}
}

// featureStatusIcon returns a human-readable feature status label.
func featureStatusIcon(status string) string {
	switch status {
	case "pending":
		return "⏳ pending"
	case "in_progress":
		return "🔄 in_progress"
	case "completed":
		return "✅ completed"
	case "failed":
		return "❌ failed"
	case "cancelled":
		return "🚫 cancelled"
	default:
		return status
	}
}

// ─── Start Command ─────────────────────────────────────────────────────────

// autoCmdFlags holds the flags exposed by `missions auto`. It is a plain struct
// so engineConfigFromFlags can be unit-tested without a cobra.Command.
type autoCmdFlags struct {
	Project     string
	Model       string
	Agent       string
	AutoApprove bool
	BaseRef     string
	Timeout     time.Duration
	MaxRetries  int
	MaxParallel int
	CleanStreak int
}

// engineConfigFromFlags builds an EngineConfig from the auto command's flags.
// Zero-value flags fall back to engine defaults. This is a pure function so it
// can be unit-tested directly without spinning up a cobra command or a store.
func engineConfigFromFlags(f autoCmdFlags) missions.EngineConfig {
	cfg := missions.DefaultEngineConfig()

	// Timeout: only override when explicitly set (a zero duration would create
	// an unbounded worker, which is almost never what the user wants).
	if f.Timeout > 0 {
		cfg.WorkerTimeout = f.Timeout
	}
	if f.MaxRetries > 0 {
		cfg.MaxRetries = f.MaxRetries
	}
	if f.MaxParallel > 0 {
		cfg.MaxParallel = f.MaxParallel
	}
	if f.CleanStreak > 0 {
		cfg.VerifyCleanStreak = f.CleanStreak
	}
	if f.BaseRef != "" {
		cfg.BaseRef = f.BaseRef
	}
	return cfg
}

func newAutoCmd() *cobra.Command {
	var f autoCmdFlags

	cmd := &cobra.Command{
		Use:   "auto \"<goal>\" --project <name> [--model <model>] [--agent <agent>] [--yes]",
		Short: "Autonomous mission: plan + approve + run in one shot",
		Long: `Run a fully autonomous mission: generate a plan from the goal, create the mission,
approve it, and start running — all without interactive prompts.

Requires --project to resolve the repo path for worktree-based execution.
Use --yes to auto-approve the plan without confirmation.

Example:
  ywai missions auto "Add a /health endpoint that returns 200 OK" --project myapp --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			goal := args[0]

			store, err := openStore()
			if err != nil {
				return fmt.Errorf("open missions store: %w", err)
			}

			if !f.AutoApprove && isInteractiveTerminal() {
				fmt.Fprintf(cmd.OutOrStdout(), "Goal: %s\n", goal)
				fmt.Fprintf(cmd.OutOrStdout(), "Project: %s\n\n", f.Project)
				fmt.Fprintf(cmd.OutOrStdout(), "Press Enter to generate the plan, or Ctrl+C to cancel...")
				_, _ = fmt.Scanln()
			}

			opts := missions.AutoPlanOpts{
				Project:     f.Project,
				Model:       f.Model,
				Agent:       f.Agent,
				AutoApprove: f.AutoApprove,
				RepoPath:    resolveProjectPath(store, f.Project),
			}

			mission, err := missions.PlanAndApprove(store, goal, opts)
			if err != nil {
				return fmt.Errorf("plan and approve: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Mission %q (%s) created with %d features.\n", mission.Name, mission.ID, len(mission.Features))

			if f.AutoApprove {
				// Start running the mission immediately.
				fmt.Fprintf(cmd.OutOrStdout(), "Starting mission...\n")
				config := engineConfigFromFlags(f)
				if f.Project != "" {
					if pStore, pErr := missions.NewProjectStore(store.BaseDir()); pErr == nil {
						config.RepoResolver = missions.NewProjectRepoResolver(pStore)
					} else {
						fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load projects: %v\n", pErr)
					}
				}
				engine := missions.NewEngine(store, config, nil)
				if err := engine.RunMission(mission.ID); err != nil {
					return fmt.Errorf("run mission: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Mission %q completed.\n", mission.ID)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Run 'ywai missions run %s' to start execution.\n", mission.ID)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&f.Project, "project", "p", "", "Project name (required)")
	cmd.Flags().StringVar(&f.Model, "model", "", "Model override (e.g. gpt-4o)")
	cmd.Flags().StringVar(&f.Agent, "agent", "", "Agent override")
	cmd.Flags().BoolVarP(&f.AutoApprove, "yes", "y", false, "Auto-approve plan without confirmation")
	cmd.Flags().StringVar(&f.BaseRef, "base", "", "Git ref to branch worktrees from (default: HEAD)")
	cmd.Flags().DurationVar(&f.Timeout, "timeout", 0, "Per-feature worker timeout (default: 30m)")
	cmd.Flags().IntVar(&f.MaxRetries, "max-retries", 0, "Max retries per failed feature (default: 3)")
	cmd.Flags().IntVar(&f.MaxParallel, "max-parallel", 1, "Max features running concurrently (default: 1)")
	cmd.Flags().IntVar(&f.CleanStreak, "clean-streak", 0, "Consecutive clean verify runs required per feature (default: 1)")
	_ = cmd.MarkFlagRequired("project")

	return cmd
}

// ─── Project Command ───────────────────────────────────────────────────────

func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage registered projects",
		Long: `Register and manage projects for autonomous mission execution.

A registered project maps a short name to a repository path on disk.
The path is used to create git worktrees during autonomous runs.

Subcommands:
  add   Register a project
  list  List all registered projects
  rm    Remove a registered project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newProjectAddCmd())
	cmd.AddCommand(newProjectListCmd())
	cmd.AddCommand(newProjectRmCmd())
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	var description string
	cmd := &cobra.Command{
		Use:   "add <name> <path>",
		Short: "Register a project",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openStore()
			if err != nil {
				return err
			}
			pStore, err := missions.NewProjectStore(store.BaseDir())
			if err != nil {
				return fmt.Errorf("open project store: %w", err)
			}
			p, err := pStore.Create(args[0], args[1], description)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Project %q registered at %s\n", p.Name, p.Path)
			return nil
		},
	}
	cmd.Flags().StringVarP(&description, "description", "d", "", "Optional description")
	return cmd
}

func newProjectListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openStore()
			if err != nil {
				return err
			}
			pStore, err := missions.NewProjectStore(store.BaseDir())
			if err != nil {
				return fmt.Errorf("open project store: %w", err)
			}
			projects := pStore.List()
			if len(projects) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No projects registered.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %s\n", "NAME", "PATH")
			for _, p := range projects {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %s\n", p.Name, p.Path)
			}
			return nil
		},
	}
}

func newProjectRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a registered project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openStore()
			if err != nil {
				return err
			}
			pStore, err := missions.NewProjectStore(store.BaseDir())
			if err != nil {
				return fmt.Errorf("open project store: %w", err)
			}
			if err := pStore.Delete(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Project %q removed.\n", args[0])
			return nil
		},
	}
}

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [--file plan.json]",
		Short: "Start a new mission",
		Long: `Start a new mission with interactive planning or from a plan file.

Interactive mode (default):
  Guides you through defining the mission goal, generating a structured plan
  with milestones and features, and approving it.

File mode (--file):
  Reads a plan.json file and creates a mission from it. The plan file must
  include name, description, milestones, and features.

Non-interactive terminals:
  If stdin is not a TTY, --file is required.`,
		RunE: runStart,
	}

	cmd.Flags().String("file", "", "Path to a plan.json file")
	cmd.Flags().String("project", "", "Project name for the mission")
	cmd.Flags().String("model", "", "OpenCode model to use (e.g. openai/gpt-4o)")
	cmd.Flags().String("agent", "", "OpenCode agent profile to use")
	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	filePath, _ := cmd.Flags().GetString("file")
	project, _ := cmd.Flags().GetString("project")

	store, err := openStore()
	if err != nil {
		return fmt.Errorf("failed to open missions store: %w", err)
	}

	// File mode
	if filePath != "" {
		mission, err := missions.PlanFromFile(store, filePath)
		if err != nil {
			return fmt.Errorf("failed to start mission from file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Mission %q (%s) created and activated from plan file.\n", mission.Name, mission.ID)
		return nil
	}

	// Interactive mode: require TTY
	if !isInteractiveTerminal() {
		return fmt.Errorf("interactive mode requires a terminal; use --file to start from a plan file")
	}

	// Try the iterative (Droid-style) flow when opencode is reachable; fall
	// back transparently to the one-shot planner inside the function.
	repoPath := resolveProjectPath(store, project)
	client := opencodeClient()
	mission, err := missions.RunInteractivePlanningWithClient(store, os.Stdin, cmd.OutOrStdout(), project, client, repoPath)
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Mission %q (%s) is now active with %d features across %d milestones.\n",
		mission.Name, mission.ID, len(mission.Features), len(mission.Milestones))
	return nil
}

// ─── List Command ──────────────────────────────────────────────────────────

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all missions",
		Long:  "List all missions sorted by creation date (newest first).",
		RunE:  runList,
	}

	cmd.Flags().String("format", "", "Output format: json")
	cmd.Flags().String("project", "", "Filter missions by project")
	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	store, err := openStore()
	if err != nil {
		return fmt.Errorf("failed to open missions store: %w", err)
	}

	missionsList, err := store.ListMissions()
	if err != nil {
		return fmt.Errorf("failed to list missions: %w", err)
	}

	format, _ := cmd.Flags().GetString("format")
	projectFilter, _ := cmd.Flags().GetString("project")

	if format == "json" {
		if missionsList == nil {
			missionsList = []*missions.Mission{}
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(missionsList); err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
		return nil
	}

	if len(missionsList) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No missions found.")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Start one with: ywai missions start")
		return nil
	}

	// Table header
	if projectFilter != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "%-16s %-30s %-14s %-18s %s\n", "ID", "Name", "Status", "Created", "Features")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%-16s %-30s %-14s %-18s %s\n", "ID", "Name", "Status", "Created", "Features")
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("─", 100))

	for _, m := range missionsList {
		if projectFilter != "" && m.Project != projectFilter {
			continue
		}
		featCount := len(m.Features)
		projectDisplay := ""
		if projectFilter == "" && m.Project != "" {
			projectDisplay = " [" + m.Project + "]"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-16s %-30s%-14s %-18s %d\n",
			m.ID, truncate(m.Name, 28)+projectDisplay, statusIcon(string(m.Status)), formatTime(m.CreatedAt), featCount)
	}

	return nil
}

// ─── Run Command ───────────────────────────────────────────────────────────

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <mission-id>",
		Short: "Execute a mission's features using opencode workers",
		Long: `Run all pending features of a mission. Each feature spawns an opencode
subprocess with its worker prompt and waits for a structured handoff.
Features are processed sequentially.`,
		Args: cobra.ExactArgs(1),
		RunE: runRun,
	}
	cmd.Flags().String("model", "", "Override OpenCode model for this run")
	cmd.Flags().String("agent", "", "Override OpenCode agent profile for this run")
	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	missionID := args[0]

	store, err := openStore()
	if err != nil {
		return fmt.Errorf("failed to open missions store: %w", err)
	}

	mission, err := store.LoadMission(missionID)
	if err != nil {
		return fmt.Errorf("failed to load mission %q: %w", missionID, err)
	}

	// Apply optional model/agent overrides
	if model, _ := cmd.Flags().GetString("model"); model != "" {
		mission.Model = model
	}
	if agent, _ := cmd.Flags().GetString("agent"); agent != "" {
		mission.Agent = agent
	}
	if mission.Model != "" || mission.Agent != "" {
		_ = store.SaveMission(mission)
	}

	// Create a broadcast function that logs to stdout for CLI users.
	broadcast := func(evtType string, payload interface{}) {
		fmt.Fprintf(cmd.OutOrStdout(), "[%s] %v\n", evtType, payload)
	}

	engine := missions.NewEngine(store, missions.DefaultEngineConfig(), broadcast)

	fmt.Fprintf(cmd.OutOrStdout(), "Running mission %q (%s) with %d features across %d milestones...\n",
		mission.Name, mission.ID, len(mission.Features), len(mission.Milestones))

	if err := engine.RunMission(missionID); err != nil {
		return fmt.Errorf("mission %q failed: %w", missionID, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Mission %q (%s) completed successfully!\n", mission.Name, mission.ID)
	return nil
}

// ─── Show Command ──────────────────────────────────────────────────────────

func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <mission-id>",
		Short: "Show mission detail",
		Long:  "Display detailed information about a mission including milestones and features.",
		Args:  cobra.ExactArgs(1),
		RunE:  runShow,
	}

	cmd.Flags().String("format", "", "Output format: json")
	return cmd
}

func runShow(cmd *cobra.Command, args []string) error {
	missionID := args[0]

	store, err := openStore()
	if err != nil {
		return fmt.Errorf("failed to open missions store: %w", err)
	}

	mission, err := store.LoadMission(missionID)
	if err != nil {
		return fmt.Errorf("failed to load mission %q: %w", missionID, err)
	}

	format, _ := cmd.Flags().GetString("format")

	if format == "json" {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(mission); err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
		return nil
	}

	// Human-readable output
	fmt.Fprintf(cmd.OutOrStdout(), "Mission: %s\n", mission.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "ID:      %s\n", mission.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "Status:  %s\n", statusIcon(string(mission.Status)))
	fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", formatTime(mission.CreatedAt))
	fmt.Fprintf(cmd.OutOrStdout(), "Updated: %s\n", formatTime(mission.UpdatedAt))
	if mission.CompletedAt != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Completed: %s\n", formatTime(*mission.CompletedAt))
	}

	// Milestones
	fmt.Fprintln(cmd.OutOrStdout(), "")
	if len(mission.Milestones) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Milestones (%d):\n", len(mission.Milestones))
		for _, ms := range mission.Milestones {
			summary := missions.GetMilestoneStatus(mission, ms.Name)
			progress := ""
			if summary != nil {
				progress = fmt.Sprintf(" [%d/%d completed]", summary.Completed, summary.Total)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  • %s — %s%s\n", ms.Name, ms.Description, progress)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Milestones: (none)")
	}

	// Features
	fmt.Fprintln(cmd.OutOrStdout(), "")
	if len(mission.Features) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Features (%d):\n", len(mission.Features))
		fmt.Fprintf(cmd.OutOrStdout(), "  %-25s %-16s %-20s %s\n", "ID", "Status", "Milestone", "Description")
		fmt.Fprintln(cmd.OutOrStdout(), "  "+strings.Repeat("─", 90))
		for _, f := range mission.Features {
			fmt.Fprintf(cmd.OutOrStdout(), "  %-25s %-16s %-20s %s\n",
				f.ID, featureStatusIcon(string(f.Status)), f.Milestone, truncate(f.Description, 50))
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Features: (none)")
	}

	return nil
}

// ─── Resume Command ────────────────────────────────────────────────────────

func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume <mission-id>",
		Short: "Resume a paused mission",
		Long:  "Resume a paused mission, transitioning it back to active state.",
		Args:  cobra.ExactArgs(1),
		RunE:  runResume,
	}
}

func runResume(cmd *cobra.Command, args []string) error {
	missionID := args[0]

	store, err := openStore()
	if err != nil {
		return fmt.Errorf("failed to open missions store: %w", err)
	}

	mission, err := store.LoadMission(missionID)
	if err != nil {
		return fmt.Errorf("failed to load mission %q: %w", missionID, err)
	}

	if err := missions.ResumeMission(store, mission); err != nil {
		return fmt.Errorf("failed to resume mission %q: %w", missionID, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Mission %q (%s) resumed — status: %s\n", mission.Name, mission.ID, statusIcon(string(mission.Status)))
	return nil
}

// ─── Cancel Command ────────────────────────────────────────────────────────

func newCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <mission-id>",
		Short: "Cancel a mission",
		Long: `Cancel an active or paused mission.

By default, cancel prompts for confirmation in interactive terminals.
Use --force to skip the confirmation prompt.`,
		Args: cobra.ExactArgs(1),
		RunE: runCancel,
	}

	cmd.Flags().Bool("force", false, "Skip confirmation prompt")
	return cmd
}

func runCancel(cmd *cobra.Command, args []string) error {
	missionID := args[0]
	force, _ := cmd.Flags().GetBool("force")

	store, err := openStore()
	if err != nil {
		return fmt.Errorf("failed to open missions store: %w", err)
	}

	mission, err := store.LoadMission(missionID)
	if err != nil {
		return fmt.Errorf("failed to load mission %q: %w", missionID, err)
	}

	// Confirmation for interactive terminals
	if !force && isInteractiveTerminal() {
		fmt.Fprintf(cmd.ErrOrStderr(), "Are you sure you want to cancel mission %q (%s)? [y/N] ", mission.Name, mission.ID)
		var response string
		_, err := fmt.Scanln(&response)
		if err != nil || (strings.ToLower(strings.TrimSpace(response)) != "y" && strings.ToLower(strings.TrimSpace(response)) != "yes") {
			fmt.Fprintln(cmd.OutOrStdout(), "Cancel aborted.")
			return nil
		}
	} else if !force {
		// Non-interactive: require --force
		return fmt.Errorf("confirmation required; use --force to cancel without confirmation")
	}

	if err := missions.CancelMission(store, mission); err != nil {
		return fmt.Errorf("failed to cancel mission %q: %w", missionID, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Mission %q (%s) cancelled.\n", mission.Name, mission.ID)
	return nil
}

// ─── Utilities ─────────────────────────────────────────────────────────────

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ─── Validate Contract Command ───────────────────────────────────────────────

func newValidateContractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate-contract <mission-id>",
		Short: "Check validation contract coverage",
		Long:  "Verify that every assertion in the validation contract is claimed by exactly one feature.",
		Args:  cobra.ExactArgs(1),
		RunE:  runValidateContract,
	}
	return cmd
}

func runValidateContract(cmd *cobra.Command, args []string) error {
	missionID := args[0]

	store, err := openStore()
	if err != nil {
		return fmt.Errorf("failed to open missions store: %w", err)
	}

	mission, err := store.LoadMission(missionID)
	if err != nil {
		return fmt.Errorf("failed to load mission %q: %w", missionID, err)
	}

	if err := missions.CheckValidationContractCoverage(store, mission); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Coverage check FAILED: %v\n", err)
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Coverage check PASSED: all assertions are claimed by features.\n")
	return nil
}

// ─── Show Contract Command ───────────────────────────────────────────────────

func newShowContractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-contract <mission-id>",
		Short: "Show validation contract",
		Long:  "Display the validation contract for a mission with all assertions.",
		Args:  cobra.ExactArgs(1),
		RunE:  runShowContract,
	}
	return cmd
}

func runShowContract(cmd *cobra.Command, args []string) error {
	missionID := args[0]

	store, err := openStore()
	if err != nil {
		return fmt.Errorf("failed to open missions store: %w", err)
	}

	missionDir := store.MissionDir(missionID)
	parser := missions.NewContractParser(missionDir)
	contract, err := parser.LoadContract()
	if err != nil {
		return fmt.Errorf("failed to load validation contract: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Validation Contract for mission %s\n\n", missionID)

	// Group by area
	areas := make(map[string][]missions.ContractAssertion)
	for _, assertion := range contract.Assertions {
		areas[assertion.Area] = append(areas[assertion.Area], assertion)
	}

	for area, assertions := range areas {
		fmt.Fprintf(cmd.OutOrStdout(), "## %s\n", area)
		for _, assertion := range assertions {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", assertion.ID, assertion.Title)
			fmt.Fprintf(cmd.OutOrStdout(), "    Tool: %s\n", assertion.Tool)
			fmt.Fprintf(cmd.OutOrStdout(), "    Evidence: %v\n", assertion.Evidence)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "")
	}

	return nil
}

// ─── Show Architecture Command ────────────────────────────────────────────────

func newShowArchitectureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show-architecture <mission-id>",
		Short: "Show architecture document",
		Long:  "Display the architecture.md document for a mission.",
		Args:  cobra.ExactArgs(1),
		RunE:  runShowArchitecture,
	}
	return cmd
}

func runShowArchitecture(cmd *cobra.Command, args []string) error {
	missionID := args[0]

	store, err := openStore()
	if err != nil {
		return fmt.Errorf("failed to open missions store: %w", err)
	}

	missionDir := store.MissionDir(missionID)
	archPath := missionDir + "/architecture.md"

	content, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("failed to read architecture.md: %w", err)
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), string(content))
	return nil
}

// ─── Planner wiring helpers ────────────────────────────────────────────────

// opencodeClient returns the default opencode client (server if reachable,
// local stub otherwise). Used to drive the iterative planner.
func opencodeClient() opencode.Client {
	return opencode.DefaultClient(context.Background())
}

// resolveProjectPath resolves a project name to its filesystem path via the
// project store. Returns empty string when the project isn't registered (the
// planner then runs without repo grounding).
func resolveProjectPath(store *missions.MissionsStore, projectName string) string {
	if projectName == "" {
		return ""
	}
	pStore, err := missions.NewProjectStore(store.BaseDir())
	if err != nil {
		return ""
	}
	proj, err := pStore.Get(projectName)
	if err != nil {
		return ""
	}
	return proj.Path
}
