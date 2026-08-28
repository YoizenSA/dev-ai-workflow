package main

import (
	"fmt"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/workflows"
)

// exportInstalledWorkflows re-exports every workflow in the data dir to the
// active agent host.
//
// Two reasons this has to run on every install, not only from the control
// server. Installing agent profiles prunes every agent .md that is not part of
// the installed profile set, which deletes the workflow sub-agents and leaves
// their slash commands pointing at agents that no longer exist. And a command
// written by an older ywai still carries frontmatter that agent host no longer
// interprets the same way — a stale `subtask: true` makes opencode v1 run the
// orchestrator as a subtask instead of switching the session to it, so its own
// delegations become sub-sub-agent calls.
//
// Returns the number of workflows exported.
func exportInstalledWorkflows(dryRun bool) (int, error) {
	store := workflows.NewStore(config.DataWorkflowsDir())
	list, err := store.List()
	if err != nil {
		return 0, fmt.Errorf("listing workflows: %w", err)
	}
	if len(list) == 0 {
		return 0, nil
	}
	if dryRun {
		return len(list), nil
	}

	exporter := workflows.NewExporter()
	exported := 0
	for _, summary := range list {
		wf, err := store.Load(summary.Name)
		if err != nil {
			fmt.Printf("  Warning: skipping workflow %s: %v\n", summary.Name, err)
			continue
		}
		if _, err := exporter.Apply(wf); err != nil {
			fmt.Printf("  Warning: failed to export workflow %s: %v\n", summary.Name, err)
			continue
		}
		exported++
	}
	return exported, nil
}
