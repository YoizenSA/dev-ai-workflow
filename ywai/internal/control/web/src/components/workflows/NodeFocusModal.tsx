import { useEffect, useMemo, useState } from 'react'
import Editor from '@monaco-editor/react'
import Modal from '../shared/Modal'
import { useWorkflowStore } from '../../stores/workflowStore'
import type { WorkflowNode } from '../../api/types'

// Long-text fields editable in focus mode, per node type. Each maps to a
// node.data key (or the synthetic "name" for the node's top-level name).
interface FocusField {
	key: string
	label: string
	language: string
}

function fieldsFor(node: WorkflowNode): FocusField[] {
	const md = 'markdown'
	const txt = 'plaintext'
	switch (node.type) {
		case 'start':
			// START configures the orchestrator (parent agent).
			return [
				{ key: 'agentDefinition', label: 'Orchestrator system prompt', language: md },
				{ key: 'description', label: 'Description', language: txt },
				{ key: 'label', label: 'Label', language: txt },
			]
		case 'subAgent':
			return [
				{ key: 'agentDefinition', label: 'System prompt / identity', language: md },
				{ key: 'prompt', label: 'Task prompt', language: md },
				{ key: 'description', label: 'Description', language: txt },
				{ key: 'tools', label: 'Tools', language: txt },
			]
		case 'prompt':
			return [{ key: 'prompt', label: 'Prompt', language: md }]
		case 'mcp':
			return [{ key: 'aiParams', label: 'AI Parameter Configuration', language: md }]
		case 'askUserQuestion':
			return [{ key: 'questionText', label: 'Question', language: md }]
		case 'ifElse':
			return [{ key: 'condition', label: 'Condition', language: txt }]
		case 'switch':
		case 'branch':
			return [{ key: 'expression', label: 'Expression', language: txt }]
		case 'skill':
			return [{ key: 'description', label: 'Description', language: txt }]
		default:
			return [{ key: 'label', label: 'Label', language: txt }]
	}
}

// NodeFocusModal — a distraction-free Monaco editor for a node's long fields.
export default function NodeFocusModal({ nodeId, onClose }: { nodeId: string | null; onClose: () => void }) {
	const current = useWorkflowStore((s) => s.current)
	const updateNode = useWorkflowStore((s) => s.updateNode)
	const updateNodeData = useWorkflowStore((s) => s.updateNodeData)
	const node = current?.nodes.find((n) => n.id === nodeId) || null
	// Field tabs + a trailing "JSON" tab that edits the whole node.data.
	const fields = useMemo(() => (node ? [...fieldsFor(node), { key: '__json', label: 'JSON', language: 'json' }] : []), [node])
	const [active, setActive] = useState(0)
	// Local buffer for the JSON tab so typing an intermediate (invalid) state
	// doesn't get clobbered; applied to the store only when it parses.
	const [jsonText, setJsonText] = useState('')
	const [jsonError, setJsonError] = useState<string | null>(null)
	useEffect(() => {
		if (node) setJsonText(JSON.stringify(node.data, null, 2))
		setJsonError(null)
		// Re-seed when the focused node changes, not on every data tick.
		// eslint-disable-next-line react-hooks/exhaustive-deps
	}, [nodeId])

	if (!node) return null
	const field = fields[Math.min(active, fields.length - 1)]
	const isJson = field.key === '__json'
	const value = (node.data as Record<string, unknown>)[field.key]

	return (
		<Modal open={!!nodeId} onClose={onClose} title={`Focus — ${node.name || node.type}`} width="min(1100px, 92vw)">
			<div className="wf-focus">
				<div className="wf-focus-tabs">
					{fields.map((f, i) => (
						<button
							key={f.key}
							className={`btn btn-sm${i === active ? ' is-active' : ''}`}
							onClick={() => setActive(i)}
						>
							{f.label}
						</button>
					))}
					{isJson && jsonError && <span className="wf-focus-err">{jsonError}</span>}
				</div>
				<div className="wf-focus-editor">
					{isJson ? (
						<Editor
							height="60vh"
							language="json"
							theme="vs-dark"
							value={jsonText}
							onChange={(v) => {
								const text = v ?? ''
								setJsonText(text)
								try {
									const parsed = JSON.parse(text)
									updateNodeData(node.id, parsed)
									setJsonError(null)
								} catch (e) {
									setJsonError((e as Error).message)
								}
							}}
							options={{ minimap: { enabled: false }, fontSize: 13, scrollBeyondLastLine: false }}
						/>
					) : (
						<Editor
							height="60vh"
							language={field.language}
							theme="vs-dark"
							value={typeof value === 'string' ? value : ''}
							onChange={(v) => updateNode(node.id, { [field.key]: v ?? '' })}
							options={{
								minimap: { enabled: false },
								wordWrap: 'on',
								fontSize: 13,
								scrollBeyondLastLine: false,
								lineNumbers: 'on',
							}}
						/>
					)}
				</div>
			</div>
		</Modal>
	)
}
