import { render, screen } from '@testing-library/react'
import { EmptyState } from '../src/components/ui/EmptyState'
import { Button } from '../src/components/ui/Button'

describe('EmptyState', () => {
  it('renders title', () => {
    render(<EmptyState title="No results found" />)
    expect(screen.getByText('No results found')).toBeInTheDocument()
  })

  it('renders description', () => {
    render(<EmptyState title="No data" description="There is no data to display" />)
    expect(screen.getByText('There is no data to display')).toBeInTheDocument()
  })

  it('renders icon', () => {
    render(<EmptyState title="Empty" icon={<span data-testid="icon" />} />)
    expect(screen.getByTestId('icon')).toBeInTheDocument()
  })

  it('renders action button', () => {
    render(
      <EmptyState
        title="No items"
        action={<Button>Add Item</Button>}
      />
    )
    expect(screen.getByRole('button', { name: /add item/i })).toBeInTheDocument()
  })
})