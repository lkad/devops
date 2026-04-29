import { render, screen } from '@testing-library/react'
import { StatsCard, StatsGrid } from '../src/components/ui/StatsCard'

describe('StatsCard', () => {
  it('renders label and value', () => {
    render(<StatsCard label="Total Servers" value="42" />)
    expect(screen.getByText(/total servers/i)).toBeInTheDocument()
    expect(screen.getByText('42')).toBeInTheDocument()
  })

  it('renders with icon', () => {
    render(<StatsCard label="Servers" value="10" icon={<span data-testid="icon" />} />)
    expect(screen.getByTestId('icon')).toBeInTheDocument()
  })

  it('renders positive trend', () => {
    render(<StatsCard label="Active" value="25" trend={{ value: 10 }} />)
    expect(screen.getByText(/10%/)).toBeInTheDocument()
  })

  it('renders negative trend', () => {
    render(<StatsCard label="Inactive" value="5" trend={{ value: -5 }} />)
    expect(screen.getByText(/5%/)).toBeInTheDocument()
  })

  it('renders trend with label', () => {
    render(<StatsCard label="Growth" value="100" trend={{ value: 15, label: 'vs last week' }} />)
    expect(screen.getByText(/15% vs last week/i)).toBeInTheDocument()
  })
})

describe('StatsGrid', () => {
  it('renders children', () => {
    render(
      <StatsGrid>
        <StatsCard label="Card 1" value="1" />
        <StatsCard label="Card 2" value="2" />
      </StatsGrid>
    )
    expect(screen.getByText('Card 1')).toBeInTheDocument()
    expect(screen.getByText('Card 2')).toBeInTheDocument()
  })
})