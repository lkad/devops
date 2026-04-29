import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@/api/client'
import { DataTable } from '@/components/ui/DataTable'
import { EmptyState } from '@/components/ui/EmptyState'
import styles from './AlertHistory.module.css'

interface AlertHistoryEntry {
  id: string
  channelId: string
  channelName: string
  alertName: string
  status: 'triggered' | 'resolved'
  triggeredAt: string
  resolvedAt?: string
  message: string
}

const formatTime = (timestamp: string): string => {
  return new Date(timestamp).toLocaleString()
}

export function AlertHistory() {
  const [channelFilter, setChannelFilter] = useState('')
  const [statusFilter, setStatusFilter] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['alert-history'],
    queryFn: () => apiClient.get<{ history: AlertHistoryEntry[] }>('/api/v1/alert-history'),
  })

  const history: AlertHistoryEntry[] = data?.history ?? []

  const filteredHistory = useMemo(() => {
    return history.filter(entry => {
      const matchesChannel = !channelFilter || entry.channelId === channelFilter
      const matchesStatus = !statusFilter || entry.status === statusFilter
      return matchesChannel && matchesStatus
    })
  }, [history, channelFilter, statusFilter])

  const columns = useMemo(() => [
    {
      id: 'triggeredAt',
      header: 'Time',
      accessor: (row: AlertHistoryEntry) => formatTime(row.triggeredAt),
    },
    {
      id: 'channelName',
      header: 'Channel',
      accessor: (row: AlertHistoryEntry) => row.channelName,
    },
    {
      id: 'alertName',
      header: 'Alert',
      accessor: (row: AlertHistoryEntry) => row.alertName,
    },
    {
      id: 'message',
      header: 'Message',
      accessor: (row: AlertHistoryEntry) => row.message,
    },
    {
      id: 'status',
      header: 'Status',
      accessor: (row: AlertHistoryEntry) => (
        <span className={`${styles.statusBadge} ${row.status === 'triggered' ? styles.statusTriggered : styles.statusResolved}`}>
          {row.status}
        </span>
      ),
    },
  ], [])

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Alert History</h1>
      </div>

      <div className={styles.filters}>
        <select
          value={channelFilter}
          onChange={(e) => setChannelFilter(e.target.value)}
          className={styles.filterSelect}
        >
          <option value="">All Channels</option>
        </select>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className={styles.filterSelect}
        >
          <option value="">All Statuses</option>
          <option value="triggered">Triggered</option>
          <option value="resolved">Resolved</option>
        </select>
      </div>

      {isLoading ? (
        <div className={styles.loading}>Loading history...</div>
      ) : filteredHistory.length === 0 ? (
        <EmptyState
          title="No alerts found"
          description="Alert history will appear here when alerts are triggered"
        />
      ) : (
        <div className={styles.tableContainer}>
          <DataTable
            data={filteredHistory}
            columns={columns}
            pageSize={10}
          />
        </div>
      )}
    </div>
  )
}