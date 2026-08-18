package main

import (
	"os"
	"strings"
	"testing"
)

func TestUpdateReexecsAfterBinaryReplace(t *testing.T) {
	data, err := os.ReadFile("commands.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	idx := strings.Index(body, "var updateCmd")
	if idx < 0 {
		t.Fatal("updateCmd not found")
	}
	chunk := body[idx:]
	if end := strings.Index(chunk, "\nvar agentsCmd"); end > 0 {
		chunk = chunk[:end]
	}
	if !strings.Contains(chunk, "reexecSelf") {
		t.Fatal("ywai update must re-exec the new binary after self-update")
	}
}

func TestInstallEcosystem_InstallsEngramOutsideAgentLoop(t *testing.T) {
	data, err := os.ReadFile("root.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	start := strings.Index(body, "func installEcosystem")
	if start < 0 {
		t.Fatal("installEcosystem not found")
	}
	rest := body[start:]
	end := strings.Index(rest, "\nfunc engramMCPHostNames")
	if end < 0 {
		end = strings.Index(rest, "\nfunc summarizeAgents")
	}
	if end < 0 {
		t.Fatal("could not bound installEcosystem")
	}
	fn := rest[:end]
	if strings.Count(fn, "gentlai.InstallEcosystem") != 1 {
		t.Fatalf("InstallEcosystem must run once, got %d", strings.Count(fn, "gentlai.InstallEcosystem"))
	}
	loop := fn[strings.Index(fn, "for _, a := range agents"):]
	loopEnd := strings.Index(loop, "\n\t}")
	if loopEnd < 0 {
		t.Fatal("agent loop not found")
	}
	if strings.Contains(loop[:loopEnd], "gentlai.InstallEcosystem") {
		t.Fatal("InstallEcosystem must not run inside the per-agent loop")
	}
}
