import { Plus, Trash2 } from 'lucide-react'
import { useWorkflowStore } from '../../stores/workflowStore'
import type { WorkflowNode, WorkflowNodeData } from '../../api/types'

// NodeDetail renders a per-type editor for the currently selected node.
export default function NodeDetail() {
	const current = useWorkflowStore((s) => s.current)
	const selectedId = useWorkflowStore((s) => s.selectedNodeId)
	const removeNode = useWorkflowStore((s) => s.removeNode)

	const node = current?.nodes.find((n) => n.id === selectedId) || null

	if (!current) {
		return <div className="empty">No workflow loaded.</div>
	}
	if (!node) {
		return <div className="empty">Select a node to edit its fields, or drag a node type from the palette onto the canvas.</div>
	}

	return (
		<div className="workflow-detail">
			<h3>{node.type}</h3>
			<div className="field">
				<label>Node name</label>
				<input
					value={node.name}
					onChange={(e) => update(node, { _name: e.target.value })}
				/>
			</div>

			<NodeFields node={node} />

			<button
				className="btn btn-danger"
				onClick={() => removeNode(node.id)}
				style={{ marginTop: 'auto' }}
			>
				<Trash2 size={14} /> Delete node
			</button>
		</div>
	)
}

// NodeFields switches on the node type to render the relevant inputs.
function NodeFields({ node }: { node: WorkflowNode }) {
	switch (node.type) {
		case 'start':
		case 'end':
			return (
				<div className="field">
					<label>Label</label>
					<input
						value={node.data.label ?? ''}
						onChange={(e) => update(node, { label: e.target.value })}
					/>
				</div>
			)
		case 'subAgent':
			return (
				<>
					<div className="field">
						<label>Description (frontmatter)</label>
						<input
							value={node.data.description ?? ''}
							placeholder="What this agent does (≤200 chars)"
							onChange={(e) => update(node, { description: e.target.value })}
						/>
					</div>
					<div className="field">
						<label>System prompt / identity</label>
						<textarea
							value={node.data.agentDefinition ?? ''}
							placeholder="Who the agent IS…"
							onChange={(e) => update(node, { agentDefinition: e.target.value })}
						/>
					</div>
					<div className="field">
						<label>Task prompt</label>
						<textarea
							value={node.data.prompt ?? ''}
							placeholder="What to TELL the agent to do…"
							onChange={(e) => update(node, { prompt: e.target.value })}
						/>
					</div>
					<div className="row">
						<div className="field">
							<label>Model</label>
							<select
								value={node.data.model ?? 'inherit'}
								onChange={(e) => update(node, { model: e.target.value })}
							>
								<option value="inherit">inherit</option>
								<option value="sonnet">sonnet</option>
								<option value="opus">opus</option>
								<option value="haiku">haiku</option>
							</select>
						</div>
						<div className="field">
							<label>Mode</label>
							<select
								value={node.data.mode ?? 'all'}
								onChange={(e) => update(node, { mode: e.target.value })}
							>
								<option value="all">all</option>
								<option value="primary">primary</option>
							</select>
						</div>
					</div>
					<div className="field">
						<label>Tools (comma-separated)</label>
						<input
							value={node.data.tools ?? ''}
							placeholder="read, edit, write, bash"
							onChange={(e) => update(node, { tools: e.target.value })}
						/>
					</div>
				</>
			)
		case 'askUserQuestion':
			return (
				<>
					<div className="field">
						<label>Question</label>
						<textarea
							value={node.data.questionText ?? ''}
							onChange={(e) => update(node, { questionText: e.target.value })}
						/>
					</div>
					<div className="field">
						<label>Options (2–4)</label>
						{(node.data.options ?? []).map((opt, i) => (
							<div className="option-row" key={i}>
								<input
									value={opt.label ?? ''}
									placeholder={`Option ${i + 1}`}
									onChange={(e) => updateOption(node, i, { label: e.target.value })}
								/>
								<button
									className="btn btn-icon"
									onClick={() => removeOption(node, i)}
									title="Remove option"
								>
									<Trash2 size={12} />
								</button>
							</div>
						))}
						{(node.data.options?.length ?? 0) < 4 && (
							<button className="btn btn-ghost" onClick={() => addOption(node)}>
								<Plus size={12} /> Add option
							</button>
						)}
					</div>
				</>
			)
		case 'prompt':
			return (
				<div className="field">
					<label>Prompt text</label>
					<textarea
						value={node.data.prompt ?? ''}
						placeholder="Prompt template. Use {{variables}}."
						onChange={(e) => update(node, { prompt: e.target.value })}
					/>
				</div>
			)
		case 'ifElse':
			return (
				<div className="field">
					<label>Condition</label>
					<input
						value={node.data.condition ?? ''}
						placeholder="e.g. tests pass"
						onChange={(e) => update(node, { condition: e.target.value })}
					/>
				</div>
			)
		case 'switch':
		case 'branch':
			return (
				<div className="field">
					<label>Expression</label>
					<input
						value={node.data.expression ?? ''}
						onChange={(e) => update(node, { expression: e.target.value })}
					/>
				</div>
			)
		case 'skill':
			return (
				<>
					<div className="field">
						<label>Skill name</label>
						<input
							value={node.data.name ?? ''}
							placeholder="e.g. diagnosing-bugs"
							onChange={(e) => update(node, { name: e.target.value })}
						/>
					</div>
					<div className="field">
						<label>Execution mode</label>
						<select
							value={node.data.executionMode ?? 'load'}
							onChange={(e) => update(node, { executionMode: e.target.value })}
						>
							<option value="load">load (context)</option>
							<option value="execute">execute (run)</option>
						</select>
					</div>
				</>
			)
		case 'mcp':
			return (
				<div className="row">
					<div className="field">
						<label>Server</label>
						<input
							value={node.data.server ?? ''}
							onChange={(e) => update(node, { server: e.target.value })}
						/>
					</div>
					<div className="field">
						<label>Tool</label>
						<input
							value={node.data.tool ?? ''}
							onChange={(e) => update(node, { tool: e.target.value })}
						/>
					</div>
				</div>
			)
		case 'subAgentFlow':
			return (
				<div className="field">
					<label>Flow ID</label>
					<input
						value={node.data.flowId ?? ''}
						onChange={(e) => update(node, { flowId: e.target.value })}
					/>
				</div>
			)
		case 'group':
			return (
				<div className="field">
					<label>Label</label>
					<input
						value={node.data.label ?? ''}
						onChange={(e) => update(node, { label: e.target.value })}
					/>
				</div>
			)
		default:
			return null
	}
}

// ─── helpers ──────────────────────────────────────────────────────────────
// update centralizes the patch logic. `_name` is a synthetic key to edit the
// node's top-level name field (kept separate from data.name).
function update(node: WorkflowNode, patch: Partial<WorkflowNodeData> & { _name?: string }) {
	const store = useWorkflowStore.getState()
	if (patch._name !== undefined) {
		const { _name, ...rest } = patch
		store.updateNodeData(node.id, { ...node.data, ...rest })
		// node.name lives on the node, not data — patch it via a full update.
		const current = store.current
		if (current) {
			useWorkflowStore.setState({
				current: {
					...current,
					nodes: current.nodes.map((n) =>
						n.id === node.id ? { ...n, name: _name, data: { ...n.data, ...rest } } : n,
					),
				},
				dirty: true,
			})
		}
		return
	}
	store.updateNode(node.id, patch)
}

function updateOption(node: WorkflowNode, index: number, patch: { label?: string }) {
	const store = useWorkflowStore.getState()
	const options = [...(node.data.options ?? [])]
	options[index] = { ...options[index], ...patch }
	store.updateNode(node.id, { options })
}

function addOption(node: WorkflowNode) {
	const store = useWorkflowStore.getState()
	const options = [...(node.data.options ?? []), { label: `Option ${(node.data.options?.length ?? 0) + 1}` }]
	store.updateNode(node.id, { options })
}

function removeOption(node: WorkflowNode, index: number) {
	const store = useWorkflowStore.getState()
	const options = (node.data.options ?? []).filter((_, i) => i !== index)
	store.updateNode(node.id, { options })
}
