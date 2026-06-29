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
	Play,
	CheckCircle2,
	AlertTriangle,
	Network,
	X,
	FileCode2,
	RefreshCw,
	LayoutGrid,
} from 'lucide-react'
import { useWorkflowStore, disconnectEdgeId } from '../../stores/workflowStore'
import type { WorkflowConnection, WorkflowNodeType } from '../../api/types'
import Modal from '../shared/Modal'
import YdSelect from '../shared/YdSelect'
import { NODE_META, nodeTypes, toFlowNode } from './nodes'
import NodeDetail from './NodeDetail'
import './WorkflowEditor.css'

const PALETTE_TYPES: WorkflowNodeType[] = [
	'subAgent',
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
	const validateCurrent = useWorkflowStore((s) => s.validateCurrent)
	const exportCurrent = useWorkflowStore((s) => s.exportCurrent)
	const clearExport = useWorkflowStore((s) => s.clearExport)
	const clearError = useWorkflowStore((s) => s.clearError)
	const selectNode = useWorkflowStore((s) => s.selectNode)
	const connect = useWorkflowStore((s) => s.connect)
	const disconnect = useWorkflowStore((s) => s.disconnect)
	const applyNodeChanges = useWorkflowStore((s) => s.applyNodeChanges)
	const autoLayout = useWorkflowStore((s) => s.autoLayout)
	const { fitView } = useReactFlow()

	// local UI state
	const [selectedName, setSelectedName] = useState('')
	const [newOpen, setNewOpen] = useState(false)
	const [newName, setNewName] = useState('')
	const [newDesc, setNewDesc] = useState('')
	const [importOpen, setImportOpen] = useState(false)
	const [importText, setImportText] = useState('')
	const [mermaidPreview, setMermaidPreview] = useState<string | null>(null)
	const [mermaidLoading, setMermaidLoading] = useState(false)
	const importRaw = useWorkflowStore((s) => s.importRaw)
	const fileInput = useRef<HTMLInputElement>(null)

	useEffect(() => {
		if (mounted) list()
	}, [mounted, list])

	// auto-load first workflow when the list arrives and nothing is selected.
	useEffect(() => {
		if (mounted && summaries.length > 0 && !selectedName && !current) {
			setSelectedName(summaries[0].name)
			load(summaries[0].name)
		}
	}, [mounted, summaries, selectedName, current, load])

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
	const nodeSignature = useMemo(
		() => current?.nodes.map((n) => n.id + n.type).join('|') ?? '',
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

	const flowEdges: Edge[] = useMemo(() => {
		if (!current) return []
		return current.connections.map((c) => {
			const id = disconnectEdgeId(c as WorkflowConnection)
			return {
				id,
				source: c.from,
				target: c.to,
				sourceHandle: c.fromPort || undefined,
				targetHandle: c.toPort || undefined,
				animated: false,
				markerEnd: { type: 'arrowclosed' as const },
			}
		})
	}, [current])

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

	const showMermaid = useCallback(async () => {
		setMermaidLoading(true)
		// The Mermaid is derived client-side (buildMermaidPreview); no round-trip.
		setMermaidPreview(mermaid)
		setMermaidLoading(false)
	}, [mermaid])

	return (
		<div className="workflow-page">
			{/* Toolbar */}
			<div className="workflow-toolbar">
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

				<button className="btn" onClick={() => setNewOpen(true)}>
					<Plus size={14} /> New
				</button>
				<button
					className="btn"
					onClick={() => fileInput.current?.click()}
					title="Import a cc-wf-studio workflow.json"
				>
					<Upload size={14} /> Import
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
				<button
					className="btn btn-primary"
					onClick={() => saveCurrent()}
					disabled={!current || !dirty}
				>
					<Save size={14} /> Save {dirty ? '*' : ''}
				</button>
				<button
					className="btn"
					onClick={() => validateCurrent()}
					disabled={!current}
					title="Validate the workflow graph"
				>
					<CheckCircle2 size={14} /> Validate
				</button>
				<button
					className="btn"
					onClick={() => exportCurrent(false)}
					disabled={!current || exporting}
					title="Preview the opencode export (dry-run)"
				>
					<FileCode2 size={14} /> Export preview
				</button>
				<button
					className="btn btn-primary"
					onClick={() => exportCurrent(true)}
					disabled={!current || exporting}
					title="Write opencode artifacts to ~/.config/opencode"
				>
					<Play size={14} /> Export to opencode
				</button>
				<button
					className="btn"
					onClick={handleAutoLayout}
					disabled={!current}
					title="Auto-arrange nodes in a left-to-right layout"
				>
					<LayoutGrid size={14} /> Auto-layout
				</button>
				<button
					className="btn"
					onClick={() => showMermaid()}
					disabled={!current}
					title="Show the Mermaid diagram"
				>
					<Network size={14} /> Mermaid
				</button>

				<span className="spacer" />

				<button className="btn btn-danger" onClick={() => deleteCurrent()} disabled={!current}>
					<Trash2 size={14} /> Delete
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
					<aside className="workflow-palette">
						<h4>Nodes</h4>
						{PALETTE_TYPES.map((t) => {
							const Icon = NODE_META[t].icon
							return (
								<div
									key={t}
									className="palette-item"
									draggable
									onDragStart={(e) => onDragStart(e, t)}
									onDoubleClick={() =>
										useWorkflowStore.getState().addNode(t, 300 + Math.random() * 200, 150 + Math.random() * 200)
									}
									title={`Double-click or drag to add a ${NODE_META[t].kind} node`}
								>
									<span className="palette-icon">
										<Icon size={14} />
									</span>
									{NODE_META[t].kind}
								</div>
							)
						})}
					</aside>

					<div
						className="workflow-canvas-wrap"
						onDrop={onDrop}
						onDragOver={(e) => e.preventDefault()}
					>
						<ReactFlow
							nodes={flowNodes}
							edges={flowEdges}
							nodeTypes={nodeTypes}
							onNodesChange={onNodesChange}
							onConnect={onConnect}
							onEdgeClick={(_, edge) => disconnect(edge.id)}
							onNodeClick={(_, n) => selectNode(n.id)}
							onPaneClick={() => selectNode(null)}
							fitView
							proOptions={{ hideAttribution: true }}
						>
							<Background variant={BackgroundVariant.Dots} gap={16} size={1} />
							<Controls />
							<MiniMap pannable zoomable />
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
					<label className="field-label">Paste a cc-wf-studio workflow.json below</label>
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
								? 'Dry-run preview. These files would be written to ~/.config/opencode:'
								: '✅ Files written to ~/.config/opencode. Restart opencode to pick them up.'}
						</p>
						{(exportPlan.files ?? []).map((f, i) => (
							<div className={`artifact kind-${f.kind}`} key={i}>
								<span className="kind">{f.kind}</span>
								<span>{f.path.replace(/^.*opencode/, '~/.config/opencode')}</span>
							</div>
						))}
						<div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>
							<button className="btn" onClick={clearExport}>Close</button>
							<button
								className="btn btn-primary"
								onClick={() => exportCurrent(true)}
								disabled={exporting}
							>
								<RefreshCw size={14} /> {exportPlan.dryRun ? 'Apply (write files)' : 'Re-export'}
							</button>
						</div>
					</div>
				)}
			</Modal>

			{/* Mermaid preview modal */}
			<Modal open={mermaidPreview !== null} onClose={() => setMermaidPreview(null)} title="Mermaid diagram" width="640px">
				{mermaidLoading ? (
					<div className="empty">Generating…</div>
				) : (
					<pre className="mermaid-preview">{mermaidPreview}</pre>
				)}
			</Modal>
		</div>
	)
}

// buildMermaidPreview renders a Mermaid flowchart LR from the current workflow,
// mirroring the Go exporter's renderMermaid so the preview matches the export.
function buildMermaidPreview(current: ReturnType<typeof useWorkflowStore.getState>['current']): string {
	if (!current) return ''
	const lines: string[] = ['flowchart LR']
	const idMap: Record<string, string> = {}
	current.nodes.forEach((n, i) => {
		const mid = slug(n.id) || `N${i}`
		idMap[n.id] = mid
		const label = (n.data.label || n.data.name || n.data.description || n.type).replace(/"/g, "'")
		const shape = shapeFor(n.type)
		lines.push(`  ${mid}${shape.open}"${label}"${shape.close}`)
	})
	current.connections.forEach((c) => {
		if (idMap[c.from] && idMap[c.to]) {
			lines.push(`  ${idMap[c.from]} --> ${idMap[c.to]}`)
		}
	})
	return lines.join('\n')
}

function slug(s: string): string {
	return s.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
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
