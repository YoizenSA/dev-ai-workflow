import { BrowserRouter as Router, Routes, Route } from 'react-router-dom'
import Layout from './components/layout/Layout'
import Kanban from './components/kanban/Kanban'
import Missions from './components/missions/Missions'

function App() {
  return (
    <Router>
      <Layout>
        <Routes>
          <Route path="/" element={<Kanban />} />
          <Route path="/missions" element={<Missions />} />
        </Routes>
      </Layout>
    </Router>
  )
}

export default App
