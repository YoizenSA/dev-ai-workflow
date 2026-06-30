import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import {
	Play,
	Square,
	Bot,
	HelpCircle,
	GitBranch,
	Blocks,
	BookOpen,
	Plug,
	Box,
	FileText,
} from 'lucide-react'
import type { WorkflowNode, WorkflowNodeData, WorkflowNodeType } from '../../api/types'

// Public node-data shape carried by xyflow nodes (same as WorkflowNode['data']).
export interface WorkflowNodePayload extends Record<string, unknown> {
	__type: WorkflowNodeType
	name?: string
	label?: string
	description?: string
	prompt?: string
	questionText?: string
	condition?: string
	expression?: string
	model?: string
	mode?: string
	tools?: string
	agentType?: string
	executionMode?: string
	server?: string
	tool?: string
	mcpMode?: string
	taskDescription?: string
	flowId?: string
	aiParams?: string
	width?: number
	height?: number
	options?: { id?: string; label?: string }[]
	branches?: { id?: string; label?: string; value?: string }[]
}

// NODE_META: icon, palette title (kind), the UPPERCASE type label shown in the
// node's meta row, and a one-line palette subtitle.
export const NODE_META: Record<WorkflowNodeType, { icon: typeof Play; kind: string; type: string; desc: string }> = {
	start: { icon: Play, kind: 'Start', type: 'START', desc: 'Workflow entry point' },
	end: { icon: Square, kind: 'End', type: 'END', desc: 'Workflow exit point' },
	subAgent: { icon: Bot, kind: 'Sub-Agent', type: 'SUB-AGENT', desc: 'Execute a specialized task' },
	askUserQuestion: { icon: HelpCircle, kind: 'Ask User', type: 'ASK USER QUESTION', desc: 'Pause and ask the user' },
	ifElse: { icon: GitBranch, kind: 'If/Else', type: 'IF/ELSE', desc: 'Two-way conditional branch' },
	switch: { icon: GitBranch, kind: 'Switch', type: 'SWITCH', desc: 'Multi-way branch' },
	branch: { icon: GitBranch, kind: 'Switch', type: 'SWITCH', desc: 'Multi-way branch' },
	prompt: { icon: FileText, kind: 'Prompt', type: 'PROMPT', desc: 'Template with variables' },
	skill: { icon: BookOpen, kind: 'Skill', type: 'SKILL', desc: 'Execute a Claude Code Skill' },
	mcp: { icon: Plug, kind: 'MCP Tool', type: 'MCP TOOL', desc: 'Execute an MCP tool' },
	subAgentFlow: { icon: Blocks, kind: 'Sub-Agent Flow', type: 'SUB-AGENT FLOW', desc: 'Execute a Sub-Agent flow' },
	codex: { icon: Box, kind: 'Codex', type: 'CODEX', desc: 'Codex agent' },
	group: { icon: Box, kind: 'Group', type: 'GROUP', desc: 'Visual container' },
}

function title(d: WorkflowNodePayload): string {
	if (d.name && d.name.trim()) return d.name
	if (d.label && d.label.trim()) return d.label
	return NODE_META[d.__type].kind
}

// subtitle is the one-line summary directly under the title.
function subtitle(d: WorkflowNodePayload): string {
	switch (d.__type) {
		case 'subAgent':
			return d.description || ''
		case 'askUserQuestion':
			return d.questionText || ''
		case 'ifElse':
			return d.condition || ''
		case 'switch':
		case 'branch':
			return d.expression || ''
		case 'prompt':
			return d.prompt ? d.prompt.slice(0, 64) : ''
		case 'skill':
			return d.name || ''
		case 'mcp':
			return [d.server, d.tool].filter(Boolean).join('/') || ''
		case 'subAgentFlow':
			return d.flowId || ''
		default:
			return ''
	}
}

// chips returns the per-type key/value badges shown in the node body, so each
// node type surfaces its defining fields directly on the canvas.
function chips(d: WorkflowNodePayload): { k: string; v: string }[] {
	switch (d.__type) {
		case 'subAgent': {
			const out: { k: string; v: string }[] = []
			if (d.model) out.push({ k: 'model', v: d.model })
			if (d.mode) out.push({ k: 'mode', v: d.mode })
			if (d.tools) out.push({ k: 'tools', v: d.tools })
			return out
		}
		case 'askUserQuestion':
			return [{ k: 'options', v: String(d.options?.length ?? 0) }]
		case 'switch':
		case 'branch':
			return [{ k: 'branches', v: String(d.branches?.length ?? 0) }]
		case 'skill':
			return d.executionMode ? [{ k: 'mode', v: d.executionMode }] : []
		case 'mcp': {
			const out: { k: string; v: string }[] = []
			const mode = d.mcpMode ?? 'aiParameterConfig'
			const label =
				mode === 'aiToolSelection'
					? 'tool: AI'
					: mode === 'manualParameterConfig'
						? 'tool: manual'
						: 'params: AI'
			out.push({ k: 'mode', v: label })
			return out
		}
		default:
			return []
	}
}

// outPorts lists the labeled source handles. Branching nodes expose one handle
// per outcome (true/false, each option, each branch); everything else gets a
// single unlabeled output. End nodes have none.
function outPorts(d: WorkflowNodePayload): { id?: string; label: string }[] {
	switch (d.__type) {
		case 'ifElse':
			return [
				{ id: 'true', label: 'true' },
				{ id: 'false', label: 'false' },
			]
		case 'askUserQuestion': {
			const opts = d.options ?? []
			if (!opts.length) return [{ label: '' }]
			return opts.map((o, i) => ({ id: o.id || o.label || `opt-${i}`, label: o.label || `Option ${i + 1}` }))
		}
		case 'switch':
		case 'branch': {
			const br = d.branches ?? []
			if (!br.length) return [{ label: '' }]
			return br.map((b, i) => ({ id: b.id || b.value || b.label || `branch-${i}`, label: b.label || b.value || `Case ${i + 1}` }))
		}
		case 'end':
			return []
		default:
			return [{ label: '' }]
	}
}

// PORT_ROW_H must match the .wf-port row height in CSS so handles align to labels.
const PORT_ROW_H = 22

function WorkflowNodeView({ data, selected }: NodeProps) {
	const d = data as WorkflowNodePayload
	const meta = NODE_META[d.__type]
	const Icon = meta.icon

	// Group: a visual container box (sized via data.width/height) drawn behind
	// the other nodes. Not a React Flow parent — membership is purely visual.
	if (d.__type === 'group') {
		return (
			<div className={`wf-group ${selected ? 'is-selected' : ''}`} style={{ width: '100%', height: '100%' }}>
				<div className="wf-group-title">
					<Box size={12} /> {title(d)}
				</div>
			</div>
		)
	}
	const sub = subtitle(d)
	const badges = chips(d)
	const ports = outPorts(d)
	const multiPort = ports.length > 1

	return (
		<div className={`wf-node type-${d.__type} ${selected ? 'is-selected' : ''} ${multiPort ? 'has-ports' : ''}`}>
			{d.__type !== 'start' && <Handle type="target" position={Position.Left} />}
			<div className="wf-node-meta">
				<span className="wf-node-icon">
					<Icon size={12} />
				</span>
				<span className="wf-node-type">{meta.type}</span>
				{d.agentType && <span className="wf-badge">{d.agentType === 'claudeCode' ? 'CLAUDE CODE' : d.agentType}</span>}
			</div>
			<div className="wf-node-title">{title(d)}</div>
			{sub && <div className="wf-node-sub">{sub}</div>}
			{badges.length > 0 && (
				<div className="wf-node-chips">
					{badges.map((b) => (
						<span className="wf-chip" key={b.k} title={`${b.k}: ${b.v}`}>
							<span className="wf-chip-k">{b.k}</span>
							<span className="wf-chip-v">{b.v}</span>
						</span>
					))}
				</div>
			)}
			{multiPort && (
				<div className="wf-node-ports">
					{ports.map((p) => (
						<div className="wf-port" key={p.id ?? p.label}>
							<span className="wf-port-label">{p.label}</span>
						</div>
					))}
				</div>
			)}
			{/* Source handles. Single nodes use one centered handle; branching nodes
			    align each labeled handle to its port row above. */}
			{multiPort
				? ports.map((p, i) => (
						<Handle
							key={p.id ?? i}
							id={p.id}
							type="source"
							position={Position.Right}
							style={{ top: `calc(100% - ${(ports.length - i) * PORT_ROW_H - PORT_ROW_H / 2}px)` }}
						/>
					))
				: ports.length === 1 && <Handle id={ports[0].id} type="source" position={Position.Right} />}
		</div>
	)
}

export const WorkflowNodeRenderer = memo(WorkflowNodeView)

// nodeTypes must be a module-level const (xyflow warns otherwise).
export const nodeTypes = { workflow: WorkflowNodeRenderer }

// toFlowNode converts a domain WorkflowNode into the xyflow node shape the
// canvas renders. The __type discriminator lets the renderer switch on style.
export function toFlowNode(n: WorkflowNode) {
	const isGroup = n.type === 'group'
	return {
		id: n.id,
		type: 'workflow',
		position: n.position,
		// Groups sit behind everything else so they read as containers.
		zIndex: isGroup ? 0 : 1,
		// React Flow needs explicit parent dimensions so `extent: 'parent'`
		// constrains children correctly (without them children clamp to 0,0).
		...(isGroup ? { style: { width: n.data.width ?? 360, height: n.data.height ?? 240 } } : {}),
		// Real grouping: children are confined to and move with their parent group.
		...(n.parentId ? { parentId: n.parentId, extent: 'parent' as const } : {}),
		data: { ...n.data, __type: n.type, name: n.name } as WorkflowNodePayload,
	}
}

// fromFlowData recovers the domain NodeData from an xyflow node payload,
// stripping the synthetic __type field.
export function fromFlowData(data: WorkflowNodePayload): { type: WorkflowNodeType; data: WorkflowNodeData; name: string } {
	const { __type, ...rest } = data
	return { type: __type, data: rest as unknown as WorkflowNodeData, name: rest.name ?? '' }
}
