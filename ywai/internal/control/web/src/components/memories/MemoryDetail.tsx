import { useState } from 'react'
import type { EngramObservation } from '../../api/types'
import { useMemoriesStore } from '../../stores/memoriesStore'

interface Props {
	observation: EngramObservation
}

const TYPES = [
	'pattern',
	'observation',
	'decision',
	'architecture',
	'bugfix',
	'preference',
	'config',
	'discovery',
	'summary',
	'topic',
]

const SCOPES = ['project', 'personal', 'global']

export default function MemoryDetail({ observation }: Props) {
	const o = observation
	const [editing, setEditing] = useState(false)
	const [title, setTitle] = useState(o.title ?? '')
	const [content, setContent] = useState(o.content ?? '')
	const [type, setType] = useState(o.type ?? '')
	const [scope, setScope] = useState(o.scope ?? 'project')
	const [project, setProject] = useState(o.project ?? '')
	const [topicKey, setTopicKey] = useState(o.topic_key ?? '')
	const updateObservation = useMemoriesStore((s) => s.updateObservation)
	const deleteObservation = useMemoriesStore((s) => s.deleteObservation)

	const startEdit = () => {
		setTitle(o.title ?? '')
		setContent(o.content ?? '')
		setType(o.type ?? '')
		setScope(o.scope ?? 'project')
		setProject(o.project ?? '')
		setTopicKey(o.topic_key ?? '')
		setEditing(true)
	}

	const saveEdit = async () => {
		await updateObservation(String(o.id), {
			title,
			content,
			type,
			scope,
			project,
			topic_key: topicKey,
		})
		setEditing(false)
	}

	return (
		<div className="card card-pad memory-detail-panel">
			<div className="memory-detail-header">
				<div className="memory-detail-meta">
					<span className="muted memory-detail-id">
						#{o.id}
						{o.sync_id ? ` · ${o.sync_id}` : ''}
					</span>
					<div className="memory-detail-pills">
						<span className="pill pill-info">{o.type ?? 'memory'}</span>
						{o.scope && <span className="pill pill-muted">{o.scope}</span>}
						{o.topic_key && (
							<span className="pill pill-info" title={o.topic_key}>
								{o.topic_key}
							</span>
						)}
						{(o.revision_count ?? 0) > 1 && (
							<span className="pill pill-primary">v{o.revision_count}</span>
						)}
					</div>
				</div>
				<div className="memory-detail-actions">
					{!editing && (
						<button className="btn btn-ghost btn-sm" onClick={startEdit}>
							Edit
						</button>
					)}
					{editing && (
						<>
							<button className="btn btn-primary btn-sm" onClick={saveEdit}>
								Save
							</button>
							<button
								className="btn btn-ghost btn-sm"
								onClick={() => setEditing(false)}
							>
								Cancel
							</button>
						</>
					)}
					<button
						className="btn btn-danger btn-sm"
						onClick={() => {
							if (confirm('Delete this memory?'))
								deleteObservation(String(o.id))
						}}
					>
						Delete
					</button>
				</div>
			</div>

			{editing ? (
				<div className="memory-detail-form">
					<label className="memory-field">
						<span className="memory-field-label">Title</span>
						<input
							className="input"
							value={title}
							onChange={(e) => setTitle(e.target.value)}
						/>
					</label>
					<div className="memory-field-row">
						<label className="memory-field">
							<span className="memory-field-label">Type</span>
							<select
								className="input"
								value={type}
								onChange={(e) => setType(e.target.value)}
							>
								<option value="">—</option>
								{TYPES.map((t) => (
									<option key={t} value={t}>
										{t}
									</option>
								))}
							</select>
						</label>
						<label className="memory-field">
							<span className="memory-field-label">Scope</span>
							<select
								className="input"
								value={scope}
								onChange={(e) => setScope(e.target.value)}
							>
								{SCOPES.map((s) => (
									<option key={s} value={s}>
										{s}
									</option>
								))}
							</select>
						</label>
					</div>
					<div className="memory-field-row">
						<label className="memory-field">
							<span className="memory-field-label">Project</span>
							<input
								className="input"
								value={project}
								onChange={(e) => setProject(e.target.value)}
							/>
						</label>
						<label className="memory-field">
							<span className="memory-field-label">Topic key</span>
							<input
								className="input"
								value={topicKey}
								onChange={(e) => setTopicKey(e.target.value)}
								placeholder="architecture/auth-model"
							/>
						</label>
					</div>
					<label className="memory-field">
						<span className="memory-field-label">Content</span>
						<textarea
							className="textarea"
							value={content}
							onChange={(e) => setContent(e.target.value)}
							rows={12}
						/>
					</label>
				</div>
			) : (
				<>
					{o.title && (
						<h3 className="memory-detail-title">{o.title}</h3>
					)}
					<p className="memory-detail-content">{o.content}</p>
					<div className="memory-detail-foot muted">
						{o.project && <span>Project: {o.project}</span>}
						{o.session_id && <span>Session: {o.session_id}</span>}
						{o.created_at && <span>Created: {o.created_at}</span>}
					</div>
				</>
			)}
		</div>
	)
}
