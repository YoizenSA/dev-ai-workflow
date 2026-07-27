package workflows

import (
	"fmt"
	"sort"
)

// adjacency builds a from-node → list-of-target-nodes map from connections.
// Only connections whose endpoints exist in nodes are kept.
func (wf *Workflow) adjacency() map[string][]string {
	exists := make(map[string]bool, len(wf.Nodes))
	for _, n := range wf.Nodes {
		exists[n.ID] = true
	}
	adj := make(map[string][]string, len(wf.Nodes))
	for _, c := range wf.Connections {
		if !exists[c.From] || !exists[c.To] {
			continue
		}
		adj[c.From] = append(adj[c.From], c.To)
	}
	return adj
}

// dedupConnections returns the connections with duplicates removed. A
// connection is a duplicate of another when they share the same (from, to)
// pair — regardless of port. Two arrows from A to B add no information to the
// graph, so collapsing them keeps the exported Mermaid diagram and execution
// steps clean (no repeated edges) even when a branching node (if/else/switch)
// routes two outcomes to the same target.
func (wf *Workflow) dedupConnections() []Connection {
	seen := make(map[string]bool, len(wf.Connections))
	out := make([]Connection, 0, len(wf.Connections))
	for _, c := range wf.Connections {
		key := c.From + "->" + c.To
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

// nodeByID indexes the workflow's nodes by ID.
func (wf *Workflow) nodeByID() map[string]*Node {
	m := make(map[string]*Node, len(wf.Nodes))
	for i := range wf.Nodes {
		m[wf.Nodes[i].ID] = &wf.Nodes[i]
	}
	return m
}

// findNode returns the first node of a given type, or nil.
func (wf *Workflow) findNode(nodeType string) *Node {
	for i := range wf.Nodes {
		if wf.Nodes[i].Type == nodeType {
			return &wf.Nodes[i]
		}
	}
	return nil
}

// countByType returns how many nodes of each type the workflow has.
func (wf *Workflow) countByType() map[string]int {
	m := make(map[string]int, len(wf.Nodes))
	for _, n := range wf.Nodes {
		m[n.Type]++
	}
	return m
}

// hasCycle reports whether the workflow graph contains a cycle, via DFS.
// Operates over the static workflow graph.
func (wf *Workflow) hasCycle() bool {
	adj := wf.adjacency()

	const (
		white = 0 // unvisited
		gray  = 1 // on current DFS stack
		black = 2 // fully explored
	)
	color := make(map[string]int, len(wf.Nodes))
	for _, n := range wf.Nodes {
		color[n.ID] = white
	}

	var dfs func(string) bool
	dfs = func(node string) bool {
		color[node] = gray
		for _, next := range adj[node] {
			switch color[next] {
			case gray:
				return true // back edge → cycle
			case white:
				if dfs(next) {
					return true
				}
			}
		}
		color[node] = black
		return false
	}

	for _, n := range wf.Nodes {
		if color[n.ID] == white && dfs(n.ID) {
			return true
		}
	}
	return false
}

// backEdges returns the connections that close a cycle, keyed "from->to".
//
// A rework loop (review→fix) is a legitimate edge, but it is not an ordering
// constraint: treating it as one makes the graph unsortable, which used to send
// buildSteps to declaration order — the order nodes happen to sit in the JSON.
// Removing these edges leaves the DAG that describes the forward pass; the loop
// is then rendered as explicit routing instead of silently reordering the run.
func (wf *Workflow) backEdges() map[string]bool {
	adj := wf.adjacency()

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(wf.Nodes))
	for _, n := range wf.Nodes {
		color[n.ID] = white
	}

	back := make(map[string]bool)
	var dfs func(string)
	dfs = func(node string) {
		color[node] = gray
		for _, next := range adj[node] {
			switch color[next] {
			case gray:
				back[node+"->"+next] = true
			case white:
				dfs(next)
			}
		}
		color[node] = black
	}
	// Walk in declaration order so the same graph always yields the same set.
	for _, n := range wf.Nodes {
		if color[n.ID] == white {
			dfs(n.ID)
		}
	}
	return back
}

// executionLayers groups nodes into forward-pass layers over the DAG that
// remains once back edges are dropped. Every node in a layer is independent of
// every other node in the same layer: no data flows between them, so they can
// be delegated concurrently.
//
// Layer 0 holds the entry nodes; each later layer holds the nodes whose
// dependencies are all satisfied by earlier layers. Ties keep declaration order
// so the exported prompt is byte-stable across runs.
func (wf *Workflow) executionLayers() [][]string {
	back := wf.backEdges()
	index := make(map[string]int, len(wf.Nodes))
	for i, n := range wf.Nodes {
		index[n.ID] = i
	}

	exists := make(map[string]bool, len(wf.Nodes))
	for _, n := range wf.Nodes {
		exists[n.ID] = true
	}
	adj := make(map[string][]string, len(wf.Nodes))
	inDegree := make(map[string]int, len(wf.Nodes))
	for _, n := range wf.Nodes {
		inDegree[n.ID] = 0
	}
	for _, c := range wf.Connections {
		if !exists[c.From] || !exists[c.To] || back[c.From+"->"+c.To] {
			continue
		}
		adj[c.From] = append(adj[c.From], c.To)
		inDegree[c.To]++
	}

	var layers [][]string
	remaining := len(wf.Nodes)
	ready := make([]string, 0, len(wf.Nodes))
	for _, n := range wf.Nodes {
		if inDegree[n.ID] == 0 {
			ready = append(ready, n.ID)
		}
	}
	for len(ready) > 0 {
		layers = append(layers, ready)
		remaining -= len(ready)
		next := make([]string, 0)
		for _, u := range ready {
			for _, v := range adj[u] {
				inDegree[v]--
				if inDegree[v] == 0 {
					next = append(next, v)
				}
			}
		}
		sort.SliceStable(next, func(i, j int) bool { return index[next[i]] < index[next[j]] })
		ready = next
	}
	// Defensive: a node left over would mean a cycle survived back-edge removal.
	// Append the leftovers in declaration order rather than dropping them.
	if remaining > 0 {
		var rest []string
		placed := make(map[string]bool)
		for _, l := range layers {
			for _, id := range l {
				placed[id] = true
			}
		}
		for _, n := range wf.Nodes {
			if !placed[n.ID] {
				rest = append(rest, n.ID)
			}
		}
		layers = append(layers, rest)
	}
	return layers
}

// topoOrder returns nodes in topological order using longest-path layering so
// that linear chains read left→right (matching the OrchestratorTab layout).
// Cyclic graphs return an error. Disconnected nodes are appended last.
func (wf *Workflow) topoOrder() ([]string, error) {
	if wf.hasCycle() {
		return nil, fmt.Errorf("workflow graph has a cycle")
	}
	adj := wf.adjacency()
	inDegree := make(map[string]int, len(wf.Nodes))
	for _, n := range wf.Nodes {
		inDegree[n.ID] = 0
	}
	for _, targets := range adj {
		for _, t := range targets {
			inDegree[t]++
		}
	}

	// Kahn's algorithm; tie-break by node order for determinism.
	order := make([]string, 0, len(wf.Nodes))
	queue := make([]string, 0)
	for _, n := range wf.Nodes {
		if inDegree[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		order = append(order, cur)
		for _, t := range adj[cur] {
			inDegree[t]--
			if inDegree[t] == 0 {
				queue = append(queue, t)
			}
		}
	}
	return order, nil
}
