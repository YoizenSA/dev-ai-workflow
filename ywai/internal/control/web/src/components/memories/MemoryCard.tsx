import type { EngramObservation } from '../../api/types'

interface Props {
	observation: EngramObservation
	active: boolean
	onClick: () => void
}

function pillClass(type?: string): string {
	switch (type) {
		case 'save':
		case 'pattern':
			return 'pill-success'
		case 'summary':
			return 'pill-accent'
		case 'topic':
			return 'pill-info'
		case 'observation':
			return 'pill-primary'
		case 'decision':
		case 'architecture':
			return 'pill-warning'
		case 'bugfix':
			return 'pill-danger'
		default:
			return 'pill-muted'
	}
}

function formatRelative(iso?: string): string {
	if (!iso) return ''
	const d = new Date(iso.replace(' ', 'T') + 'Z')
	if (isNaN(d.getTime())) return iso
	const diff = (Date.now() - d.getTime()) / 1000
	if (diff < 60) return 'just now'
	if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
	if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
	if (diff < 86400 * 30) return `${Math.floor(diff / 86400)}d ago`
	return d.toLocaleDateString()
}

export default function MemoryCard({ observation, active, onClick }: Props) {
	const o = observation
	const revisions = o.revision_count ?? 0
	return (
		<div
			className={`memory-card${active ? ' active' : ''}`}
			onClick={onClick}
		>
			<div className="memory-card-head">
				<div className="memory-card-pills">
					<span className={`pill ${pillClass(o.type)}`}>
						{o.type ?? 'memory'}
					</span>
					{o.scope && (
						<span className="pill pill-muted">{o.scope}</span>
					)}
					{o.topic_key && (
						<span className="pill pill-info" title={o.topic_key}>
							{o.topic_key}
						</span>
					)}
					{revisions > 1 && (
						<span className="pill pill-primary" title="Revisions">
							v{revisions}
						</span>
					)}
				</div>
				<span className="memory-card-time">{formatRelative(o.created_at)}</span>
			</div>
			{o.title && <strong className="memory-card-title">{o.title}</strong>}
			<div className="memory-card-body">
				{o.content ? o.content.slice(0, 180) : ''}
				{o.content && o.content.length > 180 ? '…' : ''}
			</div>
			<div className="memory-card-foot muted">
				{o.project && <span className="memory-card-project">{o.project}</span>}
				{o.session_id && (
					<span className="memory-card-session" title={o.session_id}>
						{o.session_id.slice(0, 14)}…
					</span>
				)}
			</div>
		</div>
	)
}
