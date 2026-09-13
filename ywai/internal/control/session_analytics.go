package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// SessionAnalytics is an aggregate view of real OpenCode session activity:
// skills invoked, tools used, cost/tokens, broken down by project.
type SessionAnalytics struct {
	GeneratedAt  string               `json:"generatedAt"`
	DBPath       string               `json:"dbPath"`
	Days         int                  `json:"days"` // 0 = all time
	ProjectID    string               `json:"projectId,omitempty"`
	Summary      SessionAnalyticsSum  `json:"summary"`
	Insights     []string             `json:"insights"`
	UnusedSkills []string             `json:"unusedSkills"`
	Projects     []SessionProjectStat `json:"projects"`
	Skills       []SessionNamedCount  `json:"skills"`
	Tools        []SessionNamedCount  `json:"tools"`
	Agents       []SessionNamedCount  `json:"agents"`
	Models       []SessionNamedCount  `json:"models"`
}

// SessionAnalyticsSum holds top-line KPIs.
type SessionAnalyticsSum struct {
	Sessions           int     `json:"sessions"`
	Projects           int     `json:"projects"`
	SkillCalls         int     `json:"skillCalls"`
	DistinctSkills     int     `json:"distinctSkills"`
	ToolCalls          int     `json:"toolCalls"`
	TotalCost          float64 `json:"totalCost"`
	TokensInput        int64   `json:"tokensInput"`
	TokensOutput       int64   `json:"tokensOutput"`
	SessionsWithSkill  int     `json:"sessionsWithSkill"`
	RootSessions       int     `json:"rootSessions"`
	AvgToolsPerSession float64 `json:"avgToolsPerSession"`
	AvgCostPerSession  float64 `json:"avgCostPerSession"`
	SkillCoverage      float64 `json:"skillCoverage"` // sessionsWithSkill / sessions
	DelegationCalls    int     `json:"delegationCalls"`
	InstalledSkills    int     `json:"installedSkills"`
	UnusedSkillCount   int     `json:"unusedSkillCount"`
}

// SessionProjectStat is one OpenCode project row with usage stats.
type SessionProjectStat struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Worktree   string  `json:"worktree"`
	Sessions   int     `json:"sessions"`
	SkillCalls int     `json:"skillCalls"`
	ToolCalls  int     `json:"toolCalls"`
	Cost       float64 `json:"cost"`
	TokensIn   int64   `json:"tokensInput"`
	TokensOut  int64   `json:"tokensOutput"`
}

// SessionNamedCount is a skill/tool/agent/model name with usage counts.
type SessionNamedCount struct {
	Name      string  `json:"name"`
	Count     int     `json:"count"`
	Sessions  int     `json:"sessions,omitempty"`
	Share     float64 `json:"share"` // fraction of the ranking total (0–1)
	Cost      float64 `json:"cost,omitempty"`
	TokensIn  int64   `json:"tokensInput,omitempty"`
	TokensOut int64   `json:"tokensOutput,omitempty"`
}

// AnalyticsQuery filters SessionAnalytics.
type AnalyticsQuery struct {
	ProjectID   string // exact project id
	Worktree    string // substring match on project.worktree (case-insensitive)
	Days        int    // lookback window; 0 = all time
	ToolsLimit  int    // max tools to return (default 30)
	SkillsLimit int    // max skills to return (default 50)
}

func defaultOpenCodeDBPath() string {
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

func openOpenCodeDB(path string) (*sql.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("opencode database path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("opencode database not found at %s: %w", path, err)
	}
	// Read-only against the main file; TEMP tables still work and keep OpenCode free to write.
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open opencode db: %w", err)
	}
	return db, nil
}

// LoadSessionAnalytics reads OpenCode's SQLite DB and aggregates usage.
// dbPath may be empty to use the default ~/.local/share/opencode/opencode.db.
//
// A sandboxed editor records into its own OpenCode install, so the canonical DB
// is usually not the only one. Each install is read on its own — a UNION across
// attached DBs reads the same rows but loses the per-table indexes, and the
// message tables are large enough (thousands of JSON blobs, scanned per query)
// that the difference is seconds versus minutes. TEMP tables narrow the window
// once; aggregates are combined afterwards.
func LoadSessionAnalytics(ctx context.Context, dbPath string, q AnalyticsQuery) (*SessionAnalytics, error) {
	explicit := dbPath != ""
	if !explicit {
		dbPath = defaultOpenCodeDBPath()
	}
	if q.ToolsLimit <= 0 {
		q.ToolsLimit = 30
	}
	if q.SkillsLimit <= 0 {
		q.SkillsLimit = 50
	}
	if q.Days < 0 {
		q.Days = 0
	}

	primary, err := loadOneAnalytics(ctx, dbPath, q)
	if err != nil || explicit {
		return primary, err
	}
	for _, extra := range discoverExtraOpenCodeDBs(dbPath) {
		// One unreadable install must not blank the whole report.
		other, err := loadOneAnalytics(ctx, extra, q)
		if err != nil {
			continue
		}
		mergeAnalytics(primary, other, q)
	}
	return primary, nil
}

// hasTable reports whether the SQLite database contains the named table.
func hasTable(ctx context.Context, db *sql.DB, name string) bool {
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name,
	).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

func loadOneAnalytics(ctx context.Context, dbPath string, q AnalyticsQuery) (*SessionAnalytics, error) {
	db, err := openOpenCodeDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	out := &SessionAnalytics{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		DBPath:       dbPath,
		Days:         q.Days,
		ProjectID:    q.ProjectID,
		Projects:     []SessionProjectStat{},
		Skills:       []SessionNamedCount{},
		Tools:        []SessionNamedCount{},
		Agents:       []SessionNamedCount{},
		Models:       []SessionNamedCount{},
		Insights:     []string{},
		UnusedSkills: []string{},
	}

	// Schema generations: current servers write session_v2/session_message;
	// pre-v2 installs only have session/part, which freeze once the server
	// migrates — reading them then silently zeroes the report. Prefer v2
	// when present, fall back to legacy otherwise.
	useV2 := hasTable(ctx, db, "session_v2") && hasTable(ctx, db, "session_message")
	if !useV2 && !hasTable(ctx, db, "session") {
		// A freshly created environment database has never run a session:
		// opencode creates its schema lazily. An explicit empty report beats a
		// raw SQL error ("no such table: session") for that case.
		out.Insights = []string{"No sessions recorded in this environment yet."}
		return out, nil
	}

	// Narrow sessions once; every later query joins this TEMP set.
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS sa_sess`); err != nil {
		return nil, fmt.Errorf("prep temp sessions: %w", err)
	}
	var (
		createSQL string
		args      []any
	)
	if useV2 {
		var where string
		where, args = sessionFilterV2(q)
		createSQL = `
CREATE TEMP TABLE sa_sess AS
SELECT
  s.id AS session_id,
  s.project_id AS project_id,
  COALESCE(s.directory, '') AS directory,
  COALESCE(NULLIF(s.agent, ''), '(default)') AS agent,
  COALESCE(s.model, '') AS model,
  COALESCE(s.cost, 0) AS cost,
  COALESCE(s.tokens_input, 0) AS tokens_input,
  COALESCE(s.tokens_output, 0) AS tokens_output
FROM session_v2 s
LEFT JOIN project p ON p.id = s.project_id
WHERE ` + where
	} else {
		var where string
		where, args = sessionFilter(q)
		createSQL = `
CREATE TEMP TABLE sa_sess AS
SELECT
  s.id AS session_id,
  s.project_id AS project_id,
  COALESCE(s.directory, '') AS directory,
  COALESCE(NULLIF(s.agent, ''), '(default)') AS agent,
  COALESCE(s.model, '') AS model,
  COALESCE(s.cost, 0) AS cost,
  COALESCE(s.tokens_input, 0) AS tokens_input,
  COALESCE(s.tokens_output, 0) AS tokens_output
FROM session s
JOIN project p ON p.id = s.project_id
WHERE ` + where
	}
	if _, err := db.ExecContext(ctx, createSQL, args...); err != nil {
		return nil, fmt.Errorf("create temp sessions: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS sa_sess_id ON sa_sess(session_id)`); err != nil {
		return nil, fmt.Errorf("index temp sessions: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS sa_sess_project ON sa_sess(project_id)`); err != nil {
		return nil, fmt.Errorf("index temp sessions project: %w", err)
	}

	// Normalize tool/skill parts once. v1 stores one part row per tool call
	// ({type,tool,state.input.name}); v2 stores message rows whose content[]
	// mixes tool calls ({type,name,state.input.id}). Downstream queries read
	// only sa_parts, so both generations share them verbatim.
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS sa_parts`); err != nil {
		return nil, fmt.Errorf("prep temp parts: %w", err)
	}
	partsSQL := `
CREATE TEMP TABLE sa_parts AS
SELECT
  pt.session_id AS session_id,
  json_extract(pt.data, '$.tool') AS tool,
  CASE WHEN json_extract(pt.data, '$.tool') = 'skill'
       THEN COALESCE(json_extract(pt.data, '$.state.input.name'), '')
       ELSE '' END AS skill
FROM part pt
JOIN sa_sess ss ON ss.session_id = pt.session_id
WHERE json_extract(pt.data, '$.type') = 'tool'`
	if useV2 {
		partsSQL = `
CREATE TEMP TABLE sa_parts AS
SELECT
  sm.session_id AS session_id,
  je.value ->> 'name' AS tool,
  CASE WHEN je.value ->> 'name' = 'skill'
       THEN COALESCE(json_extract(je.value, '$.state.input.id'),
                     json_extract(je.value, '$.state.input.name'), '')
       ELSE '' END AS skill
FROM session_message sm, json_each(sm.data, '$.content') je
JOIN sa_sess ss ON ss.session_id = sm.session_id
WHERE je.value ->> 'type' = 'tool'`
	}
	if _, err := db.ExecContext(ctx, partsSQL); err != nil {
		return nil, fmt.Errorf("create temp parts: %w", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS sa_parts_session ON sa_parts(session_id)`); err != nil {
		return nil, fmt.Errorf("index temp parts: %w", err)
	}

	// ── Projects + session KPIs ──────────────────────────────────────
	projSQL := `
SELECT
  ss.project_id,
  COALESCE(NULLIF(p.name, ''), '') AS name,
  COALESCE(p.worktree, ss.directory, '') AS worktree,
  COUNT(ss.session_id) AS sessions,
  COALESCE(SUM(ss.cost), 0) AS cost,
  COALESCE(SUM(ss.tokens_input), 0) AS tin,
  COALESCE(SUM(ss.tokens_output), 0) AS tout
FROM sa_sess ss
LEFT JOIN project p ON p.id = ss.project_id
GROUP BY ss.project_id, name, worktree
ORDER BY sessions DESC`

	rows, err := db.QueryContext(ctx, projSQL)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}

	var (
		totalSessions int
		totalCost     float64
		tin, tout     int64
	)
	for rows.Next() {
		var st SessionProjectStat
		if err := rows.Scan(&st.ID, &st.Name, &st.Worktree, &st.Sessions, &st.Cost, &st.TokensIn, &st.TokensOut); err != nil {
			rows.Close()
			return nil, err
		}
		if st.Name == "" {
			st.Name = displayName(st.Worktree)
		}
		out.Projects = append(out.Projects, st)
		totalSessions += st.Sessions
		totalCost += st.Cost
		tin += st.TokensIn
		tout += st.TokensOut
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out.Summary.Sessions = totalSessions
	out.Summary.Projects = len(out.Projects)
	out.Summary.TotalCost = totalCost
	out.Summary.TokensInput = tin
	out.Summary.TokensOutput = tout

	// ── Skills (sa_parts is already narrowed + indexed) ──────────
	skillSQL := `
SELECT skill, COUNT(*) AS cnt, COUNT(DISTINCT session_id) AS sessions
FROM sa_parts
WHERE skill != ''
GROUP BY skill
ORDER BY cnt DESC
LIMIT ?`
	srows, err := db.QueryContext(ctx, skillSQL, q.SkillsLimit)
	if err != nil {
		return nil, fmt.Errorf("query skills: %w", err)
	}
	skillCalls := 0
	for srows.Next() {
		var n SessionNamedCount
		if err := srows.Scan(&n.Name, &n.Count, &n.Sessions); err != nil {
			srows.Close()
			return nil, err
		}
		out.Skills = append(out.Skills, n)
		skillCalls += n.Count
	}
	srows.Close()
	if err := srows.Err(); err != nil {
		return nil, err
	}
	out.Summary.SkillCalls = skillCalls
	out.Summary.DistinctSkills = len(out.Skills)

	var withSkill int
	err = db.QueryRowContext(ctx, `
SELECT COUNT(DISTINCT session_id) FROM sa_parts WHERE skill != ''`).Scan(&withSkill)
	if err != nil {
		return nil, fmt.Errorf("query sessions-with-skill: %w", err)
	}
	out.Summary.SessionsWithSkill = withSkill

	skillMap := map[string]int{}
	prows, err := db.QueryContext(ctx, `
SELECT ss.project_id, COUNT(*)
FROM sa_parts sp
JOIN sa_sess ss ON ss.session_id = sp.session_id
WHERE sp.skill != ''
GROUP BY ss.project_id`)
	if err != nil {
		return nil, fmt.Errorf("query skill-by-project: %w", err)
	}
	for prows.Next() {
		var id string
		var c int
		if err := prows.Scan(&id, &c); err != nil {
			prows.Close()
			return nil, err
		}
		skillMap[id] = c
	}
	prows.Close()
	for i := range out.Projects {
		out.Projects[i].SkillCalls = skillMap[out.Projects[i].ID]
	}

	// ── Tools (top N) ────────────────────────────────────────────────
	// Prefer LIKE prefilter on "type":"tool" when present; always validate with json_extract.
	toolSQL := `
SELECT tool, COUNT(*) AS cnt, COUNT(DISTINCT session_id) AS sessions
FROM sa_parts
WHERE tool IS NOT NULL AND tool != ''
GROUP BY tool
ORDER BY cnt DESC
LIMIT ?`
	trows, err := db.QueryContext(ctx, toolSQL, q.ToolsLimit)
	if err != nil {
		return nil, fmt.Errorf("query tools: %w", err)
	}
	for trows.Next() {
		var n SessionNamedCount
		if err := trows.Scan(&n.Name, &n.Count, &n.Sessions); err != nil {
			trows.Close()
			return nil, err
		}
		out.Tools = append(out.Tools, n)
	}
	trows.Close()
	if err := trows.Err(); err != nil {
		return nil, err
	}

	toolMap := map[string]int{}
	totalTools := 0
	tprows, err := db.QueryContext(ctx, `
SELECT ss.project_id, COUNT(*)
FROM sa_parts sp
JOIN sa_sess ss ON ss.session_id = sp.session_id
GROUP BY ss.project_id`)
	if err != nil {
		return nil, fmt.Errorf("query tool-by-project: %w", err)
	}
	for tprows.Next() {
		var id string
		var c int
		if err := tprows.Scan(&id, &c); err != nil {
			tprows.Close()
			return nil, err
		}
		toolMap[id] = c
		totalTools += c
	}
	tprows.Close()
	out.Summary.ToolCalls = totalTools
	for i := range out.Projects {
		out.Projects[i].ToolCalls = toolMap[out.Projects[i].ID]
	}

	// ── Agents ───────────────────────────────────────────────────────
	arows, err := db.QueryContext(ctx, `
SELECT agent, COUNT(*), COALESCE(SUM(cost), 0), COALESCE(SUM(tokens_input), 0), COALESCE(SUM(tokens_output), 0)
FROM sa_sess
GROUP BY agent
ORDER BY COUNT(*) DESC
LIMIT 40`)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	for arows.Next() {
		var n SessionNamedCount
		if err := arows.Scan(&n.Name, &n.Count, &n.Cost, &n.TokensIn, &n.TokensOut); err != nil {
			arows.Close()
			return nil, err
		}
		out.Agents = append(out.Agents, n)
	}
	arows.Close()
	applyShares(out.Agents, totalSessions)

	// ── Models ───────────────────────────────────────────────────────
	mrows, err := db.QueryContext(ctx, `
SELECT model, COUNT(*), COALESCE(SUM(cost), 0), COALESCE(SUM(tokens_input), 0), COALESCE(SUM(tokens_output), 0)
FROM sa_sess
GROUP BY model`)
	if err != nil {
		return nil, fmt.Errorf("query models: %w", err)
	}
	modelAgg := map[string]*SessionNamedCount{}
	for mrows.Next() {
		var raw string
		var cnt int
		var cost float64
		var tin, tout int64
		if err := mrows.Scan(&raw, &cnt, &cost, &tin, &tout); err != nil {
			mrows.Close()
			return nil, err
		}
		label := normalizeModelLabel(raw)
		if cur, ok := modelAgg[label]; ok {
			cur.Count += cnt
			cur.Cost += cost
			cur.TokensIn += tin
			cur.TokensOut += tout
		} else {
			modelAgg[label] = &SessionNamedCount{
				Name: label, Count: cnt, Cost: cost, TokensIn: tin, TokensOut: tout,
			}
		}
	}
	mrows.Close()
	for _, m := range modelAgg {
		out.Models = append(out.Models, *m)
	}
	sort.SliceStable(out.Models, func(i, j int) bool {
		return out.Models[i].Count > out.Models[j].Count
	})
	if len(out.Models) > 40 {
		out.Models = out.Models[:40]
	}
	applyShares(out.Models, totalSessions)
	applyShares(out.Skills, skillCalls)
	applyShares(out.Tools, totalTools)

	if out.Projects == nil {
		out.Projects = []SessionProjectStat{}
	}
	if out.Skills == nil {
		out.Skills = []SessionNamedCount{}
	}
	if out.Tools == nil {
		out.Tools = []SessionNamedCount{}
	}
	if out.Agents == nil {
		out.Agents = []SessionNamedCount{}
	}
	if out.Models == nil {
		out.Models = []SessionNamedCount{}
	}

	sort.SliceStable(out.Projects, func(i, j int) bool {
		if out.Projects[i].Sessions != out.Projects[j].Sessions {
			return out.Projects[i].Sessions > out.Projects[j].Sessions
		}
		return out.Projects[i].SkillCalls > out.Projects[j].SkillCalls
	})

	enrichAnalytics(out)
	return out, nil
}

// enrichAnalytics fills derived KPIs, unused skills, and human-readable insights.
func enrichAnalytics(a *SessionAnalytics) {
	if a == nil {
		return
	}
	s := &a.Summary
	if s.Sessions > 0 {
		s.AvgToolsPerSession = float64(s.ToolCalls) / float64(s.Sessions)
		s.AvgCostPerSession = s.TotalCost / float64(s.Sessions)
		s.SkillCoverage = float64(s.SessionsWithSkill) / float64(s.Sessions)
		if s.RootSessions == 0 {
			// Fallback when SQL path did not set them.
			s.RootSessions = s.Sessions
		}
	}

	// Installed skills vs used
	installed := listInstalledSkillNames()
	s.InstalledSkills = len(installed)
	used := map[string]bool{}
	for _, sk := range a.Skills {
		used[strings.ToLower(sk.Name)] = true
		// also bare name if path-like
		if i := strings.LastIndex(sk.Name, "/"); i >= 0 {
			used[strings.ToLower(sk.Name[i+1:])] = true
		}
	}
	unused := make([]string, 0)
	for _, name := range installed {
		if !used[strings.ToLower(name)] {
			unused = append(unused, name)
		}
	}
	sort.Strings(unused)
	a.UnusedSkills = unused
	s.UnusedSkillCount = len(unused)

	// Delegation calls from the tools list.
	for _, t := range a.Tools {
		if t.Name == "delegate" || t.Name == "task" || strings.HasPrefix(t.Name, "delegation") {
			s.DelegationCalls += t.Count
		}
	}

	a.Insights = buildInsights(a)
}

func buildInsights(a *SessionAnalytics) []string {
	var out []string
	s := a.Summary
	if s.Sessions == 0 {
		return []string{"No sessions in this time window. Open OpenCode and work a bit, then re-run."}
	}

	if len(a.Agents) > 0 {
		top := a.Agents[0]
		out = append(out, fmt.Sprintf(
			"Top agent is %s (%d sessions, %.0f%% of volume).",
			top.Name, top.Count, top.Share*100,
		))
	}
	if len(a.Skills) > 0 {
		top := a.Skills[0]
		out = append(out, fmt.Sprintf(
			"Most loaded skill is %s (%d calls across %d sessions).",
			top.Name, top.Count, top.Sessions,
		))
	} else {
		out = append(out, "No skill tool calls in this window — agents never invoked the skill loader.")
	}

	out = append(out, fmt.Sprintf(
		"Skill coverage: %.0f%% of sessions loaded at least one skill (%d/%d).",
		s.SkillCoverage*100, s.SessionsWithSkill, s.Sessions,
	))

	if s.UnusedSkillCount > 0 && s.InstalledSkills > 0 {
		preview := a.UnusedSkills
		if len(preview) > 5 {
			preview = preview[:5]
		}
		out = append(out, fmt.Sprintf(
			"%d of %d installed skills never used in this window (e.g. %s).",
			s.UnusedSkillCount, s.InstalledSkills, strings.Join(preview, ", "),
		))
	}

	if len(a.Models) > 0 {
		out = append(out, fmt.Sprintf(
			"Primary model: %s (%.0f%% of sessions).",
			a.Models[0].Name, a.Models[0].Share*100,
		))
	}

	if s.DelegationCalls > 0 {
		out = append(out, fmt.Sprintf(
			"Delegation/task tools fired %d times — multi-agent orchestration is active.",
			s.DelegationCalls,
		))
	}

	if s.AvgToolsPerSession > 0 {
		out = append(out, fmt.Sprintf(
			"Avg %.1f tool calls per session · avg cost $%.4f/session.",
			s.AvgToolsPerSession, s.AvgCostPerSession,
		))
	}

	return out
}

// listInstalledSkillNames reads skill folder names from common OpenCode / ywai paths.
func listInstalledSkillNames() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dirs := []string{
		filepath.Join(home, ".config", "opencode", "skills"),
		filepath.Join(home, ".ywai", "skills"),
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		dirs = append(dirs, filepath.Join(xdg, "opencode", "skills"))
	}
	seen := map[string]bool{}
	var names []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			// Skip obvious non-skills
			n := e.Name()
			if seen[n] {
				continue
			}
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

func applyShares(items []SessionNamedCount, total int) {
	if total <= 0 {
		return
	}
	for i := range items {
		items[i].Share = float64(items[i].Count) / float64(total)
	}
}

func sortNamedByCount(items []SessionNamedCount) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})
}

// normalizeModelLabel turns OpenCode's JSON model blob into provider/id.
func normalizeModelLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "(default)"
	}
	if strings.HasPrefix(raw, "{") {
		var m struct {
			ID         string `json:"id"`
			ProviderID string `json:"providerID"`
			Variant    string `json:"variant"`
		}
		if err := json.Unmarshal([]byte(raw), &m); err == nil && m.ID != "" {
			label := m.ID
			if m.ProviderID != "" {
				label = m.ProviderID + "/" + m.ID
			}
			if v := strings.TrimSpace(m.Variant); v != "" && v != "default" {
				label += " (" + v + ")"
			}
			return label
		}
	}
	return raw
}

// sessionFilterV2 is sessionFilter for the session_v2 generation: project
// rows may be missing, so the worktree match falls back to the session
// directory and the project id matches either side of the LEFT JOIN.
func sessionFilterV2(q AnalyticsQuery) (string, []any) {
	parts := []string{"1=1"}
	var args []any

	if q.ProjectID != "" {
		parts = append(parts, "(p.id = ? OR s.project_id = ?)")
		args = append(args, q.ProjectID, q.ProjectID)
	}
	if w := strings.TrimSpace(q.Worktree); w != "" {
		parts = append(parts, "LOWER(COALESCE(p.worktree, s.directory)) LIKE ?")
		args = append(args, "%"+strings.ToLower(w)+"%")
	}
	if q.Days > 0 {
		// OpenCode stores time_created as unix ms.
		cutoff := time.Now().AddDate(0, 0, -q.Days).UnixMilli()
		parts = append(parts, "s.time_created >= ?")
		args = append(args, cutoff)
	}
	parts = append(parts, "(s.time_archived IS NULL OR s.time_archived = 0)")

	return strings.Join(parts, " AND "), args
}

// sessionFilter builds a SQL WHERE clause for session+project (aliases s, p).
func sessionFilter(q AnalyticsQuery) (string, []any) {
	parts := []string{"1=1"}
	var args []any

	if q.ProjectID != "" {
		parts = append(parts, "p.id = ?")
		args = append(args, q.ProjectID)
	}
	if w := strings.TrimSpace(q.Worktree); w != "" {
		parts = append(parts, "LOWER(p.worktree) LIKE ?")
		args = append(args, "%"+strings.ToLower(w)+"%")
	}
	if q.Days > 0 {
		// OpenCode stores time_created as unix ms.
		cutoff := time.Now().AddDate(0, 0, -q.Days).UnixMilli()
		parts = append(parts, "s.time_created >= ?")
		args = append(args, cutoff)
	}
	parts = append(parts, "(s.time_archived IS NULL OR s.time_archived = 0)")

	return strings.Join(parts, " AND "), args
}

func displayName(worktree string) string {
	worktree = strings.TrimRight(worktree, string(filepath.Separator))
	base := filepath.Base(worktree)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return worktree
	}
	return base
}
