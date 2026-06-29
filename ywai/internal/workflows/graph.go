package workflows

import "fmt"

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
// Mirrors kanban.Store.HasCycle but operates over the static workflow graph.
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
