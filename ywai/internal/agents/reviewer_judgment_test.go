package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readRepoAgentFile(t *testing.T, parts ...string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(append([]string{"..", "..", "agents"}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// The orchestrator runs judgment-day on every review by launching two blind
// @reviewer judges; the reviewer itself stays a single-pass leaf.
func TestOrchestratorOwnsJudgment(t *testing.T) {
	doc, err := LoadDelegations(filepath.Join("..", "..", "agents"))
	if err != nil {
		t.Fatal(err)
	}
	if got := doc.Agents["orchestrator"].Task["reviewer"]; got != "allow" {
		t.Errorf("orchestrator->reviewer = %q, want allow", got)
	}
	if got := doc.Agents["reviewer"].Task["reviewer"]; got == "allow" {
		t.Errorf("reviewer->reviewer = allow, want reviewer to stay a leaf")
	}
	if !strings.Contains(readRepoAgentFile(t, "core", "orchestrator", "skills.txt"), "judgment-day") {
		t.Error("orchestrator skills.txt missing judgment-day")
	}
	orch := readRepoAgentFile(t, "core", "orchestrator", "AGENT.md")
	for _, want := range []string{"judgment-day", "two blind `@reviewer`"} {
		if !strings.Contains(orch, want) {
			t.Errorf("orchestrator AGENT.md missing %q", want)
		}
	}
	if strings.Contains(readRepoAgentFile(t, "core", "reviewer", "skills.txt"), "judgment-day") {
		t.Error("reviewer skills.txt still lists judgment-day; the orchestrator runs it")
	}
}
