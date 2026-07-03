import { useState, useEffect, useCallback } from 'react'

export interface Project {
  id: string
  name: string
  path: string
  agentType: string
  syncEnabled: boolean
  createdAt?: string
  updatedAt?: string
}

const API_BASE = '/api/hub/projects'

export function useProjects() {
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const fetchProjects = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await fetch(API_BASE)
      if (!res.ok) {
        setError('Failed to fetch projects')
        setProjects([])
        return
      }
      const data: Project[] = await res.json()
      setProjects(data)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to fetch projects')
      setProjects([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchProjects()
  }, [fetchProjects])

  const addProject = useCallback(async (name: string, path: string, agentType: string) => {
    const res = await fetch(API_BASE, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, path, agentType }),
    })
    if (!res.ok) {
      throw new Error('Failed to add project')
    }
    const newProject: Project = await res.json()
    setProjects((prev) => [...prev, newProject])
  }, [])

  const removeProject = useCallback(async (id: string) => {
    const res = await fetch(`${API_BASE}/${id}`, {
      method: 'DELETE',
    })
    if (!res.ok) {
      throw new Error('Failed to remove project')
    }
    setProjects((prev) => prev.filter((p) => p.id !== id))
  }, [])

  const refresh = useCallback(() => {
    fetchProjects()
  }, [fetchProjects])

  return { projects, loading, error, addProject, removeProject, refresh }
}
