import { useState } from 'react'

export interface AddProjectData {
  name: string
  path: string
  agentType: string
}

interface AddProjectModalProps {
  onAdd: (data: AddProjectData) => void
  onCancel: () => void
}

const AGENT_OPTIONS = ['opencode', 'claude-code', 'gemini-cli', 'cursor']

export function AddProjectModal({ onAdd, onCancel }: AddProjectModalProps) {
  const [name, setName] = useState('')
  const [path, setPath] = useState('')
  const [agentType, setAgentType] = useState(AGENT_OPTIONS[0])
  const [errors, setErrors] = useState<{ name?: string; path?: string }>({})

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const newErrors: { name?: string; path?: string } = {}
    if (!name.trim()) {
      newErrors.name = 'Name is required'
    }
    if (!path.trim()) {
      newErrors.path = 'Path is required'
    }
    setErrors(newErrors)
    if (Object.keys(newErrors).length === 0) {
      onAdd({ name, path, agentType })
    }
  }

  return (
    <div role="dialog" className="add-project-modal">
      <form onSubmit={handleSubmit}>
        <div className="add-project-modal__field">
          <label htmlFor="project-name">Name</label>
          <input id="project-name" aria-label="Name" value={name} onChange={(e) => setName(e.target.value)} />
          {errors.name && <div className="add-project-modal__error">{errors.name}</div>}
        </div>
        <div className="add-project-modal__field">
          <label htmlFor="project-path">Path</label>
          <input id="project-path" aria-label="Path" value={path} onChange={(e) => setPath(e.target.value)} />
          {errors.path && <div className="add-project-modal__error">{errors.path}</div>}
        </div>
        <div className="add-project-modal__field">
          <label htmlFor="project-agent-type">Agent Type</label>
          <select id="project-agent-type" aria-label="Agent Type" value={agentType} onChange={(e) => setAgentType(e.target.value)}>
            {AGENT_OPTIONS.map((opt) => (
              <option key={opt} value={opt}>{opt}</option>
            ))}
          </select>
        </div>
        <div className="add-project-modal__actions">
          <button type="button" onClick={onCancel}>Cancel</button>
          <button type="submit">Add</button>
        </div>
      </form>
    </div>
  )
}
