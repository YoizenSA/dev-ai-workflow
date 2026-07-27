import { BrowserRouter as Router, Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/layout/Layout'
import Memories from './components/memories/Memories'
import Evals from './components/evals/Evals'
import Settings from './components/settings/Settings'
import McpStore from './components/mcp-store/McpStore'
import AdoConfig from './components/ado-config/AdoConfig'
import WorkflowEditor from './components/workflows/WorkflowEditor'
import Chat from './components/chat/Chat'
import { HealthDashboard } from './components/health/HealthDashboard'

function App() {
  return (
    <Router>
      <Layout>
        <Routes>
          <Route path="/" element={<Navigate to="/workflows" replace />} />
          <Route path="/workflows" element={<WorkflowEditor />} />
          <Route path="/memories" element={<Memories />} />
          <Route path="/evals" element={<Evals />} />
          <Route path="/settings" element={<Settings />} />
          <Route path="/mcp-store" element={<McpStore />} />
          <Route path="/ado" element={<AdoConfig />} />
          <Route path="/chat" element={<Chat />} />
          <Route path="/health" element={<HealthDashboard />} />
          <Route path="/hub" element={<Navigate to="/workflows" replace />} />
        </Routes>
      </Layout>
    </Router>
  )
}

export default App
