import { useEffect, useRef, useState } from 'react'
import { X, Send } from 'lucide-react'
import { useWorkflowStore } from '../../stores/workflowStore'
import { useOpencodeModels } from './NodeDetail'
import YdSelect from '../shared/YdSelect'

// RefinementChatPanel is a multi-turn chat for Edit-with-AI. It shows the
// workflow's conversation history (persisted with the workflow) and sends each
// message through the store's aiEdit, which forwards the recent turns to the
// backend for conversational context. Replaces the old single-turn modal.
export default function RefinementChatPanel({ onClose }: { onClose: () => void }) {
	const current = useWorkflowStore((s) => s.current)
	const aiEditing = useWorkflowStore((s) => s.aiEditing)
	const aiEdit = useWorkflowStore((s) => s.aiEdit)
	const models = useOpencodeModels()

	const [text, setText] = useState('')
	const [model, setModel] = useState('')
	const endRef = useRef<HTMLDivElement>(null)

	const messages = current?.conversationHistory?.messages ?? []
	const iterations = current?.conversationHistory?.currentIteration ?? 0
	const maxIterations = current?.conversationHistory?.maxIterations ?? 20

	useEffect(() => {
		endRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' })
	}, [messages.length, aiEditing])

	const send = async () => {
		const t = text.trim()
		if (!t || aiEditing) return
		setText('')
		// The store updates `current`; the canvas observes it via selector and
		// re-renders automatically.
		await aiEdit(t, model || undefined)
	}

	return (
		<div className="wf-chat-panel" data-tour="refinement-chat">
			<div className="wf-chat-header">
				<span className="wf-chat-title">Edit with AI</span>
				<span className="wf-chat-count">
					{iterations}/{maxIterations}
				</span>
				<button className="btn btn-icon" onClick={onClose} aria-label="Close chat">
					<X size={14} />
				</button>
			</div>

			<div className="wf-chat-body">
				{messages.length === 0 ? (
					<div className="empty">
						Describe a change in plain language. The AI rewrites the workflow and loads it
						into the editor as a single undo step.
					</div>
				) : (
					messages.map((m) => (
						<div key={m.id} className={`wf-chat-msg wf-chat-${m.sender}`}>
							<div className="wf-chat-bubble">{m.content}</div>
						</div>
					))
				)}
				{aiEditing && (
					<div className="wf-chat-msg wf-chat-ai">
						<div className="wf-chat-bubble wf-chat-typing">Thinking…</div>
					</div>
				)}
				<div ref={endRef} />
			</div>

			<div className="wf-chat-input-row">
				<YdSelect
					options={[
						{ value: '', label: 'default' },
						...models.map((m) => ({ value: m.id, label: `${m.provider}/${m.name}` })),
					]}
					value={model}
					onChange={setModel}
					ariaLabel="Model"
				/>
				<textarea
					className="textarea wf-chat-input"
					value={text}
					onChange={(e) => setText(e.target.value)}
					onKeyDown={(e) => {
						if (e.key === 'Enter' && !e.shiftKey) {
							e.preventDefault()
							send()
						}
					}}
					placeholder="e.g. Add a reviewer sub-agent after dev."
					rows={2}
					disabled={aiEditing}
				/>
				<button
					className="btn btn-primary btn-icon"
					onClick={send}
					disabled={aiEditing || !text.trim()}
					aria-label="Send"
				>
					<Send size={14} />
				</button>
			</div>
		</div>
	)
}
