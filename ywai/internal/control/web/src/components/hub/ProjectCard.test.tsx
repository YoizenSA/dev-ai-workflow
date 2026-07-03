import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProjectCard } from './ProjectCard'

const baseProject = {
  id: '1',
  name: 'ywai',
  path: '/home/user/ywai',
  agentType: 'opencode',
  syncEnabled: true,
}

describe('ProjectCard', () => {
  it('should render project name', () => {
    render(<ProjectCard project={baseProject} onDelete={vi.fn()} />)
    expect(screen.getByText('ywai')).toBeInTheDocument()
  })

  it('should render project path', () => {
    render(<ProjectCard project={baseProject} onDelete={vi.fn()} />)
    expect(screen.getByText('/home/user/ywai')).toBeInTheDocument()
  })

  it('should render agent type', () => {
    render(<ProjectCard project={baseProject} onDelete={vi.fn()} />)
    expect(screen.getByText('opencode')).toBeInTheDocument()
  })

  it('should render sync status as enabled', () => {
    render(<ProjectCard project={baseProject} onDelete={vi.fn()} />)
    expect(screen.getByText(/sync enabled/i)).toBeInTheDocument()
  })

  it('should render sync status as disabled', () => {
    render(<ProjectCard project={{ ...baseProject, syncEnabled: false }} onDelete={vi.fn()} />)
    expect(screen.getByText(/sync disabled/i)).toBeInTheDocument()
  })

  it('should show green indicator when sync is enabled', () => {
    render(<ProjectCard project={baseProject} onDelete={vi.fn()} />)
    const indicator = screen.getByTestId('sync-indicator')
    expect(indicator).toHaveClass('sync-enabled')
  })

  it('should show red indicator when sync is disabled', () => {
    render(<ProjectCard project={{ ...baseProject, syncEnabled: false }} onDelete={vi.fn()} />)
    const indicator = screen.getByTestId('sync-indicator')
    expect(indicator).toHaveClass('sync-disabled')
  })

  it('should have a delete button', () => {
    render(<ProjectCard project={baseProject} onDelete={vi.fn()} />)
    expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument()
  })

  it('should call onDelete when delete button is clicked', async () => {
    const user = userEvent.setup()
    const onDelete = vi.fn()

    render(<ProjectCard project={baseProject} onDelete={onDelete} />)

    const deleteBtn = screen.getByRole('button', { name: /delete/i })
    await user.click(deleteBtn)

    expect(onDelete).toHaveBeenCalledWith('1')
  })
})
