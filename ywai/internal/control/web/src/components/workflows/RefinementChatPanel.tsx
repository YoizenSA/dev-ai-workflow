import { useEffect, useRef, useState } from 'react'
import { X, Send, AlertTriangle } from 'lucide-react'
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
	const chatError = useWorkflowStore((s) => s.chatError)
	const clearChatError = useWorkflowStore((s) => s.clearChatError)
	const models = useOpencodeModels()

	const [text, setText] = useState('')
	const [model, setModel] = useState('')
	const endRef = useRef<HTMLDivElement>(null)

	const messages = current?.conversationHistory?.messages ?? []
	const iterations = current?.conversationHistory?.currentIteration ?? 0
	const maxIterations = current?.conversationHistory?.maxIterations ?? 20
	// The x/y in the header is an edit-round counter, not a character limit.
	// Disable further edits once the cap is reached so the counter is honored.
	const limitReached = iterations >= maxIterations

	useEffect(() => {
		endRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' })
	}, [messages.length, aiEditing])

	const send = async () => {
		const t = text.trim()
		if (!t || aiEditing || limitReached) return
		setText('')
		// The store updates `current`; the canvas observes it via selector and
		// re-renders automatically.
		await aiEdit(t, model || undefined)
	}

	return (
		<div className="wf-chat-panel" data-tour="refinement-chat">
			<div className="wf-chat-header">
				<span className="wf-chat-title">Edit with AI</span>
				<span
					className="wf-chat-count"
					title="Edit rounds used / max"
					aria-label={`${iterations} of ${maxIterations} edit rounds used`}
				>
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
				{limitReached && (
					<div className="wf-chat-bubble wf-chat-typing">
						Edit limit reached ({maxIterations} rounds). Save and reopen the workflow to start a new session.
					</div>
				)}
				<div ref={endRef} />
			</div>

			{/* Chat-scoped error: rendered as a direct child of the panel (not inside
			    the scrolling body) so it stays visible and isn't clipped. Uses the
			    existing .validation-issue error styling (same --danger-* tokens). */}
			{chatError && (
				<div className="validation-issue error wf-chat-error" role="alert">
					<AlertTriangle size={14} />
					<span>{chatError}</span>
					<button className="btn btn-icon" onClick={clearChatError} aria-label="Dismiss error">
						<X size={12} />
					</button>
				</div>
			)}

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
					onChange={(e) => {
						setText(e.target.value)
						if (chatError) clearChatError()
					}}
					onKeyDown={(e) => {
						if (e.key === 'Enter' && !e.shiftKey) {
							e.preventDefault()
							send()
						}
					}}
					placeholder={limitReached ? 'Edit limit reached — save and reopen.' : 'e.g. Add a reviewer sub-agent after dev.'}
					rows={2}
					disabled={aiEditing || limitReached}
				/>
				<button
					className="btn btn-primary btn-icon"
					onClick={send}
					disabled={aiEditing || !text.trim() || limitReached}
					aria-label="Send"
				>
					<Send size={14} />
				</button>
			</div>
		</div>
	)
}
