import { create } from 'zustand'
import type {
  Session,
  Delegation,
  BoardView,
  ActivityEvent,
  DelegationColumn,
  WSMessage,
} from '../api/types'
import { kanbanApi } from '../api/client'

interface KanbanState {
  sessions: Session[]
  activeSession: Session | null
  board: BoardView | null
  activities: Record<string, ActivityEvent[]> // delegationId -> activities
  pendingDecisions: ActivityEvent[]
  loading: boolean
  error: string | null

  // Actions
  fetchSessions: () => Promise<void>
  selectSession: (session: Session) => Promise<void>
  createSession: (project: string, goal: string) => Promise<Session>
  deleteSession: (id: string) => Promise<void>
  updateSession: (id: string, data: Partial<Session>) => Promise<void>
  createDelegation: (
    agent: string,
    taskSummary: string,
    dependencies?: string[],
  ) => Promise<void>
  moveDelegation: (delegationId: string, column: DelegationColumn) => Promise<void>
  fetchActivities: (delegationId: string) => Promise<void>
  resolveActivity: (delegationId: string, activityId: string, resolution: string) => Promise<void>
  handleWSMessage: (msg: WSMessage) => void
}

const emptyBoard: BoardView = {
  backlog: [],
  ready: [],
  in_progress: [],
  review: [],
  done: [],
}

export const useKanbanStore = create<KanbanState>((set, get) => ({
  sessions: [],
  activeSession: null,
  board: null,
  activities: {},
  pendingDecisions: [],
  loading: false,
  error: null,

  fetchSessions: async () => {
    set({ loading: true, error: null })
    try {
      const sessions = await kanbanApi.listSessions()
      set({ sessions, loading: false })
      // Auto-select first active session (prefer sessions with delegations)
      const active = sessions.find((s) => s.status === 'active')
      if (active && !get().activeSession) {
        await get().selectSession(active)
      }
    } catch (err) {
      set({ error: String(err), loading: false })
    }
  },

  selectSession: async (session: Session) => {
    set({ activeSession: session, loading: true, error: null })
    try {
      const board = await kanbanApi.getBoard(session.id)
      set({ board, loading: false })
    } catch (err) {
      set({ board: emptyBoard, error: String(err), loading: false })
    }
  },

  createSession: async (project: string, goal: string) => {
    const session = await kanbanApi.createSession({ project, goal })
    set((s) => ({ sessions: [...s.sessions, session] }))
    await get().selectSession(session)
    return session
  },

  deleteSession: async (id: string) => {
    await kanbanApi.deleteSession(id)
    set((s) => {
      const sessions = s.sessions.filter((sess) => sess.id !== id)
      const activeSession =
        s.activeSession?.id === id
          ? sessions.find((sess) => sess.status === 'active') ?? null
          : s.activeSession
      return { sessions, activeSession }
    })
  },

  updateSession: async (id: string, data: Partial<Session>) => {
    const updated = await kanbanApi.updateSession(id, data)
    set((s) => {
      const sessions = s.sessions.map((sess) =>
        sess.id === id ? updated : sess,
      )
      const activeSession =
        s.activeSession?.id === id ? updated : s.activeSession
      return { sessions, activeSession }
    })
  },

  createDelegation: async (agent, taskSummary, dependencies) => {
    const session = get().activeSession
    if (!session) return
    await kanbanApi.createDelegation({
      session_id: session.id,
      agent,
      task_summary: taskSummary,
      dependencies,
    })
  },

  moveDelegation: async (delegationId, column) => {
    // Optimistic update
    const prev = get().board
    if (prev) {
      const newBoard = { ...prev }
      let moved: Delegation | null = null
      for (const col of Object.keys(newBoard) as DelegationColumn[]) {
        const idx = newBoard[col].findIndex((d) => d.id === delegationId)
        if (idx !== -1) {
          moved = newBoard[col][idx]
          newBoard[col] = newBoard[col].filter((_, i) => i !== idx)
          break
        }
      }
      if (moved) {
        moved = { ...moved, column }
        newBoard[column] = [...newBoard[column], moved]
        set({ board: newBoard })
      }
    }
    try {
      await kanbanApi.updateDelegation(delegationId, { column })
    } catch {
      // Rollback
      set({ board: prev })
    }
  },

  fetchActivities: async (delegationId: string) => {
    const activities = await kanbanApi.getActivities(delegationId)
    set((s) => ({
      activities: { ...s.activities, [delegationId]: activities },
    }))
  },

  resolveActivity: async (delegationId, activityId, resolution) => {
    await kanbanApi.resolveActivity(delegationId, activityId, resolution)
    set((s) => ({
      pendingDecisions: s.pendingDecisions.filter((a) => a.id !== activityId),
    }))
  },

  handleWSMessage: (msg: WSMessage) => {
    const state = get()
    if (!state.board) return

    const board = { ...state.board }

    switch (msg.type) {
      case 'delegation.created': {
        const d = msg.payload as Delegation
        if (d.session_id === state.activeSession?.id) {
          board[d.column] = [...board[d.column], d]
          set({ board })
        }
        break
      }
      case 'delegation.status_changed':
      case 'delegation.updated': {
        const updated = msg.payload as Delegation
        if (updated.session_id !== state.activeSession?.id) break
        const newBoard: BoardView = { ...emptyBoard }
        for (const col of Object.keys(board) as DelegationColumn[]) {
          newBoard[col] = board[col].map((d) =>
            d.id === updated.id ? updated : d,
          )
        }
        // If column changed, move it
        const oldCol = Object.keys(board).find((col) =>
          board[col as DelegationColumn]?.some((d) => d.id === updated.id),
        ) as DelegationColumn | undefined
        if (oldCol && oldCol !== updated.column) {
          newBoard[oldCol] = newBoard[oldCol].filter(
            (d) => d.id !== updated.id,
          )
          newBoard[updated.column] = [
            ...newBoard[updated.column],
            updated,
          ]
        }
        set({ board: newBoard })
        break
      }
      case 'delegation.deleted': {
        const del = msg.payload as { id: string }
        const newBoard: BoardView = { ...emptyBoard }
        for (const col of Object.keys(board) as DelegationColumn[]) {
          newBoard[col] = board[col].filter((d) => d.id !== del.id)
        }
        set({ board: newBoard })
        break
      }
      default:
        break
    }
  },
}))
