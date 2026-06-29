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
			return { label: 'Group' }
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

	// validation + export
	validation: WorkflowValidationResult | null
	exportPlan: WorkflowExportPlan | null
	exporting: boolean

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
	exportCurrent: (apply: boolean) => Promise<void>
	clearExport: () => void

	// graph editing (optimistic, persisted on save)
	selectNode: (id: string | null) => void
	addNode: (type: WorkflowNodeType, x: number, y: number) => string
	updateNode: (id: string, patch: Partial<WorkflowNode['data']>) => void
	updateNodeData: (id: string, data: WorkflowNode['data']) => void
	removeNode: (id: string) => void
	connect: (c: WorkflowConnection) => void
	disconnect: (id: string) => void

	clearError: () => void
}

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
	selectedNodeId: null,

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
		set({ loading: true, error: null, current: null, dirty: false, selectedNodeId: null, validation: null })
		try {
			const wf = await workflowApi.get(name)
			set({ current: wf, loading: false })
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
			set({ current: created, dirty: false, error: null })
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
			set({ current: result.workflow, loading: false, dirty: false })
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

	exportCurrent: async (apply) => {
		const { current } = get()
		if (!current) return
		set({ exporting: true, error: null })
		try {
			const plan = await workflowApi.export(current.name, apply)
			set({ exportPlan: plan, exporting: false })
		} catch (err) {
			set({ exporting: false, error: errMsg(err) })
		}
	},

	clearExport: () => set({ exportPlan: null }),

	selectNode: (id) => set({ selectedNodeId: id }),

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
		set({ current: { ...current, nodes: [...current.nodes, node] }, dirty: true })
		return id
	},

	updateNode: (id, patch) => {
		const { current } = get()
		if (!current) return
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
		set({ current: { ...current, connections: [...current.connections, c] }, dirty: true })
	},

	disconnect: (id) => {
		const { current } = get()
		if (!current) return
		set({
			current: { ...current, connections: current.connections.filter((e) => edgeId(e) !== id) },
			dirty: true,
		})
	},

	clearError: () => set({ error: null }),
}))

export function disconnectEdgeId(c: WorkflowConnection): string {
	return edgeId(c)
}

function errMsg(err: unknown): string {
	return err instanceof Error ? err.message : String(err)
}
