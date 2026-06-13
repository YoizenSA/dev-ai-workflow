import { create } from 'zustand'
import type { Mission, Project, WSMessage } from '../api/types'
import { missionsApi } from '../api/client'

interface MissionsState {
  missions: Mission[]
  projects: Project[]
  selectedMission: Mission | null
  loading: boolean
  error: string | null

  // Actions
  fetchMissions: () => Promise<void>
  fetchProjects: () => Promise<void>
  selectMission: (id: string) => void
  createMission: (name: string, project?: string) => Promise<Mission>
  runMission: (id: string) => Promise<void>
  pauseMission: (id: string) => Promise<void>
  resumeMission: (id: string) => Promise<void>
  cancelMission: (id: string) => Promise<void>
  handleWSMessage: (msg: WSMessage) => void
}

export const useMissionsStore = create<MissionsState>((set, get) => ({
  missions: [],
  projects: [],
  selectedMission: null,
  loading: false,
  error: null,

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
    set({ selectedMission: mission })
  },

  createMission: async (name, project) => {
    const mission = await missionsApi.createMission({ name, project })
    set((s) => ({ missions: [mission, ...s.missions] }))
    return mission
  },

  runMission: async (id) => {
    await missionsApi.runMission(id)
    set((s) => ({
      missions: s.missions.map((m) =>
        m.id === id ? { ...m, status: 'active' as const } : m,
      ),
    }))
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
      default:
        break
    }
  },
}))
