import { render, screen } from '@testing-library/react'
import { LoadingSkeleton } from '../src/components/ui/LoadingSkeleton'

describe('LoadingSkeleton', () => {
  it('renders text variant by default', () => {
    render(<LoadingSkeleton />)
    expect(screen.getByTestId('skeleton')).toBeInTheDocument()
  })

  it('renders all variants', () => {
    const variants = ['text', 'title', 'avatar', 'button', 'card', 'row'] as const
    variants.forEach((variant) => {
      const { container } = render(<LoadingSkeleton variant={variant} />)
      expect(container.firstChild).toBeInTheDocument()
    })
  })

  it('applies custom width and height', () => {
    render(<LoadingSkeleton width={200} height={50} />)
    const skeleton = screen.getByTestId('skeleton')
    expect(skeleton).toHaveStyle({ width: '200px', height: '50px' })
  })

  it('applies custom className', () => {
    const { container } = render(<LoadingSkeleton className="custom-skeleton" />)
    expect(container.firstChild).toHaveClass('custom-skeleton')
  })
})