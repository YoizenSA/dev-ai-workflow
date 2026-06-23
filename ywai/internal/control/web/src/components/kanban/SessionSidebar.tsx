import { useMemo, useState } from 'react'
import {
  Archive,
  ArchiveRestore,
  ChevronDown,
  Plus,
  Trash2,
} from 'lucide-react'
import { useKanbanStore } from '../../stores/kanbanStore'
import type { Session } from '../../api/types'
import Modal from '../shared/Modal'

// Pending group-level action waiting for user confirmation.
type PendingGroupAction =
  | { kind: 'archive'; project: string; count: number }
  | { kind: 'unarchive'; project: string; count: number }
  | { kind: 'delete'; project: string; count: number }

export function SessionSidebar() {
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(() => {
    const saved = localStorage.getItem('kanban-collapsed-groups')
    return saved ? new Set(JSON.parse(saved)) : new Set()
  })
  const [showArchived, setShowArchived] = useState(false)

  const {
    sessions,
    activeSession,
    selectSession,
    deleteSession,
    updateSession,
    createSession,
    archiveGroup,
    unarchiveGroup,
    deleteGroup,
  } = useKanbanStore()

  const activeSessions = sessions.filter((s) => s.status === 'active')
  const archivedSessions = sessions.filter((s) => s.status === 'closed')

  // Confirmation state for destructive group actions. The Modal renders only
  // when this is non-null, which keeps the JSX in the rest of the file free
  // of nested ternaries.
  const [pendingAction, setPendingAction] = useState<PendingGroupAction | null>(
    null,
  )
  const [actionBusy, setActionBusy] = useState(false)

  const projectGroups = useMemo(() => {
    const groups: Record<string, Session[]> = {}
    for (const s of activeSessions) {
      const key = s.project || '(no project)'
      if (!groups[key]) groups[key] = []
      groups[key].push(s)
    }
    for (const key of Object.keys(groups)) {
      groups[key].sort(
        (a, b) =>
          new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
      )
    }
    return groups
  }, [activeSessions])

  const sortedProjectNames = Object.keys(projectGroups).sort((a, b) =>
    a.localeCompare(b),
  )

  // Archived sessions are also grouped by project, so the user can unarchive
  // (or permanently delete) an entire project in one click — same shape as
  // the active list to keep the muscle memory consistent.
  const archivedProjectGroups = useMemo(() => {
    const groups: Record<string, Session[]> = {}
    for (const s of archivedSessions) {
      const key = s.project || '(no project)'
      if (!groups[key]) groups[key] = []
      groups[key].push(s)
    }
    for (const key of Object.keys(groups)) {
      groups[key].sort(
        (a, b) =>
          new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
      )
    }
    return groups
  }, [archivedSessions])

  const sortedArchivedProjectNames = Object.keys(archivedProjectGroups).sort(
    (a, b) => a.localeCompare(b),
  )

  const toggleGroup = (project: string) => {
    setCollapsedGroups((prev) => {
      const next = new Set(prev)
      if (next.has(project)) next.delete(project)
      else next.add(project)
      localStorage.setItem('kanban-collapsed-groups', JSON.stringify([...next]))
      return next
    })
  }

  const handleArchive = async (session: Session) => {
    await updateSession(session.id, { status: 'closed' })
  }

  const handleUnarchive = async (session: Session) => {
    await updateSession(session.id, { status: 'active' })
  }

  const handleDelete = async (session: Session) => {
    if (!confirm(`Delete session "${session.goal}"? This cannot be undone.`))
      return
    await deleteSession(session.id)
  }

  const handleSelect = (session: Session) => {
    selectSession(session)
  }

  const handleNewSession = async () => {
    const project = prompt('Project name:')
    if (!project) return
    const goal = prompt('Session goal:')
    if (!goal) return
    await createSession(project, goal)
  }

  // --- Group-level bulk actions -------------------------------------------
  // Each handler opens the confirmation modal. The actual work happens in
  // `confirmGroupAction` below so the busy / error states live in one place.

  const askArchiveGroup = (project: string) => {
    const count = projectGroups[project]?.length ?? 0
    if (count === 0) return
    setPendingAction({ kind: 'archive', project, count })
  }

  const askUnarchiveGroup = (project: string) => {
    const count = archivedProjectGroups[project]?.length ?? 0
    if (count === 0) return
    setPendingAction({ kind: 'unarchive', project, count })
  }

  const askDeleteGroup = (project: string) => {
    // Count is the total across active + archived — both will be removed.
    const activeCount = projectGroups[project]?.length ?? 0
    const archivedCount = archivedProjectGroups[project]?.length ?? 0
    const count = activeCount + archivedCount
    if (count === 0) return
    setPendingAction({ kind: 'delete', project, count })
  }

  const confirmGroupAction = async () => {
    if (!pendingAction || actionBusy) return
    setActionBusy(true)
    try {
      if (pendingAction.kind === 'archive') {
        await archiveGroup(pendingAction.project)
      } else if (pendingAction.kind === 'unarchive') {
        await unarchiveGroup(pendingAction.project)
      } else {
        await deleteGroup(pendingAction.project)
      }
      setPendingAction(null)
    } catch (err) {
      // Keep the modal open with the same action so the user can retry.
      // The `setPendingAction({ ...pendingAction })` is a no-op state-wise
      // but it makes it obvious the state is preserved on failure.
      // eslint-disable-next-line no-console
      console.error('group action failed', err)
    } finally {
      setActionBusy(false)
    }
  }

  return (
    <aside className="sessions-sidebar">
      {/* Header */}
      <div className="sessions-sidebar-header">
        <div className="sessions-sidebar-title">
          <span>Sessions</span>
          <span className="sessions-sidebar-count">{sessions.length}</span>
        </div>
        <button
          className="btn btn-primary sessions-sidebar-new"
          onClick={handleNewSession}
          data-tip="New session" aria-label="New session"
        >
          <Plus size={16} strokeWidth={2.5} />
        </button>
      </div>

      {/* Project groups — scrollable */}
      <div className="sessions-sidebar-body">
        {sortedProjectNames.map((project) => {
          const groupSessions = projectGroups[project]
          const isCollapsed = collapsedGroups.has(project)
          return (
            <div key={project} className="project-group">
              <div className="project-group-header-row">
                <button
                  className="project-group-header"
                  onClick={() => toggleGroup(project)}
                >
                  <ChevronDown
                    className={`project-group-chevron ${isCollapsed ? 'collapsed' : ''}`}
                    size={16}
                  />
                  <span className="project-group-name">{project}</span>
                  <span className="project-group-count">
                    {groupSessions.length}
                  </span>
                </button>
                <div className="project-group-actions">
                  <button
                    className="session-action-btn"
                    onClick={(e) => {
                      e.stopPropagation()
                      askArchiveGroup(project)
                    }}
                    data-tip="Archive group"
                    aria-label={`Archive all sessions in ${project}`}
                    disabled={groupSessions.length === 0}
                  >
                    <Archive size={14} />
                  </button>
                  <button
                    className="session-action-btn danger"
                    onClick={(e) => {
                      e.stopPropagation()
                      askDeleteGroup(project)
                    }}
                    data-tip="Delete group"
                    aria-label={`Delete all sessions in ${project}`}
                    disabled={groupSessions.length === 0}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>

              {!isCollapsed && (
                <div className="project-group-items">
                  {groupSessions.map((session) => (
                    <div
                      key={session.id}
                      className={`session-item ${session.id === activeSession?.id ? 'active' : ''}`}
                      onClick={() => handleSelect(session)}
                    >
                      <div className="session-item-main">
                        <span
                          className={`session-item-dot ${session.status === 'active' ? 'dot-active' : 'dot-closed'}`}
                        />
                        <span className="session-item-goal">
                          {session.goal}
                        </span>
                      </div>
                      <div className="session-item-actions">
                        <button
                          className="session-action-btn"
                          onClick={(e) => {
                            e.stopPropagation()
                            handleArchive(session)
                          }}
                          data-tip="Archive" aria-label="Archive"
                        >
                          <Archive size={16} />
                        </button>
                        <button
                          className="session-action-btn danger"
                          onClick={(e) => {
                            e.stopPropagation()
                            handleDelete(session)
                          }}
                          data-tip="Delete" aria-label="Delete"
                        >
                          <Trash2 size={16} />
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )
        })}

        {/* Archived section — grouped by project so the user can unarchive
            or delete an entire archived project in one click. */}
        {archivedSessions.length > 0 && (
          <div className="archived-section">
            <button
              className="archived-toggle"
              onClick={() => setShowArchived(!showArchived)}
            >
              <ChevronDown
                className={`project-group-chevron ${showArchived ? '' : 'collapsed'}`}
                size={16}
              />
              <span>Archived</span>
              <span className="project-group-count">
                {archivedSessions.length}
              </span>
            </button>
            {showArchived &&
              sortedArchivedProjectNames.map((project) => {
                const groupSessions = archivedProjectGroups[project]
                const isCollapsed = collapsedGroups.has(`archived:${project}`)
                return (
                  <div key={`archived:${project}`} className="project-group">
                    <div className="project-group-header-row">
                      <button
                        className="project-group-header"
                        onClick={() => toggleGroup(`archived:${project}`)}
                      >
                        <ChevronDown
                          className={`project-group-chevron ${isCollapsed ? 'collapsed' : ''}`}
                          size={16}
                        />
                        <span className="project-group-name">{project}</span>
                        <span className="project-group-count">
                          {groupSessions.length}
                        </span>
                      </button>
                      <div className="project-group-actions">
                        <button
                          className="session-action-btn"
                          onClick={(e) => {
                            e.stopPropagation()
                            askUnarchiveGroup(project)
                          }}
                          data-tip="Unarchive group"
                          aria-label={`Unarchive all sessions in ${project}`}
                        >
                          <ArchiveRestore size={14} />
                        </button>
                        <button
                          className="session-action-btn danger"
                          onClick={(e) => {
                            e.stopPropagation()
                            askDeleteGroup(project)
                          }}
                          data-tip="Delete group"
                          aria-label={`Delete all sessions in ${project}`}
                        >
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </div>
                    {!isCollapsed && (
                      <div className="project-group-items">
                        {groupSessions.map((session) => (
                          <div
                            key={session.id}
                            className={`session-item archived ${session.id === activeSession?.id ? 'active' : ''}`}
                            onClick={() => handleSelect(session)}
                          >
                            <div className="session-item-main">
                              <span className="session-item-dot dot-closed" />
                              <span className="session-item-goal">
                                {session.goal}
                              </span>
                            </div>
                            <div className="session-item-actions">
                              <button
                                className="session-action-btn"
                                onClick={(e) => {
                                  e.stopPropagation()
                                  handleUnarchive(session)
                                }}
                                data-tip="Restore" aria-label="Restore"
                              >
                                <ArchiveRestore size={16} />
                              </button>
                              <button
                                className="session-action-btn danger"
                                onClick={(e) => {
                                  e.stopPropagation()
                                  handleDelete(session)
                                }}
                                data-tip="Delete" aria-label="Delete"
                              >
                                <Trash2 size={16} />
                              </button>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )
              })}
          </div>
        )}

        {sessions.length === 0 && (
          <div className="sessions-sidebar-empty">
            <p className="sessions-sidebar-empty-text">No sessions yet</p>
            <p className="sessions-sidebar-empty-hint">Click + to create one</p>
          </div>
        )}
      </div>

      {/* Confirmation modal for group-level bulk actions. Per the product
          requirement, destructive actions must go through a proper modal
          (not the browser `confirm()`) so they're consistent with the rest
          of the kanban UI and easy to style / a11y. */}
      <GroupActionModal
        action={pendingAction}
        busy={actionBusy}
        onConfirm={confirmGroupAction}
        onCancel={() => !actionBusy && setPendingAction(null)}
      />
    </aside>
  )
}

// --- Confirmation dialog --------------------------------------------------
// Small adapter on top of the shared Modal that turns the typed
// `pendingAction` state into a human-readable title/body and renders the
// right primary button per action kind.

interface GroupActionModalProps {
  action: PendingGroupAction | null
  busy: boolean
  onConfirm: () => void
  onCancel: () => void
}

function GroupActionModal({
  action,
  busy,
  onConfirm,
  onCancel,
}: GroupActionModalProps) {
  if (!action) return null

  const isDelete = action.kind === 'delete'
  const isArchive = action.kind === 'archive'
  const noun = action.count === 1 ? 'session' : 'sessions'
  const projectLabel = action.project === '(no project)' ? 'no project' : action.project

  const title = isDelete
    ? `Delete ${action.count} ${noun} in "${projectLabel}"?`
    : isArchive
      ? `Archive ${action.count} ${noun} in "${projectLabel}"?`
      : `Unarchive ${action.count} ${noun} in "${projectLabel}"?`

  const body = isDelete
    ? `This permanently removes every session in the "${projectLabel}" project, along with their delegations and activity history. This cannot be undone.`
    : isArchive
      ? `Archived sessions are hidden from the active list but kept on disk. You can unarchive them later from the "Archived" section.`
      : `Restoring will move every archived session in "${projectLabel}" back to the active list.`

  const confirmLabel = isDelete
    ? `Delete ${action.count} ${noun}`
    : isArchive
      ? `Archive ${action.count} ${noun}`
      : `Unarchive ${action.count} ${noun}`

  return (
    <Modal
      open={!!action}
      onClose={onCancel}
      title={title}
      subtitle={body}
      footer={
        <>
          <button
            type="button"
            className="btn"
            onClick={onCancel}
            disabled={busy}
          >
            Cancel
          </button>
          <button
            type="button"
            className={isDelete ? 'btn btn-danger' : 'btn btn-primary'}
            onClick={onConfirm}
            disabled={busy}
          >
            {busy ? 'Working…' : confirmLabel}
          </button>
        </>
      }
    >
      {action.count > 1 && (
        <p className="group-action-summary">
          {action.kind === 'delete' ? 'Deleting' : action.kind === 'archive' ? 'Archiving' : 'Restoring'}{' '}
          <strong>{action.count}</strong> {noun} in project{' '}
          <code>{projectLabel}</code>.
        </p>
      )}
    </Modal>
  )
}
