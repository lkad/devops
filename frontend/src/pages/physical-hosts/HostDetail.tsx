import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Server, AlertCircle, RefreshCw, Activity } from 'lucide-react'
import { PageContainer } from '@/components/layout'
import { Card } from '@/components/ui/Card'
import { Badge } from '@/components/ui/Badge'
import { devicesApi } from '@/api/endpoints/devices'
import { logsApi } from '@/api/endpoints/logs'

const statusVariant = (status: string): 'success' | 'warning' | 'error' | 'info' | 'default' => {
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

export function HostDetail() {
  const { id } = useParams<{ id: string }>()
  const [refreshKey, setRefreshKey] = useState(0)

  const { data: device, isLoading: deviceLoading } = useQuery({
    queryKey: ['devices', id],
    queryFn: () => devicesApi.get(id!),
    enabled: !!id,
  })

  // Fetch logs for this device
  const { data: logsData, isLoading: logsLoading } = useQuery({
    queryKey: ['device-logs', device?.name, refreshKey],
    queryFn: () => logsApi.query({
      source: 'physical_host',
      device: device?.name,
      limit: 20,
    }),
    enabled: !!device?.name,
  })

  const logs = logsData?.data ?? []

  if (deviceLoading) {
    return (
      <PageContainer title="Host Details" description={`Host ID: ${id}`}>
        <p className="text-text-secondary">Loading host details...</p>
      </PageContainer>
    )
  }

  if (!device) {
    return (
      <PageContainer title="Host Details" description={`Host ID: ${id}`}>
        <Card>
          <div className="flex flex-col items-center justify-center py-12 text-text-secondary">
            <AlertCircle className="w-12 h-12 mb-4 opacity-50" />
            <p>Host not found</p>
          </div>
        </Card>
      </PageContainer>
    )
  }

  return (
    <PageContainer
      title={device.name}
      description={`Type: ${device.type} | Environment: ${device.environment}`}
      actions={
        <div className="flex items-center gap-2">
          <button
            onClick={() => setRefreshKey(k => k + 1)}
            className="flex items-center gap-2 px-3 py-1.5 text-sm bg-surface border border-border rounded-md hover:bg-surface-elevated transition-colors"
          >
            <RefreshCw className="w-4 h-4" />
            Refresh
          </button>
        </div>
      }
    >
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left column - Host Info */}
        <div className="lg:col-span-2 space-y-6">
          {/* Host Info Card */}
          <Card>
            <div className="flex items-start justify-between mb-4">
              <div className="flex items-center gap-3">
                <div className="p-3 bg-surface rounded-lg">
                  <Server className="w-6 h-6 text-text-secondary" />
                </div>
                <div>
                  <h2 className="text-lg font-semibold text-text">{device.name}</h2>
                  <p className="text-sm text-text-secondary">{device.type}</p>
                </div>
              </div>
              <Badge variant={statusVariant(device.status)}>
                {device.status}
              </Badge>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              <div>
                <p className="text-xs text-text-muted uppercase tracking-wide">Environment</p>
                <p className="text-sm font-medium text-text mt-1">{device.environment || 'N/A'}</p>
              </div>
              <div>
                <p className="text-xs text-text-muted uppercase tracking-wide">Registered</p>
                <p className="text-sm font-medium text-text mt-1">{formatTime(device.registeredAt)}</p>
              </div>
              <div>
                <p className="text-xs text-text-muted uppercase tracking-wide">Location</p>
                <p className="text-sm font-medium text-text mt-1">{device.labels?.location || device.labels?.['location'] || 'N/A'}</p>
              </div>
              <div>
                <p className="text-xs text-text-muted uppercase tracking-wide">Device ID</p>
                <p className="text-sm font-medium text-text mt-1 font-mono">{device.id.slice(0, 8)}...</p>
              </div>
            </div>
          </Card>

          {/* Logs Card - Full width */}
          <Card>
            <div className="flex items-center gap-2 mb-4">
              <Activity className="w-5 h-5 text-text-secondary" />
              <h3 className="text-sm font-medium text-text">Recent Logs</h3>
              {logsLoading && <span className="text-xs text-text-muted">Loading...</span>}
            </div>

            {logs.length === 0 && !logsLoading ? (
              <div className="text-center py-8 text-text-secondary">
                <p>No logs available for this device</p>
                <p className="text-sm mt-1">Logs are collected from Loki when available</p>
              </div>
            ) : (
              <div className="space-y-2 max-h-96 overflow-y-auto">
                {logs.map((log) => (
                  <div
                    key={log.id}
                    className="flex items-start gap-3 p-2 rounded bg-surface/50 hover:bg-surface transition-colors"
                  >
                    <div className="flex-shrink-0">
                      <span className={`inline-block w-2 h-2 rounded-full ${
                        log.level === 'error' ? 'bg-red-500' :
                        log.level === 'warn' ? 'bg-yellow-500' :
                        'bg-green-500'
                      }`} />
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between gap-2">
                        <span className={`text-xs font-medium ${
                          log.level === 'error' ? 'text-red-400' :
                          log.level === 'warn' ? 'text-yellow-400' :
                          'text-green-400'
                        }`}>
                          {log.level.toUpperCase()}
                        </span>
                        <span className="text-xs text-text-muted">
                          {new Date(log.timestamp).toLocaleTimeString()}
                        </span>
                      </div>
                      <p className="text-sm text-text mt-0.5 break-all">{log.message}</p>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </Card>
        </div>

        {/* Right column - Quick Stats */}
        <div className="space-y-6">
          {/* Status Overview */}
          <Card>
            <h3 className="text-sm font-medium text-text mb-4">Status Overview</h3>
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-sm text-text-secondary">Status</span>
                <Badge variant={statusVariant(device.status)}>{device.status}</Badge>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-text-secondary">Type</span>
                <span className="text-sm text-text">{device.type}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-text-secondary">Environment</span>
                <span className="text-sm text-text">{device.environment || 'N/A'}</span>
              </div>
            </div>
          </Card>

          {/* Quick Links */}
          <Card>
            <h3 className="text-sm font-medium text-text mb-4">Quick Actions</h3>
            <div className="space-y-2">
              <button
                onClick={() => setRefreshKey(k => k + 1)}
                className="w-full flex items-center justify-center gap-2 px-3 py-2 text-sm bg-surface border border-border rounded-md hover:bg-surface-elevated transition-colors"
              >
                <RefreshCw className="w-4 h-4" />
                Refresh Logs
              </button>
            </div>
          </Card>
        </div>
      </div>
    </PageContainer>
  )
}