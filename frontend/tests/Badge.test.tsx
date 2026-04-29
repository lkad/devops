import { render, screen } from '@testing-library/react'
import { Badge } from '../src/components/ui/Badge'

describe('Badge', () => {
  it('renders with children', () => {
    render(<Badge>Default</Badge>)
    expect(screen.getByText(/default/i)).toBeInTheDocument()
  })

  it('renders all variants without error', () => {
    const variants = ['neutral', 'success', 'warning', 'error', 'info'] as const
    variants.forEach((variant) => {
      const { container } = render(<Badge variant={variant}>{variant}</Badge>)
      expect(container.firstChild).toBeInTheDocument()
      expect(screen.getByText(variant)).toBeInTheDocument()
    })
  })

  it('applies custom className', () => {
    const { container } = render(<Badge className="custom-class">Test</Badge>)
    expect(container.firstChild).toHaveClass('custom-class')
  })

  it('renders with custom attributes', () => {
    render(<Badge id="test-badge">Content</Badge>)
    expect(screen.getByText('Content')).toBeInTheDocument()
  })
})