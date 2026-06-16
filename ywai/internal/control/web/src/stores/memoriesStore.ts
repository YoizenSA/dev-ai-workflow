import { create } from 'zustand'
import type {
	EngramObservation,
	EngramSession,
	EngramPrompt,
	EngramStats,
	EngramTimelineEvent,
	EngramContextResult,
	EngramStatus,
	EngramImportResult,
	EngramMergeResult,
	MemoryFilters,
	ConsolidationRun,
	ApplySelection,
	WSMessage,
} from '../api/types'
import { memoriesApi } from '../api/client'

const DEFAULT_FILTERS: MemoryFilters = {
	query: '',
	project: '',
	type: '',
	scope: '',
	limit: 50,
}

interface MemoriesState {
	// data
	observations: EngramObservation[]
	selectedObservation: EngramObservation | null
	sessions: EngramSession[]
	prompts: EngramPrompt[]
	timeline: EngramTimelineEvent[]
	context: EngramContextResult | null
	stats: EngramStats | null
	engramStatus: EngramStatus | null
	consolidation: ConsolidationRun | null

	// filters
	filters: MemoryFilters

	// ui
	loading: boolean
	loadingPrompts: boolean
	error: string | null

	// actions — observations
	fetchStatus: () => Promise<void>
	fetchObservations: (limit?: number) => Promise<void>
	applyFilters: () => Promise<void>
	setFilter: <K extends keyof MemoryFilters>(key: K, value: MemoryFilters[K]) => void
	resetFilters: () => Promise<void>
	saveMemory: (data: {
		type: string
		content: string
		title?: string
		scope?: string
		project?: string
	}) => Promise<void>
	updateObservation: (
		id: string,
		data: {
			content?: string
			title?: string
			type?: string
			scope?: string
			project?: string
			topic_key?: string
		},
	) => Promise<void>
	deleteObservation: (id: string) => Promise<void>
	selectObservation: (obs: EngramObservation | null) => void

	// actions — stats / sessions / prompts / timeline / context
	fetchStats: () => Promise<void>
	fetchSessions: (limit?: number) => Promise<void>
	deleteSession: (id: string) => Promise<void>
	fetchPrompts: (limit?: number) => Promise<void>
	deletePrompt: (id: string) => Promise<void>
	fetchTimeline: (observationId?: string) => Promise<void>
	fetchContext: (q?: string) => Promise<void>
	saveContext: (text: string) => Promise<void>

	// actions — bulk
	exportAll: () => Promise<Blob>
	importData: (file: File) => Promise<EngramImportResult>
	mergeProjects: (source: string, target: string) => Promise<EngramMergeResult>

	// actions — consolidation
	startConsolidation: (
		model: string,
		agent: string,
		scope?: { topic_key?: string; project?: string },
	) => Promise<string | null>
	applyConsolidation: (id: string, sel: ApplySelection) => Promise<void>
	discardConsolidation: (id: string) => Promise<void>

	handleWSMessage: (msg: WSMessage) => void
}

export const useMemoriesStore = create<MemoriesState>((set, get) => ({
	observations: [],
	selectedObservation: null,
	sessions: [],
	prompts: [],
	timeline: [],
	context: null,
	stats: null,
	engramStatus: null,
	consolidation: null,
	filters: { ...DEFAULT_FILTERS },
	loading: false,
	loadingPrompts: false,
	error: null,

	fetchStatus: async () => {
		try {
			const status = await memoriesApi.status()
			set({ engramStatus: status })
		} catch {
			set({ engramStatus: { connected: false } })
		}
	},

	fetchObservations: async (limit) => {
		set({ loading: true, error: null })
		try {
			const observations = await memoriesApi.listObservations(
				limit ?? get().filters.limit,
			)
			set({ observations, loading: false })
		} catch (err) {
			set({ error: String(err), loading: false })
		}
	},

	applyFilters: async () => {
		const { query, type, limit } = get().filters
		set({ loading: true, error: null })
		try {
			const observations = query
				? await memoriesApi.search(query, limit, type || undefined)
				: await memoriesApi.listObservations(limit)
			set({ observations, loading: false })
		} catch (err) {
			set({ error: String(err), loading: false })
		}
	},

	setFilter: (key, value) => {
		set((s) => ({ filters: { ...s.filters, [key]: value } }))
	},

	resetFilters: async () => {
		set({ filters: { ...DEFAULT_FILTERS } })
		await get().fetchObservations(DEFAULT_FILTERS.limit)
	},

	saveMemory: async (data) => {
		await memoriesApi.save(data)
		await get().fetchObservations()
		await get().fetchStats()
	},

	updateObservation: async (id, data) => {
		const updated = await memoriesApi.updateObservation(id, data)
		set((s) => ({
			observations: s.observations.map((o) =>
				String(o.id) === id ? updated : o,
			),
			selectedObservation:
				String(s.selectedObservation?.id) === id
					? updated
					: s.selectedObservation,
		}))
	},

	deleteObservation: async (id) => {
		await memoriesApi.deleteObservation(id)
		set((s) => ({
			observations: s.observations.filter((o) => String(o.id) !== id),
			selectedObservation:
				String(s.selectedObservation?.id) === id ? null : s.selectedObservation,
		}))
		await get().fetchStats()
	},

	fetchStats: async () => {
		try {
			set({ stats: await memoriesApi.stats() })
		} catch (err) {
			set({ error: String(err) })
		}
	},

	fetchSessions: async (limit = 100) => {
		try {
			set({ sessions: await memoriesApi.listSessions(limit) })
		} catch (err) {
			set({ error: String(err) })
		}
	},

	deleteSession: async (id) => {
		await memoriesApi.deleteSession(id)
		set((s) => ({ sessions: s.sessions.filter((sess) => sess.id !== id) }))
		await get().fetchStats()
	},

	fetchPrompts: async (limit = 100) => {
		set({ loadingPrompts: true })
		try {
			set({ prompts: await memoriesApi.listPrompts(limit), loadingPrompts: false })
		} catch (err) {
			set({ error: String(err), loadingPrompts: false })
		}
	},

	deletePrompt: async (id) => {
		await memoriesApi.deletePrompt(id)
		set((s) => ({ prompts: s.prompts.filter((p) => String(p.id) !== id) }))
		await get().fetchStats()
	},

	fetchTimeline: async (observationId?: string) => {
		try {
			set({ timeline: await memoriesApi.timeline(observationId) })
		} catch (err) {
			set({ error: String(err) })
		}
	},

	fetchContext: async (q) => {
		try {
			set({ context: await memoriesApi.context(q) })
		} catch (err) {
			set({ error: String(err) })
		}
	},

	saveContext: async (text) => {
		try {
			set({ context: await memoriesApi.saveContext(text) })
		} catch (err) {
			set({ error: String(err) })
		}
	},

	selectObservation: (obs) => set({ selectedObservation: obs }),

	exportAll: () => memoriesApi.exportAll(),
	importData: async (file) => {
		const result = await memoriesApi.importData(file)
		await get().fetchStats()
		await get().fetchObservations()
		await get().fetchSessions()
		await get().fetchPrompts()
		return result
	},
	mergeProjects: async (source, target) => {
		const result = await memoriesApi.mergeProjects(source, target)
		await get().fetchObservations()
		await get().fetchStats()
		return result
	},

	startConsolidation: async (model, agent, scope) => {
		try {
			const { run_id } = await memoriesApi.startConsolidation({
				model,
				agent,
				topic_key: scope?.topic_key,
				project: scope?.project,
			})
			set({
				consolidation: {
					id: run_id,
					model,
					agent,
					status: 'running',
					scope:
						scope && (scope.topic_key || scope.project) ? scope : undefined,
				},
			})
			return run_id
		} catch (err) {
			set({ error: String(err) })
			return null
		}
	},

	applyConsolidation: async (id, sel) => {
		await memoriesApi.applyConsolidation(id, sel)
		set({ consolidation: null })
		await get().fetchObservations()
		await get().fetchStats()
	},

	discardConsolidation: async (id) => {
		await memoriesApi.discardConsolidation(id)
		set({ consolidation: null })
	},

	handleWSMessage: (msg) => {
		if (!msg.type.startsWith('consolidation.')) return
		const c = get().consolidation
		const payload = (msg.payload ?? {}) as Record<string, unknown>
		const runId = payload.run_id as string
		if (!c || c.id !== runId) return

		const status = String(payload.status ?? c.status)
		if (msg.type === 'consolidation.completed') {
			memoriesApi
				.getConsolidation(runId)
				.then((run) => set({ consolidation: run }))
		} else if (msg.type === 'consolidation.applied') {
			set({ consolidation: { ...c, status: 'applied' } })
			get().fetchObservations()
			get().fetchStats()
		} else if (msg.type === 'consolidation.failed') {
			set({
				consolidation: {
					...c,
					status: 'failed',
					error: String(payload.error ?? ''),
				},
			})
		} else {
			set({
				consolidation: {
					...c,
					status:
						status === 'progress'
							? 'running'
							: (status as ConsolidationRun['status']),
				},
			})
		}
	},
}))
