import { useState, useMemo } from 'react'
import {
  useReactTable,
  getCoreRowModel,
  getSortedRowModel,
  getPaginationRowModel,
  flexRender,
  type ColumnDef,
  type SortingState,
} from '@tanstack/react-table'
import styles from './DataTable.module.css'

export interface DataTableProps<T> {
  data: T[]
  columns: ColumnDef<T, unknown>[]
  onRowClick?: (row: T) => void
  pageSize?: number
  className?: string
}

function SortIcon({ direction }: { direction: 'asc' | 'desc' | false }) {
  if (!direction) {
    return (
      <span className={styles.sortIcon}>
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M4 5L6 3L8 5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
          <path d="M4 7L6 9L8 7" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
      </span>
    )
  }
  return (
    <span className={`${styles.sortIcon} ${styles.active}`}>
      {direction === 'asc' ? (
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M6 3V9M6 3L3 6M6 3L9 6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
      ) : (
        <svg width="12" height="12" viewBox="0 0 12 12" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M6 9V3M6 9L3 6M6 9L9 6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"/>
        </svg>
      )}
    </span>
  )
}

export function DataTable<T>({
  data,
  columns,
  onRowClick,
  pageSize = 10,
  className,
}: DataTableProps<T>) {
  const [sorting, setSorting] = useState<SortingState>([])

  const table = useReactTable({
    data,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: {
      pagination: { pageSize },
    },
  })

  const { pageIndex, pageSize: currentPageSize } = table.getState().pagination
  const totalRows = table.getFilteredRowModel().rows.length
  const startRow = pageIndex * currentPageSize + 1
  const endRow = Math.min((pageIndex + 1) * currentPageSize, totalRows)

  const pageNumbers = useMemo(() => {
    const totalPages = table.getPageCount()
    const pages: (number | 'ellipsis')[] = []
    if (totalPages <= 7) {
      for (let i = 0; i < totalPages; i++) pages.push(i)
    } else {
      pages.push(0)
      if (pageIndex > 2) pages.push('ellipsis')
      for (let i = Math.max(1, pageIndex - 1); i <= Math.min(pageIndex + 1, totalPages - 2); i++) {
        pages.push(i)
      }
      if (pageIndex < totalPages - 3) pages.push('ellipsis')
      pages.push(totalPages - 1)
    }
    return pages
  }, [pageIndex, table])

  return (
    <div className={className}>
      <div className={styles.tableContainer}>
        <table className={styles.table}>
          <thead>
            {table.getHeaderGroups().map((headerGroup) => (
              <tr key={headerGroup.id} className={styles.headerRow}>
                {headerGroup.headers.map((header) => {
                  const canSort = header.column.getCanSort()
                  const sorted = header.column.getIsSorted()
                  return (
                    <th
                      key={header.id}
                      className={`${styles.headerCell} ${sorted ? styles.sorted : ''}`}
                      onClick={canSort ? header.column.getToggleSortingHandler() : undefined}
                      style={{ cursor: canSort ? 'pointer' : 'default' }}
                    >
                      {flexRender(header.column.columnDef.header, header.getContext())}
                      {canSort && <SortIcon direction={sorted} />}
                    </th>
                  )
                })}
              </tr>
            ))}
          </thead>
          <tbody>
            {table.getRowModel().rows.map((row) => (
              <tr
                key={row.id}
                className={styles.row}
                onClick={() => onRowClick?.(row.original)}
              >
                {row.getVisibleCells().map((cell) => (
                  <td key={cell.id} className={styles.cell}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {table.getPageCount() > 1 && (
        <div className={styles.pagination}>
          <span className={styles.paginationInfo}>
            Showing {startRow} to {endRow} of {totalRows} results
          </span>
          <div className={styles.paginationControls}>
            <button
              className={styles.pageButton}
              onClick={() => table.previousPage()}
              disabled={!table.getCanPreviousPage()}
            >
              Previous
            </button>
            {pageNumbers.map((page, idx) =>
              page === 'ellipsis' ? (
                <span key={`ellipsis-${idx}`} className={styles.ellipsis}>...</span>
              ) : (
                <button
                  key={page}
                  className={`${styles.pageButton} ${pageIndex === page ? styles.active : ''}`}
                  onClick={() => table.setPageIndex(page)}
                >
                  {page + 1}
                </button>
              )
            )}
            <button
              className={styles.pageButton}
              onClick={() => table.nextPage()}
              disabled={!table.getCanNextPage()}
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  )
}