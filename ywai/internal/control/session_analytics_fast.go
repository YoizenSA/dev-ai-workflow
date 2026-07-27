package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// loadSessionAnalyticsFast uses the system sqlite3 CLI when available.
// modernc.org/sqlite is pure-Go but ~100x slower on multi-GB OpenCode DBs.
func loadSessionAnalyticsFast(ctx context.Context, dbPath string, q AnalyticsQuery) (*SessionAnalytics, error) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		return nil, err
	}
	if q.ToolsLimit <= 0 {
		q.ToolsLimit = 30
	}
	if q.SkillsLimit <= 0 {
		q.SkillsLimit = 50
	}

	script := buildAnalyticsSQL(q)
	cmd := exec.CommandContext(ctx, "sqlite3", "-batch", "-json", dbPath)
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("sqlite3: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		return emptyAnalytics(dbPath, q), nil
	}

	chunks := splitJSONArrays(raw)
	// Expect at least 0..7 base sets; 8 activity, 9 categories, 10 session meta optional
	if len(chunks) < 8 {
		return nil, fmt.Errorf("sqlite3: expected >=8 result sets, got %d", len(chunks))
	}

	out := emptyAnalytics(dbPath, q)
	out.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	type projRow struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		Worktree string  `json:"worktree"`
		Sessions int     `json:"sessions"`
		Cost     float64 `json:"cost"`
		Tin      int64   `json:"tin"`
		Tout     int64   `json:"tout"`
	}
	type namedRow struct {
		Name     string  `json:"name"`
		Count    int     `json:"cnt"`
		Sessions int     `json:"sessions"`
		Cost     float64 `json:"cost"`
		Tin      int64   `json:"tin"`
		Tout     int64   `json:"tout"`
	}
	type countRow struct {
		N int `json:"n"`
	}
	type idCount struct {
		ID  string `json:"id"`
		Cnt int    `json:"cnt"`
	}
	type dayRow struct {
		Day      string `json:"day"`
		Sessions int    `json:"sessions"`
	}
	type metaRow struct {
		Root       int   `json:"root"`
		Child      int   `json:"child"`
		Reasoning  int64 `json:"reasoning"`
		CacheRead  int64 `json:"cache_read"`
		CacheWrite int64 `json:"cache_write"`
	}

	// 0 projects
	var projects []projRow
	if err := json.Unmarshal(chunks[0], &projects); err != nil {
		return nil, fmt.Errorf("decode projects: %w", err)
	}
	totalSessions := 0
	var totalCost float64
	var tin, tout int64
	for _, p := range projects {
		name := p.Name
		if name == "" {
			name = displayName(p.Worktree)
		}
		out.Projects = append(out.Projects, SessionProjectStat{
			ID: p.ID, Name: name, Worktree: p.Worktree,
			Sessions: p.Sessions, Cost: p.Cost, TokensIn: p.Tin, TokensOut: p.Tout,
		})
		totalSessions += p.Sessions
		totalCost += p.Cost
		tin += p.Tin
		tout += p.Tout
	}
	out.Summary.Sessions = totalSessions
	out.Summary.Projects = len(out.Projects)
	out.Summary.TotalCost = totalCost
	out.Summary.TokensInput = tin
	out.Summary.TokensOutput = tout

	// 1 skills
	var skills []namedRow
	if err := json.Unmarshal(chunks[1], &skills); err != nil {
		return nil, fmt.Errorf("decode skills: %w", err)
	}
	skillCalls := 0
	for _, s := range skills {
		out.Skills = append(out.Skills, SessionNamedCount{
			Name: s.Name, Count: s.Count, Sessions: s.Sessions,
		})
		skillCalls += s.Count
	}
	out.Summary.SkillCalls = skillCalls
	out.Summary.DistinctSkills = len(out.Skills)

	// 2 with-skill
	var withSkill []countRow
	_ = json.Unmarshal(chunks[2], &withSkill)
	if len(withSkill) > 0 {
		out.Summary.SessionsWithSkill = withSkill[0].N
	}

	// 3 skill by project
	var skillByProj []idCount
	_ = json.Unmarshal(chunks[3], &skillByProj)
	skillMap := map[string]int{}
	for _, r := range skillByProj {
		skillMap[r.ID] = r.Cnt
	}

	// 4 tools
	var tools []namedRow
	if err := json.Unmarshal(chunks[4], &tools); err != nil {
		return nil, fmt.Errorf("decode tools: %w", err)
	}
	for _, t := range tools {
		out.Tools = append(out.Tools, SessionNamedCount{
			Name: t.Name, Count: t.Count, Sessions: t.Sessions,
		})
	}

	// 5 tool by project
	var toolByProj []idCount
	_ = json.Unmarshal(chunks[5], &toolByProj)
	toolMap := map[string]int{}
	totalTools := 0
	for _, r := range toolByProj {
		toolMap[r.ID] = r.Cnt
		totalTools += r.Cnt
	}
	out.Summary.ToolCalls = totalTools
	for i := range out.Projects {
		out.Projects[i].SkillCalls = skillMap[out.Projects[i].ID]
		out.Projects[i].ToolCalls = toolMap[out.Projects[i].ID]
	}

	// 6 agents
	var agents []namedRow
	_ = json.Unmarshal(chunks[6], &agents)
	for _, a := range agents {
		out.Agents = append(out.Agents, SessionNamedCount{
			Name: a.Name, Count: a.Count, Cost: a.Cost, TokensIn: a.Tin, TokensOut: a.Tout,
		})
	}

	// 7 models
	var models []namedRow
	_ = json.Unmarshal(chunks[7], &models)
	modelAgg := map[string]*SessionNamedCount{}
	for _, m := range models {
		label := normalizeModelLabel(m.Name)
		if cur, ok := modelAgg[label]; ok {
			cur.Count += m.Count
			cur.Cost += m.Cost
			cur.TokensIn += m.Tin
			cur.TokensOut += m.Tout
		} else {
			modelAgg[label] = &SessionNamedCount{
				Name: label, Count: m.Count, Cost: m.Cost, TokensIn: m.Tin, TokensOut: m.Tout,
			}
		}
	}
	for _, m := range modelAgg {
		out.Models = append(out.Models, *m)
	}
	sortNamedByCount(out.Models)
	if len(out.Models) > 40 {
		out.Models = out.Models[:40]
	}

	// 8 activity
	if len(chunks) > 8 {
		var days []dayRow
		if err := json.Unmarshal(chunks[8], &days); err == nil {
			for _, d := range days {
				out.Activity = append(out.Activity, SessionDayCount{Day: d.Day, Sessions: d.Sessions})
			}
		}
	}

	// 9 tool categories
	if len(chunks) > 9 {
		var cats []namedRow
		if err := json.Unmarshal(chunks[9], &cats); err == nil {
			for _, c := range cats {
				out.ToolCategories = append(out.ToolCategories, SessionNamedCount{
					Name: c.Name, Count: c.Count,
				})
			}
			applyShares(out.ToolCategories, totalTools)
			sortNamedByCount(out.ToolCategories)
		}
	}

	// 10 session meta (root/child + cache tokens)
	if len(chunks) > 10 {
		var meta []metaRow
		if err := json.Unmarshal(chunks[10], &meta); err == nil && len(meta) > 0 {
			out.Summary.RootSessions = meta[0].Root
			out.Summary.ChildSessions = meta[0].Child
			out.Summary.TokensReasoning = meta[0].Reasoning
			out.Summary.TokensCacheRead = meta[0].CacheRead
			out.Summary.TokensCacheWrite = meta[0].CacheWrite
		}
	}

	applyShares(out.Agents, totalSessions)
	applyShares(out.Models, totalSessions)
	applyShares(out.Skills, skillCalls)
	applyShares(out.Tools, totalTools)
	sortNamedByCount(out.Agents)
	sortNamedByCount(out.Skills)
	sortNamedByCount(out.Tools)
	sort.SliceStable(out.Projects, func(i, j int) bool {
		if out.Projects[i].Sessions != out.Projects[j].Sessions {
			return out.Projects[i].Sessions > out.Projects[j].Sessions
		}
		return out.Projects[i].SkillCalls > out.Projects[j].SkillCalls
	})

	return out, nil
}

func emptyAnalytics(dbPath string, q AnalyticsQuery) *SessionAnalytics {
	return &SessionAnalytics{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		DBPath:         dbPath,
		Days:           q.Days,
		ProjectID:      q.ProjectID,
		Insights:       []string{},
		Activity:       []SessionDayCount{},
		ToolCategories: []SessionNamedCount{},
		UnusedSkills:   []string{},
		Projects:       []SessionProjectStat{},
		Skills:         []SessionNamedCount{},
		Tools:          []SessionNamedCount{},
		Agents:         []SessionNamedCount{},
		Models:         []SessionNamedCount{},
	}
}

func sortNamedByCount(items []SessionNamedCount) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})
}

func splitJSONArrays(raw []byte) [][]byte {
	var out [][]byte
	depth := 0
	start := -1
	for i, b := range raw {
		switch b {
		case '[':
			if depth == 0 {
				start = i
			}
			depth++
		case ']':
			depth--
			if depth == 0 && start >= 0 {
				out = append(out, raw[start:i+1])
				start = -1
			}
		}
	}
	return out
}

func buildAnalyticsSQL(q AnalyticsQuery) string {
	var b strings.Builder
	b.WriteString("CREATE TEMP TABLE sa_sess AS\n")
	b.WriteString(`SELECT
  s.id AS session_id,
  s.project_id AS project_id,
  s.parent_id AS parent_id,
  s.time_created AS time_created,
  COALESCE(NULLIF(s.agent, ''), '(default)') AS agent,
  COALESCE(s.model, '') AS model,
  COALESCE(s.cost, 0) AS cost,
  COALESCE(s.tokens_input, 0) AS tokens_input,
  COALESCE(s.tokens_output, 0) AS tokens_output,
  COALESCE(s.tokens_reasoning, 0) AS tokens_reasoning,
  COALESCE(s.tokens_cache_read, 0) AS tokens_cache_read,
  COALESCE(s.tokens_cache_write, 0) AS tokens_cache_write
FROM session s
JOIN project p ON p.id = s.project_id
WHERE 1=1`)
	if q.ProjectID != "" {
		b.WriteString(" AND p.id = ")
		b.WriteString(sqlQuote(q.ProjectID))
	}
	if w := strings.TrimSpace(q.Worktree); w != "" {
		b.WriteString(" AND LOWER(p.worktree) LIKE ")
		b.WriteString(sqlQuote("%" + strings.ToLower(w) + "%"))
	}
	if q.Days > 0 {
		cutoff := time.Now().AddDate(0, 0, -q.Days).UnixMilli()
		b.WriteString(" AND s.time_created >= ")
		b.WriteString(strconv.FormatInt(cutoff, 10))
	}
	b.WriteString(" AND (s.time_archived IS NULL OR s.time_archived = 0);\n")
	b.WriteString("CREATE INDEX sa_sess_id ON sa_sess(session_id);\n")
	b.WriteString("CREATE INDEX sa_sess_project ON sa_sess(project_id);\n")

	// 0 projects
	b.WriteString(`SELECT p.id AS id,
  COALESCE(NULLIF(p.name, ''), '') AS name,
  COALESCE(p.worktree, '') AS worktree,
  COUNT(ss.session_id) AS sessions,
  COALESCE(SUM(ss.cost), 0) AS cost,
  COALESCE(SUM(ss.tokens_input), 0) AS tin,
  COALESCE(SUM(ss.tokens_output), 0) AS tout
FROM sa_sess ss
JOIN project p ON p.id = ss.project_id
GROUP BY p.id, p.name, p.worktree
ORDER BY sessions DESC;
`)

	// 1 skills
	fmt.Fprintf(&b, `SELECT
  COALESCE(json_extract(pt.data, '$.state.input.name'), '') AS name,
  COUNT(*) AS cnt,
  COUNT(DISTINCT pt.session_id) AS sessions
FROM part pt
JOIN sa_sess ss ON ss.session_id = pt.session_id
WHERE pt.data LIKE '%%"tool":"skill"%%'
  AND json_extract(pt.data, '$.tool') = 'skill'
GROUP BY name
HAVING name != ''
ORDER BY cnt DESC
LIMIT %d;
`, q.SkillsLimit)

	// 2 with-skill
	b.WriteString(`SELECT COUNT(DISTINCT pt.session_id) AS n
FROM part pt
JOIN sa_sess ss ON ss.session_id = pt.session_id
WHERE pt.data LIKE '%"tool":"skill"%'
  AND json_extract(pt.data, '$.tool') = 'skill';
`)

	// 3 skill by project
	b.WriteString(`SELECT ss.project_id AS id, COUNT(*) AS cnt
FROM part pt
JOIN sa_sess ss ON ss.session_id = pt.session_id
WHERE pt.data LIKE '%"tool":"skill"%'
  AND json_extract(pt.data, '$.tool') = 'skill'
GROUP BY ss.project_id;
`)

	// 4 tools
	fmt.Fprintf(&b, `SELECT
  COALESCE(json_extract(pt.data, '$.tool'), '') AS name,
  COUNT(*) AS cnt,
  COUNT(DISTINCT pt.session_id) AS sessions
FROM part pt
JOIN sa_sess ss ON ss.session_id = pt.session_id
WHERE json_extract(pt.data, '$.type') = 'tool'
  AND COALESCE(json_extract(pt.data, '$.tool'), '') != ''
GROUP BY name
ORDER BY cnt DESC
LIMIT %d;
`, q.ToolsLimit)

	// 5 tool by project
	b.WriteString(`SELECT ss.project_id AS id, COUNT(*) AS cnt
FROM part pt
JOIN sa_sess ss ON ss.session_id = pt.session_id
WHERE json_extract(pt.data, '$.type') = 'tool'
GROUP BY ss.project_id;
`)

	// 6 agents
	b.WriteString(`SELECT agent AS name, COUNT(*) AS cnt,
  COALESCE(SUM(cost), 0) AS cost,
  COALESCE(SUM(tokens_input), 0) AS tin,
  COALESCE(SUM(tokens_output), 0) AS tout
FROM sa_sess
GROUP BY agent
ORDER BY cnt DESC
LIMIT 40;
`)

	// 7 models
	b.WriteString(`SELECT model AS name, COUNT(*) AS cnt,
  COALESCE(SUM(cost), 0) AS cost,
  COALESCE(SUM(tokens_input), 0) AS tin,
  COALESCE(SUM(tokens_output), 0) AS tout
FROM sa_sess
GROUP BY model;
`)

	// 8 daily activity (local calendar day)
	b.WriteString(`SELECT date(time_created/1000, 'unixepoch', 'localtime') AS day,
  COUNT(*) AS sessions
FROM sa_sess
GROUP BY day
ORDER BY day ASC;
`)

	// 9 tool categories
	b.WriteString(`SELECT
  CASE
    WHEN tool = 'skill' THEN 'skill'
    WHEN tool LIKE 'codegraph%' THEN 'codegraph'
    WHEN tool LIKE 'engram%' THEN 'engram'
    WHEN tool IN ('delegate','task') OR tool LIKE 'delegation%' THEN 'delegation'
    WHEN tool LIKE 'ywai%' OR tool LIKE '%_navigate' OR tool LIKE 'chrome%' OR tool LIKE 'puppeteer%' THEN 'mcp'
    WHEN tool IN ('bash','read','write','edit','grep','glob','webfetch','todowrite','todo','question','apply_patch','list','multiedit') THEN 'native'
    ELSE 'other'
  END AS name,
  COUNT(*) AS cnt
FROM (
  SELECT COALESCE(json_extract(pt.data, '$.tool'), '') AS tool
  FROM part pt
  JOIN sa_sess ss ON ss.session_id = pt.session_id
  WHERE json_extract(pt.data, '$.type') = 'tool'
    AND COALESCE(json_extract(pt.data, '$.tool'), '') != ''
)
GROUP BY name
ORDER BY cnt DESC;
`)

	// 10 root/child + cache tokens
	b.WriteString(`SELECT
  SUM(CASE WHEN parent_id IS NULL OR parent_id = '' THEN 1 ELSE 0 END) AS root,
  SUM(CASE WHEN parent_id IS NOT NULL AND parent_id != '' THEN 1 ELSE 0 END) AS child,
  COALESCE(SUM(tokens_reasoning), 0) AS reasoning,
  COALESCE(SUM(tokens_cache_read), 0) AS cache_read,
  COALESCE(SUM(tokens_cache_write), 0) AS cache_write
FROM sa_sess;
`)

	return b.String()
}

func sqlQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
