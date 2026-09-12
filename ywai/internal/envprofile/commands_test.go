package envprofile

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func writeCommand(t *testing.T, p Profile, name, body string) {
	t.Helper()
	if err := os.MkdirAll(CommandsDir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(CommandsDir(p), name+".md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCommandsAndCommandRun(t *testing.T) {
	testRoot(t)
	p, err := Create("dev", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got := Commands(p); len(got) != 0 {
		t.Fatalf("fresh env commands = %v, want none", got)
	}
	writeCommand(t, p, "feature-delivery", "---\r\ndescription: x\r\nagent: feature-delivery-orchestrator\r\n---\r\n\r\nbody\n")
	writeCommand(t, p, "learn-ywai", "no frontmatter here\n")

	if got := Commands(p); !slices.Equal(got, []string{"feature-delivery", "learn-ywai"}) {
		t.Fatalf("commands = %v", got)
	}
	agent, body, ok := CommandRun(p, "feature-delivery")
	if !ok || agent != "feature-delivery-orchestrator" || body != "body" {
		t.Errorf("agent = %q, body = %q, ok = %v", agent, body, ok)
	}
	// A leading slash is accepted (it is how the command reads in the TUI).
	if agent, _, ok := CommandRun(p, "/feature-delivery"); !ok || agent != "feature-delivery-orchestrator" {
		t.Errorf("slash form: agent = %q, ok = %v", agent, ok)
	}
	// Known command, no agent: the whole file is the prompt body.
	if agent, body, ok := CommandRun(p, "learn-ywai"); !ok || agent != "" || body != "no frontmatter here" {
		t.Errorf("agentless command = %q, %q, %v", agent, body, ok)
	}
	if _, _, ok := CommandRun(p, "nope"); ok {
		t.Error("unknown command must report not found")
	}
	// Never escape the commands dir.
	if _, _, ok := CommandRun(p, "../../../etc/passwd"); ok {
		t.Error("path traversal must be refused")
	}
}
