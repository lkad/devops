import { useState, useCallback, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Server } from 'lucide-react'
import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { devicesApi } from '@/api/endpoints/devices'
import styles from './HostList.module.css'

interface Column {
  key: string
  label: string
  width: number
  minWidth?: number
}

const defaultColumns: Column[] = [
  { key: 'host', label: 'Host', width: 280, minWidth: 180 },
  { key: 'status', label: 'Status', width: 100, minWidth: 80 },
  { key: 'location', label: 'Location', width: 160, minWidth: 100 },
  { key: 'environment', label: 'Environment', width: 120, minWidth: 100 },
  { key: 'registered', label: 'Registered', width: 160, minWidth: 120 },
]

const stateVariant = (status: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
  switch (status.toLowerCase()) {
    case 'active':
      return 'success'
    case 'pending':
      return 'warning'
    case 'inactive':
    case 'offline':
      return 'error'
    default:
      return 'default'
  }
}

const formatTime = (timestamp?: string) => {
  if (!timestamp) return 'Never'
  return new Date(timestamp).toLocaleString()
}

export function HostList() {
  const navigate = useNavigate()
  const [columns, setColumns] = useState<Column[]>(defaultColumns)
  const tableRef = useRef<HTMLTableElement>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['physical-hosts'],
    queryFn: () => devicesApi.list({ type: 'physical_host' }),
  })

  const hosts = data?.data ?? []

  // Handle column resize
  const handleResize = useCallback((index: number, deltaX: number) => {
    setColumns(prev => {
      const newColumns = [...prev]
      const col = newColumns[index]
      const newWidth = Math.max(col.minWidth || 80, col.width + deltaX)
      newColumns[index] = { ...col, width: newWidth }
      return newColumns
    })
  }, [])

  // Resize handler component
  const ResizeHandle = ({ index }: { index: number }) => {
    const startXRef = useRef(0)
    const isResizingRef = useRef(false)

    const onMouseDown = (e: React.MouseEvent) => {
      e.preventDefault()
      startXRef.current = e.clientX
      isResizingRef.current = true

      const onMouseMove = (e: MouseEvent) => {
        if (!isResizingRef.current) return
        const delta = e.clientX - startXRef.current
        startXRef.current = e.clientX
        handleResize(index, delta)
      }

      const onMouseUp = () => {
        isResizingRef.current = false
        document.removeEventListener('mousemove', onMouseMove)
        document.removeEventListener('mouseup', onMouseUp)
      }

      document.addEventListener('mousemove', onMouseMove)
      document.addEventListener('mouseup', onMouseUp)
    }

    return (
      <div
        className={styles.resizeHandle}
        onMouseDown={onMouseDown}
      />
    )
  }

  const renderCell = (host: typeof hosts[0], key: string) => {
    switch (key) {
      case 'host':
        return (
          <td key={key} className={styles.td} style={{ width: columns[0].width }}>
            <div className={styles.tdHost}>
              <div className={styles.iconWrapper}>
                <Server className={styles.icon} />
              </div>
              <span className={styles.hostName}>{host.name}</span>
              <span className={styles.hostId}>{host.id.slice(0, 12)}...</span>
            </div>
          </td>
        )
      case 'status':
        return (
          <td key={key} className={styles.td} style={{ width: columns[1].width }}>
            <Badge variant={stateVariant(host.status)}>{host.status}</Badge>
          </td>
        )
      case 'location':
        return (
          <td key={key} className={styles.td} style={{ width: columns[2].width }}>
            <span className="text-sm text-text-secondary">{host.labels?.location || 'N/A'}</span>
          </td>
        )
      case 'environment':
        return (
          <td key={key} className={styles.td} style={{ width: columns[3].width }}>
            <span
              className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
              style={{
                background: host.environment === 'prod' ? 'var(--color-error-muted)' :
                  host.environment === 'test' ? 'var(--color-warning-muted)' :
                    'var(--color-info-muted)',
                color: host.environment === 'prod' ? 'var(--color-error)' :
                  host.environment === 'test' ? 'var(--color-warning)' :
                    'var(--color-info)'
              }}
            >
              {host.environment || 'N/A'}
            </span>
          </td>
        )
      case 'registered':
        return (
          <td key={key} className={styles.td} style={{ width: columns[4].width }}>
            <span className="text-sm text-text-muted">{formatTime(host.registeredAt)}</span>
          </td>
        )
      default:
        return null
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-text">Physical Hosts</h1>
        <span className="text-sm text-text-muted">{hosts.length} hosts</span>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          {[1, 2, 3].map(i => (
            <div key={i} className={styles.skeleton} />
          ))}
        </div>
      ) : hosts.length === 0 ? (
        <Card>
          <div className="flex flex-col items-center justify-center py-16 text-text-secondary">
            <div className="p-4 bg-surface-elevated rounded-full mb-4">
              <Server className="w-8 h-8 text-text-muted" />
            </div>
            <p className="font-medium">No physical hosts found</p>
            <p className="text-sm mt-1 text-text-muted">Register hosts via containerlab or API</p>
          </div>
        </Card>
      ) : (
        <Card className="overflow-hidden" style={{ padding: 0 }}>
          <div className={styles.tableContainer} ref={tableRef}>
            <table className={styles.table}>
              <thead>
                <tr>
                  {columns.map((col, index) => (
                    <th
                      key={col.key}
                      className={styles.th}
                      style={{ width: col.width }}
                    >
                      {col.label}
                      <ResizeHandle index={index} />
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {hosts.map((host) => (
                  <tr
                    key={host.id}
                    className={styles.tr}
                    onClick={() => navigate(`/physical-hosts/${host.id}`)}
                  >
                    {columns.map(col => renderCell(host, col.key))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}
    </div>
  )
}