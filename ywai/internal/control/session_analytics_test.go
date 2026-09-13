package control

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestLoadSessionAnalytics_AggregatesSkillsToolsByProject(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opencode.db")
	seedMiniOpenCodeDB(t, dbPath)

	got, err := LoadSessionAnalytics(context.Background(), dbPath, AnalyticsQuery{
		Days:        0,
		ToolsLimit:  10,
		SkillsLimit: 10,
	})
	if err != nil {
		t.Fatalf("LoadSessionAnalytics: %v", err)
	}

	if got.Summary.Sessions != 3 {
		t.Fatalf("sessions=%d, want 3", got.Summary.Sessions)
	}
	if got.Summary.Projects != 2 {
		t.Fatalf("projects=%d, want 2", got.Summary.Projects)
	}
	if got.Summary.SkillCalls != 3 {
		t.Fatalf("skillCalls=%d, want 3", got.Summary.SkillCalls)
	}
	if got.Summary.DistinctSkills != 2 {
		t.Fatalf("distinctSkills=%d, want 2 (git-commit, tdd)", got.Summary.DistinctSkills)
	}
	if got.Summary.SessionsWithSkill != 3 {
		t.Fatalf("sessionsWithSkill=%d, want 3", got.Summary.SessionsWithSkill)
	}
	if got.Summary.ToolCalls < 3 {
		t.Fatalf("toolCalls=%d, want >= 3", got.Summary.ToolCalls)
	}

	// Skills ranking
	if len(got.Skills) < 1 || got.Skills[0].Name != "git-commit" {
		t.Fatalf("top skill=%v, want git-commit first", got.Skills)
	}
	if got.Skills[0].Count != 2 {
		t.Fatalf("git-commit count=%d, want 2", got.Skills[0].Count)
	}

	// Tools include bash
	foundBash := false
	for _, tool := range got.Tools {
		if tool.Name == "bash" {
			foundBash = true
			if tool.Count < 1 {
				t.Fatalf("bash count=%d", tool.Count)
			}
		}
	}
	if !foundBash {
		t.Fatalf("tools missing bash: %+v", got.Tools)
	}

	// Agents ranked
	if len(got.Agents) < 1 {
		t.Fatal("expected agents")
	}
	if got.Agents[0].Share <= 0 {
		t.Fatalf("top agent share=%v", got.Agents[0].Share)
	}

	// Models normalized from JSON
	foundModel := false
	for _, m := range got.Models {
		if m.Name == "opencode-admin/deepseek-v4-flash" {
			foundModel = true
			if m.Count < 1 {
				t.Fatalf("model count=%d", m.Count)
			}
		}
	}
	if !foundModel {
		t.Fatalf("models missing normalized label: %+v", got.Models)
	}

	// Project filter
	filtered, err := LoadSessionAnalytics(context.Background(), dbPath, AnalyticsQuery{
		ProjectID: "proj-a",
	})
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if filtered.Summary.Sessions != 2 {
		t.Fatalf("filtered sessions=%d, want 2", filtered.Summary.Sessions)
	}
	if filtered.Summary.SkillCalls != 2 {
		t.Fatalf("filtered skillCalls=%d, want 2", filtered.Summary.SkillCalls)
	}
	if len(filtered.Projects) != 1 || filtered.Projects[0].ID != "proj-a" {
		t.Fatalf("filtered projects=%+v", filtered.Projects)
	}
}

func TestLoadSessionAnalytics_DaysFilter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opencode.db")
	seedMiniOpenCodeDB(t, dbPath)

	// Only last 1 day — old session (proj-b) should drop out
	got, err := LoadSessionAnalytics(context.Background(), dbPath, AnalyticsQuery{Days: 1})
	if err != nil {
		t.Fatalf("LoadSessionAnalytics: %v", err)
	}
	if got.Summary.Sessions != 2 {
		t.Fatalf("sessions in last day=%d, want 2", got.Summary.Sessions)
	}
}

func TestLoadSessionAnalytics_LegacySchemaFallback(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "opencode.db")
	seedLegacyOpenCodeDB(t, dbPath)

	got, err := LoadSessionAnalytics(context.Background(), dbPath, AnalyticsQuery{
		Days: 0, ToolsLimit: 10, SkillsLimit: 10,
	})
	if err != nil {
		t.Fatalf("LoadSessionAnalytics: %v", err)
	}
	if got.Summary.Sessions != 2 {
		t.Fatalf("sessions=%d, want 2", got.Summary.Sessions)
	}
	if got.Summary.SkillCalls != 3 {
		t.Fatalf("skillCalls=%d, want 3", got.Summary.SkillCalls)
	}
	if len(got.Skills) < 1 || got.Skills[0].Name != "git-commit" || got.Skills[0].Count != 2 {
		t.Fatalf("top skill=%v, want git-commit x2", got.Skills)
	}
}

func seedLegacyOpenCodeDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := `
CREATE TABLE project (
  id text PRIMARY KEY,
  worktree text NOT NULL,
  name text
);
CREATE TABLE session (
  id text PRIMARY KEY,
  project_id text NOT NULL,
  directory text NOT NULL,
  title text NOT NULL,
  time_created integer NOT NULL,
  time_updated integer NOT NULL,
  time_archived integer,
  agent text,
  model text,
  cost real DEFAULT 0 NOT NULL,
  tokens_input integer DEFAULT 0 NOT NULL,
  tokens_output integer DEFAULT 0 NOT NULL
);
CREATE TABLE part (
  id text PRIMARY KEY,
  message_id text NOT NULL,
  session_id text NOT NULL,
  data text NOT NULL
);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UnixMilli()
	mustExec(t, db, `INSERT INTO project (id, worktree, name) VALUES ('proj-a', '/tmp/alpha', 'alpha')`)
	mustExec(t, db, `INSERT INTO session (id, project_id, directory, title, time_created, time_updated, agent, model, cost, tokens_input, tokens_output) VALUES
		('s1', 'proj-a', '/tmp/alpha', 'one', ?, ?, 'dev', '{"id":"m"}', 1.0, 10, 5),
		('s2', 'proj-a', '/tmp/alpha', 'two', ?, ?, 'build', '{"id":"m"}', 0.5, 4, 2)`,
		now, now, now, now)
	mustExec(t, db, `INSERT INTO part (id, message_id, session_id, data) VALUES
		('p1', 'm1', 's1', '{"type":"tool","tool":"skill","state":{"input":{"name":"git-commit"}}}'),
		('p2', 'm1', 's1', '{"type":"tool","tool":"bash","state":{"input":{}}}'),
		('p3', 'm2', 's2', '{"type":"tool","tool":"skill","state":{"input":{"name":"tdd"}}}'),
		('p4', 'm2', 's2', '{"type":"tool","tool":"skill","state":{"input":{"name":"git-commit"}}}')`)
}

func TestLoadSessionAnalytics_MissingDB(t *testing.T) {
	_, err := LoadSessionAnalytics(context.Background(), filepath.Join(t.TempDir(), "nope.db"), AnalyticsQuery{})
	if err == nil {
		t.Fatal("expected error for missing db")
	}
}

func TestNormalizeModelLabel(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "(default)"},
		{`{"id":"deepseek-v4-flash","providerID":"opencode-admin","variant":"default"}`, "opencode-admin/deepseek-v4-flash"},
		{`{"id":"mimo-v2.5-pro","providerID":"xiaomi","variant":"high"}`, "xiaomi/mimo-v2.5-pro (high)"},
		{"plain-model", "plain-model"},
	}
	for _, tc := range cases {
		if got := normalizeModelLabel(tc.in); got != tc.want {
			t.Fatalf("normalizeModelLabel(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func seedMiniOpenCodeDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schema := `
CREATE TABLE project (
  id text PRIMARY KEY,
  worktree text NOT NULL,
  vcs text,
  name text,
  icon_url text,
  icon_color text,
  time_created integer NOT NULL,
  time_updated integer NOT NULL,
  time_initialized integer,
  sandboxes text NOT NULL DEFAULT '[]'
);
CREATE TABLE session_v2 (
  id text PRIMARY KEY,
  project_id text NOT NULL,
  directory text NOT NULL,
  title text NOT NULL,
  agent text,
  model text,
  cost real DEFAULT 0 NOT NULL,
  tokens_input integer DEFAULT 0 NOT NULL,
  tokens_output integer DEFAULT 0 NOT NULL,
  time_created integer NOT NULL,
  time_updated integer NOT NULL,
  time_archived integer
);
CREATE TABLE session_message (
  id text PRIMARY KEY,
  session_id text NOT NULL,
  data text NOT NULL
);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UnixMilli()
	old := time.Now().AddDate(0, 0, -10).UnixMilli()

	mustExec(t, db, `INSERT INTO project (id, worktree, name, time_created, time_updated, sandboxes) VALUES
		('proj-a', '/tmp/alpha', 'alpha', ?, ?, '[]'),
		('proj-b', '/tmp/beta', 'beta', ?, ?, '[]')`, now, now, old, old)

	modelFlash := `{"id":"deepseek-v4-flash","providerID":"opencode-admin","variant":"default"}`
	mustExec(t, db, `INSERT INTO session_v2 (id, project_id, directory, title, time_created, time_updated, agent, model, cost, tokens_input, tokens_output) VALUES
		('s1', 'proj-a', '/tmp/alpha', 'one', ?, ?, 'dev', ?, 1.5, 100, 50),
		('s2', 'proj-a', '/tmp/alpha', 'two', ?, ?, 'build', ?, 0.5, 40, 20),
		('s3', 'proj-b', '/tmp/beta', 'three', ?, ?, '', ?, 2.0, 200, 80)`,
		now, now, modelFlash, now, now, modelFlash, old, old, `{"id":"deepseek-v4-flash","providerID":"opencode-admin","variant":"max"}`)

	// v2 message rows: content[] mixes tool calls; skill names ride input.id.
	mustExec(t, db, `INSERT INTO session_message (id, session_id, data) VALUES
		('m1', 's1', ?),
		('m2', 's2', ?),
		('m3', 's3', ?)`,
		`{"content":[{"type":"tool","name":"skill","state":{"input":{"id":"git-commit"}}},{"type":"tool","name":"bash","state":{"input":{"command":"ls"}}}]}`,
		`{"content":[{"type":"tool","name":"skill","state":{"input":{"id":"tdd"}}}]}`,
		`{"content":[{"type":"tool","name":"skill","state":{"input":{"id":"git-commit"}}},{"type":"tool","name":"read","state":{"input":{"path":"x"}}}]}`,
	)
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec: %v\nSQL: %s", err, q)
	}
}

func TestLoadSessionAnalytics_RealOpenCodeDB_Smoke(t *testing.T) {
	path := defaultOpenCodeDBPath()
	if path == "" {
		t.Skip("no home")
	}
	if _, err := os.Stat(path); err != nil {
		t.Skip("no local opencode.db")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	got, err := LoadSessionAnalytics(ctx, path, AnalyticsQuery{Days: 30, ToolsLimit: 10, SkillsLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary.Sessions < 1 {
		t.Fatalf("expected some sessions in last 30d, got %+v", got.Summary)
	}
	t.Logf("sessions=%d skills=%d tools=%d cost=%.4f", got.Summary.Sessions, got.Summary.SkillCalls, got.Summary.ToolCalls, got.Summary.TotalCost)
}
