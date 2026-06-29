package workflows

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ImportOptions controls how raw cc-wf-studio JSON is normalized into a ywai
// Workflow during import.
type ImportOptions struct {
	Name string `json:"name,omitempty"` // override name; defaults to source name
}

// ImportResult describes a finished import.
type ImportResult struct {
	Workflow *Workflow `json:"workflow"`
	Warnings []string  `json:"warnings,omitempty"`
}

// Import parses cc-wf-studio workflow JSON into a ywai Workflow. cc-wf-studio's
// schema and ywai's model.go share the same field names (we mirrored them), so
// in most cases this is a straight unmarshal plus normalization. We accept the
// legacy "branch" node type and a few field aliases for forward-compat.
func Import(raw []byte, opts ImportOptions) (*ImportResult, error) {
	var wf Workflow
	if err := json.Unmarshal(raw, &wf); err != nil {
		return nil, fmt.Errorf("parse workflow json: %w", err)
	}

	var warnings []string

	// Name override / validation.
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		name = strings.TrimSpace(wf.Name)
	}
	if name == "" {
		// Fall back to a slugified description or the id.
		name = sanitizeSlug(wf.Description)
		if name == "" {
			name = sanitizeSlug(wf.ID)
		}
		if name == "" {
			return nil, fmt.Errorf("workflow has no name; provide one via the import options")
		}
		warnings = append(warnings, "workflow had no name; derived one — please rename it")
	}
	if err := ValidateName(name); err != nil {
		return nil, fmt.Errorf("invalid workflow name %q: %w", name, err)
	}
	wf.Name = name

	// Defaults.
	if wf.Version == "" {
		wf.Version = "1.0.0"
	}
	if wf.ID == "" {
		wf.ID = wf.Name
	}

	// Normalize the legacy "branch" type to "switch".
	for i := range wf.Nodes {
		if wf.Nodes[i].Type == NodeTypeBranch {
			wf.Nodes[i].Type = NodeTypeSwitch
			warnings = append(warnings, fmt.Sprintf("node %q: legacy 'branch' type converted to 'switch'", wf.Nodes[i].ID))
		}
		// cc-wf-studio uses "label" in prompt nodes; nothing to remap here.
	}

	// Ensure the required graph scaffolding exists. cc-wf-studio always emits
	// start/end nodes; if absent we add them so the workflow is immediately
	// exportable.
	ensureEndpoints(&wf)

	now := time.Now().UTC()
	if wf.CreatedAt.IsZero() {
		wf.CreatedAt = now
	}
	wf.UpdatedAt = now

	return &ImportResult{Workflow: &wf, Warnings: warnings}, nil
}

// ensureEndpoints adds start/end nodes if missing and wires any dangling node
// inputs/outputs to them so the graph is connected.
func ensureEndpoints(wf *Workflow) {
	byID := wf.nodeByID()
	hasStart := wf.findNode(NodeTypeStart) != nil
	hasEnd := wf.findNode(NodeTypeEnd) != nil

	if !hasStart {
		start := Node{
			ID:   "start-node-default",
			Type: NodeTypeStart,
			Name: "start-node-default",
			Data: NodeData{Label: "Start"},
		}
		wf.Nodes = append(wf.Nodes, start)
		// Wire start → first non-end node that has no incoming edges.
		if target := firstSink(wf, byID, NodeTypeStart); target != "" {
			wf.Connections = append(wf.Connections, Connection{From: start.ID, To: target, FromPort: "out", ToPort: "input"})
		}
	}
	if !hasEnd {
		end := Node{
			ID:   "end-node-default",
			Type: NodeTypeEnd,
			Name: "end-node-default",
			Data: NodeData{Label: "End"},
		}
		wf.Nodes = append(wf.Nodes, end)
		// Wire last node with no outgoing edges → end.
		if source := firstSourceless(wf, byID, NodeTypeEnd); source != "" {
			wf.Connections = append(wf.Connections, Connection{From: source, To: end.ID, FromPort: "out", ToPort: "input"})
		}
	}
}

// firstSink returns the id of the first node (excluding 'exclude' and groups)
// that has no incoming connection.
func firstSink(wf *Workflow, byID map[string]*Node, excludeType string) string {
	incoming := make(map[string]bool)
	for _, c := range wf.Connections {
		incoming[c.To] = true
	}
	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		if n.Type == excludeType || n.Type == NodeTypeEnd || n.Type == NodeTypeGroup {
			continue
		}
		if !incoming[n.ID] {
			return n.ID
		}
	}
	return ""
}

// firstSourceless returns the id of the first non-start/group node with no
// outgoing connection.
func firstSourceless(wf *Workflow, byID map[string]*Node, excludeType string) string {
	outgoing := make(map[string]bool)
	for _, c := range wf.Connections {
		outgoing[c.From] = true
	}
	// Iterate in reverse so we pick the "last" node for LR layouts.
	for i := len(wf.Nodes) - 1; i >= 0; i-- {
		n := &wf.Nodes[i]
		if n.Type == excludeType || n.Type == NodeTypeStart || n.Type == NodeTypeGroup {
			continue
		}
		if !outgoing[n.ID] {
			return n.ID
		}
	}
	return ""
}
