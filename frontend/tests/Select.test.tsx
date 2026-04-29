import { render, screen } from '@testing-library/react'
import { Select } from '../src/components/ui/Select'

describe('Select', () => {
  const options = [
    { value: 'dev', label: 'Development' },
    { value: 'test', label: 'Testing' },
    { value: 'prod', label: 'Production' },
  ]

  it('renders with default props', () => {
    render(<Select options={options} />)
    expect(screen.getByRole('combobox')).toBeInTheDocument()
  })

  it('renders with label', () => {
    render(<Select label="Environment" options={options} />)
    expect(screen.getByLabelText(/environment/i)).toBeInTheDocument()
  })

  it('renders all options', () => {
    render(<Select options={options} />)
    const select = screen.getByRole('combobox')
    expect(select).toHaveLength(3)
    expect(screen.getByText('Development')).toBeInTheDocument()
    expect(screen.getByText('Testing')).toBeInTheDocument()
    expect(screen.getByText('Production')).toBeInTheDocument()
  })

  it('displays error message', () => {
    render(<Select options={options} error="Please select an option" />)
    expect(screen.getByText(/please select an option/i)).toBeInTheDocument()
  })

  it('handles value changes', () => {
    render(<Select options={options} value="dev" onChange={() => {}} />)
    expect(screen.getByRole('combobox')).toHaveValue('dev')
  })
})