package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/control"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(evalCmd)
	evalCmd.AddCommand(evalRunCmd)
	evalCmd.AddCommand(evalSessionsCmd)

	for _, c := range []*cobra.Command{evalRunCmd, evalSessionsCmd} {
		c.Flags().Int("days", 30, "Lookback window in days (0 = all time)")
		c.Flags().String("project", "", "Filter by OpenCode project id")
		c.Flags().String("worktree", "", "Filter by worktree path substring")
		c.Flags().Bool("json", false, "Print full JSON instead of tables")
		c.Flags().Int("top", 15, "How many ranked rows to show per category")
	}
}

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Evaluate agent usage from real OpenCode sessions",
	Long: `Evaluate how agents, skills, models, and tools are used in practice.

Reads OpenCode's local SQLite database (~/.local/share/opencode/opencode.db)
and ranks usage by project and time window. Same data as the control UI
Session Analytics tab at /evals.

Examples:
  ywai eval run
  ywai eval run --days 7
  ywai eval sessions --project <id> --json
  ywai eval sessions --worktree dev-ai-workflow --top 20`,
}

var evalRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run session usage evaluation (agents, skills, models, tools)",
	Long: `Run a session usage evaluation over real OpenCode history.

Ranks the most-used agents, skills, models, and tools for the selected
time window. This is not a synthetic LLM task harness — it measures what
your agents actually do in day-to-day sessions.

See also: control UI → Evals → Session Analytics.`,
	RunE: runEvalSessions,
}

var evalSessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Same as 'eval run' — rank agents/skills/models from sessions",
	RunE:  runEvalSessions,
}

func runEvalSessions(cmd *cobra.Command, _ []string) error {
	days, _ := cmd.Flags().GetInt("days")
	project, _ := cmd.Flags().GetString("project")
	worktree, _ := cmd.Flags().GetString("worktree")
	asJSON, _ := cmd.Flags().GetBool("json")
	top, _ := cmd.Flags().GetInt("top")
	if top <= 0 {
		top = 15
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
	defer cancel()

	q := control.AnalyticsQuery{
		ProjectID:   strings.TrimSpace(project),
		Worktree:    strings.TrimSpace(worktree),
		Days:        days,
		ToolsLimit:  top,
		SkillsLimit: top,
	}

	start := time.Now()
	fmt.Fprintln(os.Stderr, "Scanning OpenCode sessions…")
	got, err := control.LoadSessionAnalytics(ctx, "", q)
	if err != nil {
		return fmt.Errorf("session analytics: %w\n\nTip: ensure OpenCode has been used at least once so ~/.local/share/opencode/opencode.db exists.\nOverride path with OPENCODE_DB=/path/to/opencode.db", err)
	}
	fmt.Fprintf(os.Stderr, "Done in %s\n", time.Since(start).Round(time.Millisecond))

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(got)
	}

	printEvalReport(got, top)
	return nil
}

func printEvalReport(a *control.SessionAnalytics, top int) {
	window := "all time"
	if a.Days > 0 {
		window = fmt.Sprintf("last %d days", a.Days)
	}

	fmt.Printf("\nSession usage evaluation (%s)\n", window)
	fmt.Printf("DB: %s\n\n", a.DBPath)

	s := a.Summary
	fmt.Printf("Sessions:      %d  across  %d project(s)\n", s.Sessions, s.Projects)
	fmt.Printf("Skill calls:   %d  (%d distinct) · %.0f%% sessions used a skill\n",
		s.SkillCalls, s.DistinctSkills, pct(s.SessionsWithSkill, s.Sessions))
	fmt.Printf("Tool calls:    %d\n", s.ToolCalls)
	fmt.Printf("Cost / tokens: $%.4f · %s in / %s out\n\n",
		s.TotalCost, compactInt(s.TokensInput), compactInt(s.TokensOutput))

	printRankTable("Most used agents", a.Agents, top, "sessions", true)
	printRankTable("Most used skills", a.Skills, top, "calls", false)
	printRankTable("Most used models", a.Models, top, "sessions", true)
	printRankTable("Top tools", a.Tools, top, "calls", false)

	if len(a.Projects) > 0 {
		fmt.Println("By project")
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "PROJECT\tSESSIONS\tSKILLS\tTOOLS\tCOST\tPATH")
		limit := top
		if limit > len(a.Projects) {
			limit = len(a.Projects)
		}
		for _, p := range a.Projects[:limit] {
			fmt.Fprintf(w, "%s\t%d\t%d\t%d\t$%.2f\t%s\n",
				trunc(p.Name, 28), p.Sessions, p.SkillCalls, p.ToolCalls, p.Cost, trunc(p.Worktree, 48))
		}
		_ = w.Flush()
		fmt.Println()
	}

	fmt.Println("UI: open http://localhost:5768/evals (Session Analytics tab)")
	fmt.Println("    ywai serve --no-update   # if the control UI is not running")
}

func printRankTable(title string, rows []control.SessionNamedCount, top int, unit string, showCost bool) {
	fmt.Println(title)
	if len(rows) == 0 {
		fmt.Println("  (none)")
		fmt.Println()
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	if showCost {
		fmt.Fprintln(w, "#\tNAME\tCOUNT\tSHARE\tCOST")
	} else {
		fmt.Fprintln(w, "#\tNAME\tCOUNT\tSHARE\tSESSIONS")
	}
	limit := top
	if limit > len(rows) {
		limit = len(rows)
	}
	for i, r := range rows[:limit] {
		if showCost {
			fmt.Fprintf(w, "%d\t%s\t%d %s\t%.0f%%\t$%.2f\n",
				i+1, trunc(r.Name, 48), r.Count, unit, r.Share*100, r.Cost)
		} else {
			sess := "—"
			if r.Sessions > 0 {
				sess = fmt.Sprintf("%d", r.Sessions)
			}
			fmt.Fprintf(w, "%d\t%s\t%d %s\t%.0f%%\t%s\n",
				i+1, trunc(r.Name, 48), r.Count, unit, r.Share*100, sess)
		}
	}
	_ = w.Flush()
	fmt.Println()
}

func pct(part, total int) float64 {
	if total <= 0 {
		return 0
	}
	return 100 * float64(part) / float64(total)
}

func compactInt(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
