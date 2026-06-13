import { useEffect, useCallback } from 'react'
import { useKanbanStore } from '../../stores/kanbanStore'
import { useWebSocket } from '../../hooks/useWebSocket'
import type { Delegation, DelegationColumn, WSMessage } from '../../api/types'
import './Kanban.css'

const AGENT_COLORS: Record<string, string> = {
  dev: '#4ea3f0',
  qa: '#f0a04e',
  reviewer: '#a04ef0',
  architect: '#4ef0a0',
  devops: '#f04e6e',
}

const COLUMNS: { id: DelegationColumn; label: string }[] = [
  { id: 'backlog', label: 'Backlog' },
  { id: 'ready', label: 'Ready' },
  { id: 'in_progress', label: 'In Progress' },
  { id: 'review', label: 'Review' },
  { id: 'done', label: 'Done' },
]

function DelegationCard({ delegation }: { delegation: Delegation }) {
  const handleDragStart = (e: React.DragEvent) => {
    e.dataTransfer.setData('text/plain', delegation.id)
    e.dataTransfer.effectAllowed = 'move'
  }

  const agentColor = AGENT_COLORS[delegation.agent] ?? 'var(--accent)'

  return (
    <div
      className="card"
      draggable
      onDragStart={handleDragStart}
      style={{ borderLeft: `3px solid ${agentColor}` }}
    >
      <div className="card-header">
        <span className="tag" style={{ background: agentColor }}>
          {delegation.agent}
        </span>
        {delegation.pending_action && (
          <span className="tag" style={{ background: '#f0a04e' }}>!</span>
        )}
      </div>
      <div className="card-body">
        <p style={{ margin: 0, fontSize: '0.9rem' }}>
          {delegation.task_summary}
        </p>
      </div>
      {delegation.handoff_preview && (
        <div className="card-footer">
          <p className="muted" style={{ fontSize: '0.8rem', margin: 0 }}>
            {delegation.handoff_preview.slice(0, 80)}
            {delegation.handoff_preview.length > 80 ? '...' : ''}
          </p>
        </div>
      )}
      {delegation.blocker && (
        <div className="card-footer">
          <p style={{ color: 'var(--danger)', fontSize: '0.8rem', margin: 0 }}>
            Blocked: {delegation.blocker}
          </p>
        </div>
      )}
    </div>
  )
}

export default function Kanban() {
  const {
    board,
    sessions,
    activeSession,
    loading,
    fetchSessions,
    selectSession,
    createSession,
    createDelegation,
    moveDelegation,
  } = useKanbanStore()

  const handleWSMessage = useCallback(
    (msg: WSMessage) => {
      useKanbanStore.getState().handleWSMessage(msg)
    },
    [],
  )

  useWebSocket('/ws', handleWSMessage)

  useEffect(() => {
    fetchSessions()
  }, [fetchSessions])

  const handleDrop = (e: React.DragEvent, column: DelegationColumn) => {
    e.preventDefault()
    const delegationId = e.dataTransfer.getData('text/plain')
    if (delegationId) {
      moveDelegation(delegationId, column)
    }
  }

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'move'
  }

  const handleNewDelegation = async () => {
    const agent = prompt('Agent (dev, qa, reviewer, architect, devops):')
    if (!agent) return
    const task = prompt('Task summary:')
    if (!task) return
    await createDelegation(agent, task)
  }

  const handleNewSession = async () => {
    const project = prompt('Project name:')
    if (!project) return
    const goal = prompt('Goal:')
    if (!goal) return
    await createSession(project, goal)
  }

  if (loading && !board) {
    return (
      <div className="kanban">
        <p className="muted">Loading...</p>
      </div>
    )
  }

  return (
    <div className="kanban">
      <header className="page-header">
        <div className="page-heading">
          <span className="page-eyebrow">Delegations</span>
          <h1 className="page-title">Kanban Board</h1>
          <p className="page-subtitle">
            {activeSession
              ? `${activeSession.project} — ${activeSession.goal}`
              : sessions.length === 0
                ? 'No sessions yet'
                : 'Select a session'}
          </p>
        </div>
        <div className="page-actions">
          {activeSession && (
            <button className="btn btn-primary" onClick={handleNewDelegation}>
              + New Delegation
            </button>
          )}
          {!activeSession && (
            <button className="btn btn-primary" onClick={handleNewSession}>
              + New Session
            </button>
          )}
        </div>
      </header>

      {/* Session selector */}
      {sessions.length > 1 && (
        <div className="row" style={{ marginBottom: 'var(--space-4)', flexWrap: 'wrap' }}>
          {sessions.map((s) => (
            <button
              key={s.id}
              className={`btn ${s.id === activeSession?.id ? 'btn-primary' : ''}`}
              onClick={() => selectSession(s)}
              style={{ fontSize: '0.85rem' }}
            >
              {s.project}
            </button>
          ))}
        </div>
      )}

      <div className="board">
        {COLUMNS.map((col) => {
          const delegations = board?.[col.id] ?? []
          return (
            <div
              key={col.id}
              className="column"
              data-column={col.id}
              onDrop={(e) => handleDrop(e, col.id)}
              onDragOver={handleDragOver}
            >
              <div className="column-header">
                <h2>{col.label}</h2>
                <span className="column-count">{delegations.length}</span>
              </div>
              <div className="column-cards">
                {delegations.map((d) => (
                  <DelegationCard key={d.id} delegation={d} />
                ))}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
