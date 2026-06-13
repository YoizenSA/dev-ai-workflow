import type {
  Session,
  Delegation,
  BoardView,
  GraphView,
  ActivityEvent,
  Mission,
  Project,
} from './types'

const BASE = ''

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const body = await res.text().catch(() => res.statusText)
    throw new Error(`${res.status}: ${body}`)
  }
  return res.json()
}

// ─── Kanban API ────────────────────────────────────────────────────────────

export const kanbanApi = {
  // Sessions
  listSessions: () => request<Session[]>('/api/sessions'),
  getSession: (id: string) => request<Session>(`/api/sessions/${id}`),
  createSession: (data: { project: string; goal: string }) =>
    request<Session>('/api/sessions', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  updateSession: (id: string, data: Partial<Session>) =>
    request<Session>(`/api/sessions/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),
  deleteSession: (id: string) =>
    fetch(`${BASE}/api/sessions/${id}`, { method: 'DELETE' }).then((r) => {
      if (!r.ok) throw new Error(`${r.status}`)
    }),

  // Board
  getBoard: (sessionId: string) =>
    request<BoardView>(`/api/sessions/${sessionId}/board`),
  getGraph: (sessionId: string) =>
    request<GraphView>(`/api/sessions/${sessionId}/graph`),

  // Delegations
  createDelegation: (data: {
    session_id: string
    agent: string
    task_summary: string
    dependencies?: string[]
  }) =>
    request<Delegation>('/api/delegations', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  getDelegation: (id: string) =>
    request<Delegation>(`/api/delegations/${id}`),
  updateDelegation: (
    id: string,
    data: { column?: string; status?: string; blocker?: string },
  ) =>
    request<Delegation>(`/api/delegations/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    }),

  // Activities
  createActivity: (
    delegationId: string,
    data: { type: string; content: string; options?: string[] },
  ) =>
    request<ActivityEvent>(
      `/api/delegations/${delegationId}/activities`,
      { method: 'POST', body: JSON.stringify(data) },
    ),
  getActivities: (delegationId: string) =>
    request<ActivityEvent[]>(
      `/api/delegations/${delegationId}/activities`,
    ),
  resolveActivity: (delegationId: string, activityId: string, resolution: string) =>
    request<ActivityEvent>(
      `/api/delegations/${delegationId}/activities/${activityId}`,
      {
        method: 'PATCH',
        body: JSON.stringify({ resolution }),
      },
    ),

  // Pending decisions
  getPendingDecisions: (sessionId: string) =>
    request<ActivityEvent[]>(`/api/sessions/${sessionId}/decisions`),
}

// ─── Missions API ──────────────────────────────────────────────────────────

export const missionsApi = {
  // Missions
  listMissions: () =>
    request<{ missions: Mission[] }>('/missions/api/missions').then(
      (r) => r.missions,
    ),
  getMission: (id: string) =>
    request<Mission>(`/missions/api/missions/${id}`),
  createMission: (data: { name: string; project?: string }) =>
    request<Mission>('/missions/api/missions', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
  approvePlan: (missionId: string) =>
    request<void>('/missions/api/missions/approve', {
      method: 'POST',
      body: JSON.stringify({ mission_id: missionId }),
    }),
  runMission: (id: string) =>
    request<void>(`/missions/api/missions/${id}/run`, { method: 'POST' }),
  pauseMission: (id: string) =>
    request<void>(`/missions/api/missions/${id}/pause`, { method: 'POST' }),
  resumeMission: (id: string) =>
    request<void>(`/missions/api/missions/${id}/resume`, { method: 'POST' }),
  cancelMission: (id: string) =>
    request<void>(`/missions/api/missions/${id}/cancel`, { method: 'POST' }),

  // Projects
  listProjects: () =>
    request<{ projects: Project[] }>('/missions/api/projects').then(
      (r) => r.projects,
    ),
  createProject: (name: string, path: string) =>
    request<Project>('/missions/api/projects', {
      method: 'POST',
      body: JSON.stringify({ name, path }),
    }),
  deleteProject: (name: string) =>
    fetch(`${BASE}/missions/api/projects/${name}`, { method: 'DELETE' }).then(
      (r) => {
        if (!r.ok) throw new Error(`${r.status}`)
      },
    ),
}
