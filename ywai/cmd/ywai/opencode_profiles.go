package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	agentprofiles "github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/opencodeprofile"
)

func seedOpenCodeIsolatedProfiles(dryRun, overwrite bool) {
	sourceDir := config.AgentsSourceDir()
	if !config.IsDirPopulated(sourceDir) {
		sourceDir = config.DataAgentsDir()
	}
	all, err := agentprofiles.LoadProfiles(sourceDir)
	if err != nil {
		fmt.Printf("  Warning: isolated profiles: load agents: %v\n", err)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("  Warning: isolated profiles: %v\n", err)
		return
	}
	src := config.OpenCodeConfigDir()

	var doc *agentprofiles.DelegationsDoc
	if d, err := agentprofiles.LoadDelegations(sourceDir); err == nil {
		doc = d
	}

	for _, name := range opencodeprofile.Names() {
		dirs, err := opencodeprofile.DirsFor(home, name)
		if err != nil {
			continue
		}
		filtered := opencodeprofile.FilterProfiles(all, name)
		if dryRun {
			fmt.Printf("  Would seed OpenCode profile %s (%d agents) → %s\n", name, len(filtered), dirs.Config)
			continue
		}
		if err := os.MkdirAll(dirs.Data, 0o755); err != nil {
			fmt.Printf("  Warning: [%s] mkdir data: %v\n", name, err)
			continue
		}
		if err := opencodeprofile.CopySharedConfig(src, dirs.Config); err != nil {
			fmt.Printf("  Warning: [%s] copy config: %v\n", name, err)
			continue
		}
		agentsDir := filepath.Join(dirs.Config, "agents")
		if err := agentprofiles.InstallOpenCodeMarkdown(agentsDir, filtered, overwrite); err != nil {
			fmt.Printf("  Warning: [%s] agents: %v\n", name, err)
			continue
		}
		pruneProfileAgents(agentsDir, filtered)
		agentprofiles.RemoveAgentsWithoutDescription(agentsDir)
		agentprofiles.RemoveRetiredAgents(agentsDir)

		cfgPath := filepath.Join(dirs.Config, "opencode.json")
		if _, err := os.Stat(cfgPath); err != nil {
			cfgPath = filepath.Join(dirs.Config, "opencode.jsonc")
		}
		if doc != nil && len(doc.Agents) > 0 {
			if err := agentprofiles.ApplyDelegations(cfgPath, agentsDir, doc); err != nil {
				fmt.Printf("  Warning: [%s] delegations: %v\n", name, err)
			}
		}
		fmt.Printf("  [%s] isolated OpenCode profile → %s\n", name, dirs.Config)
	}
}

func pruneProfileAgents(agentsDir string, keep map[string]agentprofiles.AgentProfile) {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return
	}
	want := make(map[string]bool, len(keep))
	for k := range keep {
		want[filepath.Base(k)] = true
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".md")
		if !want[base] {
			_ = os.Remove(filepath.Join(agentsDir, e.Name()))
		}
	}
}
