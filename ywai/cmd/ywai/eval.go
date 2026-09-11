package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agent"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/control"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/envprofile"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/evals"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite" // pure-Go SQLite driver, for the OpenCode DB readback
)

func init() {
	rootCmd.AddCommand(evalCmd)
	evalCmd.AddCommand(evalRunCmd)
	evalCmd.AddCommand(evalSessionsCmd)
	evalCmd.AddCommand(evalBenchCmd)
	evalCmd.AddCommand(evalCompareCmd)

	for _, c := range []*cobra.Command{evalRunCmd, evalSessionsCmd} {
		c.Flags().Int("days", 30, "Lookback window in days (0 = all time)")
		c.Flags().String("project", "", "Filter by OpenCode project id")
		c.Flags().String("worktree", "", "Filter by worktree path substring")
		c.Flags().Bool("json", false, "Print full JSON instead of tables")
		c.Flags().Int("top", 15, "How many ranked rows to show per category")
	}

	evalBenchCmd.Flags().String("task", "", "Task id to run (see the Agent Benchmarks tab or /api/evals/tasks)")
	evalBenchCmd.Flags().String("model", "", "Comma-separated model ids to benchmark")
	evalBenchCmd.Flags().String("provider", "opencode-admin", "Provider id passed to the OpenCode server")
	evalBenchCmd.Flags().Int("rounds", 1, "Attempts per model")
	evalBenchCmd.Flags().Float64("min-score", 0, "Fail when a model's avgWeighted is below this")
	evalBenchCmd.Flags().Bool("require-hard", false, "Fail when any counted attempt misses the task's hard expectation")
	evalBenchCmd.Flags().String("baseline-run", "none", "Run to diff against: latest, a run id, or none")
	evalBenchCmd.Flags().Float64("max-regression", 0, "Fail when a weightedDelta falls below minus this (needs --baseline-run)")
	evalBenchCmd.Flags().Float64("max-cost-usd", 0, "Fail when a model's totalCostUsd exceeds this (0 = no gate)")
	evalBenchCmd.Flags().String("out", "", "Also write the summary JSON to this path")
	evalBenchCmd.Flags().String("profile", "", "Run inside an isolated environment (base URL, DB and eval store default from the profile)")
	evalBenchCmd.Flags().String("base-url", "", "OpenCode server URL (wins over --profile and OPENCODE_URL)")
	evalBenchCmd.Flags().String("db-path", "", "OpenCode SQLite database path (wins over the profile default)")
	evalBenchCmd.Flags().String("store-dir", "", "Eval run store dir (wins over the profile default; baselines stay namespaced per profile)")

	evalCompareCmd.Flags().String("profiles", "", "Comma-separated profile names to compare (e.g. dev,qa)")
	evalCompareCmd.Flags().String("task", "", "Task id to run in every profile")
	evalCompareCmd.Flags().String("model", "", "Comma-separated model ids to benchmark")
	evalCompareCmd.Flags().String("provider", "opencode-admin", "Provider id passed to the OpenCode server")
	evalCompareCmd.Flags().Int("rounds", 1, "Attempts per model in each profile")
	evalCompareCmd.Flags().Float64("min-score", 0, "Mark a profile FAIL when a model's avgWeighted is below this")
	evalCompareCmd.Flags().Bool("require-hard", false, "Mark a profile FAIL when any counted attempt misses the hard expectation")
	evalCompareCmd.Flags().Float64("max-cost-usd", 0, "Mark a profile FAIL when a model's totalCostUsd exceeds this (0 = no gate)")
	evalCompareCmd.Flags().String("out", "", "Also write the per-profile summary JSON to this path")
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

// printRankTable prints one ranked name/count table (agents, skills, tools…).
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

// ─── eval bench ─────────────────────────────────────────────────────────────
//
// The headless counterpart of the Agent Benchmarks tab: one task, a fixed set
// of models, gates a CI job can act on. It talks to the OpenCode server the
// same way the control server's bench handler does (evals_bench.go), but it
// needs no server of its own and its exit code, not a browser, is the result:
// 0 pass, 1 gate miss, 2 harness fault.

const (
	benchExitPass       = 0
	benchExitGateMiss   = 1
	benchExitHarnessErr = 2
)

// benchConfig gathers everything one benchmark needs. The flow below is a pure
// function of it, which is what makes the gates testable without a server.
type benchConfig struct {
	taskID        string
	models        []string
	provider      string
	rounds        int
	minScore      float64
	requireHard   bool
	baselineRun   string // latest | <runId> | none
	maxRegression float64
	maxCostUSD    float64
	out           string
	profile       string // isolated environment name ("", global)

	// Injection points for tests; empty means the real default.
	rootDir  string // project root for project-local task overrides ("" = cwd)
	baseURL  string // OpenCode server ("" = OPENCODE_URL or localhost:4096)
	dbPath   string // OpenCode SQLite database ("" = default location)
	storeDir string // eval run store ("" = ~/.ywai)
}

// benchError carries the exit class of a failure: a gate miss exits 1 so CI
// reads "your model regressed", a harness fault exits 2 so it reads "the rig
// broke".
type benchError struct {
	code int
	msg  string
}

func (e *benchError) Error() string { return e.msg }

func benchGateMiss(format string, a ...any) *benchError {
	return &benchError{code: benchExitGateMiss, msg: fmt.Sprintf(format, a...)}
}

func benchHarnessErr(format string, a ...any) *benchError {
	return &benchError{code: benchExitHarnessErr, msg: fmt.Sprintf(format, a...)}
}

// benchExitCode maps any error to the command's exit code. An error that is
// not a benchError is a harness fault — the classification is a property of
// where the error was raised, so wrapping must preserve it.
func benchExitCode(err error) int {
	if err == nil {
		return benchExitPass
	}
	var be *benchError
	if errors.As(err, &be) {
		return be.code
	}
	return benchExitHarnessErr
}

var evalBenchCmd = &cobra.Command{
	Use:   "bench",
	Short: "Run one evals task headlessly across models and gate on the result",
	Long: `Run one evals task against one or more models headlessly and gate on the result.

Talks to the OpenCode server directly (OPENCODE_URL, default localhost:4096);
no control server needs to be running. Prints the same summary JSON as the
Agent Benchmarks tab — taskId plus per-model summaries, plus baseline deltas
when --baseline-run resolves — and exits 0 on pass, 1 on a gate miss, and 2 on
a harness fault (no server, unknown task, busy server).

Examples:
  ywai eval bench --task find-session-deletes --model anthropic/claude-sonnet-4-5
  ywai eval bench --task find-session-deletes --model m1,m2 --rounds 3 \
      --min-score 0.8 --require-hard --baseline-run latest --max-regression 0.05`,
	RunE: runEvalBench,
}

func runEvalBench(cmd *cobra.Command, _ []string) error {
	cfg, err := benchConfigFromFlags(cmd)
	if err == nil {
		var summary []byte
		summary, err = benchRun(cmd.Context(), cfg)
		if err == nil && cfg.out != "" {
			if werr := os.WriteFile(cfg.out, summary, 0o644); werr != nil {
				err = benchHarnessErr("write %s: %v", cfg.out, werr)
			} else {
				os.Stdout.Write(summary) // stdout always gets the summary too
				return nil
			}
		} else if err == nil {
			os.Stdout.Write(summary)
			return nil
		}
	}
	fmt.Fprintf(os.Stderr, "eval bench: %v\n", err)
	os.Exit(benchExitCode(err))
	return nil // unreachable
}

func benchConfigFromFlags(cmd *cobra.Command) (benchConfig, error) {
	get := func(name string) string {
		s, _ := cmd.Flags().GetString(name)
		return strings.TrimSpace(s)
	}
	fget := func(name string) float64 {
		f, _ := cmd.Flags().GetFloat64(name)
		return f
	}

	var models []string
	for _, m := range strings.Split(get("model"), ",") {
		if m = strings.TrimSpace(m); m != "" {
			models = append(models, m)
		}
	}
	cfg := benchConfig{
		taskID:        get("task"),
		models:        models,
		provider:      get("provider"),
		minScore:      fget("min-score"),
		baselineRun:   get("baseline-run"),
		maxRegression: fget("max-regression"),
		maxCostUSD:    fget("max-cost-usd"),
		out:           get("out"),
		profile:       get("profile"),
		baseURL:       get("base-url"),
		dbPath:        get("db-path"),
		storeDir:      get("store-dir"),
	}
	cfg.rounds, _ = cmd.Flags().GetInt("rounds")
	if cfg.rounds < 1 {
		cfg.rounds = 1 // same floor the control server applies
	}
	cfg.requireHard, _ = cmd.Flags().GetBool("require-hard")

	if cfg.provider == "" {
		cfg.provider = "opencode-admin"
	}
	switch {
	case cfg.taskID == "":
		return cfg, benchHarnessErr("--task is required")
	case len(cfg.models) == 0:
		return cfg, benchHarnessErr("--model is required (comma-separated model ids)")
	}
	return cfg, nil
}

// benchApplyProfile fills empty baseURL/dbPath/storeDir from the --profile
// environment. Explicit flags win: only empty fields take profile defaults.
// Baselines stay namespaced because the default store is <profile>/evals, so a
// qa latest never leaks into dev. When the caller uses the profile URL and the
// profile server is down, it is auto-started.
func benchApplyProfile(ctx context.Context, cfg *benchConfig) error {
	name := strings.TrimSpace(cfg.profile)
	if name == "" {
		return nil
	}
	p, err := envprofile.Get(name)
	if err != nil {
		return benchHarnessErr("unknown profile %q: %v", name, err)
	}
	env := envprofile.Env(p)
	dirs := envprofile.Dirs(p)
	if cfg.baseURL == "" {
		cfg.baseURL = env["OPENCODE_URL"]
		if running, _ := envprofile.Status(p); !running {
			bin, _ := agent.FindOpenCode()
			if bin == "" {
				return benchHarnessErr("profile %q server is stopped and opencode2 binary not found in PATH", p.Name)
			}
			if err := envprofile.Start(ctx, p, bin); err != nil {
				return benchHarnessErr("auto-start profile %q server: %v", p.Name, err)
			}
		}
	}
	if cfg.dbPath == "" {
		if v := env["OPENCODE_DB"]; v != "" {
			cfg.dbPath = v
		} else {
			cfg.dbPath = filepath.Join(dirs["data"], "opencode", "opencode.db")
		}
	}
	if cfg.storeDir == "" {
		cfg.storeDir = dirs["evals"]
	}
	return nil
}

// benchOutcome is the structured result of one benchmark execution, shared by
// bench (single profile) and compare (one entry per profile).
type benchOutcome struct {
	task      evals.Task
	attempts  []evals.Attempt
	summaries []evals.ModelTaskSummary
	baseRun   *evals.Run
	deltas    []evals.ModelDelta
	baseURL   string
}

// benchExecute runs one benchmark and returns its structured outcome. Gates
// are not applied here so callers (bench, compare) can decide pass/fail from
// the same numbers.
func benchExecute(ctx context.Context, cfg benchConfig) (benchOutcome, error) {
	var out benchOutcome
	if err := benchApplyProfile(ctx, &cfg); err != nil {
		return out, err
	}
	root := cfg.rootDir
	if root == "" {
		root, _ = os.Getwd()
	}
	task, err := evals.FindTask(root, cfg.taskID)
	if err != nil {
		return out, benchHarnessErr("%v", err)
	}
	out.task = task

	db, err := benchOpenDB(cfg.dbPath)
	if err != nil {
		return out, err
	}
	defer db.Close()

	// The baseline is the one thing that needs the shared run store, and a run
	// without --baseline-run must not touch it at all.
	if mode := strings.ToLower(cfg.baselineRun); mode != "" && mode != "none" {
		store, err := benchOpenStore(cfg.storeDir)
		if err != nil {
			return out, err
		}
		run, err := benchResolveBaseline(store, cfg.baselineRun, task.ID)
		if err != nil {
			return out, err
		}
		out.baseRun = &run
	}

	baseURL := cfg.baseURL
	if baseURL == "" {
		baseURL = benchOpenCodeURL()
	}
	out.baseURL = baseURL
	runner := &evals.Runner{
		BaseURL: baseURL,
		DB:      db,
		// Each attempt is a full agent session; the ceiling is per-request, not per-run.
		Client: &http.Client{Timeout: 30 * time.Minute},
	}

	ctx, cancel := context.WithTimeout(ctx, 6*time.Hour)
	defer cancel()

	fmt.Fprintf(os.Stderr, "Bench %s on %s (rounds=%d, provider=%s) via %s\n",
		task.ID, strings.Join(cfg.models, ","), cfg.rounds, cfg.provider, baseURL)
	if err := runner.Preflight(ctx, task.Agent, cfg.models[0], cfg.provider); err != nil {
		return out, benchHarnessErr("preflight: %v", err)
	}

	run := evals.Run{
		ID:        fmt.Sprintf("run-%d", time.Now().UnixMilli()),
		TaskID:    task.ID,
		TaskName:  task.Name,
		Agent:     task.Agent,
		Provider:  cfg.provider,
		Rounds:    cfg.rounds,
		Models:    cfg.models,
		Attempts:  []evals.Attempt{},
		Status:    "running",
		StartedAt: time.Now().UTC(),
	}
	attempts, err := runner.Execute(ctx, task, evals.RunRequest{
		TaskID:   task.ID,
		Models:   cfg.models,
		Provider: cfg.provider,
		Rounds:   cfg.rounds,
	}, func(a evals.Attempt) {
		fmt.Fprintln(os.Stderr, benchAttemptLine(a))
	})
	if err != nil {
		return out, benchHarnessErr("run: %v", err)
	}
	run.Attempts = attempts
	run.Status = "done"
	run.EndedAt = time.Now().UTC()

	out.attempts = attempts
	out.summaries = evals.Aggregate([]evals.Run{run}, task.ID)
	if out.baseRun != nil {
		out.deltas = evals.DiffModels(out.summaries, evals.Aggregate([]evals.Run{*out.baseRun}, task.ID))
	}
	return out, nil
}

// benchRun executes one headless benchmark and returns the summary JSON. The
// error carries its exit class, so the command can exit 1 for a gate miss and
// 2 for a harness fault without re-interpreting messages.
func benchRun(ctx context.Context, cfg benchConfig) ([]byte, error) {
	out, err := benchExecute(ctx, cfg)
	if err != nil {
		return nil, err
	}
	task := out.task

	if err := benchCheckGates(cfg, cfg.models, out.attempts, out.summaries, out.deltas, out.baseRun != nil); err != nil {
		return nil, err
	}

	body := map[string]any{"taskId": task.ID, "summaries": out.summaries}
	if out.baseRun != nil {
		body["baseline"] = map[string]any{"runId": out.baseRun.ID, "deltas": out.deltas}
	}
	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return nil, benchHarnessErr("encode summary: %v", err)
	}
	return append(data, '\n'), nil
}

// ─── eval compare ─────────────────────────────────────────────────────────
// Headless cross-profile comparison: the same task runs sequentially in each
// named profile (each with its own server, database and eval store) and the
// per-profile summaries print as one side-by-side table plus a verdict line.

var evalCompareCmd = &cobra.Command{
	Use:   "compare",
	Short: "Run one evals task in several profiles and compare the results",
	Long: `Run one evals task sequentially in each named profile and compare results.

Each profile uses its own server, database and eval store, so baselines never
leak across profiles. Prints one side-by-side table (pass/fail, avgWeighted,
latency, cost) plus a verdict line naming the leading profile.

Examples:
  ywai eval compare --profiles dev,qa --task find-session-deletes --model anthropic/claude-sonnet-4-5
  ywai eval compare --profiles dev,qa --task find-session-deletes --model m1,m2 --rounds 2`,
	RunE: runEvalCompare,
}

// compareEntry is one profile's outcome inside a compare run.
type compareEntry struct {
	Profile   string                   `json:"profile"`
	Summaries []evals.ModelTaskSummary `json:"summaries"`
	GateError string                   `json:"gateError,omitempty"`
	RunError  string                   `json:"runError,omitempty"`
}

func runEvalCompare(cmd *cobra.Command, _ []string) error {
	get := func(name string) string {
		s, _ := cmd.Flags().GetString(name)
		return strings.TrimSpace(s)
	}
	var profiles []string
	for _, p := range strings.Split(get("profiles"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			profiles = append(profiles, p)
		}
	}
	taskID := get("task")
	var models []string
	for _, m := range strings.Split(get("model"), ",") {
		if m = strings.TrimSpace(m); m != "" {
			models = append(models, m)
		}
	}
	provider := get("provider")
	if provider == "" {
		provider = "opencode-admin"
	}
	rounds, _ := cmd.Flags().GetInt("rounds")
	if rounds < 1 {
		rounds = 1
	}
	minScore, _ := cmd.Flags().GetFloat64("min-score")
	requireHard, _ := cmd.Flags().GetBool("require-hard")
	maxCostUSD, _ := cmd.Flags().GetFloat64("max-cost-usd")
	outPath := get("out")

	switch {
	case len(profiles) == 0:
		fmt.Fprintf(os.Stderr, "eval compare: %v\n", benchHarnessErr("--profiles is required (comma-separated profile names)"))
		os.Exit(benchExitHarnessErr)
		return nil // unreachable
	case taskID == "":
		fmt.Fprintf(os.Stderr, "eval compare: %v\n", benchHarnessErr("--task is required"))
		os.Exit(benchExitHarnessErr)
		return nil // unreachable
	case len(models) == 0:
		fmt.Fprintf(os.Stderr, "eval compare: %v\n", benchHarnessErr("--model is required (comma-separated model ids)"))
		os.Exit(benchExitHarnessErr)
		return nil // unreachable
	}

	gateCfg := benchConfig{minScore: minScore, requireHard: requireHard, maxCostUSD: maxCostUSD}
	entries := make([]compareEntry, 0, len(profiles))
	// Sequential per profile: parallel runs would contend for provider quota
	// and skew exactly the latency numbers being compared.
	for _, name := range profiles {
		cfg := benchConfig{
			taskID:      taskID,
			models:      models,
			provider:    provider,
			rounds:      rounds,
			baselineRun: "none",
			profile:     name,
		}
		fmt.Fprintf(os.Stderr, "Compare %s in profile %q\n", taskID, name)
		out, err := benchExecute(cmd.Context(), cfg)
		entry := compareEntry{Profile: name}
		if err != nil {
			entry.RunError = err.Error()
			entries = append(entries, entry)
			fmt.Fprintf(os.Stderr, "  profile %s: ERROR %v\n", name, err)
			continue
		}
		entry.Summaries = out.summaries
		if gerr := benchCheckGates(gateCfg, models, out.attempts, out.summaries, nil, false); gerr != nil {
			entry.GateError = gerr.Error()
		}
		entries = append(entries, entry)
	}

	printCompareTable(entries, models)
	printCompareVerdict(entries, models)

	if outPath != "" {
		body := map[string]any{"taskId": taskID, "results": entries}
		data, err := json.MarshalIndent(body, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "eval compare: %v\n", err)
			os.Exit(benchExitHarnessErr)
			return nil // unreachable
		}
		if err := os.WriteFile(outPath, append(data, '\n'), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "eval compare: %v\n", err)
			os.Exit(benchExitHarnessErr)
			return nil // unreachable
		}
	}
	for _, e := range entries {
		if e.RunError != "" {
			os.Exit(benchExitHarnessErr)
			return nil // unreachable
		}
	}
	for _, e := range entries {
		if e.GateError != "" {
			os.Exit(benchExitGateMiss)
			return nil // unreachable
		}
	}
	return nil
}

// printCompareTable renders one row per (profile, model) pair so multi-model
// compares stay readable; single-model runs read as one row per profile.
func printCompareTable(entries []compareEntry, models []string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PROFILE\tMODEL\tRESULT\tAVG\tLATENCY\tCOST")
	for _, e := range entries {
		if e.RunError != "" {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", e.Profile, strings.Join(models, ","), "ERROR", "—", "—", "—")
			continue
		}
		byModel := make(map[string]evals.ModelTaskSummary, len(e.Summaries))
		for _, s := range e.Summaries {
			byModel[s.Model] = s
		}
		for _, m := range models {
			result := "PASS"
			if e.GateError != "" {
				result = "FAIL"
			}
			s, ok := byModel[m]
			if !ok {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", e.Profile, m, result, "—", "—", "—")
				continue
			}
			cost := "unknown"
			if s.CostKnown {
				cost = fmt.Sprintf("$%.4f", s.TotalCostUSD)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%.3f\t%.1fs\t%s\n",
				e.Profile, m, result, s.AvgWeighted, s.AvgSeconds, cost)
		}
	}
	_ = w.Flush()
	for _, e := range entries {
		if e.RunError != "" {
			fmt.Printf("  %s: error: %s\n", e.Profile, e.RunError)
		} else if e.GateError != "" {
			fmt.Printf("  %s: %s\n", e.Profile, e.GateError)
		}
	}
}

// printCompareVerdict names the leading (profile, model) pair by avgWeighted.
// Profiles that errored never win: there is no score to rank.
func printCompareVerdict(entries []compareEntry, models []string) {
	bestProfile, bestModel, bestAvg := "", "", -1.0
	found := false
	for _, e := range entries {
		if e.RunError != "" {
			continue
		}
		for _, s := range e.Summaries {
			if !found || s.AvgWeighted > bestAvg {
				bestProfile, bestModel, bestAvg = e.Profile, s.Model, s.AvgWeighted
				found = true
			}
		}
	}
	if !found {
		fmt.Println("Verdict: no profile completed")
		return
	}
	if len(models) == 1 {
		fmt.Printf("Verdict: %s leads on %s (avg %.3f)\n", bestProfile, bestModel, bestAvg)
		return
	}
	fmt.Printf("Verdict: %s leads on %s (avg %.3f)\n", bestProfile, bestModel, bestAvg)
}

// benchCheckGates applies the gates in contract order and returns the first
// miss, so the printed reason is deterministic when several would fail.
func benchCheckGates(cfg benchConfig, models []string, attempts []evals.Attempt, summaries []evals.ModelTaskSummary, deltas []evals.ModelDelta, hasBaseline bool) *benchError {
	// 1. min-score, per model avgWeighted. A model with no counted attempt has
	// no score at all, which for a gate counts as zero: silence is not a pass.
	avg := make(map[string]float64, len(summaries))
	for _, s := range summaries {
		avg[s.Model] = s.AvgWeighted
	}
	for _, m := range models {
		if avg[m] < cfg.minScore {
			return benchGateMiss("min-score: model %s avgWeighted %.3f < %.3f", m, avg[m], cfg.minScore)
		}
	}

	// 2. require-hard: every counted (answered) attempt hit the hard
	// expectation. Unanswered attempts stay uncounted, per Aggregate's rules.
	if cfg.requireHard {
		for _, a := range attempts {
			if a.Score.Answered && !a.Score.GotHard {
				return benchGateMiss("require-hard: model %s round %d missed the hard expectation", a.Model, a.Round)
			}
		}
	}

	// 3. regression against the baseline, on the shared DiffModels math. A nil
	// weightedDelta means the baseline had no summary for that model, so there
	// is nothing to regress against and it cannot fail.
	if hasBaseline {
		for _, d := range deltas {
			if d.WeightedDelta != nil && *d.WeightedDelta < -cfg.maxRegression {
				return benchGateMiss("regression: model %s weightedDelta %+.3f beyond --max-regression %.3f", d.Model, *d.WeightedDelta, cfg.maxRegression)
			}
		}
	}

	// 4. cost: per-model total for this run, only when priced; the default of
	// 0 leaves the gate off.
	if cfg.maxCostUSD > 0 {
		for _, s := range summaries {
			if s.CostKnown && s.TotalCostUSD > cfg.maxCostUSD {
				return benchGateMiss("cost: model %s totalCostUsd %.4f > --max-cost-usd %.4f", s.Model, s.TotalCostUSD, cfg.maxCostUSD)
			}
		}
	}
	return nil
}

// benchAttemptLine is the per-attempt progress line, so a caller watching a
// run that costs real minutes can see it move.
func benchAttemptLine(a evals.Attempt) string {
	line := fmt.Sprintf("  round %d %s: weighted %.2f (%d/%d, hard=%v) %.1fs",
		a.Round, a.Model, a.Score.Weighted, len(a.Score.Hits), a.Score.Total, a.Score.GotHard, a.Seconds)
	if a.Error != "" {
		line += " ERROR: " + a.Error
	}
	return line
}

// benchResolveBaseline resolves --baseline-run: latest picks the newest stored
// run of the task, anything else is a run id. An unresolvable baseline is a
// harness fault — a gate silently comparing against nothing would be worse
// than failing the run.
func benchResolveBaseline(store *evals.Store, mode, taskID string) (evals.Run, error) {
	if strings.EqualFold(mode, "latest") {
		runs, err := store.ListRuns() // newest first
		if err != nil {
			return evals.Run{}, benchHarnessErr("list runs: %v", err)
		}
		for _, r := range runs {
			if r.TaskID == taskID {
				return r, nil
			}
		}
		return evals.Run{}, benchHarnessErr("--baseline-run latest: no stored run of task %q", taskID)
	}
	run, err := store.GetRun(mode)
	if err != nil {
		return evals.Run{}, benchHarnessErr("--baseline-run %s: %v", mode, err)
	}
	if run.TaskID != taskID {
		return evals.Run{}, benchHarnessErr("--baseline-run %s is a run of task %q, not %q", mode, run.TaskID, taskID)
	}
	return run, nil
}

// benchOpenCodeURL mirrors the control server's bench target: OPENCODE_URL
// wins, otherwise OpenCode's own default port.
func benchOpenCodeURL() string {
	if u := strings.TrimSpace(os.Getenv("OPENCODE_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://localhost:4096"
}

// benchDBPath is where the OpenCode SQLite database lives; the readbacks that
// score attempts (tool calls, tokens) come out of it.
func benchDBPath() string {
	if p := strings.TrimSpace(os.Getenv("OPENCODE_DB")); p != "" {
		return p
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
		return filepath.Join(xdg, "opencode", "opencode.db")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "opencode", "opencode.db")
}

// benchOpenDB opens the OpenCode database read-only, the way the control
// server does: TEMP tables still work and OpenCode stays free to write.
func benchOpenDB(path string) (*sql.DB, error) {
	if path == "" {
		path = benchDBPath()
	}
	if _, err := os.Stat(path); err != nil {
		return nil, benchHarnessErr("opencode database not found at %s (set OPENCODE_DB): %v", path, err)
	}
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, benchHarnessErr("open opencode database: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, benchHarnessErr("open opencode database: %v", err)
	}
	return db, nil
}

// benchOpenStore opens the shared eval run store so --baseline-run can see
// runs started from the control UI; empty means the default data dir.
func benchOpenStore(dir string) (*evals.Store, error) {
	if dir == "" {
		dir = config.DataDir()
	}
	store, err := evals.OpenStore(dir)
	if err != nil {
		return nil, benchHarnessErr("open eval run store at %s: %v", dir, err)
	}
	return store, nil
}
