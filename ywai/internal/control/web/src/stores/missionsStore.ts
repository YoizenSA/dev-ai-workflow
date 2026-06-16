import { create } from 'zustand'
import type { Mission, Project, PlanMission, WSMessage, FeatureLogLine } from '../api/types'
import { missionsApi } from '../api/client'

interface MissionsState {
  missions: Mission[]
  projects: Project[]
  selectedMission: Mission | null
  loading: boolean
  error: string | null
  featureLogs: Record<string, string[]>
  expandedFeatures: Set<string>
  reportContent: string
  reportLoading: boolean

  // Actions
  fetchMissions: () => Promise<void>
  fetchProjects: () => Promise<void>
  selectMission: (id: string) => void
  generatePlan: (
    goal: string,
    project?: string,
    model?: string,
    agent?: string,
  ) => Promise<PlanMission>
  approvePlan: (plan: PlanMission) => Promise<Mission>
  refineGoal: (goal: string, context?: string) => Promise<string>
  runMission: (id: string) => Promise<void>
  autoMission: (data: {
    goal: string
    project?: string
    model?: string
    agent?: string
    autoApprove?: boolean
  }) => Promise<string>
  pauseMission: (id: string) => Promise<void>
  resumeMission: (id: string) => Promise<void>
  cancelMission: (id: string) => Promise<void>
  deleteMission: (id: string) => Promise<void>
  fetchFeatureLogs: (missionId: string, featureId: string) => Promise<void>
  fetchReport: (missionId: string) => Promise<void>
  toggleFeatureExpanded: (featureId: string) => void
  handleWSMessage: (msg: WSMessage) => void
}

export const useMissionsStore = create<MissionsState>((set, get) => ({
  missions: [],
  projects: [],
  selectedMission: null,
  loading: false,
  error: null,
  featureLogs: {},
  expandedFeatures: new Set<string>(),
  reportContent: '',
  reportLoading: false,

  fetchMissions: async () => {
    set({ loading: true, error: null })
    try {
      const missions = await missionsApi.listMissions()
      set({ missions, loading: false })
    } catch (err) {
      set({ error: String(err), loading: false })
    }
  },

  fetchProjects: async () => {
    try {
      const projects = await missionsApi.listProjects()
      set({ projects })
    } catch {
      // Non-critical
    }
  },

  selectMission: (id: string) => {
    const mission = get().missions.find((m) => m.id === id) ?? null
    set({ selectedMission: mission, featureLogs: {}, expandedFeatures: new Set(), reportContent: '', reportLoading: false })
    // Fetch full detail (with features) if cached version is a summary
    if (mission && !mission.features) {
      missionsApi.getMission(id).then((detailed) => {
        set((s) => ({
          selectedMission: detailed,
          missions: s.missions.map((m) =>
            m.id === detailed.id ? detailed : m,
          ),
        }))
      }).catch(() => {
        // Silently fail — summary data is still usable
      })
    }
  },

  generatePlan: async (goal, project, model, agent) => {
    const { plan } = await missionsApi.generatePlan({ goal, project, model, agent })
    return plan
  },

  approvePlan: async (plan) => {
    const { mission } = await missionsApi.approvePlan(plan)
    set((s) => ({ missions: [mission, ...s.missions] }))
    return mission
  },

  refineGoal: async (goal, context) => {
    const { refined } = await missionsApi.refineGoal(goal, context)
    return refined
  },

  runMission: async (id) => {
    await missionsApi.runMission(id)
    set((s) => ({
      missions: s.missions.map((m) =>
        m.id === id ? { ...m, status: 'active' as const } : m,
      ),
    }))
  },

  autoMission: async (data) => {
    const { missionId } = await missionsApi.autoMission(data)
    // Refresh the list so the newly-created, auto-running mission appears.
    await get().fetchMissions()
    return missionId
  },

  pauseMission: async (id) => {
    await missionsApi.pauseMission(id)
    set((s) => ({
      missions: s.missions.map((m) =>
        m.id === id ? { ...m, status: 'paused' as const } : m,
      ),
    }))
  },

  resumeMission: async (id) => {
    await missionsApi.resumeMission(id)
    set((s) => ({
      missions: s.missions.map((m) =>
        m.id === id ? { ...m, status: 'active' as const } : m,
      ),
    }))
  },

  cancelMission: async (id) => {
    await missionsApi.cancelMission(id)
    set((s) => ({
      missions: s.missions.map((m) =>
        m.id === id ? { ...m, status: 'cancelled' as const } : m,
      ),
    }))
  },

  deleteMission: async (id) => {
    await missionsApi.deleteMission(id)
    set((s) => ({
      missions: s.missions.filter((m) => m.id !== id),
      selectedMission: s.selectedMission?.id === id ? null : s.selectedMission,
    }))
  },

  fetchFeatureLogs: async (missionId, featureId) => {
    try {
      const res = await missionsApi.getFeatureLogs(missionId, featureId)
      const lines = res.content ? res.content.split('\n').filter(Boolean) : []
      set((s) => ({
        featureLogs: { ...s.featureLogs, [featureId]: lines },
      }))
    } catch {
      // Non-critical — leave existing or empty
    }
  },

  fetchReport: async (missionId) => {
    set({ reportLoading: true })
    try {
      const content = await missionsApi.getMissionArtifact(missionId, 'report')
      set({ reportContent: content, reportLoading: false })
    } catch {
      set({ reportContent: '', reportLoading: false })
    }
  },

  toggleFeatureExpanded: (featureId) => {
    set((s) => {
      const next = new Set(s.expandedFeatures)
      if (next.has(featureId)) {
        next.delete(featureId)
      } else {
        next.add(featureId)
      }
      return { expandedFeatures: next }
    })
  },

  handleWSMessage: (msg: WSMessage) => {
    switch (msg.type) {
      case 'initial_state': {
        const payload = msg.payload
        const missions = Array.isArray(payload) ? (payload as Mission[]) : []
        set({ missions })
        break
      }
      case 'mission.created': {
        const mission = msg.payload as Mission
        set((s) => ({ missions: [mission, ...s.missions] }))
        break
      }
      case 'mission.updated':
      case 'mission.status_changed': {
        const updated = msg.payload as Mission
        set((s) => ({
          missions: s.missions.map((m) =>
            m.id === updated.id ? updated : m,
          ),
          selectedMission:
            s.selectedMission?.id === updated.id
              ? updated
              : s.selectedMission,
        }))
        break
      }
      case 'mission.deleted': {
        const del = msg.payload as { id: string }
        set((s) => ({
          missions: s.missions.filter((m) => m.id !== del.id),
          selectedMission:
            s.selectedMission?.id === del.id ? null : s.selectedMission,
        }))
        break
      }
      case 'log_update': {
        const log = msg as unknown as FeatureLogLine
        if (log.missionId !== get().selectedMission?.id) break
        set((s) => ({
          featureLogs: {
            ...s.featureLogs,
            [log.featureId]: [...(s.featureLogs[log.featureId] ?? []), log.line],
          },
        }))
        break
      }
      default:
        break
    }
  },
}))
