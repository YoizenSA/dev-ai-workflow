import { renderHook, waitFor, act } from '@testing-library/react'
import { useProjects } from './useProjects'

const mockProjects = [
  { id: '1', name: 'ywai', path: '/home/user/ywai', agentType: 'opencode', syncEnabled: true },
  { id: '2', name: 'docs', path: '/home/user/docs', agentType: 'claude-code', syncEnabled: false },
]

const API_BASE = '/api/hub/projects'

beforeEach(() => {
  vi.restoreAllMocks()
})

describe('useProjects', () => {
  it('should fetch projects on mount and return them', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: true,
      json: async () => mockProjects,
    } as Response)

    const { result } = renderHook(() => useProjects())

    expect(result.current.loading).toBe(true)
    expect(result.current.projects).toEqual([])
    expect(result.current.error).toBeNull()

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.projects).toEqual(mockProjects)
    expect(result.current.error).toBeNull()
    expect(globalThis.fetch).toHaveBeenCalledWith(API_BASE)
  })

  it('should handle fetch error', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
    } as Response)

    const { result } = renderHook(() => useProjects())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.projects).toEqual([])
    expect(result.current.error).toBe('Failed to fetch projects')
  })

  it('should handle network error', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValueOnce(new Error('Network Error'))

    const { result } = renderHook(() => useProjects())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    expect(result.current.projects).toEqual([])
    expect(result.current.error).toBe('Network Error')
  })

  it('should add a project via POST', async () => {
    const newProject = { id: '3', name: 'blog', path: '/home/user/blog', agentType: 'gemini-cli', syncEnabled: false }

    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({ ok: true, json: async () => mockProjects } as Response) // initial GET
      .mockResolvedValueOnce({ ok: true, json: async () => newProject } as Response)   // POST

    const { result } = renderHook(() => useProjects())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.addProject('blog', '/home/user/blog', 'gemini-cli')
    })

    expect(globalThis.fetch).toHaveBeenCalledWith(API_BASE, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: 'blog', path: '/home/user/blog', agentType: 'gemini-cli' }),
    })
  })

  it('should handle addProject error', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({ ok: true, json: async () => mockProjects } as Response)
      .mockResolvedValueOnce({ ok: false, status: 400, statusText: 'Bad Request' } as Response)

    const { result } = renderHook(() => useProjects())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.addProject('', '/bad', 'opencode'))
      .rejects.toThrow('Failed to add project')
  })

  it('should remove a project via DELETE', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({ ok: true, json: async () => mockProjects } as Response) // initial GET
      .mockResolvedValueOnce({ ok: true } as Response)                                  // DELETE

    const { result } = renderHook(() => useProjects())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await act(async () => {
      await result.current.removeProject('1')
    })

    expect(globalThis.fetch).toHaveBeenCalledWith(`${API_BASE}/1`, {
      method: 'DELETE',
    })
  })

  it('should handle removeProject error', async () => {
    vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce({ ok: true, json: async () => mockProjects } as Response)
      .mockResolvedValueOnce({ ok: false, status: 404, statusText: 'Not Found' } as Response)

    const { result } = renderHook(() => useProjects())

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    await expect(result.current.removeProject('999'))
      .rejects.toThrow('Failed to remove project')
  })
})
