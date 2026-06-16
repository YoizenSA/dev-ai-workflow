import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'
import Layout from './components/layout/Layout'
import Kanban from './components/kanban/Kanban'
import Missions from './components/missions/Missions'
import Memories from './components/memories/Memories'
import Evals from './components/evals/Evals'
import Settings from './components/settings/Settings'

function App() {
  return (
    <Router>
      <Layout>
        <Routes>
          <Route path="/" element={<Kanban />} />
          <Route path="/missions" element={<Missions />} />
          <Route path="/memories" element={<Memories />} />
          <Route path="/evals" element={<Evals />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </Layout>
    </Router>
  )
}

export default App
