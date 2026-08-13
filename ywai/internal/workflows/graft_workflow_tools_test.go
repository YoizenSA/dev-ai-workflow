package workflows

// graft_workflow_tools_test.go — RED tests for slice 2:
//
// Acceptance item 5: "Workflow/tool configs should not request
// codegraph tools for NEW execution. Historical analytics/eval
// field labels may remain for compatibility and MUST NOT be included
// in this RED slice."
//
// We restrict the scan to the tool-request fields that drive new
// execution: the per-node "tools" CSV (drives toolsToPermissions in
// the exporter) and the workflow-level "allowedTools" CSV. Historical
// analytics/eval labels are explicitly out of scope per the slice
// contract.
//
// The test walks every workflows/*.json file under the repo, parses
// each top-level "nodes" array, and asserts:
//   1. No node's tools CSV mentions codegraph_* or the literal
//      "codegraph" tool.
//   2. No workflow-level allowedTools CSV mentions codegraph_*.
//
// Tools CSV example: "read, edit, codegraph_*, context7_*, skill"
// — these expand into the agent's opencode-native tool set when the
// workflow is exported to a real /<workflow> command.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// workflowFile is the minimal shape we need from each workflow JSON
// file. We decode into this instead of the full Workflow model so the
// test stays robust to schema drift — the slice cares about tools
// strings, not the entire workflow shape.
type workflowFile struct {
	Nodes []struct {
		Data struct {
			Tools string `json:"tools"`
		} `json:"data"`
	} `json:"nodes"`

	// Top-level fields that may carry tool grants for the workflow's
	// command surface. allowedTools is the canonical example. We keep
	// the model loose: any field name ending in "Tools" or "tools" on
	// the top level is scanned, so future schema additions that
	// inherit the same contract are caught without further work.
	TopLevel map[string]json.RawMessage `json:"-"`
}

// topLevelKeys lists every top-level key we scan for codegraph
// references in tools/allowedTools CSVs. Keep in sync with the
// Workflow model if new tool-grant fields are added.
var topLevelToolKeys = []string{
	"allowedTools",
	"tools",
	"orchestratorTools",
}

// TestWorkflows_NoCodegraphInNodeTools walks every workflows/*.json
// file under the repo, decodes the nodes array, and asserts each
// node's data.tools CSV does not reference the legacy codegraph tool
// wildcard. This is the slice-2 equivalent of "the workflow export
// must not generate codegraph_* permission lines".
func TestWorkflows_NoCodegraphInNodeTools(t *testing.T) {
	root := filepath.Join("..", "..", "workflows")
	files := listWorkflowJSON(t, root)

	for _, path := range files {
		var wf workflowFile
		raw := readJSONFile(t, path)
		if err := json.Unmarshal(raw, &wf); err != nil {
			t.Fatalf("unmarshal %s: %v", path, err)
		}
		for i, n := range wf.Nodes {
			if csvHasCodegraph(n.Data.Tools) {
				t.Errorf("%s node[%d].data.tools = %q contains codegraph — slice 2 must replace with graft_*",
					path, i, n.Data.Tools)
			}
		}
	}
}

// TestWorkflows_NoCodegraphInTopLevelToolFields scans top-level
// tool-grant fields (allowedTools, tools, orchestratorTools) for
// codegraph references. Historical analytics/eval labels are
// intentionally out of scope per the slice contract.
func TestWorkflows_NoCodegraphInTopLevelToolFields(t *testing.T) {
	root := filepath.Join("..", "..", "workflows")
	files := listWorkflowJSON(t, root)

	for _, path := range files {
		var top map[string]json.RawMessage
		raw := readJSONFile(t, path)
		if err := json.Unmarshal(raw, &top); err != nil {
			t.Fatalf("unmarshal %s: %v", path, err)
		}
		for _, key := range topLevelToolKeys {
			val, ok := top[key]
			if !ok {
				continue
			}
			var s string
			if err := json.Unmarshal(val, &s); err != nil {
				continue
			}
			if csvHasCodegraph(s) {
				t.Errorf("%s top-level %q = %q contains codegraph — slice 2 must replace",
					path, key, s)
			}
		}
	}
}

// csvHasCodegraph reports whether a comma-separated tool list
// references codegraph via the wildcard, a bare token, or a
// codegraph_* form. Bare "codegraph" is allowed only if it is a
// complete token, not a substring of another identifier.
func csvHasCodegraph(csv string) bool {
	for _, tok := range strings.Split(csv, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if tok == "codegraph" {
			return true
		}
		if strings.HasPrefix(tok, "codegraph_") {
			return true
		}
	}
	return false
}

// listWorkflowJSON walks root and returns every *.json file path,
// sorted. Sorted output makes failures reproducible.
func listWorkflowJSON(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".json" {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return out
}

// readJSONFile reads a workflow JSON file and returns its raw bytes.
// The caller is responsible for unmarshaling into the right shape.
func readJSONFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}