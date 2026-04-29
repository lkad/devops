import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Search, Plus } from 'lucide-react'
import { physicalHostsApi } from '@/api/endpoints/physicalHosts'
import { Button } from '@/components/ui/Button'
import { DataTable } from '@/components/ui/DataTable'
import { EmptyState } from '@/components/ui/EmptyState'
import styles from './PhysicalHostList.module.css'

const statusColors = {
  online: 'var(--color-online)',
  monitoring_issue: 'var(--color-monitoring-issue)',
  offline: 'var(--color-offline)',
}

const formatLastSeen = (lastSeen?: string): string => {
  if (!lastSeen) return 'Never'
  const date = new Date(lastSeen)
  return date.toLocaleString()
}

interface HostRow {
  id: string
  name: string
  status: 'online' | 'monitoring_issue' | 'offline'
  cpu: number
  memory: number
  disk: number
  services: number
  lastSeen: string
  ipAddress?: string
}

export function PhysicalHostList() {
  const navigate = useNavigate()
  const [searchQuery, setSearchQuery] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['physical-hosts'],
    queryFn: () => physicalHostsApi.list(),
  })

  const hosts: HostRow[] = data?.hosts ?? []

  const filteredHosts = useMemo(() => {
    if (!searchQuery) return hosts
    const query = searchQuery.toLowerCase()
    return hosts.filter(host =>
      host.name.toLowerCase().includes(query) ||
      (host.ipAddress && host.ipAddress.toLowerCase().includes(query))
    )
  }, [hosts, searchQuery])

  const columns = useMemo(() => [
    {
      id: 'name',
      header: 'Name',
      accessor: (row: HostRow) => row.name,
    },
    {
      id: 'status',
      header: 'Status',
      accessor: (row: HostRow) => (
        <span style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <span style={{
            width: 10,
            height: 10,
            borderRadius: '50%',
            backgroundColor: statusColors[row.status],
          }} />
          {row.status.replace('_', ' ')}
        </span>
      ),
    },
    {
      id: 'cpu',
      header: 'CPU',
      accessor: (row: HostRow) => `${row.cpu}%`,
    },
    {
      id: 'memory',
      header: 'Memory',
      accessor: (row: HostRow) => `${row.memory}%`,
    },
    {
      id: 'disk',
      header: 'Disk',
      accessor: (row: HostRow) => `${row.disk}%`,
    },
    {
      id: 'services',
      header: 'Services',
      accessor: (row: HostRow) => row.services,
    },
    {
      id: 'lastSeen',
      header: 'Last Seen',
      accessor: (row: HostRow) => formatLastSeen(row.lastSeen),
    },
  ], [])

  const handleRowClick = (row: HostRow) => {
    navigate(`/physical-hosts/${row.id}`)
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Physical Hosts</h1>
        <Button variant="primary" onClick={() => navigate('/physical-hosts/new')}>
          <Plus size={18} />
          Add Host
        </Button>
      </div>

      <div className={styles.filters}>
        <div className={styles.searchWrapper}>
          <Search size={18} className={styles.searchIcon} />
          <input
            type="text"
            placeholder="Search hosts..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className={styles.searchInput}
          />
        </div>
      </div>

      {isLoading ? (
        <div className={styles.loading}>Loading hosts...</div>
      ) : filteredHosts.length === 0 ? (
        <EmptyState
          title="No hosts found"
          description={searchQuery ? "Try adjusting your search" : "Get started by adding your first host"}
          action={{
            label: "Add Host",
            onClick: () => navigate('/physical-hosts/new')
          }}
        />
      ) : (
        <div className={styles.tableContainer}>
          <DataTable
            data={filteredHosts}
            columns={columns}
            onRowClick={handleRowClick}
            pageSize={10}
          />
        </div>
      )}
    </div>
  )
}