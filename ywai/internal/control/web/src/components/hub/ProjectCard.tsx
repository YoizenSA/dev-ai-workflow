import type { Project } from './useProjects'

interface ProjectCardProps {
  project: Project
  onDelete: (id: string) => void
}

export function ProjectCard({ project, onDelete }: ProjectCardProps) {
  return (
    <div className="project-card">
      <div className="project-card__name">{project.name}</div>
      <div className="project-card__path">{project.path}</div>
      <div className="project-card__agent">{project.agentType}</div>
      <div className="project-card__sync">
        <span
          data-testid="sync-indicator"
          className={project.syncEnabled ? 'sync-enabled' : 'sync-disabled'}
        />
        {project.syncEnabled ? 'Sync Enabled' : 'Sync Disabled'}
      </div>
      <button className="project-card__delete" onClick={() => onDelete(project.id)}>
        Delete
      </button>
    </div>
  )
}
