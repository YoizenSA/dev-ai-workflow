import { useEffect, useState, useCallback } from 'react'
import Modal from '../shared/Modal'
import ModelCombobox from '../missions/ModelCombobox'
import SearchSelect from '../shared/SearchSelect'
import { useMemoriesStore } from '../../stores/memoriesStore'
import { missionsApi } from '../../api/client'
import ConsolidationPlanReview from './ConsolidationPlanReview'
import type { ModelInfo } from '../../api/types'

interface Props {
	open: boolean
	onClose: () => void
	/** Optional starting scope. When provided the run is narrowed and the
	 *  inputs are pre-filled (the user can still clear them). */
	initialScope?: { topic_key?: string; project?: string }
}

export default function ConsolidationModal({ open, onClose, initialScope }: Props) {
	const [model, setModel] = useState('')
	const [agent, setAgent] = useState('memory')
	const [models, setModels] = useState<ModelInfo[]>([])
	const [agents, setAgents] = useState<string[]>([])
	const [loading, setLoading] = useState(false)
	const [topicKey, setTopicKey] = useState('')
	const [project, setProject] = useState('')

	useEffect(() => {
		if (open && initialScope) {
			setTopicKey(initialScope.topic_key ?? '')
			setProject(initialScope.project ?? '')
		}
	}, [open, initialScope])

	const consolidation = useMemoriesStore((s) => s.consolidation)
	const startConsolidation = useMemoriesStore((s) => s.startConsolidation)
	const projects = useMemoriesStore((s) => s.stats?.projects ?? [])

	const loadOptions = useCallback(async () => {
		setLoading(true)
		try {
			const [modelsRes, agentsRes] = await Promise.all([
				missionsApi.listModels().catch(() => null),
				missionsApi.listAgents().catch(() => null),
			])
			if (modelsRes) {
				const all = Object.values(modelsRes.modelsByProvider).flat() as ModelInfo[]
				setModels(all)
				// Use the functional setter so we don't need `model` in deps —
				// otherwise loadOptions gets recreated, the effect re-fires, and we
				// loop forever on the first selection.
				setModel((prev) => prev || all[0]?.id || '')
			}
			if (agentsRes) {
				const list = agentsRes.agents ?? []
				setAgents(list)
				// Make sure the current agent value is actually in the list —
				// otherwise the consolidation backend will fail with "unknown
				// agent" and the spinner hangs forever. Prefer "memory" if it
				// exists (purpose-built), else "general", else first.
				setAgent((prev) => {
					if (prev && list.includes(prev)) return prev
					if (list.includes('memory')) return 'memory'
					if (list.includes('general')) return 'general'
					return list[0] || prev
				})
			}
		} finally {
			setLoading(false)
		}
	}, [])

	useEffect(() => {
		if (open) loadOptions()
	}, [open, loadOptions])

	const running = consolidation?.status === 'running'
	const reviewing = consolidation?.status === 'awaiting_review'

	return (
		<Modal open={open} onClose={onClose} title="Consolidate Memories" width="640px">
			{!reviewing ? (
				<>
					<div className="field">
						<ModelCombobox
							id="cons-model"
							label="Model"
							value={model}
							models={models}
							onChange={setModel}
						/>
					</div>
					<div className="field">
						<label className="field-label">Agent</label>
						<SearchSelect
							value={agent}
							options={agents.length > 0 ? agents : ['memory']}
							onChange={setAgent}
						/>
					</div>

					<div className="field">
						<label className="field-label">
							Scope <span className="muted">(optional)</span>
						</label>
						<div className="row" style={{ gap: 'var(--space-2)' }}>
							<input
								className="input"
								placeholder="topic_key (e.g. architecture/auth)"
								value={topicKey}
								onChange={(e) => setTopicKey(e.target.value)}
								style={{ flex: 2 }}
							/>
							<select
								className="input"
								value={project}
								onChange={(e) => setProject(e.target.value)}
								style={{ flex: 1 }}
							>
								<option value="">All projects</option>
								{projects.map((p) => (
									<option key={p} value={p}>
										{p}
									</option>
								))}
							</select>
						</div>
						{(topicKey || project) && (
							<p className="muted" style={{ fontSize: '0.75rem', marginTop: 4 }}>
								Run narrowed to{' '}
								{topicKey && <strong>topic {topicKey}</strong>}
								{topicKey && project && ' · '}
								{project && <strong>project {project}</strong>}
								. The agent will not propose changes outside this scope.
							</p>
						)}
					</div>

					{loading && models.length === 0 && (
						<div className="loading-inline" style={{ marginTop: 'var(--space-2)' }}>
							<div className="spinner" />
							<span>Loading models…</span>
						</div>
					)}

					{running && (
						<div className="alert alert-info" style={{ marginTop: 'var(--space-2)' }}>
							<div className="spinner" style={{ marginRight: 8 }} />
							Consolidating… the agent is analyzing memories.
						</div>
					)}
					{consolidation?.status === 'failed' && (
						<div className="alert alert-danger" style={{ marginTop: 'var(--space-2)' }}>
							Error: {consolidation.error}
						</div>
					)}

					<div
						className="row"
						style={{
							justifyContent: 'flex-end',
							gap: 'var(--space-2)',
							marginTop: 'var(--space-4)',
						}}
					>
						<button className="btn btn-ghost" onClick={onClose}>
							Close
						</button>
						<button
							className="btn btn-accent"
							disabled={running || !model}
							onClick={() =>
								startConsolidation(model, agent, {
									topic_key: topicKey || undefined,
									project: project || undefined,
								})
							}
						>
							{running ? 'Consolidating…' : 'Consolidate'}
						</button>
					</div>
				</>
			) : (
				consolidation?.plan && (
					<ConsolidationPlanReview
						plan={consolidation.plan}
						onDone={onClose}
					/>
				)
			)}
		</Modal>
	)
}
