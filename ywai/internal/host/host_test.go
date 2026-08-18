package host

import (
	"strings"
	"testing"
)

func TestBinaryName_OpenCodeIsOpenCode2(t *testing.T) {
	if got := BinaryName(OpenCode); got != "opencode2" {
		t.Fatalf("BinaryName(OpenCode)=%q want opencode2", got)
	}
}

func TestParseID(t *testing.T) {
	cases := map[string]ID{
		"":            OpenCode,
		"opencode":    OpenCode,
		"pi":          Pi,
		"omp":         OMP,
		"oh-my-pi":    OMP,
		"claude-code": Claude,
	}
	for in, want := range cases {
		got, err := ParseID(in)
		if err != nil {
			t.Fatalf("ParseID(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("ParseID(%q)=%q want %q", in, got, want)
		}
	}
	if _, err := ParseID("nope"); err == nil {
		t.Fatal("expected error for unknown host")
	}
}

func TestAgentsDirDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, id := range All {
		d := AgentsDir(id)
		if d == "" {
			t.Fatalf("%s empty AgentsDir", id)
		}
		if seen[d] {
			t.Fatalf("duplicate AgentsDir %s", d)
		}
		seen[d] = true
	}
}

func TestSnapshotCapabilities(t *testing.T) {
	oc := Snapshot(OpenCode)
	if !oc.Chat || !oc.WorkflowRun || !oc.Evals {
		t.Fatalf("opencode caps: %+v", oc)
	}
	pi := Snapshot(Pi)
	if pi.Chat || !pi.WorkflowRun || !pi.Team {
		t.Fatalf("pi caps: %+v", pi)
	}
	omp := Snapshot(OMP)
	if omp.Chat || !omp.WorkflowRun || omp.Evals {
		t.Fatalf("omp caps: %+v", omp)
	}
}

func TestMCPPaths(t *testing.T) {
	if !strings.Contains(MCPConfigPath(OpenCode), "opencode") {
		t.Fatal(MCPConfigPath(OpenCode))
	}
	if !strings.Contains(MCPConfigPath(Pi), ".pi") {
		t.Fatal(MCPConfigPath(Pi))
	}
	if !strings.Contains(MCPConfigPath(OMP), ".omp") {
		t.Fatal(MCPConfigPath(OMP))
	}
}
