import { render, screen } from '@testing-library/react'
import { Input } from '../src/components/ui/Input'

describe('Input', () => {
  it('renders input element', () => {
    render(<Input />)
    expect(screen.getByRole('textbox')).toBeInTheDocument()
  })

  it('renders with label', () => {
    render(<Input label="Email" />)
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument()
  })

  it('renders with placeholder', () => {
    render(<Input placeholder="Enter email" />)
    expect(screen.getByPlaceholderText(/enter email/i)).toBeInTheDocument()
  })

  it('handles disabled state', () => {
    render(<Input disabled />)
    expect(screen.getByRole('textbox')).toBeDisabled()
  })

  it('displays error message when error prop is provided', () => {
    render(<Input error="This field is required" />)
    expect(screen.getByText(/this field is required/i)).toBeInTheDocument()
  })

  it('handles value changes', () => {
    render(<Input value="test@example.com" onChange={() => {}} />)
    expect(screen.getByRole('textbox')).toHaveValue('test@example.com')
  })

  it('applies id when label is provided', () => {
    render(<Input label="Username" />)
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
  })
})