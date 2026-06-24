import { create } from 'zustand'
import { configApi } from '../api/client'
import type {
	AgentGraph,
	AgentGraphNode,
	AgentGraphEdge,
	DelegationRule,
	DelegationTrigger,
} from '../api/types'

/**
 * Static delegation graph for the Orchestrator tab.
 *
 * The capability graph (what an agent MAY delegate) is derived server-side from
 * each agent's `permission.task` map. Mutations go through the existing
 * task-permissions / model endpoints — the graph here is the read model plus
 * optimistic local patches mirroring kanbanStore's optimistic-update pattern.
 */

type TaskValue = 'allow' | 'ask' | 'deny'

interface AgentsDiagramState {
	// data
	graph: AgentGraph
	selected: string | null // agent id
	loading: boolean
	error: string | null

	// delegation-rules (prompt-body section) for the selected agent
	delegationRules: DelegationRule[]
	delegationTriggers: DelegationTrigger[]
	hasDelegationRules: boolean
	loadingRules: boolean

	// actions
	load: () => Promise<void>
	selectAgent: (id: string | null) => void

	/** Cycle a delegation edge allow -> ask -> deny, or create one if absent. */
	cycleEdge: (source: string, target: string) => Promise<void>

	/** Create or update a delegation edge to an explicit value. */
	setEdge: (source: string, target: string, value: TaskValue) => Promise<void>

	/** Remove a delegation edge entirely. */
	removeEdge: (source: string, target: string) => Promise<void>

	/** Update an agent's model override. */
	setAgentModel: (agent: string, model: string) => Promise<void>

	/** Load the prompt-body delegation rules + triggers for an agent. */
	loadDelegationRules: (agent: string) => Promise<void>

	/** Save the prompt-body delegation rules + triggers for an agent. */
	saveDelegationRules: (
		agent: string,
		rules: DelegationRule[],
		triggers: DelegationTrigger[],
	) => Promise<void>
}

const CYCLE: Record<TaskValue, TaskValue> = {
	allow: 'ask',
	ask: 'deny',
	deny: 'allow',
}

/** Compute the full task map an edge mutation must PUT (preserving siblings). */
function buildTaskMap(
	graph: AgentGraph,
	source: string,
	mutate: (current: Record<string, TaskValue>) => Record<string, TaskValue>,
): Record<string, TaskValue> {
	const current: Record<string, TaskValue> = {}
	const node = graph.nodes.find((n) => n.id === source)
	if (node?.hasWildcard && node.wildcardValue) {
		current['*'] = node.wildcardValue as TaskValue
	}
	for (const e of graph.edges) {
		if (e.source === source) {
			current[e.target] = e.value
		}
	}
	return mutate(current)
}

/** Locally patch the graph after a task map write. */
function applyTaskWrite(
	graph: AgentGraph,
	source: string,
	taskMap: Record<string, TaskValue>,
): AgentGraph {
	// Drop existing edges from source, keep wildcard as node attribute.
	const keptEdges = graph.edges.filter((e) => e.source !== source)
	const newEdges: AgentGraphEdge[] = []
	let wildcardValue: string | undefined
	let hasWildcard = false
	for (const [target, value] of Object.entries(taskMap)) {
		if (target === '*') {
			hasWildcard = true
			wildcardValue = value
			continue
		}
		if (value === 'allow' || value === 'ask') {
			newEdges.push({ id: `${source}->${target}`, source, target, value })
		}
	}
	const nodes = graph.nodes.map((n) =>
		n.id === source ? { ...n, hasWildcard, wildcardValue } : n,
	)
	// Ensure ghost targets exist.
	const known = new Set(nodes.map((n) => n.id))
	for (const e of newEdges) {
		if (!known.has(e.target)) {
			known.add(e.target)
			nodes.push({ id: e.target, name: e.target, ghost: true })
		}
	}
	return { nodes, edges: [...keptEdges, ...newEdges] }
}

export const useAgentsDiagramStore = create<AgentsDiagramState>((set, get) => ({
	graph: { nodes: [], edges: [] },
	selected: null,
	loading: false,
	error: null,
	delegationRules: [],
	delegationTriggers: [],
	hasDelegationRules: false,
	loadingRules: false,

	load: async () => {
		set({ loading: true, error: null })
		try {
			const graph = await configApi.getAgentGraph()
			set({ graph, loading: false })
		} catch (err) {
			set({ loading: false, error: err instanceof Error ? err.message : String(err) })
		}
	},

	// Reset rules state when changing selection so stale rules don't leak.
	selectAgent: (id) =>
		set({
			selected: id,
			delegationRules: [],
			delegationTriggers: [],
			hasDelegationRules: false,
		}),

	cycleEdge: async (source, target) => {
		const current = get().graph.edges.find(
			(e) => e.source === source && e.target === target,
		)
		const next = CYCLE[current?.value ?? 'deny']
		await get().setEdge(source, target, next)
	},

	setEdge: async (source, target, value) => {
		const { graph } = get()
		// Optimistic local update first.
		const taskMap = buildTaskMap(graph, source, (cur) => {
			if (value === 'deny') {
				delete cur[target]
			} else {
				cur[target] = value
			}
			return cur
		})
		const optimistic = applyTaskWrite(graph, source, taskMap)
		set({ graph: optimistic })

		try {
			await configApi.updateAgentTaskPermissions(source, taskMap)
		} catch (err) {
			// Roll back to the last server-derived graph on failure.
			set({ graph, error: err instanceof Error ? err.message : String(err) })
			throw err
		}
	},

	removeEdge: async (source, target) => {
		await get().setEdge(source, target, 'deny')
	},

	setAgentModel: async (agent, model) => {
		const { graph } = get()
		const optimistic: AgentGraph = {
			nodes: graph.nodes.map((n) => (n.id === agent ? { ...n, model } : n)),
			edges: graph.edges,
		}
		set({ graph: optimistic })
		try {
			await configApi.updateAgentModel(agent, model)
		} catch (err) {
			set({ graph, error: err instanceof Error ? err.message : String(err) })
			throw err
		}
	},

	loadDelegationRules: async (agent) => {
		set({ loadingRules: true })
		try {
			const resp = await configApi.getDelegationRules(agent)
			set({
				delegationRules: resp.rules,
				delegationTriggers: resp.triggers,
				hasDelegationRules: resp.hasRules,
				loadingRules: false,
			})
		} catch (err) {
			set({
				loadingRules: false,
				delegationRules: [],
				delegationTriggers: [],
				hasDelegationRules: false,
				error: err instanceof Error ? err.message : String(err),
			})
		}
	},

	saveDelegationRules: async (agent, rules, triggers) => {
		// Optimistic local update.
		set({ delegationRules: rules, delegationTriggers: triggers, hasDelegationRules: true })
		try {
			await configApi.updateDelegationRules(agent, { rules, triggers })
		} catch (err) {
			set({ error: err instanceof Error ? err.message : String(err) })
			throw err
		}
	},
}))

/** Select a single agent node by id (memoizable selector). */
export function selectAgentNode(state: AgentsDiagramState): AgentGraphNode | null {
	if (!state.selected) return null
	return state.graph.nodes.find((n) => n.id === state.selected) ?? null
}
