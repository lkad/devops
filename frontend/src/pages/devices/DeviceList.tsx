import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Search, Plus } from 'lucide-react'
import { devicesApi } from '@/api/endpoints/devices'
import { Button } from '@/components/ui/Button'
import { Badge } from '@/components/ui/Badge'
import { DataTable } from '@/components/ui/DataTable'
import { EmptyState } from '@/components/ui/EmptyState'
import styles from './DeviceList.module.css'

const statusVariant = (status: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
  switch (status.toLowerCase()) {
    case 'active':
      return 'success'
    case 'maintenance':
      return 'warning'
    case 'suspended':
      return 'warning'
    case 'retired':
      return 'error'
    default:
      return 'default'
  }
}

const formatLastSeen = (lastSeen?: string): string => {
  if (!lastSeen) return 'Never'
  const date = new Date(lastSeen)
  return date.toLocaleString()
}

interface DeviceRow {
  id: string
  name: string
  type: string
  status: string
  dataCenter?: string
  ipAddress?: string
  lastSeen?: string
}

export function DeviceList() {
  const navigate = useNavigate()
  const [searchQuery, setSearchQuery] = useState('')
  const [typeFilter, setTypeFilter] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['devices'],
    queryFn: () => devicesApi.list(),
  })

  const devices: DeviceRow[] = data?.devices ?? []

  const deviceTypes = useMemo(() => {
    const types = new Set(devices.map(d => d.type))
    return Array.from(types).sort()
  }, [devices])

  const filteredDevices = useMemo(() => {
    return devices.filter(device => {
      const matchesSearch = device.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        (device.ipAddress && device.ipAddress.toLowerCase().includes(searchQuery.toLowerCase()))
      const matchesType = !typeFilter || device.type === typeFilter
      return matchesSearch && matchesType
    })
  }, [devices, searchQuery, typeFilter])

  const columns = useMemo(() => [
    {
      id: 'name',
      header: 'Name',
      accessor: (row: DeviceRow) => (
        <span className={styles.monoCell}>{row.name}</span>
      ),
    },
    {
      id: 'type',
      header: 'Type',
      accessor: (row: DeviceRow) => row.type,
    },
    {
      id: 'status',
      header: 'Status',
      accessor: (row: DeviceRow) => (
        <Badge variant={statusVariant(row.status)}>
          {row.status}
        </Badge>
      ),
    },
    {
      id: 'dataCenter',
      header: 'Data Center',
      accessor: (row: DeviceRow) => row.dataCenter,
    },
    {
      id: 'ipAddress',
      header: 'IP Address',
      accessor: (row: DeviceRow) => (
        <span className={styles.monoCell}>{row.ipAddress}</span>
      ),
    },
    {
      id: 'lastSeen',
      header: 'Last Seen',
      accessor: (row: DeviceRow) => formatLastSeen(row.lastSeen),
    },
  ], [])

  const handleRowClick = (row: DeviceRow) => {
    navigate(`/devices/${row.id}`)
  }

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Devices</h1>
        <Button variant="primary" onClick={() => navigate('/devices/new')}>
          <Plus size={18} />
          Add Device
        </Button>
      </div>

      <div className={styles.filters}>
        <div className={styles.searchWrapper}>
          <Search size={18} className={styles.searchIcon} />
          <input
            type="text"
            placeholder="Search by name or IP..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className={styles.searchInput}
          />
        </div>
        <select
          value={typeFilter}
          onChange={(e) => setTypeFilter(e.target.value)}
          className={styles.filterSelect}
        >
          <option value="">All Types</option>
          {deviceTypes.map(type => (
            <option key={type} value={type}>{type}</option>
          ))}
        </select>
      </div>

      {isLoading ? (
        <div className={styles.loading}>Loading devices...</div>
      ) : filteredDevices.length === 0 ? (
        <EmptyState
          title="No devices found"
          description={searchQuery || typeFilter ? "Try adjusting your filters" : "Get started by adding your first device"}
          action={{
            label: "Add Device",
            onClick: () => navigate('/devices/new')
          }}
        />
      ) : (
        <div className={styles.tableContainer}>
          <DataTable
            data={filteredDevices}
            columns={columns}
            onRowClick={handleRowClick}
            pageSize={10}
          />
        </div>
      )}
    </div>
  )
}