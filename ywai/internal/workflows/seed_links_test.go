package workflows

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Yoizen/dev-ai-workflow/ywai/internal/agents"
	"github.com/Yoizen/dev-ai-workflow/ywai/internal/config"
)

// Every seed node that owns an identity must track a real agent under agents/
// rather than carry a copy of its prompt.
//
// A copy is not a smaller version of the same thing: agentDefinition wins over
// agentRef at export time, so a node holding both looks linked in the editor
// while exporting stale text. Nodes with neither are worse — they export an
// agent with no persona at all, which is how qa-exploratory's orchestrator
// shipped.
func TestSeedWorkflowsLinkTheirAgents(t *testing.T) {
	profiles, err := agents.LoadProfiles(config.AgentsSourceDir())
	if err != nil || len(profiles) == 0 {
		t.Skipf("agents source dir unavailable: %v", err)
	}
	resolves := func(ref string) bool {
		if _, ok := profiles[ref]; ok {
			return true
		}
		for key := range profiles {
			if key[strings.LastIndex(key, "/")+1:] == ref {
				return true
			}
		}
		return false
	}

	seeds, err := filepath.Glob("../../workflows/*.json")
	if err != nil || len(seeds) == 0 {
		t.Fatalf("no seed workflows found: %v", err)
	}
	for _, seed := range seeds {
		t.Run(filepath.Base(seed), func(t *testing.T) {
			data, err := os.ReadFile(seed)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			var wf Workflow
			if err := json.Unmarshal(data, &wf); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			for i := range wf.Nodes {
				n := &wf.Nodes[i]
				if n.Type != NodeTypeStart && n.Type != NodeTypeSubAgent {
					continue
				}
				ref := strings.TrimSpace(n.Data.AgentRef)
				if ref == "" {
					t.Errorf("node %q has no agentRef: its prompt cannot be shared, "+
						"tiered by a profile, or updated from one place", n.ID)
					continue
				}
				if def := strings.TrimSpace(n.Data.AgentDefinition); def != "" {
					t.Errorf("node %q sets both agentRef and agentDefinition; the inline "+
						"copy silently wins and the link is inert", n.ID)
				}
				if !resolves(ref) {
					t.Errorf("node %q links %q, which resolves to no agent under agents/", n.ID, ref)
				}
			}
		})
	}
}
