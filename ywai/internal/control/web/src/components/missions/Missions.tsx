import { useEffect, useCallback } from 'react'
import { useMissionsStore } from '../../stores/missionsStore'
import { useWebSocket } from '../../hooks/useWebSocket'
import type { Mission, WSMessage } from '../../api/types'
import './Missions.css'

const STATUS_LABELS: Record<string, string> = {
  pending: 'Pending',
  planning: 'Planning',
  active: 'Active',
  paused: 'Paused',
  completed: 'Completed',
  failed: 'Failed',
  cancelled: 'Cancelled',
  validating: 'Validating',
}

const STATUS_COLORS: Record<string, string> = {
  pending: 'var(--text-muted)',
  planning: '#a04ef0',
  active: '#4ea3f0',
  paused: '#f0a04e',
  completed: '#4ef0a0',
  failed: '#f04e6e',
  cancelled: 'var(--text-muted)',
  validating: '#f0e04e',
}

function MissionRow({ mission }: { mission: Mission }) {
  const { runMission, pauseMission, resumeMission, cancelMission, selectMission, selectedMission } =
    useMissionsStore()

  const isActive = selectedMission?.id === mission.id
  const completedFeatures = mission.features.filter(
    (f) => f.status === 'completed',
  ).length
  const totalFeatures = mission.features.length
  const progress =
    totalFeatures > 0 ? Math.round((completedFeatures / totalFeatures) * 100) : 0

  return (
    <div
      className={`card ${isActive ? 'active' : ''}`}
      onClick={() => selectMission(mission.id)}
      style={{ cursor: 'pointer' }}
    >
      <div className="card-header">
        <div className="row" style={{ flex: 1 }}>
          <span
            className="tag"
            style={{ background: STATUS_COLORS[mission.status] ?? 'var(--text-muted)' }}
          >
            {STATUS_LABELS[mission.status] ?? mission.status}
          </span>
          {mission.agent && (
            <span className="tag">{mission.agent}</span>
          )}
        </div>
        <div className="row">
          {mission.status === 'pending' && (
            <button className="btn btn-sm" onClick={(e) => { e.stopPropagation(); runMission(mission.id) }}>
              Run
            </button>
          )}
          {mission.status === 'active' && (
            <button className="btn btn-sm" onClick={(e) => { e.stopPropagation(); pauseMission(mission.id) }}>
              Pause
            </button>
          )}
          {mission.status === 'paused' && (
            <button className="btn btn-sm" onClick={(e) => { e.stopPropagation(); resumeMission(mission.id) }}>
              Resume
            </button>
          )}
          {['pending', 'active', 'paused'].includes(mission.status) && (
            <button
              className="btn btn-sm"
              style={{ color: 'var(--danger)' }}
              onClick={(e) => { e.stopPropagation(); cancelMission(mission.id) }}
            >
              Cancel
            </button>
          )}
        </div>
      </div>
      <div className="card-body">
        <h3 style={{ margin: '0 0 var(--space-2) 0' }}>{mission.name}</h3>
        {mission.project && (
          <p className="muted" style={{ margin: 0, fontSize: '0.85rem' }}>
            {mission.project}
          </p>
        )}
        {totalFeatures > 0 && (
          <div style={{ marginTop: 'var(--space-2)' }}>
            <div className="row" style={{ justifyContent: 'space-between' }}>
              <span className="muted" style={{ fontSize: '0.8rem' }}>
                {completedFeatures}/{totalFeatures} features
              </span>
              <span className="muted" style={{ fontSize: '0.8rem' }}>
                {progress}%
              </span>
            </div>
            <div
              style={{
                height: 4,
                background: 'var(--surface-strong)',
                borderRadius: 2,
                marginTop: 4,
              }}
            >
              <div
                style={{
                  height: '100%',
                  width: `${progress}%`,
                  background: 'var(--accent)',
                  borderRadius: 2,
                  transition: 'width 200ms',
                }}
              />
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

export default function Missions() {
  const {
    missions,
    projects,
    loading,
    selectedMission,
    fetchMissions,
    fetchProjects,
    createMission,
  } = useMissionsStore()

  const handleWSMessage = useCallback((msg: WSMessage) => {
    useMissionsStore.getState().handleWSMessage(msg)
  }, [])

  useWebSocket('/missions/ws', handleWSMessage)

  useEffect(() => {
    fetchMissions()
    fetchProjects()
  }, [fetchMissions, fetchProjects])

  const handleNewMission = async () => {
    const name = prompt('Mission name:')
    if (!name) return
    await createMission(name)
  }

  const activeMissions = missions.filter(
    (m) => !['completed', 'cancelled', 'failed'].includes(m.status),
  )
  const completedMissions = missions.filter(
    (m) => ['completed', 'cancelled', 'failed'].includes(m.status),
  )

  return (
    <div className="missions">
      <header className="page-header">
        <div className="page-heading">
          <span className="page-eyebrow">Missions</span>
          <h1 className="page-title">Mission Control</h1>
          <p className="page-subtitle">
            {activeMissions.length} active · {completedMissions.length} completed ·{' '}
            {projects.length} projects
          </p>
        </div>
        <div className="page-actions">
          <button className="btn btn-primary" onClick={handleNewMission}>
            + New Mission
          </button>
        </div>
      </header>

      {loading && missions.length === 0 ? (
        <p className="muted">Loading missions...</p>
      ) : (
        <>
          {/* Active Missions */}
          {activeMissions.length > 0 && (
            <section style={{ marginBottom: 'var(--space-5)' }}>
              <h2 style={{ marginBottom: 'var(--space-3)' }}>Active</h2>
              <div className="grid-2">
                {activeMissions.map((m) => (
                  <MissionRow key={m.id} mission={m} />
                ))}
              </div>
            </section>
          )}

          {/* Selected Mission Detail */}
          {selectedMission && (
            <section style={{ marginBottom: 'var(--space-5)' }}>
              <h2 style={{ marginBottom: 'var(--space-3)' }}>
                {selectedMission.name} — Details
              </h2>
              <div className="card">
                <div className="card-body">
                  {selectedMission.features.length > 0 ? (
                    <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                      <thead>
                        <tr>
                          <th style={{ textAlign: 'left' }}>Feature</th>
                          <th>Status</th>
                          <th>Skill</th>
                        </tr>
                      </thead>
                      <tbody>
                        {selectedMission.features.map((f) => (
                          <tr key={f.id}>
                            <td>{f.description}</td>
                            <td>
                              <span
                                className="tag"
                                style={{
                                  background:
                                    STATUS_COLORS[f.status] ?? 'var(--text-muted)',
                                }}
                              >
                                {f.status}
                              </span>
                            </td>
                            <td className="muted">{f.skillName ?? '—'}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  ) : (
                    <p className="muted">No features yet</p>
                  )}
                </div>
              </div>
            </section>
          )}

          {/* Completed Missions */}
          {completedMissions.length > 0 && (
            <section>
              <h2 style={{ marginBottom: 'var(--space-3)' }}>Completed</h2>
              <div className="grid-2">
                {completedMissions.map((m) => (
                  <MissionRow key={m.id} mission={m} />
                ))}
              </div>
            </section>
          )}

          {missions.length === 0 && (
            <p className="muted">No missions yet. Create one to get started.</p>
          )}
        </>
      )}
    </div>
  )
}


