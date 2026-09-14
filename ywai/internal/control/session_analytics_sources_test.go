package control

import "testing"

func TestMergeAnalyticsCombinesCountsAndRerank(t *testing.T) {
	dst := &SessionAnalytics{
		DBPath:  "a.db",
		Summary: SessionAnalyticsSum{Sessions: 10, ToolCalls: 100, SkillCalls: 4, TotalCost: 1.5, TokensInput: 900},
		Tools: []SessionNamedCount{
			{Name: "read", Count: 60}, {Name: "grep", Count: 40},
		},
		Skills:   []SessionNamedCount{{Name: "tdd", Count: 4}},
		Projects: []SessionProjectStat{{ID: "p1", Sessions: 10, ToolCalls: 100}},
	}
	src := &SessionAnalytics{
		DBPath:  "b.db",
		Summary: SessionAnalyticsSum{Sessions: 5, ToolCalls: 90, SkillCalls: 2, TotalCost: 0.5, TokensInput: 100},
		Tools: []SessionNamedCount{
			{Name: "grep", Count: 70}, {Name: "bash", Count: 20},
		},
		Skills:   []SessionNamedCount{{Name: "tdd", Count: 2}},
		Projects: []SessionProjectStat{{ID: "p1", Sessions: 5, ToolCalls: 90}, {ID: "p2", Sessions: 1}},
	}

	mergeAnalytics(dst, src, AnalyticsQuery{ToolsLimit: 30, SkillsLimit: 50})

	if dst.Summary.Sessions != 15 || dst.Summary.ToolCalls != 190 {
		t.Fatalf("summary not summed: %+v", dst.Summary)
	}
	if dst.Summary.TotalCost != 2.0 || dst.Summary.TokensInput != 1000 {
		t.Fatalf("cost/tokens not summed: %+v", dst.Summary)
	}
	// grep (40+70=110) must outrank read (60) after the merge re-ranks.
	if dst.Tools[0].Name != "grep" || dst.Tools[0].Count != 110 {
		t.Fatalf("tools not merged/reranked: %+v", dst.Tools)
	}
	if len(dst.Tools) != 3 {
		t.Fatalf("want read+grep+bash, got %+v", dst.Tools)
	}
	if got := dst.Tools[0].Share; got <= 0.57 || got >= 0.58 {
		t.Fatalf("share must be recomputed against merged total, got %v", got)
	}
	// One project seen in both installs stays one row with combined sessions.
	if len(dst.Projects) != 2 || dst.Projects[0].ID != "p1" || dst.Projects[0].Sessions != 15 {
		t.Fatalf("projects not merged: %+v", dst.Projects)
	}
	if dst.Summary.Projects != 2 {
		t.Fatalf("summary.Projects must follow merged list, got %d", dst.Summary.Projects)
	}
	if dst.Skills[0].Count != 6 || dst.Summary.DistinctSkills != 1 {
		t.Fatalf("skills not merged: %+v / %d", dst.Skills, dst.Summary.DistinctSkills)
	}
}

func TestMergeNamedRespectsLimit(t *testing.T) {
	a := []SessionNamedCount{{Name: "x", Count: 1}, {Name: "y", Count: 2}}
	b := []SessionNamedCount{{Name: "z", Count: 3}}
	if got := mergeNamed(a, b, 2); len(got) != 2 || got[0].Name != "z" {
		t.Fatalf("limit/rank wrong: %+v", got)
	}
	if got := mergeNamed(a, b, 0); len(got) != 3 {
		t.Fatalf("limit<=0 must keep all: %+v", got)
	}
}

func TestExplicitOpenCodeDBSkipsDiscovery(t *testing.T) {
	t.Setenv("OPENCODE_DB", "/tmp/whatever.db")
	if extras := discoverExtraOpenCodeDBs("/tmp/whatever.db"); extras != nil {
		t.Fatalf("explicit override must stay single-source, got %v", extras)
	}
}
