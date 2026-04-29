import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@/api/client'
import { DataTable } from '@/components/ui/DataTable'
import { EmptyState } from '@/components/ui/EmptyState'
import styles from './AuditLogs.module.css'

interface AuditLogEntry {
  id: string
  timestamp: string
  userId: string
  userName: string
  action: string
  entityType: string
  entityId: string
  details: string
}

const formatTime = (timestamp: string): string => {
  return new Date(timestamp).toLocaleString()
}

const getActionClass = (action: string): string => {
  switch (action.toLowerCase()) {
    case 'create':
      return styles.actionCreate
    case 'update':
      return styles.actionUpdate
    case 'delete':
      return styles.actionDelete
    default:
      return styles.actionOther
  }
}

export function AuditLogs() {
  const [entityFilter, setEntityFilter] = useState('')
  const [actionFilter, setActionFilter] = useState('')
  const [userFilter, setUserFilter] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['audit-logs'],
    queryFn: () => apiClient.get<{ logs: AuditLogEntry[] }>('/api/v1/audit-logs'),
  })

  const logs: AuditLogEntry[] = data?.logs ?? [
    { id: '1', timestamp: '2024-01-15T10:30:00Z', userId: 'u1', userName: 'admin', action: 'create', entityType: 'device', entityId: 'd1', details: 'Created device dev-server-01' },
    { id: '2', timestamp: '2024-01-15T11:00:00Z', userId: 'u1', userName: 'admin', action: 'update', entityType: 'pipeline', entityId: 'p1', details: 'Updated pipeline build-pipeline stages' },
    { id: '3', timestamp: '2024-01-15T12:30:00Z', userId: 'u2', userName: 'developer', action: 'delete', entityType: 'log', entityId: 'l1', details: 'Deleted old log entries' },
    { id: '4', timestamp: '2024-01-15T14:00:00Z', userId: 'u3', userName: 'operator', action: 'create', entityType: 'alert', entityId: 'a1', details: 'Created alert rule cpu-alert' },
  ]

  const filteredLogs = useMemo(() => {
    return logs.filter(log => {
      const matchesEntity = !entityFilter || log.entityType.toLowerCase().includes(entityFilter.toLowerCase())
      const matchesAction = !actionFilter || log.action.toLowerCase().includes(actionFilter.toLowerCase())
      const matchesUser = !userFilter || log.userName.toLowerCase().includes(userFilter.toLowerCase())
      return matchesEntity && matchesAction && matchesUser
    })
  }, [logs, entityFilter, actionFilter, userFilter])

  const columns = useMemo(() => [
    {
      id: 'timestamp',
      header: 'Time',
      accessor: (row: AuditLogEntry) => formatTime(row.timestamp),
    },
    {
      id: 'userName',
      header: 'User',
      accessor: (row: AuditLogEntry) => row.userName,
    },
    {
      id: 'action',
      header: 'Action',
      accessor: (row: AuditLogEntry) => (
        <span className={`${styles.actionBadge} ${getActionClass(row.action)}`}>
          {row.action}
        </span>
      ),
    },
    {
      id: 'entityType',
      header: 'Entity',
      accessor: (row: AuditLogEntry) => `${row.entityType}:${row.entityId.slice(0, 8)}`,
    },
    {
      id: 'details',
      header: 'Details',
      accessor: (row: AuditLogEntry) => row.details,
    },
  ], [])

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Audit Logs</h1>
      </div>

      <div className={styles.filters}>
        <input
          type="text"
          placeholder="Filter by user..."
          value={userFilter}
          onChange={(e) => setUserFilter(e.target.value)}
          className={styles.filterInput}
        />
        <select
          value={entityFilter}
          onChange={(e) => setEntityFilter(e.target.value)}
          className={styles.filterSelect}
        >
          <option value="">All Entities</option>
          <option value="device">Device</option>
          <option value="pipeline">Pipeline</option>
          <option value="alert">Alert</option>
          <option value="log">Log</option>
        </select>
        <select
          value={actionFilter}
          onChange={(e) => setActionFilter(e.target.value)}
          className={styles.filterSelect}
        >
          <option value="">All Actions</option>
          <option value="create">Create</option>
          <option value="update">Update</option>
          <option value="delete">Delete</option>
        </select>
      </div>

      {isLoading ? (
        <div className={styles.loading}>Loading audit logs...</div>
      ) : filteredLogs.length === 0 ? (
        <EmptyState
          title="No audit logs found"
          description="Audit logs will appear here when users perform actions"
        />
      ) : (
        <div className={styles.tableContainer}>
          <DataTable data={filteredLogs} columns={columns} pageSize={10} />
        </div>
      )}
    </div>
  )
}