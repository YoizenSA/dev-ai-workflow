package control

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// OpenCode keeps one SQLite DB per install root, and a sandboxed editor gets its
// own HOME: the VS Code snaps each carry a full ~/.local/share/opencode tree, and
// snap keeps several numbered revisions of it side by side. Analytics that read
// only the canonical path silently miss every session recorded from inside those
// sandboxes.
var openCodeDBGlobs = []string{
	"snap/*/*/.local/share/opencode/opencode.db",
	".var/app/*/data/opencode/opencode.db", // flatpak, same sandboxed-HOME story
}

// analyticsRequiredColumns are the columns the analytics SQL reads. An install
// from a much older OpenCode is skipped rather than failing the whole report.
var analyticsRequiredColumns = map[string][]string{
	"session": {
		"id", "project_id", "parent_id", "time_created", "agent", "model", "cost",
		"tokens_input", "tokens_output", "tokens_reasoning",
		"tokens_cache_read", "tokens_cache_write", "time_archived",
	},
	"project": {"id", "name", "worktree"},
	"part":    {"id", "session_id", "data"},
}

// discoverExtraOpenCodeDBs returns readable OpenCode DBs other than primary,
// skipping copies of a tree already accounted for. An explicit OPENCODE_DB is a
// deliberate override, so it stays a single source.
func discoverExtraOpenCodeDBs(primary string) []string {
	if strings.TrimSpace(os.Getenv("OPENCODE_DB")) != "" {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	seenPath := map[string]bool{resolvePath(primary): true}
	seenTree := map[string]bool{}
	if fp, ok := analyticsFingerprint(primary); ok {
		seenTree[fp] = true
	}
	var out []string
	for _, g := range openCodeDBGlobs {
		matches, err := filepath.Glob(filepath.Join(home, g))
		if err != nil {
			continue
		}
		sort.Strings(matches)
		for _, m := range matches {
			key := resolvePath(m)
			if seenPath[key] {
				continue
			}
			seenPath[key] = true
			fp, ok := analyticsFingerprint(m)
			if !ok || seenTree[fp] {
				continue
			}
			seenTree[fp] = true
			out = append(out, m)
		}
	}
	return out
}

func resolvePath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return p
}

// analyticsFingerprint identifies the session set a DB holds, so two snap
// revisions of one editor are recognised as the same tree and counted once.
// It reports false when the DB is unreadable or predates a required column.
func analyticsFingerprint(path string) (string, bool) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(2000)")
	if err != nil {
		return "", false
	}
	defer db.Close()
	for table, want := range analyticsRequiredColumns {
		have := map[string]bool{}
		rows, err := db.Query("SELECT name FROM pragma_table_info(?)", table)
		if err != nil {
			return "", false
		}
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				rows.Close()
				return "", false
			}
			have[n] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return "", false
		}
		for _, c := range want {
			if !have[c] {
				return "", false
			}
		}
	}
	var count, maxTime int64
	var first, last sql.NullString
	if err := db.QueryRow(
		`SELECT COUNT(*), COALESCE(MAX(time_created),0), MIN(id), MAX(id) FROM session`,
	).Scan(&count, &maxTime, &first, &last); err != nil {
		return "", false
	}
	return strconv.FormatInt(count, 10) + ":" + strconv.FormatInt(maxTime, 10) +
		":" + first.String + ":" + last.String, true
}

// mergeAnalytics folds src into dst. Counts add up; anything derived is zeroed so
// enrichAnalytics can recompute it from the combined totals rather than from one
// install's view of the world.
func mergeAnalytics(dst, src *SessionAnalytics, q AnalyticsQuery) {
	if dst == nil || src == nil {
		return
	}
	d, s := &dst.Summary, &src.Summary
	d.Sessions += s.Sessions
	d.SkillCalls += s.SkillCalls
	d.ToolCalls += s.ToolCalls
	d.TotalCost += s.TotalCost
	d.TokensInput += s.TokensInput
	d.TokensOutput += s.TokensOutput
	d.TokensReasoning += s.TokensReasoning
	d.TokensCacheRead += s.TokensCacheRead
	d.TokensCacheWrite += s.TokensCacheWrite
	d.SessionsWithSkill += s.SessionsWithSkill
	d.ChildSessions += s.ChildSessions
	d.RootSessions += s.RootSessions

	e := &dst.Engram
	e.Sessions += src.Engram.Sessions
	e.WriteOnly += src.Engram.WriteOnly
	e.WithSummary += src.Engram.WithSummary
	e.Saves += src.Engram.Saves
	e.Searches += src.Engram.Searches
	e.Updates += src.Engram.Updates
	// Coverage is a ratio over the combined session count; enrichAnalytics recomputes it.

	dst.DBPath += " + " + src.DBPath
	dst.Projects = mergeProjectStats(dst.Projects, src.Projects)
	dst.Activity = mergeActivity(dst.Activity, src.Activity)
	dst.Agents = mergeNamed(dst.Agents, src.Agents, 0)
	dst.Models = mergeNamed(dst.Models, src.Models, 0)
	dst.ToolCategories = mergeNamed(dst.ToolCategories, src.ToolCategories, 0)
	dst.Skills = mergeNamed(dst.Skills, src.Skills, q.SkillsLimit)
	dst.Tools = mergeNamed(dst.Tools, src.Tools, q.ToolsLimit)

	d.Projects = len(dst.Projects)
	d.DistinctSkills = len(dst.Skills)
	// Recomputed by enrichAnalytics, which only fills DelegationCalls when unset.
	d.DelegationCalls = 0

	applyShares(dst.Agents, d.Sessions)
	applyShares(dst.Models, d.Sessions)
	applyShares(dst.Skills, d.SkillCalls)
	applyShares(dst.Tools, d.ToolCalls)
	applyShares(dst.ToolCategories, d.ToolCalls)
	enrichAnalytics(dst)
}

// mergeNamed adds counts per name and re-ranks. limit <= 0 keeps every entry.
//
// Each install contributes only its own top-N, so a name ranked just below the
// cut in every install lands lower than its true total. Widening the per-install
// limit would trade a real cost on the large DB for a rounding error in the tail.
func mergeNamed(dst, src []SessionNamedCount, limit int) []SessionNamedCount {
	idx := make(map[string]int, len(dst)+len(src))
	out := make([]SessionNamedCount, 0, len(dst)+len(src))
	for _, list := range [][]SessionNamedCount{dst, src} {
		for _, it := range list {
			if i, ok := idx[it.Name]; ok {
				out[i].Count += it.Count
				out[i].Sessions += it.Sessions
				out[i].Cost += it.Cost
				out[i].TokensIn += it.TokensIn
				out[i].TokensOut += it.TokensOut
				continue
			}
			idx[it.Name] = len(out)
			out = append(out, it)
		}
	}
	sortNamedByCount(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func mergeProjectStats(dst, src []SessionProjectStat) []SessionProjectStat {
	idx := make(map[string]int, len(dst)+len(src))
	out := make([]SessionProjectStat, 0, len(dst)+len(src))
	for _, list := range [][]SessionProjectStat{dst, src} {
		for _, it := range list {
			if i, ok := idx[it.ID]; ok {
				out[i].Sessions += it.Sessions
				out[i].SkillCalls += it.SkillCalls
				out[i].ToolCalls += it.ToolCalls
				out[i].Cost += it.Cost
				out[i].TokensIn += it.TokensIn
				out[i].TokensOut += it.TokensOut
				continue
			}
			idx[it.ID] = len(out)
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sessions != out[j].Sessions {
			return out[i].Sessions > out[j].Sessions
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func mergeActivity(dst, src []SessionDayCount) []SessionDayCount {
	idx := make(map[string]int, len(dst)+len(src))
	out := make([]SessionDayCount, 0, len(dst)+len(src))
	for _, list := range [][]SessionDayCount{dst, src} {
		for _, it := range list {
			if i, ok := idx[it.Day]; ok {
				out[i].Sessions += it.Sessions
				continue
			}
			idx[it.Day] = len(out)
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out
}
