import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
	ReactFlow,
	ReactFlowProvider,
	Background,
	BackgroundVariant,
	Controls,
	MiniMap,
	useReactFlow,
	applyNodeChanges as applyRfNodeChanges,
	type Node,
	type Edge,
	type Connection,
	type NodeChange,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import {
	Plus,
	Save,
	Trash2,
	Upload,
	CheckCircle2,
	AlertTriangle,
	Network,
	X,
	FileCode2,
	RefreshCw,
	LayoutGrid,
	Undo2,
	Redo2,
	Sparkles,
	Zap,
	Grid3x3,
	Mouse,
	Highlighter,
	Map as MapIcon,
	Play,
	Terminal,
	Settings2,
	Plug,
	HelpCircle,
	MessageSquare,
	Square,
	Pencil,
} from 'lucide-react'
import { useWorkflowStore, disconnectEdgeId } from '../../stores/workflowStore'
import type { WorkflowConnection, WorkflowNodeType } from '../../api/types'
import Modal from '../shared/Modal'
import YdSelect from '../shared/YdSelect'
import MermaidDiagram from '../shared/MermaidDiagram'
import { NODE_META, nodeTypes, toFlowNode } from './nodes'
import NodeDetail, { useOpencodeModels } from './NodeDetail'
import NodeFocusModal from './NodeFocusModal'
import SlashCommandOptionsModal from './SlashCommandOptionsModal'
import McpSyncModal from './McpSyncModal'
import Tour from './Tour'
import CommentaryPanel from './CommentaryPanel'
import RefinementChatPanel from './RefinementChatPanel'
import './WorkflowEditor.css'

// Export targets shown as per-runtime buttons in the toolbar.
const EXPORT_TARGETS = [
	{ value: 'opencode', label: 'opencode', dir: '~/.config/opencode' },
	{ value: 'claude-code', label: 'Claude Code', dir: '~/.claude' },
] as const

const PALETTE_TYPES: WorkflowNodeType[] = [
	'subAgent',
	'subAgentFlow',
	'askUserQuestion',
	'prompt',
	'ifElse',
	'switch',
	'skill',
	'mcp',
	'group',
]

export default function WorkflowEditor() {
	return (
		<ReactFlowProvider>
			<WorkflowEditorInner />
		</ReactFlowProvider>
	)
}

function WorkflowEditorInner() {
	const [mounted, setMounted] = useState(false)
	useEffect(() => setMounted(true), [])

	// store selectors
	const summaries = useWorkflowStore((s) => s.summaries)
	const current = useWorkflowStore((s) => s.current)
	const loading = useWorkflowStore((s) => s.loading)
	const dirty = useWorkflowStore((s) => s.dirty)
	const error = useWorkflowStore((s) => s.error)
	const validation = useWorkflowStore((s) => s.validation)
	const exportPlan = useWorkflowStore((s) => s.exportPlan)
	const exporting = useWorkflowStore((s) => s.exporting)
	const selectedNodeId = useWorkflowStore((s) => s.selectedNodeId)

	const list = useWorkflowStore((s) => s.list)
	const load = useWorkflowStore((s) => s.load)
	const createNew = useWorkflowStore((s) => s.createNew)
	const saveCurrent = useWorkflowStore((s) => s.saveCurrent)
	const deleteCurrent = useWorkflowStore((s) => s.deleteCurrent)
	const renameCurrent = useWorkflowStore((s) => s.renameCurrent)
	const validateCurrent = useWorkflowStore((s) => s.validateCurrent)
	const exportCurrent = useWorkflowStore((s) => s.exportCurrent)
	const clearExport = useWorkflowStore((s) => s.clearExport)
	const clearError = useWorkflowStore((s) => s.clearError)
	const selectNode = useWorkflowStore((s) => s.selectNode)
	const focusNodeId = useWorkflowStore((s) => s.focusNodeId)
	const setFocusNode = useWorkflowStore((s) => s.setFocusNode)
	const connect = useWorkflowStore((s) => s.connect)
	const disconnect = useWorkflowStore((s) => s.disconnect)
	const applyNodeChanges = useWorkflowStore((s) => s.applyNodeChanges)
	const autoLayout = useWorkflowStore((s) => s.autoLayout)
	const runWorkflow = useWorkflowStore((s) => s.runWorkflow)
	const stopWorkflow = useWorkflowStore((s) => s.stopWorkflow)
	const running = useWorkflowStore((s) => s.running)
	const runOutput = useWorkflowStore((s) => s.runOutput)
	const setSlashCommandOptions = useWorkflowStore((s) => s.setSlashCommandOptions)
	const undo = useWorkflowStore((s) => s.undo)
	const redo = useWorkflowStore((s) => s.redo)
	const setNodeParent = useWorkflowStore((s) => s.setNodeParent)
	const canUndo = useWorkflowStore((s) => s.past.length > 0)
	const canRedo = useWorkflowStore((s) => s.future.length > 0)
	const { fitView, getIntersectingNodes, getInternalNode } = useReactFlow()

	// local UI state
	const [selectedName, setSelectedName] = useState('')
	const [newOpen, setNewOpen] = useState(false)
	const [newName, setNewName] = useState('')
	const [newDesc, setNewDesc] = useState('')
	const [importOpen, setImportOpen] = useState(false)
	const [importText, setImportText] = useState('')
	const [mermaidPreview, setMermaidPreview] = useState<string | null>(null)
	const [mermaidLoading, setMermaidLoading] = useState(false)
	const [aiModel, setAiModel] = useState('')
	const aiModels = useOpencodeModels()
	const [exportTarget, setExportTarget] = useState('opencode')
	// Run modal: args prompt before spawning the orchestrator.
	const [runOpen, setRunOpen] = useState(false)
	const [runArgs, setRunArgs] = useState('')
	// Run output panel visibility.
	const [runPanelOpen, setRunPanelOpen] = useState(false)
	// Slash command options modal (workflow-level frontmatter).
	const [slashOpen, setSlashOpen] = useState(false)
	// MCP sync modal (opencode → claude-code).
	const [mcpSyncOpen, setMcpSyncOpen] = useState(false)
	// Rename modal.
	const [renameOpen, setRenameOpen] = useState(false)
	const [renameValue, setRenameValue] = useState('')
	// Onboarding tour (driver.js). forceRun re-triggers it from the Help button.
	const [tourRun, setTourRun] = useState(0)
	const startTour = () => setTourRun((n) => n + 1)
	// Commentary panel (live run feed) toggle.
	const [commentaryOpen, setCommentaryOpen] = useState(false)
	// Refinement chat panel (multi-turn Edit-with-AI) toggle.
	const [chatOpen, setChatOpen] = useState(false)
	// Canvas view options.
	const [animatedEdges, setAnimatedEdges] = useState(false)
	const [snapGrid, setSnapGrid] = useState(false)
	const [scrollPan, setScrollPan] = useState(false)
	const [highlight, setHighlight] = useState(false)
	const [showMinimap, setShowMinimap] = useState(true)
	const importRaw = useWorkflowStore((s) => s.importRaw)
	const fileInput = useRef<HTMLInputElement>(null)
	// Node id from a deep link (?node=), applied once the workflow has loaded.
	const pendingNode = useRef<string | null>(null)

	useEffect(() => {
		if (mounted) list()
	}, [mounted, list])

	// Deep link: on mount, honor ?wf=<name>&node=<id> from the URL.
	useEffect(() => {
		const sp = new URLSearchParams(window.location.search)
		const wf = sp.get('wf')
		pendingNode.current = sp.get('node')
		if (wf) {
			setSelectedName(wf)
			load(wf)
		}
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [])

	// auto-load first workflow when the list arrives and nothing is selected.
	useEffect(() => {
		if (mounted && summaries.length > 0 && !selectedName && !current) {
			setSelectedName(summaries[0].name)
			load(summaries[0].name)
		}
	}, [mounted, summaries, selectedName, current, load])

	// Once the deep-linked workflow is loaded, select its node (one-shot).
	useEffect(() => {
		if (pendingNode.current && current?.name === selectedName) {
			if (current.nodes.some((n) => n.id === pendingNode.current)) selectNode(pendingNode.current)
			pendingNode.current = null
		}
	}, [current, selectedName, selectNode])

	// Reflect the selected workflow + node in the URL (shareable deep link).
	useEffect(() => {
		const sp = new URLSearchParams(window.location.search)
		selectedName ? sp.set('wf', selectedName) : sp.delete('wf')
		selectedNodeId ? sp.set('node', selectedNodeId) : sp.delete('node')
		const qs = sp.toString()
		window.history.replaceState(null, '', `${window.location.pathname}${qs ? `?${qs}` : ''}`)
	}, [selectedName, selectedNodeId])

	const onSelect = useCallback(
		(name: string) => {
			if (!name) return
			setSelectedName(name)
			load(name)
		},
		[load],
	)

	const handleCreate = useCallback(async () => {
		if (!newName.trim()) return
		await createNew(newName.trim(), newDesc.trim() || undefined)
		setSelectedName(newName.trim())
		setNewOpen(false)
		setNewName('')
		setNewDesc('')
	}, [createNew, newName, newDesc])

	const handleImport = useCallback(async () => {
		try {
			const raw = JSON.parse(importText)
			await importRaw(raw)
			setImportOpen(false)
			setImportText('')
		} catch {
			// store will surface the error via the API call path
		}
	}, [importRaw, importText])

	const handleFileImport = useCallback(
		(file: File) => {
			const reader = new FileReader()
			reader.onload = () => {
				setImportText(String(reader.result ?? ''))
				setImportOpen(true)
			}
			reader.readAsText(file)
		},
		[],
	)

	const mermaid = useMemo(() => buildMermaidPreview(current), [current])

	// ─── xyflow nodes/edges ────────────────────────────────────────────────
	// The canvas owns its own node state so React Flow can move nodes (drag,
	// dimensions). It is resynced from the store whenever the workflow itself
	// changes (load / add / import / delete), but NOT on every store tick —
	// otherwise dragging would fight the store and snap back.
	const [flowNodes, setFlowNodes] = useState<Node[]>([])

	// Serial of the underlying workflow nodes, to detect when we must resync
	// (different nodes/edges, not just moved positions).
	// Includes node data (not position) so field edits in the detail panel — chips,
	// subtitles, group size — repaint the canvas live. Position is excluded so a
	// drag in progress (persisted only on drag-end) doesn't fight the store.
	const nodeSignature = useMemo(
		() => current?.nodes.map((n) => n.id + n.type + (n.parentId ?? '') + JSON.stringify(n.data)).join('|') ?? '',
		[current],
	)

	useEffect(() => {
		if (!current) {
			setFlowNodes([])
			return
		}
		setFlowNodes(current.nodes.map((n) => ({ ...toFlowNode(n), selected: n.id === selectedNodeId })))
		// Only resync on structural changes, not on selection/position ticks.
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [nodeSignature])

	// Keep the `selected` flag in sync with selection without resyncing positions.
	useEffect(() => {
		setFlowNodes((prev) => prev.map((n) => ({ ...n, selected: n.id === selectedNodeId })))
	}, [selectedNodeId])

	// Fit the view to ALL nodes whenever a different workflow loads, so wide
	// graphs (e.g. grouped PLANNING/IMPLEMENTATION/TESTING) aren't cut off at the
	// edge. Keyed by name so it doesn't refit on every edit.
	const loadedName = current?.name
	useEffect(() => {
		if (!loadedName) return
		const id = requestAnimationFrame(() => fitView({ padding: 0.15, maxZoom: 1, duration: 250 }))
		return () => cancelAnimationFrame(id)
	}, [loadedName, fitView])

	const flowEdges: Edge[] = useMemo(() => {
		if (!current) return []
		// Highlight: when a node is selected and highlighting is on, dim every
		// edge not touching it so the selected node's wiring stands out.
		const dim = highlight && !!selectedNodeId
		// Dedupe by edge id: two connections with the same (from/port -> to/port)
		// would render two edges with one React key. This can happen when a
		// workflow saved with duplicates (or produced by import/AI) is loaded; the
		// store also dedupes on normalize, but this keeps the canvas safe even for
		// an in-flight unnormalized state.
		const seen = new Set<string>()
		const edges: Edge[] = []
		for (const c of current.connections) {
			const id = disconnectEdgeId(c as WorkflowConnection)
			if (seen.has(id)) continue
			seen.add(id)
			const touches = c.from === selectedNodeId || c.to === selectedNodeId
			edges.push({
				id,
				source: c.from,
				target: c.to,
				sourceHandle: c.fromPort || undefined,
				targetHandle: c.toPort || undefined,
				animated: animatedEdges,
				// Dashed curved connectors.
				style: { strokeDasharray: '6 4', strokeWidth: 1.5, opacity: dim && !touches ? 0.12 : 1 },
				markerEnd: { type: 'arrowclosed' as const },
			})
		}
		return edges
	}, [current, animatedEdges, highlight, selectedNodeId])

	const onConnect = useCallback(
		(params: Connection) => {
			if (!params.source || !params.target) return
			connect({
				from: params.source,
				to: params.target,
				fromPort: params.sourceHandle ?? undefined,
				toPort: params.targetHandle ?? undefined,
			})
		},
		[connect],
	)

	// Apply React Flow node changes to the local canvas state (so dragging is
	// fluid) and mirror position changes into the store for persistence.
	const onNodesChange = useCallback(
		(changes: NodeChange[]) => {
			setFlowNodes((prev) => applyRfNodeChanges(changes, prev))
			// Only persist final position changes (drag end) to avoid thrashing
			// the store on every pointermove.
			const moved = changes.filter(
				(c): c is Extract<NodeChange, { type: 'position' }> =>
					c.type === 'position' && c.dragging === false,
			)
			if (moved.length) applyNodeChanges(moved)
		},
		[applyNodeChanges],
	)

	// drag-drop from palette
	const onDragStart = useCallback((e: React.DragEvent, type: WorkflowNodeType) => {
		e.dataTransfer.setData('application/workflow-node-type', type)
		e.dataTransfer.effectAllowed = 'move'
	}, [])

	const onDrop = useCallback(
		(e: React.DragEvent) => {
			e.preventDefault()
			const type = e.dataTransfer.getData('application/workflow-node-type') as WorkflowNodeType
			if (!type) return
			// Approximate drop position relative to canvas center.
			const bounds = (e.currentTarget as HTMLElement).getBoundingClientRect()
			const x = e.clientX - bounds.left - 80
			const y = e.clientY - bounds.top - 30
			useWorkflowStore.getState().addNode(type, x, y)
		},
		[],
	)

	// Auto-layout recomputes store positions, then pushes them into the canvas
	// (which owns its own node state and won't resync on position-only changes)
	// and fits the view with a readable zoom cap.
	const handleAutoLayout = useCallback(() => {
		autoLayout()
		const laid = useWorkflowStore.getState().current
		if (laid) setFlowNodes(laid.nodes.map((n) => ({ ...toFlowNode(n), selected: n.id === selectedNodeId })))
		requestAnimationFrame(() => fitView({ padding: 0.2, maxZoom: 1, duration: 300 }))
	}, [autoLayout, fitView, selectedNodeId])

	// Undo/redo swap `current`; since the canvas owns its node state and only
	// resyncs on structural (id/type) changes, push the restored nodes in
	// explicitly so position-only undo steps also repaint.
	const resyncCanvas = useCallback(() => {
		const wf = useWorkflowStore.getState().current
		if (wf) setFlowNodes(wf.nodes.map((n) => ({ ...toFlowNode(n), selected: n.id === selectedNodeId })))
	}, [selectedNodeId])

	// On drag end, (re-)assign the node to whatever group it was dropped into —
	// real grouping: a member is confined to and moves with its group. Positions
	// are converted to/from the group's coordinate space.
	const handleNodeDragStop = useCallback(
		(_e: MouseEvent | TouchEvent, node: Node) => {
			if ((node.data as { __type?: string })?.__type === 'group') return // don't nest groups
			const groups = getIntersectingNodes(node).filter(
				(n) => (n.data as { __type?: string })?.__type === 'group',
			)
			const target = groups[0]
			const currentParent = node.parentId ?? undefined
			if (target?.id === currentParent) return

			const abs = getInternalNode(node.id)?.internals.positionAbsolute ?? node.position
			if (target) {
				const gAbs = getInternalNode(target.id)?.internals.positionAbsolute ?? target.position
				setNodeParent(node.id, target.id, { x: abs.x - gAbs.x, y: abs.y - gAbs.y })
			} else {
				setNodeParent(node.id, null, { x: abs.x, y: abs.y })
			}
			resyncCanvas()
		},
		[getIntersectingNodes, getInternalNode, setNodeParent, resyncCanvas],
	)

	const handleUndo = useCallback(() => {
		undo()
		resyncCanvas()
	}, [undo, resyncCanvas])

	const handleRedo = useCallback(() => {
		redo()
		resyncCanvas()
	}, [redo, resyncCanvas])

	// Keyboard: Ctrl/Cmd+Z undo, Ctrl/Cmd+Shift+Z (or Ctrl+Y) redo. Ignored
	// while typing in a field so text-edit undo stays native to the input.
	useEffect(() => {
		const onKey = (e: KeyboardEvent) => {
			const mod = e.ctrlKey || e.metaKey
			if (!mod || e.key.toLowerCase() !== 'z' && e.key.toLowerCase() !== 'y') return
			const t = e.target as HTMLElement | null
			if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return
			e.preventDefault()
			if (e.key.toLowerCase() === 'y' || e.shiftKey) handleRedo()
			else handleUndo()
		}
		window.addEventListener('keydown', onKey)
		return () => window.removeEventListener('keydown', onKey)
	}, [handleUndo, handleRedo])

	const handleRun = useCallback(async () => {
		setRunOpen(false)
		setRunPanelOpen(true)
		await runWorkflow(runArgs, aiModel || undefined)
	}, [runWorkflow, runArgs, aiModel])

	const handleRename = useCallback(async () => {
		const v = renameValue.trim()
		if (!v || v === current?.name) {
			setRenameOpen(false)
			return
		}
		await renameCurrent(v)
		if (!useWorkflowStore.getState().error) {
			setSelectedName(v)
			setRenameOpen(false)
			setRenameValue('')
		}
	}, [renameCurrent, renameValue, current])

	const showMermaid = useCallback(async () => {
		setMermaidLoading(true)
		// The Mermaid is derived client-side (buildMermaidPreview); no round-trip.
		setMermaidPreview(mermaid)
		setMermaidLoading(false)
	}, [mermaid])

	return (
		<div className="workflow-page">
			{/* Onboarding tour (driver.js). Auto-runs once; forceRun via Help button. */}
			<Tour key={tourRun} forceRun={tourRun > 0} />

			{/* Toolbar — grouped by intent; one primary (Save), secondary actions
			    icon-only with glass tooltips (data-tip + aria-label), destructive
			    action isolated on the right. */}
			<div className="workflow-toolbar">
				{/* Workflow selector */}
					<div data-tour="workflow-select">
						<YdSelect
							className="workflow-select"
							options={summaries.map((s) => ({
								value: s.name,
								label: `${s.name} (${s.nodeCount} nodes)`,
							}))}
							value={selectedName}
							onChange={(v) => onSelect(v)}
							placeholder="— select workflow —"
							ariaLabel="Select workflow"
						/>
					</div>

				<span className="wf-tb-sep" />

				{/* File: new + import (icon-only) */}
				<div className="wf-tb-group">
					<button
						className="btn btn-icon"
						onClick={() => setNewOpen(true)}
						data-tip="New workflow"
						aria-label="New workflow"
					>
						<Plus />
					</button>
					<button
						className="btn btn-icon"
						onClick={() => fileInput.current?.click()}
						data-tip="Import a workflow.json"
						aria-label="Import workflow JSON"
					>
						<Upload />
					</button>
					<input
						ref={fileInput}
						type="file"
						accept="application/json,.json"
						style={{ display: 'none' }}
						onChange={(e) => {
							const f = e.target.files?.[0]
							if (f) handleFileImport(f)
							e.target.value = ''
						}}
					/>
				</div>

				<span className="wf-tb-sep" />

				{/* Primary action */}
				<button
					className="btn btn-primary"
					onClick={() => saveCurrent()}
					disabled={!current || !dirty}
					data-tip={current && dirty ? 'Save changes' : ''}
					aria-label="Save workflow"
				>
					<Save /> Save {dirty ? '*' : ''}
				</button>

				<span className="wf-tb-sep" />

				{/* Editing: validate · undo · redo · auto-layout (icon-only) */}
				<div className="wf-tb-group">
					<button
						className="btn btn-icon"
						onClick={() => validateCurrent()}
						disabled={!current}
						data-tip={current ? 'Validate the workflow graph' : ''}
						aria-label="Validate workflow"
					>
						<CheckCircle2 />
					</button>
					<button
						className="btn btn-icon"
						onClick={handleUndo}
						disabled={!canUndo}
						data-tip={canUndo ? 'Undo (Ctrl+Z)' : ''}
						aria-label="Undo"
					>
						<Undo2 />
					</button>
					<button
						className="btn btn-icon"
						onClick={handleRedo}
						disabled={!canRedo}
						data-tip={canRedo ? 'Redo (Ctrl+Shift+Z)' : ''}
						aria-label="Redo"
					>
						<Redo2 />
					</button>
					<button
						className="btn btn-icon"
						onClick={handleAutoLayout}
						disabled={!current}
						data-tip={current ? 'Auto-arrange nodes (left-to-right)' : ''}
						aria-label="Auto-layout"
					>
						<LayoutGrid />
					</button>
				</div>

				<span className="wf-tb-sep" />

				{/* Export: target matters, keep the label */}
					<div className="wf-tb-group" data-tour="export">
						<span className="wf-export-label">Export</span>
					{EXPORT_TARGETS.map((t) => (
						<button
							key={t.value}
							className="btn btn-sm"
							onClick={() => {
								setExportTarget(t.value)
								exportCurrent(false, t.value)
							}}
							disabled={!current || exporting}
							data-tip={current && !exporting ? `Export to ${t.label} (${t.dir}) — preview, then apply` : ''}
							aria-label={`Export to ${t.label}`}
						>
							<FileCode2 /> {t.label}
						</button>
					))}
				</div>

				<span className="wf-tb-sep" />

			{/* Extend: AI edit + Run + Mermaid (icon-only) */}
			<div className="wf-tb-group">
				<button
					className={`btn btn-icon ${chatOpen ? 'is-active' : ''}`}
					onClick={() => {
						setChatOpen((v) => !v)
						setCommentaryOpen(false)
					}}
					disabled={!current}
					data-tip={current ? 'Edit with AI (multi-turn chat)' : ''}
					aria-label="Edit with AI"
					data-tour="ai-refine-button"
				>
					<Sparkles />
				</button>
				<button
					className="btn btn-icon"
					onClick={() => setSlashOpen(true)}
					disabled={!current}
					data-tip={current ? 'Slash command options (allowed-tools, model, hooks)' : ''}
					aria-label="Slash command options"
				>
					<Settings2 />
				</button>
				<button
					className="btn btn-icon"
					onClick={() => setMcpSyncOpen(true)}
					disabled={!current}
					data-tip={current ? 'Sync MCP servers to Claude Code' : ''}
					aria-label="Sync MCP servers"
				>
					<Plug />
				</button>
				{running ? (
					<button
						className="btn btn-icon danger is-running"
						onClick={() => stopWorkflow()}
						data-tip="Stop the running workflow"
						aria-label="Stop workflow"
					>
						<Square />
					</button>
				) : (
					<button
						className="btn btn-icon"
						onClick={() => setRunOpen(true)}
						disabled={!current}
						data-tip={current ? 'Run the workflow' : ''}
						aria-label="Run workflow"
						data-tour="run-button"
					>
						<Play />
					</button>
				)}
				<button
					className={`btn btn-icon ${runOutput.length > 0 ? 'has-output' : ''}`}
					onClick={() => setRunPanelOpen((v) => !v)}
					disabled={runOutput.length === 0 && !running}
					data-tip={runOutput.length > 0 || running ? 'Toggle the run output panel' : ''}
					aria-label="Run output"
				>
					<Terminal />
				</button>
				<button
					className={`btn btn-icon ${commentaryOpen ? 'is-active' : ''}`}
					onClick={() => {
						setCommentaryOpen((v) => !v)
						setChatOpen(false)
					}}
					disabled={runOutput.length === 0 && !running}
					data-tip={runOutput.length > 0 || running ? 'Toggle the commentary feed' : ''}
					aria-label="Commentary"
				>
					<MessageSquare />
				</button>
					<button
						className="btn btn-icon"
						onClick={() => showMermaid()}
						disabled={!current}
						data-tip={current ? 'Show the Mermaid diagram' : ''}
						aria-label="Mermaid diagram"
					>
						<Network />
					</button>
				</div>

				<span className="spacer" />

				{/* Help — re-runs the onboarding tour. */}
				<button
					className="btn btn-icon"
					onClick={startTour}
					data-tip="Show the guided tour"
					aria-label="Show tour"
				>
					<HelpCircle />
				</button>

				{/* Rename — next to Delete, both workflow-level ops */}
				<button
					className="btn btn-icon"
					onClick={() => {
						setRenameValue(current?.name ?? '')
						setRenameOpen(true)
					}}
					disabled={!current}
					data-tip={current ? 'Rename workflow' : ''}
					aria-label="Rename workflow"
				>
					<Pencil />
				</button>

				{/* Destructive — isolated */}
				<button
					className="btn btn-icon danger"
					onClick={() => deleteCurrent()}
					disabled={!current}
					data-tip={current ? 'Delete workflow' : ''}
					aria-label="Delete workflow"
				>
					<Trash2 />
				</button>
			</div>

			{/* error */}
			{error && (
				<div className="validation-issue error">
					<AlertTriangle size={14} />
					<span>{error}</span>
					<button className="btn btn-icon" onClick={clearError}>
						<X size={12} />
					</button>
				</div>
			)}

			{/* validation summary */}
			{validation && (
				<div className="validation-list">
					{!validation.valid && (validation.errors?.length ?? 0) === 0 && (
						<div className="validation-issue error">
							<AlertTriangle size={14} /> Workflow has errors.
						</div>
					)}
					{(validation.errors ?? []).map((iss, i) => (
						<div className="validation-issue error" key={`e${i}`}>
							<AlertTriangle size={14} />
							<span>{iss.nodeId ? `[${iss.nodeId}] ` : ''}{iss.message}</span>
						</div>
					))}
					{(validation.warnings ?? []).map((iss, i) => (
						<div className="validation-issue warning" key={`w${i}`}>
							<AlertTriangle size={14} />
							<span>{iss.nodeId ? `[${iss.nodeId}] ` : ''}{iss.message}</span>
						</div>
					))}
					{validation.valid && (validation.warnings?.length ?? 0) === 0 && (
						<div className="validation-issue" style={{ color: 'var(--success)' }}>
							<CheckCircle2 size={14} /> Workflow is valid.
						</div>
					)}
				</div>
			)}

			{loading && <div className="empty">Loading…</div>}

			{/* Body: palette + canvas + detail */}
			{current && (
				<div className="workflow-body">
					<aside className="workflow-palette" data-tour="palette">
						<h4>Node Palette</h4>
						{PALETTE_TYPES.map((t) => {
							const Icon = NODE_META[t].icon
							return (
								<div
									key={t}
									className={`palette-item palette-${t}`}
									draggable
									onDragStart={(e) => onDragStart(e, t)}
									onDoubleClick={() =>
										useWorkflowStore.getState().addNode(t, 300 + Math.random() * 200, 150 + Math.random() * 200)
									}
									title={`Double-click or drag to add a ${NODE_META[t].kind} node`}
								>
									<span className="palette-icon">
										<Icon size={16} />
									</span>
									<span className="palette-text">
										<span className="palette-kind">{NODE_META[t].kind}</span>
										<span className="palette-desc">{NODE_META[t].desc}</span>
									</span>
								</div>
							)
						})}
					</aside>

					<div
						className="workflow-canvas-wrap"
						onDrop={onDrop}
						onDragOver={(e) => e.preventDefault()}
					>
						<div className="wf-canvas-tools">
							{([
								['Animated edges', Zap, animatedEdges, () => setAnimatedEdges((v) => !v)],
								['Snap to grid', Grid3x3, snapGrid, () => setSnapGrid((v) => !v)],
								['Scroll to pan', Mouse, scrollPan, () => setScrollPan((v) => !v)],
								['Highlight connections', Highlighter, highlight, () => setHighlight((v) => !v)],
								['Minimap', MapIcon, showMinimap, () => setShowMinimap((v) => !v)],
							] as const).map(([label, Icon, active, toggle]) => (
								<button
									key={label}
									className={`btn btn-icon${active ? ' is-active' : ''}`}
									aria-pressed={active}
									title={label}
									onClick={toggle}
								>
									<Icon size={14} />
								</button>
							))}
						</div>
						<ReactFlow data-tour="canvas"
							nodes={flowNodes}
							edges={flowEdges}
							nodeTypes={nodeTypes}
							onNodesChange={onNodesChange}
							onNodeDragStop={handleNodeDragStop}
							onConnect={onConnect}
							onEdgeClick={(_, edge) => disconnect(edge.id)}
							onNodeClick={(_, n) => selectNode(n.id)}
							onNodeDoubleClick={(_, n) => {
								// Double-clicking a sub-workflow node opens that workflow; any other
								// node opens the Monaco focus editor.
								const d = n.data as { __type?: string; flowId?: string }
								if (d.__type === 'subAgentFlow' && d.flowId) onSelect(d.flowId)
								else setFocusNode(n.id)
							}}
							onPaneClick={() => selectNode(null)}
							fitView
							snapToGrid={snapGrid}
							snapGrid={[16, 16]}
							panOnScroll={scrollPan}
							proOptions={{ hideAttribution: true }}
						>
							<Background variant={BackgroundVariant.Dots} gap={16} size={1} />
							<Controls />
							{showMinimap && <MiniMap pannable zoomable />}
						</ReactFlow>
					</div>

					<NodeDetail />
				</div>
			)}

			{/* New workflow modal */}
			<Modal open={newOpen} onClose={() => setNewOpen(false)} title="New workflow">
				<div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
					<div className="field">
						<label className="field-label">Name (lowercase, [a-z0-9_-])</label>
						<input
							className="input"
							value={newName}
							onChange={(e) => setNewName(e.target.value)}
							placeholder="e.g. daily-task"
							autoFocus
						/>
					</div>
					<div className="field">
						<label className="field-label">Description</label>
						<input className="input" value={newDesc} onChange={(e) => setNewDesc(e.target.value)} />
					</div>
				</div>
				<div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
					<button className="btn" onClick={() => setNewOpen(false)}>Cancel</button>
					<button className="btn btn-primary" onClick={handleCreate} disabled={!newName.trim()}>
						Create
					</button>
				</div>
			</Modal>

			{/* Import modal */}
			<Modal open={importOpen} onClose={() => setImportOpen(false)} title="Import workflow JSON" width="640px">
				<div className="field">
					<label className="field-label">Paste a workflow.json below</label>
					<textarea
						className="textarea mono"
						value={importText}
						onChange={(e) => setImportText(e.target.value)}
						style={{ minHeight: 260 }}
						placeholder='{ "name": "...", "nodes": [...] }'
					/>
				</div>
				<div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
					<button className="btn" onClick={() => setImportOpen(false)}>Cancel</button>
					<button className="btn btn-primary" onClick={handleImport} disabled={!importText.trim()}>
						Import
					</button>
				</div>
			</Modal>

			{/* Export plan modal */}
			<Modal open={!!exportPlan} onClose={clearExport} title={`Export plan — ${exportPlan?.workflowName ?? ''}`} width="640px">
				{exportPlan && (
					<div className="export-plan">
						<p>
							{exportPlan.dryRun
								? `Dry-run preview. These files would be written for ${exportTarget === 'claude-code' ? 'Claude Code (~/.claude)' : 'opencode (~/.config/opencode)'}:`
								: `✅ Files written. Restart ${exportTarget === 'claude-code' ? 'Claude Code' : 'opencode'} to pick them up.`}
						</p>
						{exportPlan.dryRun && exportPlan.estimatedTokens > 0 && (
							<p className="wf-token-hint">
								Estimated orchestrator prompt size: <strong>≈ {exportPlan.estimatedTokens.toLocaleString()} tokens</strong>
								{exportPlan.estimatedTokens > 30000 && (
									<span className="wf-token-warn"> — large; consider splitting the workflow.</span>
								)}
							</p>
						)}
						{(exportPlan.files ?? []).map((f, i) => (
							<div className={`artifact kind-${f.kind}`} key={i}>
								<span className="kind">{f.kind}</span>
								<span>{f.path.replace(/^.*\.config\/opencode/, '~/.config/opencode').replace(/^.*\.claude/, '~/.claude')}</span>
							</div>
						))}
						<div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
							<button className="btn" onClick={clearExport}>Close</button>
							<button
								className="btn btn-primary"
								onClick={() => exportCurrent(true, exportTarget)}
								disabled={exporting}
							>
								<RefreshCw size={14} /> {exportPlan.dryRun ? 'Apply (write files)' : 'Re-export'}
							</button>
						</div>
					</div>
				)}
			</Modal>

			{/* Mermaid preview modal */}
			<Modal open={mermaidPreview !== null} onClose={() => setMermaidPreview(null)} title="Mermaid diagram" width="92vw">
				{mermaidLoading ? (
					<div className="empty">Generating…</div>
				) : (
					<MermaidDiagram code={mermaidPreview ?? ''} />
				)}
			</Modal>

			{/* MCP sync modal (opencode → claude-code). */}
			<McpSyncModal
				open={mcpSyncOpen}
				workflowName={current?.name ?? ''}
				onClose={() => setMcpSyncOpen(false)}
			/>

			{/* Slash command options modal (workflow-level frontmatter). */}
			<SlashCommandOptionsModal
				open={slashOpen}
				options={current?.slashCommandOptions}
				onClose={() => setSlashOpen(false)}
				onSave={(opts) => {
					setSlashCommandOptions(opts)
					setSlashOpen(false)
				}}
			/>

			{/* Run args modal — prompts for the task before spawning. */}
			<Modal open={runOpen} onClose={() => setRunOpen(false)} title={`Run "${current?.name ?? ''}"`} width="520px">
				<div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
					<p style={{ fontSize: 13, color: 'var(--text-muted)', margin: 0 }}>
						Describe what you want the workflow to do. The agents will work through the steps and you'll see their output live.
					</p>
					<div className="field">
						<label className="field-label">What should it do?</label>
						<input
							className="input"
							value={runArgs}
							onChange={(e) => setRunArgs(e.target.value)}
							placeholder="e.g. Build a login form with validation"
							autoFocus
						/>
						<span className="field-help">Leave empty to run with the workflow's default task.</span>
					</div>
					<div className="field">
						<label className="field-label">Model (optional)</label>
						<YdSelect
							options={[
								{ value: '', label: 'Default' },
								...aiModels.map((m) => ({ value: m.id, label: `${m.provider}/${m.name}` })),
							]}
							value={aiModel}
							onChange={setAiModel}
							ariaLabel="Model"
						/>
					</div>
				</div>
				<div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 12 }}>
					<button className="btn" onClick={() => setRunOpen(false)}>Cancel</button>
					<button className="btn btn-primary" onClick={handleRun}>
						<Play size={14} /> Run
					</button>
				</div>
			</Modal>

			{/* Run output panel — live stream of the orchestrator's output. */}
			{runPanelOpen && (runOutput.length > 0 || running) && (
				<RunOutputPanel
					lines={runOutput}
					running={running}
					onClose={() => setRunPanelOpen(false)}
				/>
			)}

			{/* Commentary panel — classified narration feed of the run. */}
			{commentaryOpen && (runOutput.length > 0 || running) && (
				<CommentaryPanel onClose={() => setCommentaryOpen(false)} />
			)}

			{/* Refinement chat panel — multi-turn Edit-with-AI. */}
			{chatOpen && current && (
				<RefinementChatPanel onClose={() => setChatOpen(false)} />
			)}

			{/* Rename modal */}
			<Modal open={renameOpen} onClose={() => setRenameOpen(false)} title="Rename workflow" width="440px">
				<div className="field">
					<label className="field-label">New name</label>
					<input
						className="input mono"
						value={renameValue}
						onChange={(e) => setRenameValue(e.target.value)}
						placeholder="my-workflow-name"
						autoFocus
						onKeyDown={(e) => {
							if (e.key === 'Enter') handleRename()
						}}
					/>
					<span className="field-help">Lowercase letters, digits, hyphens. Used as the slash command name.</span>
				</div>
				<div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 12 }}>
					<button className="btn" onClick={() => setRenameOpen(false)}>Cancel</button>
					<button
						className="btn btn-primary"
						onClick={handleRename}
						disabled={!renameValue.trim() || renameValue.trim() === current?.name}
					>
						<Pencil size={14} /> Rename
					</button>
				</div>
			</Modal>

			{/* Focus mode — Monaco editor for the node's long fields */}
			<NodeFocusModal nodeId={focusNodeId} onClose={() => setFocusNode(null)} />
		</div>
	)
}

// RunOutputPanel is the live output console for a workflow run. Renders the
// streamed stdout/stderr lines with simple stream coloring and a running badge.
function RunOutputPanel({
	lines,
	running,
	onClose,
}: {
	lines: { stream: string; text: string; ts: number }[]
	running: boolean
	onClose: () => void
}) {
	const endRef = useRef<HTMLDivElement>(null)
	useEffect(() => {
		endRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' })
	}, [lines.length])
	return (
		<div className="wf-run-panel" data-tour="run-output">
			<div className="wf-run-panel-header">
				<span className="wf-run-panel-title">
					<Terminal size={14} /> Run output
					{running && <span className="wf-run-live">● live</span>}
				</span>
				<button className="btn btn-icon" onClick={onClose} aria-label="Close output panel">
					<X size={14} />
				</button>
			</div>
			<div className="wf-run-panel-body">
				{lines.length === 0 ? (
					<div className="empty">Waiting for output…</div>
				) : (
					lines.map((l, i) => (
						<div key={i} className={`wf-run-line wf-run-${l.stream}`}>
							{l.text}
						</div>
					))
				)}
				<div ref={endRef} />
			</div>
		</div>
	)
}

// buildMermaidPreview renders a Mermaid flowchart LR from the current workflow,
// mirroring the Go exporter's renderMermaid so the preview matches the export.
// Node ids are stable short ids (N0, N1, …) because Mermaid v11 mis-parses ids
// that contain dashes (it reads the `-` as an edge operator), which broke the
// previous slug-based ids like `start-node-default`.
//
// Group nodes render as Mermaid `subgraph` blocks containing their children
// (nodes whose parentId points at the group). Top-level nodes (no parentId)
// render at the flowchart root.
function buildMermaidPreview(current: ReturnType<typeof useWorkflowStore.getState>['current']): string {
	if (!current) return ''
	const lines: string[] = ['flowchart LR']
	const idMap: Record<string, string> = {}

	// Assign a stable short id to every node.
	current.nodes.forEach((n, i) => {
		idMap[n.id] = `N${i}`
	})

	// Partition: top-level nodes vs groups vs grouped children.
	const groups = current.nodes.filter((n) => n.type === 'group')
	const groupIds = new Set(groups.map((g) => g.id))
	const topNodes = current.nodes.filter((n) => !n.parentId && n.type !== 'group')
	const childrenOf = (gid: string) =>
		current.nodes.filter((n) => n.parentId === gid && n.type !== 'group')

	const emitNode = (n: typeof current.nodes[number], indent = '  ') => {
		const mid = idMap[n.id]
		const label = (n.data.label || n.data.name || n.data.description || n.type).replace(/"/g, "'")
		const shape = shapeFor(n.type)
		lines.push(`${indent}${mid}${shape.open}"${label}"${shape.close}`)
	}

	// Top-level nodes first.
	topNodes.forEach((n) => emitNode(n))

	// Then each group as a subgraph with its children nested inside.
	groups.forEach((g) => {
		const gid = idMap[g.id]
		const glabel = (g.data.label || g.name || g.id).replace(/"/g, "'")
		lines.push(`  subgraph ${gid} ["${glabel}"]`)
		childrenOf(g.id).forEach((c) => emitNode(c, '    '))
		// If a group has no children, Mermaid still needs a member to render the
		// subgraph box; emit an invisible placeholder.
		if (childrenOf(g.id).length === 0) {
			lines.push(`    ${gid}_empty[ ]:::hidden`)
		}
		lines.push('  end')
	})

	// Edges last (Mermaid resolves ids declared anywhere).
	current.connections.forEach((c) => {
		if (idMap[c.from] && idMap[c.to]) {
			lines.push(`  ${idMap[c.from]} --> ${idMap[c.to]}`)
		}
	})

	// Style the empty-group placeholders invisible so an empty group still shows
	// its box but no stray node.
	if (groups.some((g) => childrenOf(g.id).length === 0)) {
		lines.push('  classDef hidden display:none;')
	}

	// Avoid an unused-var warning in strict builds when there are no groups.
	void groupIds
	return lines.join('\n')
}

function shapeFor(type: string): { open: string; close: string } {
	switch (type) {
		case 'start':
			return { open: '([', close: '])' }
		case 'end':
			return { open: '{', close: '}' }
		case 'askUserQuestion':
		case 'ifElse':
		case 'switch':
		case 'branch':
			return { open: '{', close: '}' }
		default:
			return { open: '[', close: ']' }
	}
}
