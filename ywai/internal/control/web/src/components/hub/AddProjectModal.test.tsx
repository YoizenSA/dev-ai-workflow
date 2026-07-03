import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AddProjectModal } from './AddProjectModal'

describe('AddProjectModal', () => {
  it('should render the modal with form fields', () => {
    render(<AddProjectModal onAdd={vi.fn()} onCancel={vi.fn()} />)

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByLabelText(/name/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/path/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/agent type/i)).toBeInTheDocument()
  })

  it('should call onCancel when cancel button is clicked', async () => {
    const user = userEvent.setup()
    const onCancel = vi.fn()

    render(<AddProjectModal onAdd={vi.fn()} onCancel={onCancel} />)

    const cancelBtn = screen.getByRole('button', { name: /cancel/i })
    await user.click(cancelBtn)

    expect(onCancel).toHaveBeenCalledOnce()
  })

  it('should show validation error when name is empty on submit', async () => {
    const user = userEvent.setup()

    render(<AddProjectModal onAdd={vi.fn()} onCancel={vi.fn()} />)

    const submitBtn = screen.getByRole('button', { name: /add|submit|create/i })
    await user.click(submitBtn)

    expect(screen.getByText(/name is required/i)).toBeInTheDocument()
  })

  it('should show validation error when path is empty on submit', async () => {
    const user = userEvent.setup()

    render(<AddProjectModal onAdd={vi.fn()} onCancel={vi.fn()} />)

    await user.type(screen.getByLabelText(/name/i), 'my-project')

    const submitBtn = screen.getByRole('button', { name: /add|submit|create/i })
    await user.click(submitBtn)

    expect(screen.getByText(/path is required/i)).toBeInTheDocument()
  })

  it('should call onAdd with form data on valid submit', async () => {
    const user = userEvent.setup()
    const onAdd = vi.fn()

    render(<AddProjectModal onAdd={onAdd} onCancel={vi.fn()} />)

    await user.type(screen.getByLabelText(/name/i), 'my-project')
    await user.type(screen.getByLabelText(/path/i), '/home/user/my-project')
    await user.selectOptions(screen.getByLabelText(/agent type/i), 'opencode')

    const submitBtn = screen.getByRole('button', { name: /add|submit|create/i })
    await user.click(submitBtn)

    await waitFor(() => {
      expect(onAdd).toHaveBeenCalledWith({
        name: 'my-project',
        path: '/home/user/my-project',
        agentType: 'opencode',
      })
    })
  })
})
