import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Search, Plus } from 'lucide-react'
import { devicesApi, type Device } from '@/api/endpoints/devices'
import { Button } from '@/components/ui/Button'
import { DataTable } from '@/components/ui/DataTable'
import { EmptyState } from '@/components/ui/EmptyState'
import { DeviceForm } from './DeviceForm'
import styles from './DeviceList.module.css'

// API returns snake_case - map to Device interface which expects camelCase
interface ApiDevice {
  id: string
  name: string
  type: string
  status: string
  environment: string
  labels: Record<string, string>
  registered_at?: string
  created_at?: string
  updated_at?: string
}

// Map API response to Device
const mapApiDevice = (apiDevice: ApiDevice): Device => ({
  id: apiDevice.id,
  name: apiDevice.name,
  type: apiDevice.type,
  status: apiDevice.status,
  environment: apiDevice.environment,
  labels: apiDevice.labels,
  registeredAt: apiDevice.registered_at,
  lastSeen: apiDevice.created_at, // Use created_at as proxy for lastSeen
})

export function DeviceList() {
  const navigate = useNavigate()
  const [searchQuery, setSearchQuery] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [showCreateModal, setShowCreateModal] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['devices'],
    queryFn: () => devicesApi.list(),
  })

  const devices: Device[] = useMemo(() => {
    return (data?.data ?? []).map(mapApiDevice)
  }, [data])

  const deviceTypes = useMemo(() => {
    const types = new Set(devices.map(d => d.type))
    return Array.from(types).sort()
  }, [devices])

  const filteredDevices = useMemo(() => {
    return devices.filter(device => {
      const matchesSearch = device.name.toLowerCase().includes(searchQuery.toLowerCase())
      const matchesType = !typeFilter || device.type === typeFilter
      return matchesSearch && matchesType
    })
  }, [devices, searchQuery, typeFilter])

  const handleRowClick = (row: Device) => {
    navigate(`/devices/${row.id}`)
  }

  const columns = useMemo(() => [
    { id: 'name', header: 'Name', accessorKey: 'name' },
    { id: 'type', header: 'Type', accessorKey: 'type' },
    { id: 'status', header: 'Status', accessorKey: 'status' },
    { id: 'registeredAt', header: 'Registered', accessorKey: 'registeredAt' },
    { id: 'environment', header: 'Environment', accessorKey: 'environment' },
  ], [])

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <h1 className={styles.title}>Devices</h1>
        <Button variant="primary" onClick={() => setShowCreateModal(true)}>
          <Plus size={18} />
          Add Device
        </Button>
      </div>

      <div className={styles.filters}>
        <div className={styles.searchWrapper}>
          <Search size={18} className={styles.searchIcon} />
          <input
            type="text"
            placeholder="Search by name..."
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
            onClick: () => setShowCreateModal(true)
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

      <DeviceForm
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
      />
    </div>
  )
}