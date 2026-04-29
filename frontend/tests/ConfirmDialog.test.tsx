import { render, screen, fireEvent } from '@testing-library/react'
import { ConfirmDialog } from '../src/components/ui/ConfirmDialog'

describe('ConfirmDialog', () => {
  const defaultProps = {
    isOpen: true,
    onClose: jest.fn(),
    onConfirm: jest.fn(),
    title: 'Delete Item',
    message: 'Are you sure you want to delete this item?',
  }

  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('renders when isOpen is true', () => {
    render(<ConfirmDialog {...defaultProps} />)
    expect(screen.getByText('Delete Item')).toBeInTheDocument()
    expect(screen.getByText('Are you sure you want to delete this item?')).toBeInTheDocument()
  })

  it('does not render when isOpen is false', () => {
    render(<ConfirmDialog {...defaultProps} isOpen={false} />)
    expect(screen.queryByText('Delete Item')).not.toBeInTheDocument()
  })

  it('renders confirm and cancel buttons', () => {
    render(<ConfirmDialog {...defaultProps} />)
    expect(screen.getByRole('button', { name: /confirm/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument()
  })

  it('calls onClose when cancel is clicked', () => {
    render(<ConfirmDialog {...defaultProps} />)
    fireEvent.click(screen.getByRole('button', { name: /cancel/i }))
    expect(defaultProps.onClose).toHaveBeenCalledTimes(1)
  })

  it('calls onConfirm when confirm is clicked', () => {
    render(<ConfirmDialog {...defaultProps} />)
    fireEvent.click(screen.getByRole('button', { name: /confirm/i }))
    expect(defaultProps.onConfirm).toHaveBeenCalledTimes(1)
  })

  it('uses custom labels', () => {
    render(
      <ConfirmDialog
        {...defaultProps}
        confirmLabel="Delete"
        cancelLabel="Go Back"
      />
    )
    expect(screen.getByRole('button', { name: /delete/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /go back/i })).toBeInTheDocument()
  })

  it('disables buttons when loading', () => {
    render(<ConfirmDialog {...defaultProps} loading />)
    expect(screen.getByRole('button', { name: /confirm/i })).toBeDisabled()
    expect(screen.getByRole('button', { name: /cancel/i })).toBeDisabled()
  })
})