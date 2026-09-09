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

// forwardPass walks the graph the way the orchestrator runs it and reports both
// results of that walk: the layers of the forward DAG, and the connections that
// loop back into it.
//
// A rework loop (review→fix) is a legitimate edge but not an ordering
// constraint: treating it as one makes the graph unsortable, which used to send
// buildSteps to declaration order — the order nodes happen to sit in the JSON.
// So the loop has to be found and dropped, and structure alone does not say
// which edge of a cycle is the loop: in ask→fe→qa→review→gate→fe, cutting
// review→gate breaks the cycle just as well as cutting gate→fe, and puts the
// gate before the work it gates.
//
// The rule that does distinguish them: when the forward pass stalls, resume at
// whichever stalled nodes it reached earliest, and the edges still pointing at
// those from unfinished nodes are the rework edges. In the ship workflow the
// pass reaches frontend, backend and infrastructure together from the approval
// gate, so all three of `gate→*` are cut in one go and the three stay in the
// fan-out layer they share.
//
// This replaced a DFS colouring that asked only whether an edge's target sat on
// the current stack, so a node with two parents was claimed by whichever branch
// reached it first: `gate→frontend` came out a back edge while `gate→backend`
// did not, and backend and infrastructure fell out of the fan-out layer.
func (wf *Workflow) forwardPass() (layers [][]string, back map[string]bool) {
	index := make(map[string]int, len(wf.Nodes))
	for i, n := range wf.Nodes {
		index[n.ID] = i
	}
	exists := make(map[string]bool, len(wf.Nodes))
	for _, n := range wf.Nodes {
		exists[n.ID] = true
	}

	adj := make(map[string][]string, len(wf.Nodes))
	incoming := make(map[string][]string, len(wf.Nodes))
	for _, c := range wf.dedupConnections() {
		if !exists[c.From] || !exists[c.To] {
			continue
		}
		adj[c.From] = append(adj[c.From], c.To)
		incoming[c.To] = append(incoming[c.To], c.From)
	}

	back = make(map[string]bool)
	done := make(map[string]bool, len(wf.Nodes))
	// reachedAt records the layer at which a node first had a predecessor
	// finish, so a stall can resume at the node the pass reached earliest.
	reachedAt := make(map[string]int, len(wf.Nodes))

	pending := func(id string) []string {
		var rest []string
		for _, from := range incoming[id] {
			if !done[from] && !back[from+"->"+id] {
				rest = append(rest, from)
			}
		}
		return rest
	}

	ready := make([]string, 0, len(wf.Nodes))
	for _, n := range wf.Nodes {
		if len(pending(n.ID)) == 0 {
			ready = append(ready, n.ID)
			reachedAt[n.ID] = 0
		}
	}

	// First pass: walk the graph to find the loops. Layers built here would be
	// wrong — a node held back by an edge not yet known to be a loop misses the
	// layer it belongs to — so only `back` is kept.
	for remaining := len(wf.Nodes); remaining > 0; {
		if len(ready) == 0 {
			ready = wf.resumeAfterStall(index, reachedAt, done, incoming, back)
			if len(ready) == 0 {
				break // nothing left to reach: caller appends the leftovers
			}
		}
		remaining -= len(ready)
		layer := len(reachedAt)
		for _, id := range ready {
			done[id] = true
		}
		next := make([]string, 0)
		for _, u := range ready {
			for _, v := range adj[u] {
				if done[v] || back[u+"->"+v] {
					continue
				}
				if _, seen := reachedAt[v]; !seen {
					reachedAt[v] = layer
				}
				if len(pending(v)) == 0 {
					next = append(next, v)
				}
			}
		}
		sort.SliceStable(next, func(i, j int) bool { return index[next[i]] < index[next[j]] })
		ready = next
	}

	// Second pass: layer the DAG that is left once the loops are dropped.
	inDegree := make(map[string]int, len(wf.Nodes))
	for _, n := range wf.Nodes {
		inDegree[n.ID] = 0
	}
	for to, froms := range incoming {
		for _, from := range froms {
			if !back[from+"->"+to] {
				inDegree[to]++
			}
		}
	}
	ready = ready[:0]
	for _, n := range wf.Nodes {
		if inDegree[n.ID] == 0 {
			ready = append(ready, n.ID)
		}
	}
	for len(ready) > 0 {
		layers = append(layers, ready)
		next := make([]string, 0)
		for _, u := range ready {
			for _, v := range adj[u] {
				if back[u+"->"+v] {
					continue
				}
				if inDegree[v]--; inDegree[v] == 0 {
					next = append(next, v)
				}
			}
		}
		sort.SliceStable(next, func(i, j int) bool { return index[next[i]] < index[next[j]] })
		ready = next
	}
	return layers, back
}

// resumeAfterStall picks where the forward pass carries on once every unfinished
// node still has an unfinished predecessor, and cuts the edges that held them.
//
// It resumes at the unfinished nodes the pass reached earliest — those already
// entered from outside the cycle — so the edge that re-enters work is the one
// dropped, not the edge that leads into it. Nodes reached in the same layer
// resume together, which keeps a fan-out that loops back in one layer. A cycle
// nothing reaches at all has no such node, so the first one declared stands in.
func (wf *Workflow) resumeAfterStall(index map[string]int, reachedAt map[string]int, done map[string]bool, incoming map[string][]string, back map[string]bool) []string {
	best, found := 0, false
	for _, n := range wf.Nodes {
		if done[n.ID] {
			continue
		}
		at, seen := reachedAt[n.ID]
		if !seen {
			continue
		}
		if !found || at < best {
			best, found = at, true
		}
	}

	var resume []string
	for _, n := range wf.Nodes {
		if done[n.ID] {
			continue
		}
		if at, seen := reachedAt[n.ID]; found && (!seen || at != best) {
			continue
		}
		resume = append(resume, n.ID)
		if !found {
			break // unreached cycle: one stand-in is enough to make progress
		}
	}

	for _, id := range resume {
		for _, from := range incoming[id] {
			if !done[from] {
				back[from+"->"+id] = true
			}
		}
	}
	sort.SliceStable(resume, func(i, j int) bool { return index[resume[i]] < index[resume[j]] })
	return resume
}

// backEdges returns the connections that close a cycle, keyed "from->to". The
// loop is rendered as explicit routing instead of silently reordering the run.
func (wf *Workflow) backEdges() map[string]bool {
	_, back := wf.forwardPass()
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
	layers, _ := wf.forwardPass()
	// Defensive: a leftover node would mean the pass stalled with nothing left
	// to resume from. Append those in declaration order rather than drop them.
	placed := make(map[string]bool)
	for _, l := range layers {
		for _, id := range l {
			placed[id] = true
		}
	}
	var rest []string
	for _, n := range wf.Nodes {
		if !placed[n.ID] {
			rest = append(rest, n.ID)
		}
	}
	if len(rest) > 0 {
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
