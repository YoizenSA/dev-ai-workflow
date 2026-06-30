import { useEffect, useRef } from 'react'
import { X, MessageSquare } from 'lucide-react'
import { useCommentaryStore, type CommentaryEventType } from '../../stores/commentaryStore'

// CommentaryPanel renders the live "narration" feed for a workflow run. Each
// output line from the orchestrator is classified (assistant / tool_use / error
// / summary) and shown as a colored entry, so the panel reads like a running
// commentary of what the agent is doing.
//
// Unlike a narration panel, this does NOT use a second LLM to narrate; the feed
// is parsed from the run's own output stream (see commentaryStore.pushLine).
export default function CommentaryPanel({ onClose }: { onClose: () => void }) {
	const entries = useCommentaryStore((s) => s.entries)
	const processing = useCommentaryStore((s) => s.processing)

	const endRef = useRef<HTMLDivElement>(null)
	useEffect(() => {
		endRef.current?.scrollIntoView({ behavior: 'smooth', block: 'end' })
	}, [entries.length])

	return (
		<div className="wf-commentary" data-tour="commentary">
			<div className="wf-commentary-header">
				<span className="wf-commentary-title">
					<MessageSquare size={14} /> Commentary
					{processing && <span className="wf-run-live">● live</span>}
				</span>
				<button className="btn btn-icon" onClick={onClose} aria-label="Close commentary">
					<X size={14} />
				</button>
			</div>
			<div className="wf-commentary-body">
				{entries.length === 0 ? (
					<div className="empty">Waiting for the agent to start…</div>
				) : (
					entries.map((e) => (
						<div key={e.id} className={`wf-commentary-entry wf-ce-${e.eventType}`}>
							<span className="wf-ce-tag">{labelFor(e.eventType)}</span>
							<span className="wf-ce-text">{e.text}</span>
						</div>
					))
				)}
				{processing && <div className="wf-commentary-typing">…</div>}
				<div ref={endRef} />
			</div>
		</div>
	)
}

function labelFor(t: CommentaryEventType): string {
	switch (t) {
		case 'tool_use':
			return 'tool'
		case 'error':
			return 'error'
		case 'summary':
			return 'done'
		default:
			return 'agent'
	}
}
