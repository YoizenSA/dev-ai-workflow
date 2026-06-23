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
  } = useKanbanStore()

  const activeSessions = sessions.filter((s) => s.status === 'active')
  const archivedSessions = sessions.filter((s) => s.status === 'closed')

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

        {/* Archived section */}
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
            {showArchived && (
              <div className="project-group-items">
                {archivedSessions.map((session) => (
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
        )}

        {sessions.length === 0 && (
          <div className="sessions-sidebar-empty">
            <p className="sessions-sidebar-empty-text">No sessions yet</p>
            <p className="sessions-sidebar-empty-hint">Click + to create one</p>
          </div>
        )}
      </div>
    </aside>
  )
}
