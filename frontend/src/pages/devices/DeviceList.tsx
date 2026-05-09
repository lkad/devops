import { useState, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { type ColumnDef } from '@tanstack/react-table'
import { Search, Plus, Server, Box, HardDrive, Network, Cloud, Cpu } from 'lucide-react'
import { devicesApi, type Device } from '@/api/endpoints/devices'
import { Button } from '@/components/ui/Button'
import { DataTable } from '@/components/ui/DataTable'
import { EmptyState } from '@/components/ui/EmptyState'
import { Badge } from '@/components/ui/Badge'
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
  lastSeen: apiDevice.created_at,
})

// Device type icons
const DeviceIcon = ({ type }: { type: string }) => {
  switch (type) {
    case 'k8s_cluster':
      return <Box className="w-5 h-5 text-[var(--color-info)]" />
    case 'physical_host':
      return <Server className="w-5 h-5 text-[var(--color-primary)]" />
    case 'vm':
      return <Cpu className="w-5 h-5 text-[var(--color-warning)]" />
    case 'container':
      return <Box className="w-5 h-5 text-[var(--color-success)]" />
    case 'network_device':
      return <Network className="w-5 h-5 text-[var(--color-info)]" />
    case 'cloud_instance':
      return <Cloud className="w-5 h-5 text-[var(--color-primary)]" />
    default:
      return <HardDrive className="w-5 h-5 text-text-muted" />
  }
}

// Device type display names
const getDeviceTypeName = (type: string) => {
  switch (type) {
    case 'k8s_cluster': return 'K8s Cluster'
    case 'physical_host': return 'Physical Host'
    case 'vm': return 'Virtual Machine'
    case 'container': return 'Container'
    case 'network_device': return 'Network Device'
    case 'load_balancer': return 'Load Balancer'
    case 'cloud_instance': return 'Cloud Instance'
    case 'iot_device': return 'IoT Device'
    default: return type
  }
}

// Status badge variant
const statusVariant = (status: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
  switch (status.toLowerCase()) {
    case 'active':
    case 'healthy':
    case 'running':
      return 'success'
    case 'pending':
    case 'unknown':
      return 'warning'
    case 'authenticated':
    case 'registered':
      return 'info'
    case 'inactive':
    case 'offline':
    case 'failed':
    case 'unhealthy':
    case 'stopped':
      return 'error'
    case 'maintenance':
      return 'warning'
    default:
      return 'default'
  }
}

// Environment colors
const getEnvStyle = (env: string) => {
  const envColors: Record<string, { bg: string, color: string }> = {
    prod: { bg: 'var(--color-error-muted)', color: 'var(--color-error)' },
    test: { bg: 'var(--color-warning-muted)', color: 'var(--color-warning)' },
    dev: { bg: 'var(--color-info-muted)', color: 'var(--color-info)' },
  }
  return envColors[env?.toLowerCase()] || { bg: 'var(--color-info-muted)', color: 'var(--color-info)' }
}

export function DeviceList() {
  const navigate = useNavigate()
  const [searchQuery, setSearchQuery] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [showCreateModal, setShowCreateModal] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['devices', typeFilter],
    queryFn: () => devicesApi.list(typeFilter ? { type: typeFilter } : undefined),
    enabled: true,
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

  const columns = useMemo((): ColumnDef<Device, unknown>[] => [
    {
      id: 'name',
      header: 'Name',
      cell: ({ row }) => (
        <div className="flex items-center gap-3">
          <div className="p-2 bg-surface-elevated rounded-lg">
            <DeviceIcon type={row.original.type} />
          </div>
          <div>
            <div className="font-medium text-text">{row.original.name}</div>
            <div className="text-xs text-text-muted font-mono">{row.original.id.slice(0, 12)}...</div>
          </div>
        </div>
      ),
    },
    {
      id: 'type',
      header: 'Type',
      cell: ({ row }) => (
        <span className="text-sm text-text-secondary">
          {getDeviceTypeName(row.original.type)}
        </span>
      ),
    },
    {
      id: 'status',
      header: 'Status',
      cell: ({ row }) => (
        <Badge variant={statusVariant(row.original.status)}>{row.original.status}</Badge>
      ),
    },
    {
      id: 'environment',
      header: 'Environment',
      cell: ({ row }) => {
        const envStyle = getEnvStyle(row.original.environment)
        return (
          <span
            className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
            style={{ background: envStyle.bg, color: envStyle.color }}
          >
            {row.original.environment}
          </span>
        )
      },
    },
    {
      id: 'registeredAt',
      header: 'Registered',
      cell: ({ row }) => (
        <span className="text-sm text-text-muted">
          {row.original.registeredAt ? new Date(row.original.registeredAt).toLocaleDateString() : 'Never'}
        </span>
      ),
    },
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
            <option key={type} value={type}>{getDeviceTypeName(type)}</option>
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
