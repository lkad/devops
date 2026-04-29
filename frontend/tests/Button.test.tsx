import { render, screen, fireEvent } from '@testing-library/react'
import { Button } from '../src/components/ui/Button'

describe('Button', () => {
  it('renders with children', () => {
    render(<Button>Click me</Button>)
    expect(screen.getByRole('button', { name: /click me/i })).toBeInTheDocument()
  })

  it('calls onClick when clicked', () => {
    const handleClick = jest.fn()
    render(<Button onClick={handleClick}>Click me</Button>)
    fireEvent.click(screen.getByRole('button'))
    expect(handleClick).toHaveBeenCalledTimes(1)
  })

  it('does not call onClick when disabled', () => {
    const handleClick = jest.fn()
    render(<Button disabled onClick={handleClick}>Disabled</Button>)
    expect(screen.getByRole('button')).toBeDisabled()
    fireEvent.click(screen.getByRole('button'))
    expect(handleClick).not.toHaveBeenCalled()
  })

  it('disables button when loading', () => {
    render(<Button loading>Loading</Button>)
    const button = screen.getByRole('button')
    expect(button).toBeDisabled()
    expect(button.querySelector('span')).toBeInTheDocument() // spinner
  })

  it('renders different variants without error', () => {
    const variants = ['primary', 'secondary', 'danger', 'ghost', 'default'] as const
    variants.forEach((variant) => {
      render(<Button variant={variant}>{variant}</Button>)
      expect(screen.getByRole('button', { name: variant })).toBeInTheDocument()
    })
  })

  it('renders different sizes without error', () => {
    const sizes = ['sm', 'md', 'lg'] as const
    sizes.forEach((size) => {
      render(<Button size={size}>{size}</Button>)
      expect(screen.getByRole('button', { name: size })).toBeInTheDocument()
    })
  })

  it('renders leftIcon when provided', () => {
    render(
      <Button leftIcon={<span data-testid="left-icon" />}>With Icon</Button>
    )
    expect(screen.getByTestId('left-icon')).toBeInTheDocument()
  })
})