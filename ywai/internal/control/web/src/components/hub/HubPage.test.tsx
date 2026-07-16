import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HubPage } from './HubPage'
import { useProjects } from './useProjects'

vi.mock('./useProjects')

const mockUseProjects = vi.mocked(useProjects)

beforeEach(() => {
  vi.resetAllMocks()
})

describe('HubPage', () => {
  it('should render loading state', () => {
    mockUseProjects.mockReturnValue({
      projects: [],
      loading: true,
      error: null,
      addProject: vi.fn(),
      removeProject: vi.fn(),
      refresh: vi.fn(),
    })

    render(<HubPage />)
    expect(screen.getByLabelText(/loading projects/i)).toBeInTheDocument()
  })

  it('should render error state', () => {
    mockUseProjects.mockReturnValue({
      projects: [],
      loading: false,
      error: 'Something went wrong',
      addProject: vi.fn(),
      removeProject: vi.fn(),
      refresh: vi.fn(),
    })

    render(<HubPage />)
    expect(screen.getByText(/something went wrong/i)).toBeInTheDocument()
  })

  it('should render "No projects" when empty', () => {
    mockUseProjects.mockReturnValue({
      projects: [],
      loading: false,
      error: null,
      addProject: vi.fn(),
      removeProject: vi.fn(),
      refresh: vi.fn(),
    })

    render(<HubPage />)
    expect(screen.getByText(/no projects/i)).toBeInTheDocument()
  })

  it('should render project list when projects exist', () => {
    mockUseProjects.mockReturnValue({
      projects: [
        { id: '1', name: 'ywai', path: '/home/user/ywai', agentType: 'opencode', syncEnabled: true },
        { id: '2', name: 'docs', path: '/home/user/docs', agentType: 'claude-code', syncEnabled: false },
      ],
      loading: false,
      error: null,
      addProject: vi.fn(),
      removeProject: vi.fn(),
      refresh: vi.fn(),
    })

    render(<HubPage />)

    expect(screen.getByText('ywai')).toBeInTheDocument()
    expect(screen.getByText('docs')).toBeInTheDocument()
  })

  it('should render "Add Project" button', () => {
    mockUseProjects.mockReturnValue({
      projects: [],
      loading: false,
      error: null,
      addProject: vi.fn(),
      removeProject: vi.fn(),
      refresh: vi.fn(),
    })

    render(<HubPage />)
    expect(screen.getByRole('button', { name: /add project/i })).toBeInTheDocument()
  })

  it('should open AddProjectModal when "Add Project" is clicked', async () => {
    const user = userEvent.setup()

    mockUseProjects.mockReturnValue({
      projects: [],
      loading: false,
      error: null,
      addProject: vi.fn(),
      removeProject: vi.fn(),
      refresh: vi.fn(),
    })

    render(<HubPage />)

    const addButton = screen.getByRole('button', { name: /add project/i })
    await user.click(addButton)

    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })
})
