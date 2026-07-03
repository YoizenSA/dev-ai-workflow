import { useState } from 'react'
import { useProjects } from './useProjects'
import { ProjectCard } from './ProjectCard'
import { AddProjectModal } from './AddProjectModal'

export function HubPage() {
  const { projects, loading, error, addProject, removeProject } = useProjects()
  const [showAddModal, setShowAddModal] = useState(false)

  if (loading) {
    return <div>Loading...</div>
  }

  if (error) {
    return <div>Something went wrong</div>
  }

  return (
    <div className="hub-page">
      {projects.length === 0 ? (
        <div className="hub-page__empty">No projects</div>
      ) : (
        <div className="hub-page__list">
          {projects.map((project) => (
            <ProjectCard key={project.id} project={project} onDelete={removeProject} />
          ))}
        </div>
      )}
      <button
        className="hub-page__add-btn"
        onClick={() => setShowAddModal(true)}
      >
        Add Project
      </button>
      {showAddModal && (
        <AddProjectModal
          onAdd={async (data) => {
            await addProject(data.name, data.path, data.agentType)
            setShowAddModal(false)
          }}
          onCancel={() => setShowAddModal(false)}
        />
      )}
    </div>
  )
}
