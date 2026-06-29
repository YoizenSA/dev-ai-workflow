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
	tools?: string
	agentType?: string
}

// nodeTypeIcon maps each node type to a lucide icon + one-word label.
export const NODE_META: Record<WorkflowNodeType, { icon: typeof Play; kind: string }> = {
	start: { icon: Play, kind: 'Start' },
	end: { icon: Square, kind: 'End' },
	subAgent: { icon: Bot, kind: 'Sub-agent' },
	askUserQuestion: { icon: HelpCircle, kind: 'Ask user' },
	ifElse: { icon: GitBranch, kind: 'If/Else' },
	switch: { icon: GitBranch, kind: 'Switch' },
	branch: { icon: GitBranch, kind: 'Switch' },
	prompt: { icon: FileText, kind: 'Prompt' },
	skill: { icon: BookOpen, kind: 'Skill' },
	mcp: { icon: Plug, kind: 'MCP' },
	subAgentFlow: { icon: Blocks, kind: 'Sub-flow' },
	codex: { icon: Box, kind: 'Codex' },
	group: { icon: Box, kind: 'Group' },
}

function title(d: WorkflowNodePayload): string {
	if (d.name && d.name.trim()) return d.name
	if (d.label && d.label.trim()) return d.label
	return NODE_META[d.__type].kind
}

function subtitle(d: WorkflowNodePayload): string {
	switch (d.__type) {
		case 'subAgent':
			return d.description || d.model || ''
		case 'askUserQuestion':
			return d.questionText || ''
		case 'ifElse':
			return d.condition || ''
		case 'switch':
		case 'branch':
			return d.expression || ''
		case 'prompt':
			return d.prompt ? d.prompt.slice(0, 48) : ''
		case 'skill':
			return d.name || ''
		case 'mcp':
			return [d.agentType, d.tools].filter(Boolean).join(' · ')
		default:
			return ''
	}
}

function WorkflowNodeView({ data, selected }: NodeProps) {
	const d = data as WorkflowNodePayload
	const meta = NODE_META[d.__type]
	const Icon = meta.icon
	return (
		<div className={`wf-node type-${d.__type} ${selected ? 'is-selected' : ''}`}>
			<Handle type="target" position={Position.Left} />
			<div className="wf-node-head">
				<span className="wf-node-icon">
					<Icon size={14} />
				</span>
				<span className="wf-node-title">{title(d)}</span>
			</div>
			{subtitle(d) && <div className="wf-node-sub">{subtitle(d)}</div>}
			<Handle type="source" position={Position.Right} />
		</div>
	)
}

export const WorkflowNodeRenderer = memo(WorkflowNodeView)

// nodeTypes must be a module-level const (xyflow warns otherwise).
export const nodeTypes = { workflow: WorkflowNodeRenderer }

// toFlowNode converts a domain WorkflowNode into the xyflow node shape the
// canvas renders. The __type discriminator lets the renderer switch on style.
export function toFlowNode(n: WorkflowNode) {
	return {
		id: n.id,
		type: 'workflow',
		position: n.position,
		data: { ...n.data, __type: n.type, name: n.name } as WorkflowNodePayload,
	}
}

// fromFlowData recovers the domain NodeData from an xyflow node payload,
// stripping the synthetic __type field.
export function fromFlowData(data: WorkflowNodePayload): { type: WorkflowNodeType; data: WorkflowNodeData; name: string } {
	const { __type, ...rest } = data
	return { type: __type, data: rest as unknown as WorkflowNodeData, name: rest.name ?? '' }
}
