import { useEffect, useState } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { useWorkflowStore } from '../../stores/workflowStore'
import { missionsApi, workflowApi, type McpCatalogItem } from '../../api/client'
import type { ModelInfo, WorkflowNode, WorkflowNodeData } from '../../api/types'
import YdSelect, { type SelectOption } from '../shared/YdSelect'

// Shared cache so every node editor reuses one opencode model fetch.
let modelCache: ModelInfo[] | null = null

// Caches for the skill + MCP catalogs, fetched once and reused across editors.
let skillCache: { name: string; description: string }[] | null = null
let mcpCache: McpCatalogItem[] | null = null

export function useSkills(): { name: string; description: string }[] {
	const [skills, setSkills] = useState(skillCache ?? [])
	useEffect(() => {
		if (skillCache) return
		workflowApi
			.listSkills()
			.then((s) => {
				skillCache = s ?? []
				setSkills(skillCache)
			})
			.catch(() => undefined)
	}, [])
	return skills
}

export function useMcps(): McpCatalogItem[] {
	const [mcps, setMcps] = useState<McpCatalogItem[]>(mcpCache ?? [])
	useEffect(() => {
		if (mcpCache) return
		workflowApi
			.listMcps()
			.then((m) => {
				mcpCache = m ?? []
				setMcps(mcpCache)
			})
			.catch(() => undefined)
	}, [])
	return mcps
}

export function useOpencodeModels(): ModelInfo[] {
	const [models, setModels] = useState<ModelInfo[]>(modelCache ?? [])
	useEffect(() => {
		if (modelCache) return
		missionsApi
			.listModels()
			.then((r) => {
				modelCache = Object.values(r.modelsByProvider ?? {}).flat()
				setModels(modelCache)
			})
			.catch(() => undefined)
	}, [])
	return models
}

// modelOptions builds the YdSelect option list for the subAgent Model field.
// Falls back to a small static list while the opencode model list loads.
function modelOptions(models: ModelInfo[]): SelectOption[] {
	const opts: SelectOption[] = [{ value: 'inherit', label: 'inherit' }]
	if (models.length === 0) {
		opts.push(
			{ value: 'sonnet', label: 'sonnet' },
			{ value: 'opus', label: 'opus' },
			{ value: 'haiku', label: 'haiku' },
		)
	} else {
		for (const m of models) opts.push({ value: m.id, label: `${m.provider}/${m.name}` })
	}
	return opts
}

// NodeDetail renders a per-type editor for the currently selected node.
export default function NodeDetail() {
	const current = useWorkflowStore((s) => s.current)
	const selectedId = useWorkflowStore((s) => s.selectedNodeId)
	const removeNode = useWorkflowStore((s) => s.removeNode)

	const node = current?.nodes.find((n) => n.id === selectedId) || null

	if (!current) {
		return (
			<div className="workflow-detail">
				<div className="empty">No workflow loaded.</div>
			</div>
		)
	}
	if (!node) {
		return (
			<div className="workflow-detail">
				<div className="empty">Select a node to edit its fields, or drag a node type from the palette onto the canvas.</div>
			</div>
		)
	}

	const nameMissing = !node.name.trim()

	return (
		<div className="workflow-detail">
			<h3>{node.type}</h3>
			<div className="field">
				<label className="field-label" htmlFor="wf-node-name">Node name</label>
				<input
					id="wf-node-name"
					className={`input${nameMissing ? ' invalid' : ''}`}
					value={node.name}
					aria-invalid={nameMissing}
					onChange={(e) => update(node, { _name: e.target.value })}
				/>
				{nameMissing && <span className="field-help error">Required.</span>}
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
	const models = useOpencodeModels()
	switch (node.type) {
		case 'start':
		case 'end':
			return (
				<div className="field">
					<label className="field-label" htmlFor="wf-label">Label</label>
					<input
						id="wf-label"
						className="input"
						value={node.data.label ?? ''}
						onChange={(e) => update(node, { label: e.target.value })}
					/>
				</div>
			)
		case 'subAgent':
			return <SubAgentFields node={node} models={models} />
		case 'askUserQuestion':
			return <AskUserFields node={node} />
		case 'prompt':
			return (
				<div className="field">
					<label className="field-label" htmlFor="wf-prompt">Prompt text</label>
					<textarea
						id="wf-prompt"
						className="textarea mono"
						value={node.data.prompt ?? ''}
						placeholder="Prompt template. Use {{variables}}."
						onChange={(e) => update(node, { prompt: e.target.value })}
					/>
				</div>
			)
		case 'ifElse':
			return (
				<div className="field">
					<label className="field-label" htmlFor="wf-condition">Condition</label>
					<input
						id="wf-condition"
						className="input"
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
					<label className="field-label" htmlFor="wf-expression">Expression</label>
					<input
						id="wf-expression"
						className="input"
						value={node.data.expression ?? ''}
						onChange={(e) => update(node, { expression: e.target.value })}
					/>
				</div>
			)
		case 'skill':
			return <SkillFields node={node} />
		case 'mcp':
			return <McpFields node={node} />

		case 'subAgentFlow':
			return (
				<div className="field">
					<label className="field-label" htmlFor="wf-flowid">Flow ID</label>
					<input
						id="wf-flowid"
						className="input"
						value={node.data.flowId ?? ''}
						onChange={(e) => update(node, { flowId: e.target.value })}
					/>
				</div>
			)
		case 'group':
			return (
				<div className="field">
					<label className="field-label" htmlFor="wf-group-label">Label</label>
					<input
						id="wf-group-label"
						className="input"
						value={node.data.label ?? ''}
						onChange={(e) => update(node, { label: e.target.value })}
					/>
				</div>
			)
		default:
			return null
	}
}

// SubAgentFields — the richest node: identity, prompts, model/mode, tools.
function SubAgentFields({ node, models }: { node: WorkflowNode; models: ModelInfo[] }) {
	const descMissing = !(node.data.description ?? '').trim()
	return (
		<>
			<div className="field">
				<label className="field-label" htmlFor="wf-desc">Description (frontmatter)</label>
				<input
					id="wf-desc"
					className={`input${descMissing ? ' invalid' : ''}`}
					value={node.data.description ?? ''}
					placeholder="What this agent does (≤200 chars)"
					aria-invalid={descMissing}
					onChange={(e) => update(node, { description: e.target.value })}
				/>
				<span className="field-help">Goes into the agent frontmatter. ≤200 chars.</span>
			</div>
			<div className="field">
				<label className="field-label" htmlFor="wf-identity">System prompt / identity</label>
				<textarea
					id="wf-identity"
					className="textarea mono"
					value={node.data.agentDefinition ?? ''}
					placeholder="Who the agent IS…"
					onChange={(e) => update(node, { agentDefinition: e.target.value })}
				/>
			</div>
			<div className="field">
				<label className="field-label" htmlFor="wf-task">Task prompt</label>
				<textarea
					id="wf-task"
					className="textarea mono"
					value={node.data.prompt ?? ''}
					placeholder="What to TELL the agent to do…"
					onChange={(e) => update(node, { prompt: e.target.value })}
				/>
			</div>
			<div className="row">
				<div className="field">
					<label className="field-label" htmlFor="wf-model">Model</label>
					<YdSelect
						options={modelOptions(models)}
						value={node.data.model ?? 'inherit'}
						onChange={(v) => update(node, { model: v })}
						ariaLabel="Model"
					/>
				</div>
				<div className="field">
					<label className="field-label" htmlFor="wf-mode">Mode</label>
					<YdSelect
						options={[
							{ value: 'all', label: 'all' },
							{ value: 'primary', label: 'primary' },
						]}
						value={node.data.mode ?? 'all'}
						onChange={(v) => update(node, { mode: v })}
						ariaLabel="Mode"
					/>
				</div>
			</div>
			<div className="field">
				<label className="field-label" htmlFor="wf-tools">Tools</label>
				<input
					id="wf-tools"
					className="input mono"
					value={node.data.tools ?? ''}
					placeholder="read, edit, write, bash"
					onChange={(e) => update(node, { tools: e.target.value })}
				/>
				<span className="field-help">Comma-separated.</span>
			</div>
		</>
	)
}

// AskUserFields — question + 2–4 options.
function AskUserFields({ node }: { node: WorkflowNode }) {
	return (
		<>
			<div className="field">
				<label className="field-label" htmlFor="wf-question">Question</label>
				<textarea
					id="wf-question"
					className="textarea"
					value={node.data.questionText ?? ''}
					onChange={(e) => update(node, { questionText: e.target.value })}
				/>
			</div>
			<div className="field">
				<label className="field-label" htmlFor={`wf-opt-${node.id}-0`}>Options (2–4)</label>
				{(node.data.options ?? []).map((opt, i) => (
					<div className="option-row" key={i}>
						<input
							id={`wf-opt-${node.id}-${i}`}
							className="input"
							value={opt.label ?? ''}
							placeholder={`Option ${i + 1}`}
							aria-label={`Option ${i + 1}`}
							onChange={(e) => updateOption(node, i, { label: e.target.value })}
						/>
						<button
							className="btn btn-icon"
							onClick={() => removeOption(node, i)}
							title="Remove option"
							aria-label={`Remove option ${i + 1}`}
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
}

// McpFields — server + tool, populated from the installed MCP catalog. Falls
// back to free text when the catalog is empty (e.g. no MCPs installed yet).
function McpFields({ node }: { node: WorkflowNode }) {
	const mcps = useMcps()
	const installed = mcps.filter((m) => m.installed)
	const list = installed.length > 0 ? installed : mcps
	const selected = list.find((m) => m.id === node.data.server)
	const tools = selected?.tools ?? []

	if (list.length === 0) {
		return (
			<div className="row">
				<div className="field">
					<label className="field-label" htmlFor="wf-server">Server</label>
					<input id="wf-server" className="input" value={node.data.server ?? ''} onChange={(e) => update(node, { server: e.target.value })} />
				</div>
				<div className="field">
					<label className="field-label" htmlFor="wf-tool">Tool</label>
					<input id="wf-tool" className="input" value={node.data.tool ?? ''} onChange={(e) => update(node, { tool: e.target.value })} />
				</div>
			</div>
		)
	}

	return (
		<div className="row">
			<div className="field">
				<label className="field-label" htmlFor="wf-server">Server</label>
				<YdSelect
					options={list.map((m) => ({ value: m.id, label: m.name }))}
					value={node.data.server ?? ''}
					onChange={(v) => update(node, { server: v, tool: '' })}
					placeholder="— select MCP —"
					ariaLabel="MCP server"
				/>
			</div>
			<div className="field">
				<label className="field-label" htmlFor="wf-tool">Tool</label>
				<YdSelect
					options={tools.map((t) => ({ value: t, label: t }))}
					value={node.data.tool ?? ''}
					onChange={(v) => update(node, { tool: v })}
					placeholder={selected ? '— select tool —' : 'pick a server first'}
					ariaLabel="MCP tool"
				/>
			</div>
		</div>
	)
}

// SkillFields — skill name (from installed skills) + execution mode.
function SkillFields({ node }: { node: WorkflowNode }) {
	const skills = useSkills()
	return (
		<>
			<div className="field">
				<label className="field-label" htmlFor="wf-skill-name">Skill name</label>
				{skills.length === 0 ? (
					<input
						id="wf-skill-name"
						className="input mono"
						value={node.data.name ?? ''}
						placeholder="e.g. diagnosing-bugs"
						onChange={(e) => update(node, { name: e.target.value })}
					/>
				) : (
					<YdSelect
						options={skills.map((s) => ({ value: s.name, label: s.name }))}
						value={node.data.name ?? ''}
						onChange={(v) => update(node, { name: v })}
						placeholder="— select skill —"
						ariaLabel="Skill name"
					/>
				)}
			</div>
			<div className="field">
				<label className="field-label" htmlFor="wf-exec-mode">Execution mode</label>
				<YdSelect
					options={[
						{ value: 'load', label: 'load (context)' },
						{ value: 'execute', label: 'execute (run)' },
					]}
					value={node.data.executionMode ?? 'load'}
					onChange={(v) => update(node, { executionMode: v })}
					ariaLabel="Execution mode"
				/>
			</div>
		</>
	)
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
