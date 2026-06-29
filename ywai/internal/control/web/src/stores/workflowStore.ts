import { create } from 'zustand'
import { workflowApi } from '../api/client'
import type {
	Workflow,
	WorkflowSummary,
	WorkflowNode,
	WorkflowConnection,
	WorkflowNodeType,
	WorkflowValidationResult,
	WorkflowExportPlan,
} from '../api/types'

// ─── edge id helpers ──────────────────────────────────────────────────────
// A connection's identity is its (from, fromPort)->(to, toPort) tuple.
function edgeId(c: WorkflowConnection): string {
	return [c.from, c.fromPort ?? ''].join('|') + '->' + [c.to, c.toPort ?? ''].join('|')
}

// Backend may return null for nodes/connections on an empty or legacy
// workflow; normalize to [] so the editor never maps over null.
function normalizeWorkflow(wf: Workflow): Workflow {
	return { ...wf, nodes: wf.nodes ?? [], connections: wf.connections ?? [] }
}

// ─── auto-layout ──────────────────────────────────────────────────────────
// Compact serpentine grid: topologically order the nodes (so the sequence
// follows the graph), then fill a grid row by row, alternating direction each
// row. Consecutive nodes stay adjacent, so the chain entered as a single line
// wraps into a roughly square block that fits the viewport at a readable zoom.
const LAYOUT_X_GAP = 280
const LAYOUT_Y_GAP = 150
const LAYOUT_X0 = 80
const LAYOUT_Y0 = 80
function layoutNodes(wf: Workflow): WorkflowNode[] {
	const adj = new Map<string, string[]>()
	const indeg = new Map<string, number>()
	for (const n of wf.nodes) {
		adj.set(n.id, [])
		indeg.set(n.id, 0)
	}
	for (const c of wf.connections) {
		if (!adj.has(c.from) || !indeg.has(c.to)) continue
		adj.get(c.from)!.push(c.to)
		indeg.set(c.to, (indeg.get(c.to) ?? 0) + 1)
	}
	// Kahn topological order, stable by original node order. Validated workflows
	// are acyclic; if a cycle ever slips through, leftover nodes are appended in
	// their original order so every node still gets placed.
	const order: string[] = []
	const placed = new Set<string>()
	const ready = wf.nodes.filter((n) => (indeg.get(n.id) ?? 0) === 0).map((n) => n.id)
	while (ready.length) {
		const cur = ready.shift()!
		if (placed.has(cur)) continue
		order.push(cur)
		placed.add(cur)
		for (const next of adj.get(cur) ?? []) {
			const left = (indeg.get(next) ?? 0) - 1
			indeg.set(next, left)
			if (left === 0) ready.push(next)
		}
	}
	for (const n of wf.nodes) if (!placed.has(n.id)) order.push(n.id)

	const cols = Math.max(1, Math.ceil(Math.sqrt(order.length)))
	const positioned = new Map<string, { x: number; y: number }>()
	order.forEach((id, i) => {
		const row = Math.floor(i / cols)
		const colInRow = i % cols
		// Snake: even rows left→right, odd rows right→left, so the node ending one
		// row sits directly above the node starting the next.
		const col = row % 2 === 0 ? colInRow : cols - 1 - colInRow
		positioned.set(id, { x: LAYOUT_X0 + col * LAYOUT_X_GAP, y: LAYOUT_Y0 + row * LAYOUT_Y_GAP })
	})
	return wf.nodes.map((n) => ({ ...n, position: positioned.get(n.id) ?? n.position }))
}

let idCounter = 0
function newId(prefix: string): string {
	idCounter += 1
	return `${prefix}-${Date.now().toString(36)}-${idCounter}`
}

// Default node data factory: gives every new node sensible empty fields so the
// side panel renders without null checks.
function defaultData(type: WorkflowNodeType): WorkflowNode['data'] {
	switch (type) {
		case 'start':
			return { label: 'Start' }
		case 'end':
			return { label: 'End' }
		case 'subAgent':
			return { description: '', agentDefinition: '', prompt: '', tools: '', model: 'inherit', mode: 'all' }
		case 'askUserQuestion':
			return { questionText: '', options: [{ label: 'Option A' }, { label: 'Option B' }] }
		case 'prompt':
			return { prompt: '', label: 'Prompt' }
		case 'ifElse':
			return { condition: '' }
		case 'switch':
		case 'branch':
			return { expression: '', branches: [] }
		case 'skill':
			return { name: '', executionMode: 'load' }
		case 'mcp':
			return { server: '', tool: '' }
		case 'subAgentFlow':
			return { flowId: '' }
		case 'group':
			return { label: 'Group', width: 360, height: 240 }
		default:
			return {}
	}
}

interface WorkflowState {
	// listings
	summaries: WorkflowSummary[]
	loadingList: boolean

	// current workflow
	current: Workflow | null
	loading: boolean
	dirty: boolean
	error: string | null

	// undo/redo history of `current` snapshots
	past: Workflow[]
	future: Workflow[]

	// validation + export
	validation: WorkflowValidationResult | null
	exportPlan: WorkflowExportPlan | null
	exporting: boolean

	// edit with AI
	aiEditing: boolean

	// selection
	selectedNodeId: string | null

	// ── actions ──
	list: () => Promise<void>
	load: (name: string) => Promise<void>
	createNew: (name: string, description?: string) => Promise<void>
	saveCurrent: () => Promise<void>
	deleteCurrent: () => Promise<void>
	importRaw: (raw: unknown, name?: string) => Promise<void>
	validateCurrent: () => Promise<void>
	exportCurrent: (apply: boolean, target?: string) => Promise<void>
	clearExport: () => void
	// Apply a natural-language edit via the backend AI endpoint. The result is
	// loaded into the editor (undoable) and left dirty for the user to Save.
	aiEdit: (instruction: string, model?: string) => Promise<void>

	// graph editing (optimistic, persisted on save)
	selectNode: (id: string | null) => void
	addNode: (type: WorkflowNodeType, x: number, y: number) => string
	updateNode: (id: string, patch: Partial<WorkflowNode['data']>) => void
	updateNodeData: (id: string, data: WorkflowNode['data']) => void
	removeNode: (id: string) => void
	connect: (c: WorkflowConnection) => void
	disconnect: (id: string) => void
	// Apply React Flow node changes (drag position, dimensions) back into the
	// store so nodes can be moved on the canvas.
	applyNodeChanges: (changes: import('@xyflow/react').NodeChange[]) => void
	// Re-position every node into a left-to-right layered layout derived from
	// the graph (column = longest-path depth, stacked within a column).
	autoLayout: () => void

	// Assign/clear a node's group parent (with its converted position).
	setNodeParent: (id: string, parentId: string | null, position: { x: number; y: number }) => void

	// undo/redo
	undo: () => void
	redo: () => void

	clearError: () => void
}

// HISTORY_LIMIT caps retained snapshots so a long session never grows unbounded.
const HISTORY_LIMIT = 50

export const useWorkflowStore = create<WorkflowState>((set, get) => ({
	summaries: [],
	loadingList: false,
	current: null,
	loading: false,
	dirty: false,
	error: null,
	validation: null,
	exportPlan: null,
	exporting: false,
	aiEditing: false,
	selectedNodeId: null,
	past: [],
	future: [],

	list: async () => {
		set({ loadingList: true, error: null })
		try {
			const { workflows } = await workflowApi.list()
			// Backend may return null for an empty list; normalize to [].
			set({ summaries: workflows ?? [], loadingList: false })
		} catch (err) {
			set({ loadingList: false, error: errMsg(err) })
		}
	},

	load: async (name) => {
		set({ loading: true, error: null, current: null, dirty: false, selectedNodeId: null, validation: null, past: [], future: [] })
		try {
			const wf = await workflowApi.get(name)
			set({ current: normalizeWorkflow(wf), loading: false })
		} catch (err) {
			set({ loading: false, error: errMsg(err) })
		}
	},

	createNew: async (name, description) => {
		const wf: Workflow = {
			id: name,
			name,
			description: description ?? '',
			version: '1.0.0',
			nodes: [
				{ id: 'start-node-default', type: 'start', name: 'start-node-default', position: { x: 80, y: 200 }, data: { label: 'Start' } },
				{ id: 'end-node-default', type: 'end', name: 'end-node-default', position: { x: 800, y: 200 }, data: { label: 'End' } },
			],
			connections: [],
			createdAt: new Date().toISOString(),
			updatedAt: new Date().toISOString(),
		}
		try {
			const created = await workflowApi.create(wf)
			set({ current: normalizeWorkflow(created), dirty: false, error: null, past: [], future: [] })
			await get().list()
		} catch (err) {
			set({ error: errMsg(err) })
		}
	},

	saveCurrent: async () => {
		const { current } = get()
		if (!current) return
		// Snapshot for rollback.
		const prev = current
		set({ dirty: false, error: null })
		try {
			const saved = await workflowApi.save(current.name, current)
			set({ current: { ...saved, nodes: current.nodes, connections: current.connections } })
			await get().list()
		} catch (err) {
			set({ current: prev, dirty: true, error: errMsg(err) })
		}
	},

	deleteCurrent: async () => {
		const { current } = get()
		if (!current) return
		try {
			await workflowApi.delete(current.name)
			set({ current: null, dirty: false, selectedNodeId: null, validation: null, exportPlan: null })
			await get().list()
		} catch (err) {
			set({ error: errMsg(err) })
		}
	},

	importRaw: async (raw, name) => {
		set({ loading: true, error: null })
		try {
			const result = await workflowApi.import(raw, name)
			set({ current: normalizeWorkflow(result.workflow), loading: false, dirty: false, past: [], future: [] })
			await get().list()
		} catch (err) {
			set({ loading: false, error: errMsg(err) })
		}
	},

	validateCurrent: async () => {
		const { current } = get()
		if (!current) return
		// Persist first so the backend validates the latest graph.
		await get().saveCurrent()
		try {
			const result = await workflowApi.validate(current.name)
			set({ validation: result })
		} catch (err) {
			set({ error: errMsg(err) })
		}
	},

	exportCurrent: async (apply, target = 'opencode') => {
		const { current } = get()
		if (!current) return
		set({ exporting: true, error: null })
		try {
			const plan = await workflowApi.export(current.name, apply, target)
			set({ exportPlan: plan, exporting: false })
		} catch (err) {
			set({ exporting: false, error: errMsg(err) })
		}
	},

	clearExport: () => set({ exportPlan: null }),

	aiEdit: async (instruction, model) => {
		const { current } = get()
		if (!current) return
		set({ aiEditing: true, error: null })
		try {
			const { workflow, validation } = await workflowApi.aiEdit(current.name, instruction, model)
			// Snapshot the pre-edit state so the AI change is a single undo step,
			// then load the proposal into the editor (kept dirty for the user to Save).
			snapshot()
			set({ current: normalizeWorkflow(workflow), validation, dirty: true, aiEditing: false })
		} catch (err) {
			set({ aiEditing: false, error: errMsg(err) })
		}
	},

	selectNode: (id) => set({ selectedNodeId: id }),

	applyNodeChanges: (changes) => {
		const { current } = get()
		if (!current) return
		const posById = new Map<string, { x: number; y: number }>()
		for (const ch of changes) {
			if (ch.type === 'position' && ch.position) posById.set(ch.id, { x: ch.position.x, y: ch.position.y })
		}
		if (!posById.size) return
		snapshot()
		const nodes = current.nodes.map((n) => {
			const p = posById.get(n.id)
			return p ? { ...n, position: p } : n
		})
		set({ current: { ...current, nodes }, dirty: true })
	},

	autoLayout: () => {
		const { current } = get()
		if (!current) return
		snapshot()
		set({ current: { ...current, nodes: layoutNodes(current) }, dirty: true })
	},

	addNode: (type, x, y) => {
		const id = newId(type)
		const node: WorkflowNode = {
			id,
			type,
			name: id,
			position: { x, y },
			data: defaultData(type),
		}
		const { current } = get()
		if (!current) return id
		snapshot()
		set({ current: { ...current, nodes: [...current.nodes, node] }, dirty: true })
		return id
	},

	updateNode: (id, patch) => {
		const { current } = get()
		if (!current) return
		snapshot('edit:' + id)
		set({
			current: {
				...current,
				nodes: current.nodes.map((n) =>
					n.id === id ? { ...n, data: { ...n.data, ...patch } } : n,
				),
			},
			dirty: true,
		})
	},

	updateNodeData: (id, data) => {
		const { current } = get()
		if (!current) return
		snapshot('edit:' + id)
		set({
			current: {
				...current,
				nodes: current.nodes.map((n) => (n.id === id ? { ...n, data } : n)),
			},
			dirty: true,
		})
	},

	removeNode: (id) => {
		const { current, selectedNodeId } = get()
		if (!current) return
		snapshot()
		set({
			current: {
				...current,
				nodes: current.nodes.filter((n) => n.id !== id),
				connections: current.connections.filter((c) => c.from !== id && c.to !== id),
			},
			dirty: true,
			selectedNodeId: selectedNodeId === id ? null : selectedNodeId,
		})
	},

	connect: (c) => {
		const { current } = get()
		if (!current) return
		// Avoid duplicate edges.
		const key = edgeId(c)
		if (current.connections.some((e) => edgeId(e) === key)) return
		snapshot()
		set({ current: { ...current, connections: [...current.connections, c] }, dirty: true })
	},

	disconnect: (id) => {
		const { current } = get()
		if (!current) return
		snapshot()
		set({
			current: { ...current, connections: current.connections.filter((e) => edgeId(e) !== id) },
			dirty: true,
		})
	},

	setNodeParent: (id, parentId, position) => {
		const { current } = get()
		if (!current) return
		snapshot()
		set({
			current: {
				...current,
				nodes: current.nodes.map((n) =>
					n.id === id ? { ...n, parentId: parentId ?? undefined, position } : n,
				),
			},
			dirty: true,
		})
	},

	undo: () => {
		const { past, current, future } = get()
		if (!past.length || !current) return
		const prev = past[past.length - 1]
		set({
			current: prev,
			past: past.slice(0, -1),
			future: [current, ...future].slice(0, HISTORY_LIMIT),
			dirty: true,
		})
		coalesceKey = ''
	},

	redo: () => {
		const { past, current, future } = get()
		if (!future.length || !current) return
		const next = future[0]
		set({
			current: next,
			past: [...past, current].slice(-HISTORY_LIMIT),
			future: future.slice(1),
			dirty: true,
		})
		coalesceKey = ''
	},

	clearError: () => set({ error: null }),
}))

export function disconnectEdgeId(c: WorkflowConnection): string {
	return edgeId(c)
}

function errMsg(err: unknown): string {
	return err instanceof Error ? err.message : String(err)
}

// snapshot records the current workflow into the undo stack before a mutation.
// Mutations always replace `current` with a fresh object, so the pushed
// reference stays frozen as the pre-mutation state — no deep clone needed.
// Any new edit clears the redo stack.
//
// A coalesce `key` collapses rapid repeat edits (e.g. typing in a field) into a
// single undo entry: consecutive snapshots with the same key inside the window
// reuse the first one. Structural ops pass no key, forcing a fresh entry.
let coalesceKey = ''
let coalesceAt = 0
const COALESCE_MS = 700
function snapshot(key?: string): void {
	const s = useWorkflowStore.getState()
	if (!s.current) return
	if (key) {
		const now = Date.now()
		if (key === coalesceKey && now - coalesceAt < COALESCE_MS) {
			coalesceAt = now
			return
		}
		coalesceKey = key
		coalesceAt = now
	} else {
		coalesceKey = ''
		coalesceAt = 0
	}
	useWorkflowStore.setState({
		past: [...s.past, s.current].slice(-HISTORY_LIMIT),
		future: [],
	})
}
