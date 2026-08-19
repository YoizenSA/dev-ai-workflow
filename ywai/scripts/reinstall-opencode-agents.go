//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"

	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

func main() {
	src := config.AgentsSourceDir()
	profiles, err := agentprofiles.LoadProfilesByGroup(src, agentprofiles.GroupFilter{
		Groups: []string{"qa-automation"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	dir := filepath.Join(home, ".config", "opencode", "agents")
	if err := agentprofiles.InstallOpenCodeMarkdown(dir, profiles, true); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("backups removed: %d\n", agentprofiles.RemoveAgentBackups(dir))
	fmt.Printf("retired removed: %d\n", agentprofiles.RemoveRetiredAgents(dir))
	fmt.Printf("ywai profiles written: %d → %s\n", len(profiles), dir)
}
