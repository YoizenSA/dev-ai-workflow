import { create } from 'zustand'
import { workflowApi } from '../api/client'
import { useCommentaryStore } from './commentaryStore'
import type {
	Workflow,
	WorkflowSummary,
	WorkflowNode,
	WorkflowConnection,
	WorkflowNodeType,
	WorkflowValidationResult,
	WorkflowExportPlan,
	WorkflowRunLine,
	WorkflowSlashCommandOptions,
	WorkflowConversationHistory,
	WorkflowConversationMessage,
} from '../api/types'

// ─── edge id helpers ──────────────────────────────────────────────────────
// A connection's identity is its (from, fromPort)->(to, toPort) tuple.
function edgeId(c: WorkflowConnection): string {
	return [c.from, c.fromPort ?? ''].join('|') + '->' + [c.to, c.toPort ?? ''].join('|')
}

// Backend may return null for nodes/connections on an empty or legacy
// workflow; normalize to [] so the editor never maps over null. Duplicate
// connections (same from/port -> to/port) are also dropped here so React Flow
// never receives two edges with the same id (which triggers a React key warning
// and can break edge selection/updates). Duplicates can sneak in via import or
// AI edit, which bypass the connect() dedupe guard.
function normalizeWorkflow(wf: Workflow): Workflow {
	const connections = wf.connections ?? []
	const seen = new Set<string>()
	const deduped = connections.filter((c) => {
		const id = edgeId(c)
		if (seen.has(id)) return false
		seen.add(id)
		return true
	})
	return { ...wf, nodes: wf.nodes ?? [], connections: deduped }
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
	// Chat-scoped error for Edit-with-AI (separate from the global `error`, which
	// is surfaced in the sidebar). Shown inside the refinement chat panel.
	chatError: string | null

	// run (export + spawn the orchestrator). running lines stream into runOutput.
	running: boolean
	runId: string | null
	runOutput: WorkflowRunLine[]

	// selection
	selectedNodeId: string | null
	// node open in the Monaco focus editor (null = closed)
	focusNodeId: string | null

	// ── actions ──
	list: () => Promise<void>
	load: (name: string) => Promise<void>
	createNew: (name: string, description?: string) => Promise<void>
	saveCurrent: () => Promise<void>
	deleteCurrent: () => Promise<void>
	// Rename the current workflow (renames the on-disk file + patches id/name).
	renameCurrent: (newName: string) => Promise<void>
	importRaw: (raw: unknown, name?: string) => Promise<void>
	validateCurrent: () => Promise<void>
	exportCurrent: (apply: boolean, target?: string) => Promise<void>
	clearExport: () => void
	// Apply a natural-language edit via the backend AI endpoint. The result is
	// loaded into the editor (undoable) and left dirty for the user to Save.
	aiEdit: (instruction: string, model?: string) => Promise<void>

	// Export + spawn the orchestrator via opencode. Opens a WebSocket to stream
	// output; runOutput fills as lines arrive. The promise resolves once the run
	// starts (status 202); output continues asynchronously.
	runWorkflow: (args: string, model?: string) => Promise<void>

	// Stop the active run (kills the opencode process server-side).
	stopWorkflow: () => Promise<void>

	// Set the workflow's slash-command options (allowed-tools, model, hooks…).
	// Merges into `current` and marks dirty. Pass null to clear them.
	setSlashCommandOptions: (opts: WorkflowSlashCommandOptions | null) => void

	// graph editing (optimistic, persisted on save)
	selectNode: (id: string | null) => void
	setFocusNode: (id: string | null) => void
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
	// Clear the chat-scoped error (dismissed by the refinement chat panel).
	clearChatError: () => void
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
	chatError: null,
	running: false,
	runId: null,
	runOutput: [],
	selectedNodeId: null,
	focusNodeId: null,
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

	renameCurrent: async (newName) => {
		const { current } = get()
		if (!current) return
		try {
			const renamed = await workflowApi.rename(current.name, newName)
			set({ current: normalizeWorkflow(renamed), dirty: false, selectedNodeId: null })
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
		// Snapshot the pre-edit workflow so we can summarize what actually changed
		// (the AI bubble used to just echo the instruction back — misleading).
		const before = current
		set({ aiEditing: true, chatError: null, error: null })

		// Build the conversation history: carry forward the workflow's existing
		// history, append the user's new instruction, then call the AI with the
		// recent turns as context (multi-turn refinement).
		const now = new Date().toISOString()
		const history = current.conversationHistory?.messages ?? []
		const userMsg: WorkflowConversationMessage = {
			id: `u${Date.now()}`,
			sender: 'user',
			content: instruction,
			timestamp: now,
		}

		try {
			// Send the last few turns (capped server-side too) for context.
			const recent = [...history, userMsg].slice(-6)
			const { workflow, validation } = await workflowApi.aiEdit(
				current.name,
				instruction,
				model,
				recent,
			)
			// Snapshot the pre-edit state so the AI change is a single undo step.
			snapshot()
			// Summarize the real diff (added/removed/changed) instead of echoing the
			// instruction — the bubble now reflects what the AI actually did.
			const aiMsg: WorkflowConversationMessage = {
				id: `a${Date.now()}`,
				sender: 'ai',
				content: summarizeWorkflowChanges(before, normalizeWorkflow(workflow)),
				timestamp: new Date().toISOString(),
			}
			const conv: WorkflowConversationHistory = current.conversationHistory
				? { ...current.conversationHistory }
				: {
						schemaVersion: '1.0.0',
						messages: [],
						currentIteration: 0,
						maxIterations: 20,
						createdAt: now,
						updatedAt: now,
					}
			conv.messages = [...conv.messages, userMsg, aiMsg]
			conv.currentIteration = Math.min(conv.currentIteration + 1, conv.maxIterations)
			conv.updatedAt = now
			const edited: Workflow = {
				...normalizeWorkflow(workflow),
				conversationHistory: conv,
			}
			set({ current: edited, validation, dirty: true, aiEditing: false })
		} catch (err) {
			// Surface the failure inside the chat panel (chatError) rather than only
			// in the sidebar — previously the error was swallowed and the panel just
			// showed "Thinking…" forever with nothing after.
			set({ aiEditing: false, chatError: errMsg(err) })
		}
	},

	runWorkflow: async (args, model) => {
		const { current } = get()
		if (!current) return
		set({ running: true, error: null, runOutput: [], runId: null })
		try {
			// Track the run id in a closure variable so the WS handler can see it
			// the moment the POST returns — even before the component re-renders
			// with the updated store. Using get().runId here would race: a fast run
			// could emit output before the await resolves and runId is still null,
			// dropping every line.
			let runId = ''
			const ws = openWorkflowSocket((msg) => {
				if (msg.type === 'workflow_run_output') {
					const p = msg.payload as { runId?: string; stream?: string; text?: string }
					if (p.runId === runId) {
						set({
							runOutput: [
								...get().runOutput,
								{ stream: (p.stream as 'stdout' | 'stderr') ?? 'stdout', text: p.text ?? '', ts: Date.now() },
							],
						})
						// Also feed the commentary panel (classified narration feed).
						useCommentaryStore
							.getState()
							.pushLine((p.stream as 'stdout' | 'stderr') ?? 'stdout', p.text ?? '')
					}
				} else if (msg.type === 'workflow_run_done') {
					const p = msg.payload as { runId?: string; exitCode?: number; error?: string }
					if (p.runId === runId) {
						if (p.exitCode && p.exitCode !== 0) {
							set({ running: false, error: p.error || `run exited with code ${p.exitCode}` })
							useCommentaryStore.getState().pushLine('stderr', p.error || `run exited with code ${p.exitCode}`)
						} else {
							set({ running: false })
						}
						useCommentaryStore.getState().markDone()
						// Run is finished; close the stream socket.
						closeWorkflowSocket()
					}
				}
			})
			const res = await workflowApi.run(current.name, args, model)
			runId = res.runId
			set({ runId: res.runId })
			useCommentaryStore.getState().startRun(res.runId)
			// If the backend reports already-running, reflect it; the WS still
			// carries the active run's output.
			if (res.status === 'already-running') {
				set({ running: true })
			}
			// Keep a reference so a component unmount can also close it.
			activeRunWS = ws
		} catch (err) {
			set({ running: false, error: errMsg(err) })
		}
	},

	stopWorkflow: async () => {
		const { current } = get()
		if (!current || !get().running) return
		try {
			await workflowApi.stop(current.name)
			// The backend kills the process; the WS will deliver workflow_run_done
			// shortly and clear `running`. Set running=false optimistically so the
			// UI reacts instantly even if the done event is slow.
			set({ running: false })
			closeWorkflowSocket()
		} catch (err) {
			set({ error: errMsg(err) })
		}
	},

	setSlashCommandOptions: (opts) => {
		const { current } = get()
		if (!current) return
		snapshot()
		set({ current: { ...current, slashCommandOptions: opts ?? undefined }, dirty: true })
	},

	selectNode: (id) => set({ selectedNodeId: id }),

	setFocusNode: (id) => set({ focusNodeId: id }),

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
	clearChatError: () => set({ chatError: null }),
}))

export function disconnectEdgeId(c: WorkflowConnection): string {
	return edgeId(c)
}

// summarizeWorkflowChanges produces a short, deterministic diff of what the AI
// actually did to a workflow (added/removed/changed nodes, added/removed
// connections). It replaces the old behavior where the AI bubble just echoed
// the user's instruction back. Pure function — safe to unit-test in isolation.
//
// The message is deliberately compact and English (matching the rest of the
// chat panel copy) so it reads cleanly in a small bubble.
export function summarizeWorkflowChanges(before: Workflow, after: Workflow): string {
	const beforeNodes = new Map(before.nodes.map((n) => [n.id, n]))
	const afterNodes = new Map(after.nodes.map((n) => [n.id, n]))

	const added: string[] = []
	const removed: string[] = []
	let changed = 0
	for (const n of after.nodes) {
		if (!beforeNodes.has(n.id)) added.push(n.name || n.id)
	}
	for (const n of before.nodes) {
		const prev = afterNodes.get(n.id)
		if (!prev) {
			removed.push(n.name || n.id)
		} else if (prev.name !== n.name || JSON.stringify(prev.data) !== JSON.stringify(n.data)) {
			changed++
		}
	}

	const beforeConn = new Set(before.connections.map(connectionKey))
	const afterConn = new Set(after.connections.map(connectionKey))
	const connAdded = after.connections.filter((c) => !beforeConn.has(connectionKey(c))).length
	const connRemoved = before.connections.filter((c) => !afterConn.has(connectionKey(c))).length

	const lines: string[] = []
	const hasChanges =
		added.length > 0 || removed.length > 0 || changed > 0 || connAdded > 0 || connRemoved > 0
	if (!hasChanges) {
		return 'No changes — the workflow already matches.'
	}
	lines.push('Workflow updated.')
	if (added.length > 0) lines.push(`+ ${added.length} node${added.length > 1 ? 's' : ''}: ${added.join(', ')}`)
	if (removed.length > 0) lines.push(`− ${removed.length} node${removed.length > 1 ? 's' : ''}: ${removed.join(', ')}`)
	if (changed > 0) lines.push(`~ ${changed} node${changed > 1 ? 's' : ''} updated`)
	if (connAdded > 0) lines.push(`+ ${connAdded} connection${connAdded > 1 ? 's' : ''}`)
	if (connRemoved > 0) lines.push(`− ${connRemoved} connection${connRemoved > 1 ? 's' : ''}`)
	lines.push('Review the canvas, then Save.')
	return lines.join('\n')
}

// connectionKey is a stable identity for a connection ignoring object identity,
// used by the diff. Ports are optional; absent ports are normalized to ''.
function connectionKey(c: WorkflowConnection): string {
	return `${c.from}|${c.fromPort ?? ''}>${c.to}|${c.toPort ?? ''}`
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

// ─── workflow run WebSocket ────────────────────────────────────────────────
// A single on-demand socket for streaming run output. Unlike the missions
// useWebSocket hook, this does NOT auto-reconnect: a run is a bounded event and
// a dropped socket should surface an error rather than silently resync.

// activeRunWS holds the socket for the in-progress run so it can be closed on
// unmount or when the run completes.
let activeRunWS: WebSocket | null = null

export function closeWorkflowSocket(): void {
	if (activeRunWS) {
		activeRunWS.close()
		activeRunWS = null
	}
}

// openWorkflowSocket connects to the workflows hub and dispatches parsed events
// to onMessage. Returns the socket so the caller can close it.
function openWorkflowSocket(onMessage: (msg: { type: string; payload: unknown }) => void): WebSocket {
	const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
	const url = `${proto}//${window.location.host}/api/workflows/ws`
	const ws = new WebSocket(url)
	ws.onmessage = (event) => {
		try {
			const msg = JSON.parse(event.data) as { type: string; payload: unknown }
			onMessage(msg)
		} catch {
			// ignore malformed frames
		}
	}
	ws.onerror = () => {
		useWorkflowStore.setState({ running: false, error: 'lost connection to the run stream' })
	}
	ws.onclose = () => {
		if (activeRunWS === ws) activeRunWS = null
	}
	activeRunWS = ws
	return ws
}
