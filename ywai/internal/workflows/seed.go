package workflows

import (
	"encoding/json"
	"reflect"
	"time"
)

// SeedDiff reports how an installed workflow differs from its bundled seed.
//
// Why: the seed pass never overwrites existing files (config.SeedWorkflowsFrom
// only re-applies agentRef/prompt/tools on matching node ids), so nodes added
// to a seed in the repo never reach installs that already have the workflow.
// This diff makes that drift visible in the Studio and backs the explicit
// "update from seed" action.
//
// Only node ids and node data count — layout (positions, group membership) and
// connection rewiring are ignored so a user who just dragged things around is
// not flagged as outdated.
type SeedDiff struct {
	InSync       bool     `json:"inSync"`
	Version      string   `json:"version,omitempty"`
	UpdatedAt    string   `json:"updatedAt,omitempty"`
	AddedNodes   []string `json:"addedNodes,omitempty"`
	RemovedNodes []string `json:"removedNodes,omitempty"`
	ChangedNodes []string `json:"changedNodes,omitempty"`
}

// DiffSeed compares raw local and seed workflow JSON. Unparseable input yields
// InSync — the list already skips corrupt files, so a diff here would only add
// noise; no comparison is a claim of nothing.
func DiffSeed(localRaw, seedRaw []byte) SeedDiff {
	d := SeedDiff{InSync: true}
	var local, seed Workflow
	if json.Unmarshal(seedRaw, &seed) != nil || json.Unmarshal(localRaw, &local) != nil {
		return d
	}
	d.Version = seed.Version
	if !seed.UpdatedAt.IsZero() {
		d.UpdatedAt = seed.UpdatedAt.Format(time.RFC3339)
	}

	localByID := make(map[string]Node, len(local.Nodes))
	for _, n := range local.Nodes {
		localByID[n.ID] = n
	}
	seedByID := make(map[string]Node, len(seed.Nodes))
	for _, n := range seed.Nodes {
		seedByID[n.ID] = n
	}
	for _, n := range seed.Nodes {
		prev, ok := localByID[n.ID]
		if !ok {
			d.AddedNodes = append(d.AddedNodes, nodeLabel(n))
			continue
		}
		if !reflect.DeepEqual(prev.Data, n.Data) {
			d.ChangedNodes = append(d.ChangedNodes, nodeLabel(n))
		}
	}
	for _, n := range local.Nodes {
		if _, ok := seedByID[n.ID]; !ok {
			d.RemovedNodes = append(d.RemovedNodes, nodeLabel(n))
		}
	}
	d.InSync = len(d.AddedNodes) == 0 && len(d.RemovedNodes) == 0 && len(d.ChangedNodes) == 0
	return d
}

func nodeLabel(n Node) string {
	if n.Name != "" {
		return n.Name
	}
	return n.ID
}
