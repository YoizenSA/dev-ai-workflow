package workflows

import "testing"

// A workflow sub-agent is exported under a generated id (wf-frontend), which no
// orchestrator profile can ever name. The link to its real agent is what puts it
// back under the profile — otherwise every node runs at the session model and
// switching profiles changes nothing for workflows.
func TestNodeModelFallsBackToLinkedAgentProfile(t *testing.T) {
	e := NewExporterWithDirs(t.TempDir(), t.TempDir())
	e.profileModels = map[string]string{"dev": "cheap/flash", "reviewer": "pricey/pro"}

	cases := []struct {
		name string
		node Node
		want string
	}{
		{
			name: "linked node inherits its agent's tier",
			node: Node{Data: NodeData{AgentRef: "core/dev"}},
			want: "cheap/flash",
		},
		{
			name: "bare ref resolves like a grouped one",
			node: Node{Data: NodeData{AgentRef: "reviewer"}},
			want: "pricey/pro",
		},
		{
			name: "explicit model beats the profile",
			node: Node{Data: NodeData{AgentRef: "core/dev", Model: "chosen/by-hand"}},
			want: "chosen/by-hand",
		},
		{
			name: "inherit is not an explicit choice",
			node: Node{Data: NodeData{AgentRef: "core/dev", Model: "inherit"}},
			want: "cheap/flash",
		},
		{
			name: "unlinked node stays on inherit",
			node: Node{Data: NodeData{Prompt: "one-off"}},
			want: "",
		},
		{
			name: "agent absent from the profile stays on inherit",
			node: Node{Data: NodeData{AgentRef: "core/designer"}},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := e.nodeModel(&tc.node); got != tc.want {
				t.Errorf("nodeModel = %q, want %q", got, tc.want)
			}
		})
	}
}

// The START node is the orchestrator, so it resolves through the same path.
func TestOrchestratorModelResolvesThroughStartLink(t *testing.T) {
	e := NewExporterWithDirs(t.TempDir(), t.TempDir())
	e.profileModels = map[string]string{"orchestrator": "pricey/pro"}

	wf := &Workflow{
		Name:  "wf",
		Nodes: []Node{{ID: "start", Type: NodeTypeStart, Data: NodeData{AgentRef: "core/orchestrator"}}},
	}
	if got := e.orchestratorModel(wf); got != "pricey/pro" {
		t.Errorf("orchestratorModel = %q, want pricey/pro", got)
	}
}
