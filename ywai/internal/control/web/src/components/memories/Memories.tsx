import { useEffect, useState, useCallback, useMemo, useRef } from 'react'
import { useUrlTab } from '../../hooks/useUrlTab'
import { useMemoriesStore } from '../../stores/memoriesStore'
import { useWebSocket } from '../../hooks/useWebSocket'
import type {
	WSMessage,
	EngramObservation,
	EngramSession,
	EngramPrompt,
} from '../../api/types'
import MemoryCard from './MemoryCard'
import MemoryDetail from './MemoryDetail'
import CaptureMemoryModal from './CaptureMemoryModal'
import ConsolidationModal from './ConsolidationModal'
import SettingsModal from './SettingsModal'
import './Memories.css'

type SubTab =
	| 'memories'
	| 'sessions'
	| 'topics'
	| 'timeline'
	| 'prompts'
	| 'empty'
	| 'context'

const SUB_TABS: { id: SubTab; label: string }[] = [
	{ id: 'memories', label: 'Memories' },
	{ id: 'sessions', label: 'Sessions' },
	{ id: 'topics', label: 'Topics' },
	{ id: 'timeline', label: 'Timeline' },
	{ id: 'prompts', label: 'Prompts' },
	{ id: 'empty', label: 'Empty' },
	{ id: 'context', label: 'Context' },
]

const TYPE_DOT: Record<string, string> = {
	pattern: 'var(--success)',
	observation: 'var(--tint-purple-dot)',
	decision: 'var(--warning)',
	architecture: 'var(--warning)',
	bugfix: 'var(--danger)',
	discovery: 'var(--info)',
	preference: 'var(--info)',
	config: 'var(--text-muted)',
	summary: 'var(--yz-accent)',
	topic: 'var(--info)',
}

const POLL_INTERVAL_MS = 5000

function dayBucket(iso?: string): string {
	if (!iso) return 'Unknown'
	const d = new Date(iso.replace(' ', 'T') + 'Z')
	if (isNaN(d.getTime())) return 'Unknown'
	const today = new Date()
	today.setHours(0, 0, 0, 0)
	const day = new Date(d)
	day.setHours(0, 0, 0, 0)
	const diff = (today.getTime() - day.getTime()) / 86400000
	if (diff === 0) return 'Today'
	if (diff === 1) return 'Yesterday'
	return day.toLocaleDateString(undefined, {
		weekday: 'short',
		month: 'short',
		day: 'numeric',
	})
}

function formatDateTime(iso?: string): string {
	if (!iso) return ''
	const d = new Date(iso.replace(' ', 'T') + 'Z')
	if (isNaN(d.getTime())) return iso
	return d.toLocaleString()
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

export default function Memories() {
	const [tab, setTab] = useUrlTab<SubTab>('memories', SUB_TABS.map((t) => t.id))
	const [showCapture, setShowCapture] = useState(false)
	const [showConsolidate, setShowConsolidate] = useState(false)
	const [consolidateScope, setConsolidateScope] = useState<
		{ topic_key?: string; project?: string } | undefined
	>(undefined)
	const [showSettings, setShowSettings] = useState(false)
	const [editingContext, setEditingContext] = useState<string | null>(null)
	const [savingContext, setSavingContext] = useState(false)
	const [expandedSession, setExpandedSession] = useState<string | null>(null)
	const [expandedTopic, setExpandedTopic] = useState<string | null>(null)
	const [promptQuery, setPromptQuery] = useState('')
	const [emptyQuery, setEmptyQuery] = useState('')
	const [topicQuery, setTopicQuery] = useState('')
	const searchInputRef = useRef<HTMLInputElement>(null)

	const {
		observations,
		selectedObservation,
		sessions,
		prompts,
		context,
		stats,
		engramStatus,
		filters,
		loading,
		loadingPrompts,
		fetchStatus,
		fetchObservations,
		applyFilters,
		setFilter,
		resetFilters,
		fetchStats,
		fetchSessions,
		fetchPrompts,
		fetchContext,
		saveContext,
		selectObservation,
		deleteSession,
		deletePrompt,
	} = useMemoriesStore()

	const handleWSMessage = useCallback((msg: WSMessage) => {
		useMemoriesStore.getState().handleWSMessage(msg)
	}, [])
	useWebSocket('/missions/engram/ws', handleWSMessage)

	// Initial load + tab-driven loads
	useEffect(() => {
		fetchStatus()
		fetchObservations()
		fetchStats()
		fetchSessions()
	}, [fetchStatus, fetchObservations, fetchStats, fetchSessions])

	useEffect(() => {
		if (tab === 'topics') fetchObservations(200)
		if (tab === 'timeline') fetchObservations(200)
		if (tab === 'context') fetchContext()
		if (tab === 'prompts') fetchPrompts()
		if (tab === 'empty') fetchSessions()
	}, [tab, fetchObservations, fetchContext, fetchPrompts, fetchSessions])

	// Health polling
	useEffect(() => {
		const id = window.setInterval(fetchStatus, POLL_INTERVAL_MS)
		return () => window.clearInterval(id)
	}, [fetchStatus])

	// Esc: close detail / modal
	useEffect(() => {
		const onKey = (e: KeyboardEvent) => {
			if (e.key !== 'Escape') return
			if (showCapture || showConsolidate || showSettings) return
			if (selectedObservation) selectObservation(null)
		}
		window.addEventListener('keydown', onKey)
		return () => window.removeEventListener('keydown', onKey)
	}, [selectedObservation, selectObservation, showCapture, showConsolidate, showSettings])

	// Cmd/Ctrl-K focuses the search input
	useEffect(() => {
		const onKey = (e: KeyboardEvent) => {
			if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
				e.preventDefault()
				setTab('memories')
				setTimeout(() => searchInputRef.current?.focus(), 0)
			}
		}
		window.addEventListener('keydown', onKey)
		return () => window.removeEventListener('keydown', onKey)
	}, [])

	const connected = engramStatus?.connected ?? false

	// Derived data
	const typeOptions = useMemo(() => {
		const set = new Set<string>()
		observations.forEach((o) => o.type && set.add(o.type))
		return Array.from(set).sort()
	}, [observations])

	const scopeOptions = useMemo(() => {
		const set = new Set<string>()
		observations.forEach((o) => o.scope && set.add(o.scope))
		return Array.from(set).sort()
	}, [observations])

	const filteredObservations = useMemo(() => {
		return observations.filter((o) => {
			if (filters.project && o.project !== filters.project) return false
			if (filters.type && o.type !== filters.type) return false
			if (filters.scope && o.scope !== filters.scope) return false
			return true
		})
	}, [observations, filters])

	const topicGroups = useMemo(() => {
		const groups = new Map<string, EngramObservation[]>()
		observations.forEach((o) => {
			const key = o.topic_key || '(no topic)'
			const arr = groups.get(key) ?? []
			arr.push(o)
			groups.set(key, arr)
		})
		const query = topicQuery.toLowerCase()
		return Array.from(groups.entries())
			.filter(([key, items]) => {
				if (!query) return true
				if (key.toLowerCase().includes(query)) return true
				return items.some((o) => o.title?.toLowerCase().includes(query))
			})
			.sort((a, b) => b[1].length - a[1].length)
	}, [observations, topicQuery])

	// Consolidation candidates: topics with the most revisions across their
	// observations. A high sum suggests the same idea was re-saved many times
	// and is worth merging into a single canonical entry. (no topic) is skipped.
	const consolidationCandidates = useMemo(() => {
		return topicGroups
			.filter(([key]) => key !== '(no topic)')
			.map(([key, items]) => ({
				topic_key: key,
				items,
				revisions: items.reduce(
					(acc, o) => acc + (o.revision_count ?? 1),
					0,
				),
				duplicates: items.reduce(
					(acc, o) => acc + Math.max(0, (o.duplicate_count ?? 0) - 1),
					0,
				),
			}))
			.filter((c) => c.revisions > c.items.length || c.duplicates > 0)
			.sort((a, b) => b.revisions + b.duplicates - (a.revisions + a.duplicates))
			.slice(0, 5)
	}, [topicGroups])

	const openConsolidateForTopic = (topic_key: string) => {
		setConsolidateScope({ topic_key })
		setShowConsolidate(true)
	}

	const observationsByDay = useMemo(() => {
		const groups = new Map<string, EngramObservation[]>()
		observations.forEach((o) => {
			const key = dayBucket(o.created_at)
			const arr = groups.get(key) ?? []
			arr.push(o)
			groups.set(key, arr)
		})
		// Preserve insertion order; engram already returns observations sorted
		// descending by created_at, so days fall out naturally newest-first.
		return Array.from(groups.entries())
	}, [observations])

	const filteredPrompts = useMemo(() => {
		const q = promptQuery.toLowerCase()
		if (!q) return prompts
		return prompts.filter(
			(p) =>
				(p.content ?? '').toLowerCase().includes(q) ||
				(p.project ?? '').toLowerCase().includes(q) ||
				(p.session_id ?? '').toLowerCase().includes(q),
		)
	}, [prompts, promptQuery])

	const sessionsWithObs = useMemo(
		() => sessions.filter((s) => (s.observation_count ?? 0) > 0),
		[sessions],
	)

	const emptySessions = useMemo(() => {
		const empties = sessions.filter((s) => (s.observation_count ?? 0) === 0)
		const q = emptyQuery.toLowerCase()
		if (!q) return empties
		return empties.filter(
			(s) =>
				s.id.toLowerCase().includes(q) ||
				(s.project ?? '').toLowerCase().includes(q),
		)
	}, [sessions, emptyQuery])

	const projects = stats?.projects ?? []
	const topicsCount = topicGroups.filter(
		([k]) => k !== '(no topic)',
	).length

	const activeFilterChips: { key: keyof typeof filters; label: string }[] = []
	if (filters.query)
		activeFilterChips.push({ key: 'query', label: `query: ${filters.query}` })
	if (filters.project)
		activeFilterChips.push({ key: 'project', label: `project: ${filters.project}` })
	if (filters.type)
		activeFilterChips.push({ key: 'type', label: `type: ${filters.type}` })
	if (filters.scope)
		activeFilterChips.push({ key: 'scope', label: `scope: ${filters.scope}` })
	if (filters.limit !== 50)
		activeFilterChips.push({ key: 'limit', label: `limit: ${filters.limit}` })

	return (
		<div className="memories">
			<header className="page-header">
				<div className="page-heading">
					<span className="page-eyebrow">Memories</span>
					<h1 className="page-title">Memory Management</h1>
					<p className="page-subtitle">
						Explore, capture, and consolidate engram memories.
					</p>
				</div>
				<div className="page-actions">
					<span
						className={`status-dot${connected ? ' on' : ' off'}`}
						title={
							connected
								? 'Engram is online'
								: engramStatus?.error
									? `Engram offline: ${engramStatus.error}`
									: 'Engram is offline'
						}
					>
						<span className="status-dot-inner" />
						<span className="status-dot-label">
							{connected ? 'Online' : 'Offline'}
						</span>
					</span>
					<button
						className="btn btn-ghost"
						onClick={() => setShowSettings(true)}
						title="Export, import, merge"
					>
						⚙
					</button>
					<button
						className="btn btn-outline"
						onClick={() => setShowCapture(true)}
					>
						+ Capture
					</button>
					<button
						className="btn btn-accent"
						onClick={() => setShowConsolidate(true)}
						disabled={!connected}
						data-tip={
							!connected ? 'Engram is not available' : 'Consolidate memories'
						}
					>
						⚡ Consolidate
					</button>
				</div>
			</header>

			{!connected && (
				<div className="alert alert-warning">
					Engram is not available. Initialize it with{' '}
					<code>engram serve</code>.
				</div>
			)}

			<div className="kpi-grid">
				<div className="kpi">
					<div className="kpi-top">
						<div className="kpi-value tnum">{projects.length || '—'}</div>
					</div>
					<div className="kpi-label">Projects</div>
				</div>
				<div className="kpi">
					<div className="kpi-top">
						<div className="kpi-value tnum">
							{stats?.total_sessions ?? '—'}
						</div>
					</div>
					<div className="kpi-label">Sessions</div>
				</div>
				<div className="kpi">
					<div className="kpi-top">
						<div className="kpi-value tnum">
							{stats?.total_observations ?? '—'}
						</div>
					</div>
					<div className="kpi-label">Memories</div>
				</div>
				<div className="kpi">
					<div className="kpi-top">
						<div className="kpi-value tnum">{topicsCount || '—'}</div>
					</div>
					<div className="kpi-label">Topics</div>
				</div>
				<div className="kpi">
					<div className="kpi-top">
						<div className="kpi-value tnum">
							{stats?.total_prompts ?? '—'}
						</div>
					</div>
					<div className="kpi-label">Prompts</div>
				</div>
			</div>

			<div className="tabs">
				{SUB_TABS.map((t) => (
					<button
						key={t.id}
						className={`tab${tab === t.id ? ' active' : ''}`}
						onClick={() => setTab(t.id)}
					>
						{t.label}
					</button>
				))}
			</div>

			{tab === 'memories' && (
				<section className="memories-section">
					<div className="memories-toolbar">
						<input
							ref={searchInputRef}
							className="input memories-search"
							placeholder="🔍 search memories…  (⌘K)"
							value={filters.query}
							onChange={(e) => setFilter('query', e.target.value)}
							onKeyDown={(e) => {
								if (e.key === 'Enter') applyFilters()
							}}
						/>
						<select
							className="input memories-select"
							value={filters.project}
							onChange={(e) => {
								setFilter('project', e.target.value)
							}}
						>
							<option value="">All projects</option>
							{projects.map((p) => (
								<option key={p} value={p}>
									{p}
								</option>
							))}
						</select>
						<select
							className="input memories-select"
							value={filters.type}
							onChange={(e) => setFilter('type', e.target.value)}
						>
							<option value="">All types</option>
							{typeOptions.map((t) => (
								<option key={t} value={t}>
									{t}
								</option>
							))}
						</select>
						<select
							className="input memories-select"
							value={filters.scope}
							onChange={(e) => setFilter('scope', e.target.value)}
						>
							<option value="">All scopes</option>
							{scopeOptions.map((s) => (
								<option key={s} value={s}>
									{s}
								</option>
							))}
						</select>
						<select
							className="input memories-select"
							value={filters.limit}
							onChange={(e) =>
								setFilter('limit', Number(e.target.value) as 20 | 50 | 100)
							}
						>
							<option value={20}>20</option>
							<option value={50}>50</option>
							<option value={100}>100</option>
						</select>
						<button
							className="btn btn-primary btn-sm"
							onClick={() => applyFilters()}
						>
							Apply
						</button>
						<button
							className="btn btn-ghost btn-sm"
							onClick={() => resetFilters()}
						>
							Clear
						</button>
					</div>

					{activeFilterChips.length > 0 && (
						<div className="filter-chips">
							{activeFilterChips.map((c) => (
								<button
									key={c.key}
									className="filter-chip"
									onClick={() => {
										if (c.key === 'limit') setFilter('limit', 50)
										else setFilter(c.key, '')
										applyFilters()
									}}
								>
									{c.label} <span className="chip-x">×</span>
								</button>
							))}
							<button className="chip-clear" onClick={() => resetFilters()}>
								Clear all
							</button>
						</div>
					)}

					<div className="memories-split">
						<div className="memories-list">
							{loading && filteredObservations.length === 0 ? (
								<div className="loading-inline">
									<div className="spinner" />
									<span>Loading memories…</span>
								</div>
							) : filteredObservations.length === 0 ? (
								<div className="empty-state">
									<div className="empty-title">No memories</div>
									<div className="empty-desc">
										Capture a new one or consolidate existing ones.
									</div>
								</div>
							) : (
								<>
									<div className="memories-list-count muted">
										{filteredObservations.length} of {observations.length}
									</div>
									{filteredObservations.map((o) => (
										<MemoryCard
											key={o.id}
											observation={o}
											active={selectedObservation?.id === o.id}
											onClick={() => selectObservation(o)}
										/>
									))}
								</>
							)}
						</div>
						<div className="memories-detail">
							{selectedObservation ? (
								<MemoryDetail observation={selectedObservation} />
							) : (
								<div className="empty-state">
									<div className="empty-title">Select a memory</div>
									<div className="empty-desc">
										Press <kbd>Esc</kbd> to close the detail panel.
									</div>
								</div>
							)}
						</div>
					</div>
				</section>
			)}

			{tab === 'sessions' && (
				<section className="memories-section">
					<table className="table sessions-table">
						<thead>
							<tr>
								<th>Session</th>
								<th>Project</th>
								<th>Started</th>
								<th className="num">Observations</th>
								<th></th>
							</tr>
						</thead>
						<tbody>
							{sessionsWithObs.length === 0 ? (
								<tr>
									<td colSpan={5}>
										<div className="empty-state">
											<div className="empty-title">No sessions yet</div>
										</div>
									</td>
								</tr>
							) : (
								sessionsWithObs.map((s) => (
									<SessionRow
										key={s.id}
										session={s}
										expanded={expandedSession === s.id}
										onToggle={() =>
											setExpandedSession(
												expandedSession === s.id ? null : s.id,
											)
										}
									/>
								))
							)}
						</tbody>
					</table>
				</section>
			)}

			{tab === 'topics' && (
				<section className="memories-section">
					<div className="memories-toolbar">
						<input
							className="input"
							placeholder="🔍 search topic keys…"
							value={topicQuery}
							onChange={(e) => setTopicQuery(e.target.value)}
						/>
					</div>

					{consolidationCandidates.length > 0 && (
						<div className="card card-pad candidates-card">
							<div className="candidates-head">
								<h3 className="candidates-title">
									⚡ Consolidation candidates
								</h3>
								<span className="muted small">
									Topics with the most revisions/duplicates
								</span>
							</div>
							<div className="candidates-list">
								{consolidationCandidates.map((c) => (
									<div key={c.topic_key} className="candidate-row">
										<div className="candidate-main">
											<strong className="topic-key">{c.topic_key}</strong>
											<div className="candidate-meta">
												<span className="pill pill-info">
													{c.items.length} obs
												</span>
												<span className="pill pill-primary">
													{c.revisions} revisions
												</span>
												{c.duplicates > 0 && (
													<span className="pill pill-warning">
														+{c.duplicates} duplicates
													</span>
												)}
											</div>
										</div>
										<button
											className="btn btn-accent btn-sm"
											onClick={() => openConsolidateForTopic(c.topic_key)}
											disabled={!connected}
										>
											⚡ Consolidate
										</button>
									</div>
								))}
							</div>
						</div>
					)}
					{topicGroups.length === 0 ? (
						<div className="empty-state">
							<div className="empty-title">No topics found</div>
						</div>
					) : (
						<div className="topic-list">
							{topicGroups.map(([key, items]) => {
								const isOpen = expandedTopic === key
								const types = Array.from(
									new Set(items.map((i) => i.type).filter(Boolean)),
								).slice(0, 3) as string[]
								const projectsInTopic = Array.from(
									new Set(items.map((i) => i.project).filter(Boolean)),
								) as string[]
								return (
									<div key={key} className="topic-group card card-pad">
										<div className="topic-group-head-row">
											<button
												className="topic-group-head"
												onClick={() =>
													setExpandedTopic(isOpen ? null : key)
												}
											>
												<span className={`chevron${isOpen ? ' open' : ''}`}>
													▶
												</span>
												<strong className="topic-key">{key}</strong>
												<div className="topic-meta">
													{types.map((t) => (
														<span key={t} className="pill pill-info">
															{t}
														</span>
													))}
													{projectsInTopic.length > 0 && (
														<span className="pill pill-muted">
															{projectsInTopic.join(', ')}
														</span>
													)}
													<span className="pill pill-muted">
														{items.length}{' '}
														{items.length === 1 ? 'item' : 'items'}
													</span>
												</div>
											</button>
											{key !== '(no topic)' && (
												<button
													className="btn btn-ghost btn-sm topic-consolidate"
													onClick={(e) => {
														e.stopPropagation()
														openConsolidateForTopic(key)
													}}
													disabled={!connected}
													title="Consolidate this topic only"
												>
													⚡
												</button>
											)}
										</div>
										{isOpen && (
											<div className="topic-group-items">
												{items
													.sort((a, b) =>
														(b.updated_at ?? '').localeCompare(
															a.updated_at ?? '',
														),
													)
													.map((o) => (
														<button
															key={o.id}
															className="topic-item"
															onClick={() => {
																selectObservation(o)
																setTab('memories')
															}}
														>
															<div className="topic-item-main">
																<strong>
																	{o.title ?? 'Untitled'}
																</strong>
																<span className="muted">
																	{formatRelative(o.updated_at)}
																</span>
															</div>
															<div className="topic-item-foot">
																{o.type && (
																	<span className="pill pill-info">
																		{o.type}
																	</span>
																)}
																{(o.revision_count ?? 0) > 1 && (
																	<span className="pill pill-primary">
																		v{o.revision_count}
																	</span>
																)}
															</div>
														</button>
													))}
											</div>
										)}
									</div>
								)
							})}
						</div>
					)}
				</section>
			)}

			{tab === 'timeline' && (
				<section className="memories-section">
					{observations.length === 0 ? (
						<div className="empty-state">
							<div className="empty-title">No observations yet</div>
							<div className="empty-desc">
								Capture a memory to start a timeline.
							</div>
						</div>
					) : (
						<div className="timeline-list">
							{observationsByDay.map(([day, items]) => (
								<div key={day} className="timeline-day">
									<h3 className="timeline-day-title">
										{day}
										<span className="timeline-day-count">{items.length}</span>
									</h3>
									<div className="timeline-events">
										{items.map((o) => (
											<button
												key={o.id}
												className="timeline-event"
												onClick={() => {
													selectObservation(o)
													setTab('memories')
												}}
											>
												<span
													className="timeline-dot"
													style={{
														background:
															TYPE_DOT[o.type ?? ''] ?? 'var(--text-muted)',
													}}
												/>
												<div className="timeline-event-body">
													<div className="timeline-event-head">
														{o.type && (
															<span className="pill pill-info">{o.type}</span>
														)}
														{o.project && (
															<span className="pill pill-muted">
																{o.project}
															</span>
														)}
														{(o.revision_count ?? 0) > 1 && (
															<span className="pill pill-primary">
																v{o.revision_count}
															</span>
														)}
														<span className="muted ml-auto">
															{formatDateTime(o.created_at)}
														</span>
													</div>
													{o.title && (
														<strong className="timeline-event-title">
															{o.title}
														</strong>
													)}
													<p className="timeline-event-content">
														{o.content}
													</p>
												</div>
											</button>
										))}
									</div>
								</div>
							))}
						</div>
					)}
				</section>
			)}

			{tab === 'prompts' && (
				<section className="memories-section">
					<div className="memories-toolbar">
						<input
							className="input"
							placeholder="🔍 search prompts…"
							value={promptQuery}
							onChange={(e) => setPromptQuery(e.target.value)}
						/>
					</div>
					{loadingPrompts && filteredPrompts.length === 0 ? (
						<div className="loading-inline">
							<div className="spinner" />
							<span>Loading prompts…</span>
						</div>
					) : filteredPrompts.length === 0 ? (
						<div className="empty-state">
							<div className="empty-title">No prompts found</div>
						</div>
					) : (
						<div className="prompt-list">
							{filteredPrompts.map((p) => (
								<PromptRow
									key={p.id}
									prompt={p}
									onDelete={() => {
										if (confirm('Delete this prompt?'))
											deletePrompt(String(p.id))
									}}
								/>
							))}
						</div>
					)}
				</section>
			)}

			{tab === 'empty' && (
				<section className="memories-section">
					<div className="memories-toolbar">
						<input
							className="input"
							placeholder="🔍 search empty sessions…"
							value={emptyQuery}
							onChange={(e) => setEmptyQuery(e.target.value)}
						/>
					</div>
					{emptySessions.length === 0 ? (
						<div className="empty-state">
							<div className="empty-title">
								No empty sessions{emptyQuery ? ' match the search' : ''}
							</div>
						</div>
					) : (
						<table className="table sessions-table">
							<thead>
								<tr>
									<th>Session</th>
									<th>Project</th>
									<th>Started</th>
									<th></th>
								</tr>
							</thead>
							<tbody>
								{emptySessions.map((s) => (
									<tr key={s.id}>
										<td className="mono">{s.id}</td>
										<td>{s.project ?? '—'}</td>
										<td>{formatDateTime(s.started_at)}</td>
										<td className="num">
											<button
												className="btn btn-danger btn-sm"
												onClick={() => {
													if (confirm('Delete this session?'))
														deleteSession(s.id)
												}}
											>
												Delete
											</button>
										</td>
									</tr>
								))}
							</tbody>
						</table>
					)}
				</section>
			)}

			{tab === 'context' && (
				<section className="memories-section">
					{!context && editingContext === null ? (
						<div className="loading-inline">
							<div className="spinner" />
							<span>Loading context…</span>
						</div>
					) : (
						<div className="card card-pad">
							<textarea
								className="input context-textarea"
								value={editingContext ?? context?.context ?? ''}
								onChange={(e) => setEditingContext(e.target.value)}
							/>
							<div className="context-actions">
								<button
									className="btn btn-accent btn-sm"
									disabled={savingContext || editingContext === null}
									onClick={async () => {
										if (editingContext === null) return
										setSavingContext(true)
										try {
											await saveContext(editingContext)
											setEditingContext(null)
										} finally {
											setSavingContext(false)
										}
									}}
								>
									{savingContext ? 'Saving…' : 'Save'}
								</button>
								{editingContext !== null && (
									<button
										className="btn btn-ghost btn-sm"
										onClick={() => setEditingContext(null)}
									>
										Cancel
									</button>
								)}
							</div>
						</div>
					)}
				</section>
			)}

			<CaptureMemoryModal
				open={showCapture}
				onClose={() => setShowCapture(false)}
			/>
			<ConsolidationModal
				open={showConsolidate}
				onClose={() => {
					setShowConsolidate(false)
					setConsolidateScope(undefined)
				}}
				initialScope={consolidateScope}
			/>
			<SettingsModal
				open={showSettings}
				onClose={() => setShowSettings(false)}
			/>
		</div>
	)
}

function SessionRow({
	session,
	expanded,
	onToggle,
}: {
	session: EngramSession
	expanded: boolean
	onToggle: () => void
}) {
	const observations = useMemoriesStore((s) => s.observations)
	const selectObservation = useMemoriesStore((s) => s.selectObservation)
	const items = useMemo(
		() => observations.filter((o) => o.session_id === session.id),
		[observations, session.id],
	)
	return (
		<>
			<tr className="session-row" onClick={onToggle}>
				<td className="mono">
					<span className={`chevron${expanded ? ' open' : ''}`}>▶</span>{' '}
					{session.id.slice(0, 24)}…
				</td>
				<td>{session.project ?? '—'}</td>
				<td>{formatDateTime(session.started_at)}</td>
				<td className="num">{session.observation_count}</td>
				<td></td>
			</tr>
			{expanded && (
				<tr className="session-row-detail">
					<td colSpan={5}>
						{items.length === 0 ? (
							<div className="muted small">
								No observations loaded for this session.
							</div>
						) : (
							<div className="session-detail-list">
								{items.map((o) => (
									<button
										key={o.id}
										className="session-detail-item"
										onClick={() => selectObservation(o)}
									>
										<div className="row" style={{ gap: 'var(--space-2)' }}>
											{o.type && (
												<span className="pill pill-info">{o.type}</span>
											)}
											{o.topic_key && (
												<span className="pill pill-muted">{o.topic_key}</span>
											)}
											<strong className="ellipsis">
												{o.title ?? 'Untitled'}
											</strong>
											<span className="muted ml-auto small">
												{formatRelative(o.created_at)}
											</span>
										</div>
										<p className="muted small ellipsis-2">{o.content}</p>
									</button>
								))}
							</div>
						)}
					</td>
				</tr>
			)}
		</>
	)
}

function PromptRow({
	prompt,
	onDelete,
}: {
	prompt: EngramPrompt
	onDelete: () => void
}) {
	return (
		<div className="prompt-row card card-pad">
			<div className="prompt-content">{prompt.content}</div>
			<div className="prompt-foot">
				<span className="muted small">
					{prompt.project ?? '—'}
					{prompt.session_id ? ` · ${prompt.session_id.slice(0, 18)}…` : ''}
				</span>
				<span className="muted small">{formatRelative(prompt.created_at)}</span>
				<button className="btn btn-danger btn-sm" onClick={onDelete}>
					Delete
				</button>
			</div>
		</div>
	)
}
