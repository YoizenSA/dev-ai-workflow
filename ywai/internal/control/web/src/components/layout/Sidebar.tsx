import { Link, useLocation } from 'react-router-dom'
import { useKanbanStore } from '../../stores/kanbanStore'
import { useMissionsStore } from '../../stores/missionsStore'

const NAV_ITEMS = [
  { path: '/', label: 'Kanban', icon: '📋' },
  { path: '/missions', label: 'Missions', icon: '🚀' },
]

export default function Sidebar() {
  const location = useLocation()
  const sessionCount = useKanbanStore((s) => s.sessions.filter((sess) => sess.status === 'active').length)
  const activeMissions = useMissionsStore(
    (s) => s.missions.filter((m) => !['completed', 'cancelled', 'failed'].includes(m.status)).length,
  )

  return (
    <aside className="sidebar">
      <div className="brand">
        <span className="brand-icon">y</span>
        <span className="brand-name grad-text">ywai</span>
      </div>

      <nav className="nav">
        {NAV_ITEMS.map((item) => (
          <Link
            key={item.path}
            to={item.path}
            className={`nav-item ${location.pathname === item.path ? 'active' : ''}`}
          >
            <span className="nav-icon">{item.icon}</span>
            <span className="nav-label">{item.label}</span>
            {item.path === '/' && sessionCount > 0 && (
              <span className="tag" style={{ marginLeft: 'auto', fontSize: '0.75rem' }}>
                {sessionCount}
              </span>
            )}
            {item.path === '/missions' && activeMissions > 0 && (
              <span className="tag" style={{ marginLeft: 'auto', fontSize: '0.75rem' }}>
                {activeMissions}
              </span>
            )}
          </Link>
        ))}
      </nav>
    </aside>
  )
}
