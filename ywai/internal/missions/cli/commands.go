// Package cli provides Cobra commands for ywai Missions.
// Each subcommand wires to the missions engine API.
package cli

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/missions/web"
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
	missionsCmd.AddCommand(newServeCmd())
	missionsCmd.AddCommand(newValidateContractCmd())
	missionsCmd.AddCommand(newShowContractCmd())
	missionsCmd.AddCommand(newShowArchitectureCmd())

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

	mission, err := missions.StartInteractivePlanningWithProject(store, project)
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
		fmt.Fprintln(cmd.OutOrStdout(), "No missions found.")
		fmt.Fprintln(cmd.OutOrStdout(), "Start one with: ywai missions start")
		return nil
	}

	// Table header
	if projectFilter != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "%-16s %-30s %-14s %-18s %s\n", "ID", "Name", "Status", "Created", "Features")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%-16s %-30s %-14s %-18s %s\n", "ID", "Name", "Status", "Created", "Features")
	}
	fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("─", 100))

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

// ─── Serve Command ─────────────────────────────────────────────────────────

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Mission Control Web UI server (deprecated: use 'ywai serve' instead)",
		Long:  "Start the Mission Control Web UI HTTP server on port 5769 (default).\n\nDEPRECATED: Use 'ywai serve' instead to run the unified server.",
		RunE:  runServe,
	}

	cmd.Flags().IntP("port", "p", web.DefaultPort, "Port for Mission Control Web UI")
	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(os.Stderr, "Warning: 'ywai missions serve' is deprecated. Use 'ywai serve' instead.")
	port, _ := cmd.Flags().GetInt("port")

	store, err := openStore()
	if err != nil {
		return fmt.Errorf("failed to open missions store: %w", err)
	}

	s := web.New(port, store)
	log.Printf("Mission Control Web UI starting on http://localhost:%d", port)
	if err := s.Start(); err != nil {
		return fmt.Errorf("failed to start missions web UI: %w", err)
	}
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
			fmt.Fprintf(cmd.OutOrStdout(),    "    Tool: %s\n", assertion.Tool)
			fmt.Fprintf(cmd.OutOrStdout(),    "    Evidence: %v\n", assertion.Evidence)
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
	
	fmt.Fprint(cmd.OutOrStdout(), string(content))
	return nil
}
