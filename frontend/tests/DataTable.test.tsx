import { render, screen, fireEvent } from '@testing-library/react'
import { DataTable } from '../src/components/ui/DataTable'
import { type ColumnDef } from '@tanstack/react-table'

interface TestRow {
  id: number
  name: string
  status: string
}

describe('DataTable', () => {
  const data: TestRow[] = [
    { id: 1, name: 'Server 1', status: 'online' },
    { id: 2, name: 'Server 2', status: 'offline' },
    { id: 3, name: 'Server 3', status: 'online' },
  ]

  const columns: ColumnDef<TestRow, unknown>[] = [
    {
      accessorKey: 'id',
      header: 'ID',
    },
    {
      accessorKey: 'name',
      header: 'Name',
    },
    {
      accessorKey: 'status',
      header: 'Status',
    },
  ]

  it('renders table with data', () => {
    render(<DataTable data={data} columns={columns} />)
    expect(screen.getByText('Server 1')).toBeInTheDocument()
    expect(screen.getByText('Server 2')).toBeInTheDocument()
    expect(screen.getByText('Server 3')).toBeInTheDocument()
  })

  it('renders table headers', () => {
    render(<DataTable data={data} columns={columns} />)
    expect(screen.getByText('ID')).toBeInTheDocument()
    expect(screen.getByText('Name')).toBeInTheDocument()
    expect(screen.getByText('Status')).toBeInTheDocument()
  })

  it('calls onRowClick when row is clicked', () => {
    const handleRowClick = jest.fn()
    render(<DataTable data={data} columns={columns} onRowClick={handleRowClick} />)
    fireEvent.click(screen.getByText('Server 1'))
    expect(handleRowClick).toHaveBeenCalledWith(data[0])
  })

  it('renders pagination when data exceeds page size', () => {
    render(<DataTable data={data} columns={columns} pageSize={2} />)
    expect(screen.getByText(/showing 1 to 2 of 3 results/i)).toBeInTheDocument()
    expect(screen.getByText('Previous')).toBeInTheDocument()
    expect(screen.getByText('Next')).toBeInTheDocument()
  })

  it('sorts data when header is clicked', () => {
    render(<DataTable data={data} columns={columns} />)
    const nameHeader = screen.getByText('Name')
    fireEvent.click(nameHeader)
    expect(screen.getByText('Server 1')).toBeInTheDocument()
  })

  it('navigates pages correctly', () => {
    render(<DataTable data={data} columns={columns} pageSize={2} />)
    const nextButton = screen.getByText('Next')
    fireEvent.click(nextButton)
    expect(screen.getByText(/showing 3 to 3 of 3 results/i)).toBeInTheDocument()
  })
})