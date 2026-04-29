import { render, screen, fireEvent } from '@testing-library/react'
import { Modal } from '../src/components/ui/Modal'

describe('Modal', () => {
  it('renders when isOpen is true', () => {
    render(
      <Modal isOpen onClose={() => {}}>
        Modal content
      </Modal>
    )
    expect(screen.getByText('Modal content')).toBeInTheDocument()
  })

  it('does not render when isOpen is false', () => {
    render(
      <Modal isOpen={false} onClose={() => {}}>
        Hidden content
      </Modal>
    )
    expect(screen.queryByText('Hidden content')).not.toBeInTheDocument()
  })

  it('renders title', () => {
    render(
      <Modal isOpen onClose={() => {}} title="Confirm Action">
        Content
      </Modal>
    )
    expect(screen.getByText('Confirm Action')).toBeInTheDocument()
  })

  it('renders footer when provided', () => {
    render(
      <Modal
        isOpen
        onClose={() => {}}
        footer={<button>Confirm</button>}
      >
        Content
      </Modal>
    )
    expect(screen.getByRole('button', { name: 'Confirm' })).toBeInTheDocument()
  })

  it('calls onClose when close button is clicked', () => {
    const handleClose = jest.fn()
    render(
      <Modal isOpen onClose={handleClose} title="Test">
        Content
      </Modal>
    )
    fireEvent.click(screen.getByLabelText(/close modal/i))
    expect(handleClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when escape key is pressed', () => {
    const handleClose = jest.fn()
    render(
      <Modal isOpen onClose={handleClose}>
        Content
      </Modal>
    )
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(handleClose).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when overlay is clicked', () => {
    const handleClose = jest.fn()
    render(
      <Modal isOpen onClose={handleClose}>
        Content
      </Modal>
    )
    // The overlay is the backdrop, clicking it should close
    const overlay = document.querySelector('body > div:last-child')
    if (overlay) {
      fireEvent.click(overlay)
    }
    expect(handleClose).toHaveBeenCalled()
  })
})